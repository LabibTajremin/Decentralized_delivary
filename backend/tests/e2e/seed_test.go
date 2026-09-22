package e2e

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/migrate"
	"github.com/rootlogic-lab/delivery/backend/migrations"
)

// seedDemoData brings the database to a known state using the same migrator and
// the same demo data a developer gets from `migrate up && migrate seed`.
//
// Going through the real code rather than inlining SQL means these tests fail
// if the seed drifts from what the product actually ships.
//
// Unlike the integration suite, E2E tests drive a real server that commits: an
// override written by one test outlives it and outlives the whole run. So the
// schema is torn down and rebuilt for each test rather than merely migrated up.
// Without that, a test asserting "the division value applies" passes on a fresh
// database and fails on the second run, when a previous test's area override is
// still there — which reads as a flake and is not one.
func seedDemoData(t *testing.T, url string) {
	t.Helper()
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	db := migrate.NewPgxExecer(conn)
	loaded, err := migrations.Load()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}

	// Roll back what the migrator knows about, then tear down unconditionally:
	// a schema built by an older harness has nothing recorded for `down` to
	// undo, and would survive it intact.
	if _, err := migrate.Down(ctx, db, loaded, 0); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
	for i := len(loaded) - 1; i >= 0; i-- {
		if _, err := db.Exec(ctx, loaded[i].Down); err != nil {
			t.Fatalf("tear down %04d_%s: %v", loaded[i].Version, loaded[i].Name, err)
		}
	}
	if _, err := db.Exec(ctx, `DELETE FROM schema_migrations`); err != nil {
		t.Fatalf("clear schema_migrations: %v", err)
	}

	if _, err := migrate.Up(ctx, db, loaded); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	scripts, err := migrations.Seeds()
	if err != nil {
		t.Fatalf("load seeds: %v", err)
	}
	// production=false: seeding is refused outright in production, which is
	// asserted separately in the unit tests.
	if _, err := migrate.Seed(ctx, db, false, scripts); err != nil {
		t.Fatalf("seed: %v", err)
	}
}
