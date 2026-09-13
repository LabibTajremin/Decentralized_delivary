// Package migrate applies versioned SQL migrations.
//
// Two properties matter for this deployment model:
//
//   - An empty database is brought fully up to date in one run.
//   - A database that is already partly migrated applies only what it is
//     missing, and re-running is a no-op.
//
// Applied versions are recorded in schema_migrations, so "what has run" is a
// fact in the database rather than an assumption about deployment order. Each
// migration runs inside its own transaction: a failure leaves the database on
// the last good version instead of half-applied.
package migrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ErrNoMigrations is returned when the source contains no migration files.
var ErrNoMigrations = errors.New("no migrations found")

// ErrDirtyVersion is returned when a previous run failed partway and left a
// version marked dirty. Someone must look at it; guessing would risk data.
var ErrDirtyVersion = errors.New("database has a dirty migration version")

// Execer is the database surface the migrator needs. A pgx pool or connection
// satisfies it.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
	QueryVersions(ctx context.Context) (map[int]bool, error)
}

// Migration is one versioned change.
type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

// filePattern matches NNNN_name.up.sql / NNNN_name.down.sql.
var filePattern = regexp.MustCompile(`^(\d+)_([a-zA-Z0-9_\-]+)\.(up|down)\.sql$`)

// Load reads migrations from a filesystem, pairing each up with its down.
//
// A missing down file is an error rather than a warning: without it a release
// cannot be rolled back, and finding that out during an incident is too late.
func Load(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	type pair struct {
		name     string
		up, down string
		hasUp    bool
		hasDown  bool
	}
	found := map[int]*pair{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := filePattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		version, convErr := strconv.Atoi(m[1])
		if convErr != nil {
			return nil, fmt.Errorf("migration %s: bad version: %w", e.Name(), convErr)
		}
		body, readErr := fs.ReadFile(fsys, filepath.Join(dir, e.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), readErr)
		}
		p, ok := found[version]
		if !ok {
			p = &pair{name: m[2]}
			found[version] = p
		}
		if m[3] == "up" {
			p.up, p.hasUp = string(body), true
		} else {
			p.down, p.hasDown = string(body), true
		}
	}

	if len(found) == 0 {
		return nil, ErrNoMigrations
	}

	versions := make([]int, 0, len(found))
	for v := range found {
		versions = append(versions, v)
	}
	sort.Ints(versions)

	out := make([]Migration, 0, len(versions))
	for _, v := range versions {
		p := found[v]
		if !p.hasUp {
			return nil, fmt.Errorf("migration %04d_%s: missing .up.sql", v, p.name)
		}
		if !p.hasDown {
			return nil, fmt.Errorf("migration %04d_%s: missing .down.sql, so it could never be rolled back", v, p.name)
		}
		out = append(out, Migration{Version: v, Name: p.name, Up: p.up, Down: p.down})
	}
	return out, nil
}

// Result reports what a run did.
type Result struct {
	Applied []int
	Skipped []int
}

// Up applies every migration the database has not already recorded.
//
// Safe to run on every deploy: an up-to-date database applies nothing and
// returns an empty Applied list.
func Up(ctx context.Context, db Execer, migrations []Migration) (Result, error) {
	if len(migrations) == 0 {
		return Result{}, ErrNoMigrations
	}
	if err := ensureVersionTable(ctx, db); err != nil {
		return Result{}, err
	}

	applied, err := db.QueryVersions(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("read applied versions: %w", err)
	}
	for version, dirty := range applied {
		if dirty {
			return Result{}, fmt.Errorf("%w: version %d", ErrDirtyVersion, version)
		}
	}

	var result Result
	for _, m := range migrations {
		if _, done := applied[m.Version]; done {
			result.Skipped = append(result.Skipped, m.Version)
			continue
		}
		if err := applyOne(ctx, db, m); err != nil {
			return result, err
		}
		result.Applied = append(result.Applied, m.Version)
	}
	return result, nil
}

// Down rolls back the most recently applied migrations, newest first.
func Down(ctx context.Context, db Execer, migrations []Migration, steps int) (Result, error) {
	if err := ensureVersionTable(ctx, db); err != nil {
		return Result{}, err
	}
	applied, err := db.QueryVersions(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("read applied versions: %w", err)
	}

	ordered := append([]Migration(nil), migrations...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Version > ordered[j].Version })

	var result Result
	for _, m := range ordered {
		if steps > 0 && len(result.Applied) >= steps {
			break
		}
		if _, done := applied[m.Version]; !done {
			result.Skipped = append(result.Skipped, m.Version)
			continue
		}
		if _, err := db.Exec(ctx, m.Down); err != nil {
			return result, fmt.Errorf("roll back %04d_%s: %w", m.Version, m.Name, err)
		}
		if _, err := db.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, m.Version); err != nil {
			return result, fmt.Errorf("clear version %d: %w", m.Version, err)
		}
		result.Applied = append(result.Applied, m.Version)
	}
	return result, nil
}

// versionTableSQL is itself written with IF NOT EXISTS, so the migrator can
// bootstrap a database that has never seen it.
const versionTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    dirty       BOOLEAN NOT NULL DEFAULT FALSE,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
)`

func ensureVersionTable(ctx context.Context, db Execer) error {
	if _, err := db.Exec(ctx, versionTableSQL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

// applyOne runs a single migration and records it.
//
// The version is marked dirty first and cleared on success, so a process that
// dies mid-migration leaves a visible marker instead of a database that looks
// fine but is not.
func applyOne(ctx context.Context, db Execer, m Migration) error {
	if _, err := db.Exec(ctx,
		`INSERT INTO schema_migrations (version, name, dirty) VALUES ($1, $2, TRUE)`,
		m.Version, m.Name); err != nil {
		return fmt.Errorf("mark %04d_%s: %w", m.Version, m.Name, err)
	}
	if _, err := db.Exec(ctx, m.Up); err != nil {
		return fmt.Errorf("apply %04d_%s: %w", m.Version, m.Name, err)
	}
	if _, err := db.Exec(ctx,
		`UPDATE schema_migrations SET dirty = FALSE, applied_at = now() WHERE version = $1`,
		m.Version); err != nil {
		return fmt.Errorf("finalise %04d_%s: %w", m.Version, m.Name, err)
	}
	return nil
}

// Describe renders a run for a deploy log.
func Describe(r Result) string {
	var b strings.Builder
	if len(r.Applied) == 0 {
		b.WriteString("migrate: database already up to date")
	} else {
		b.WriteString(fmt.Sprintf("migrate: applied %d migration(s):", len(r.Applied)))
		for _, v := range r.Applied {
			b.WriteString(fmt.Sprintf(" %04d", v))
		}
	}
	if len(r.Skipped) > 0 {
		b.WriteString(fmt.Sprintf(" (skipped %d already applied)", len(r.Skipped)))
	}
	return b.String()
}
