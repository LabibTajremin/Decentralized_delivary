package integration

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/migrate"
	"github.com/rootlogic-lab/delivery/backend/migrations"
	"github.com/rootlogic-lab/delivery/backend/tests/dbtest"
)

// suiteURL is the scoped connection string every test in this package uses.
// Reading DATABASE_URL directly would bypass the schema isolation and put these
// tests back in the E2E suite's way.
var suiteURL string

// TestMain brings the database to a known state before any integration test
// runs, using the same migrator the deployed binary uses.
//
// It deliberately does not hand-roll its own schema setup. A test harness that
// applies SQL its own way proves the SQL is valid but proves nothing about the
// tool that will actually apply it in production; running the real migrator
// here means every integration run is also a test of `migrate up`.
func TestMain(m *testing.M) {
	// Its own schema: Go runs package test binaries concurrently, and this
	// suite tears the schema down on every run. Sharing one with the E2E suite
	// means each destroys the other's tables.
	suiteURL = dbtest.MustSchemaURL("integration")
	if err := prepare(suiteURL); err != nil {
		fmt.Fprintf(os.Stderr, "integration: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// prepare rolls the schema all the way back and then forward.
//
// Down-then-up rather than up alone: it proves the down migrations work on
// every run (a rollback that has never been executed is a rollback that does
// not work), and it guarantees the tests below start from an empty schema
// whatever the previous run left behind.
func prepare(url string) error {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	db := migrate.NewPgxExecer(conn)
	loaded, err := migrations.Load()
	if err != nil {
		return err
	}

	// steps = 0 rolls back everything the migrator has recorded, which is the
	// path a real `migrate down` takes.
	if _, err := migrate.Down(ctx, db, loaded, 0); err != nil {
		return fmt.Errorf("roll back: %w", err)
	}

	// Then tear down unconditionally. The step above only rolls back what
	// schema_migrations records, so a database built by hand — or by an older
	// harness that applied SQL directly — survives it with its tables and rows
	// intact, and the tests below would then run against somebody else's data.
	// The down scripts are all IF EXISTS, so this is a no-op on a clean
	// database and a guarantee on a dirty one.
	for i := len(loaded) - 1; i >= 0; i-- {
		if _, err := db.Exec(ctx, loaded[i].Down); err != nil {
			return fmt.Errorf("tear down %04d_%s: %w", loaded[i].Version, loaded[i].Name, err)
		}
	}
	if _, err := db.Exec(ctx, `DELETE FROM schema_migrations`); err != nil {
		return fmt.Errorf("clear schema_migrations: %w", err)
	}
	if _, err := migrate.Up(ctx, db, loaded); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
