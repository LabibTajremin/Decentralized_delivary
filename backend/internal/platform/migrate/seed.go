package migrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// ErrSeedInProduction is returned when seeding is attempted against a
// production deployment.
//
// Demo data in a production database is not a tidy-up job: fake merchants
// become visible to real customers, and fake locations feed the radius and
// auto-tuning algorithms. The guard is in code rather than in an operator's
// memory because the cost of getting it wrong is not recoverable by deleting
// rows afterwards.
var ErrSeedInProduction = errors.New("refusing to seed a production database")

// Script is one demo-data file.
type Script struct {
	Name string
	SQL  string
}

// LoadScripts reads every .sql file in a directory, in filename order.
//
// Order is the numeric prefix, and it matters: a later script inserts rows that
// reference rows an earlier one created.
func LoadScripts(fsys fs.FS, dir string) ([]Script, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read seed dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	scripts := make([]Script, 0, len(names))
	for _, name := range names {
		full := path.Join(dir, name)
		body, readErr := fs.ReadFile(fsys, full)
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", full, readErr)
		}
		scripts = append(scripts, Script{Name: full, SQL: string(body)})
	}
	return scripts, nil
}

// Seed applies demo-data scripts in order and returns the names it ran.
//
// Scripts are expected to be upserts, so seeding twice updates rows rather
// than duplicating them. Nothing is recorded in schema_migrations: demo data is
// not schema, and a seeded database must still look un-seeded to the migrator.
func Seed(ctx context.Context, db Execer, production bool, scripts []Script) ([]string, error) {
	if production {
		return nil, ErrSeedInProduction
	}
	if len(scripts) == 0 {
		return nil, errors.New("no seed scripts found")
	}
	ran := make([]string, 0, len(scripts))
	for _, s := range scripts {
		if _, err := db.Exec(ctx, s.SQL); err != nil {
			return ran, fmt.Errorf("seed %s: %w", s.Name, err)
		}
		ran = append(ran, s.Name)
	}
	return ran, nil
}

// VersionStatus reports one migration's state in the database.
type VersionStatus struct {
	Version int
	Name    string
	Applied bool
	Dirty   bool
}

// Status reports which migrations are applied, pending or dirty.
func Status(ctx context.Context, db Execer, migrations []Migration) ([]VersionStatus, error) {
	if err := ensureVersionTable(ctx, db); err != nil {
		return nil, err
	}
	applied, err := db.QueryVersions(ctx)
	if err != nil {
		return nil, fmt.Errorf("read applied versions: %w", err)
	}
	out := make([]VersionStatus, 0, len(migrations))
	for _, m := range migrations {
		dirty, ok := applied[m.Version]
		out = append(out, VersionStatus{
			Version: m.Version,
			Name:    m.Name,
			Applied: ok,
			Dirty:   dirty,
		})
	}
	return out, nil
}

// DescribeStatus renders a status report one line per migration.
func DescribeStatus(statuses []VersionStatus) string {
	var b strings.Builder
	for i, s := range statuses {
		if i > 0 {
			b.WriteString("\n")
		}
		state := "pending"
		if s.Applied {
			state = "applied"
		}
		if s.Dirty {
			state = "DIRTY"
		}
		b.WriteString(fmt.Sprintf("%04d  %-10s  %s", s.Version, state, s.Name))
	}
	if len(statuses) == 0 {
		b.WriteString("no migrations")
	}
	return b.String()
}
