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
