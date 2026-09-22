// Package load is the scale half of the test suite: the two algorithms whose
// cost grows with the size of the country, measured against a database that
// actually holds one.
//
// Every other suite proves behaviour. These prove that the behaviour survives
// volume — and they prove it where volume is decided, which is the query
// planner. The integration suite already asserts that ALG-01 can use the GiST
// index, but it has to set `enable_seqscan = off` to make the planner prefer
// one over a table of eleven rows. That assertion is worth having and is not
// the same assertion as this one: here the table holds thousands of shops,
// nothing is forced, and the planner's own choice is what is checked. A
// planner that stops choosing the index at scale is exactly the regression
// that the small test cannot see and that takes the product down.
//
// The scale is deliberately modest by default — a gate that takes four minutes
// is a gate somebody eventually comments out. `LOAD_SCALE` raises it for a
// real soak run; the assertions are written so that a larger number only makes
// them stricter.
package load

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/migrate"
	"github.com/rootlogic-lab/delivery/backend/migrations"
	"github.com/rootlogic-lab/delivery/backend/tests/dbtest"
)

// suiteURL is this package's own schema. Go runs package test binaries
// concurrently and this one rewrites whole tables, so sharing a schema with
// the integration or E2E suites would destroy both.
var suiteURL string

// pool is shared by every test here. A load test that opened a connection per
// query would be measuring the connection.
var pool *pgxpool.Pool

// defaultScale is how many rows each fixture writes unless LOAD_SCALE says
// otherwise. Two thousand is past the point where the planner stops treating
// the table as trivially small, which is what these tests are here to check,
// and still writes in well under a second.
const defaultScale = 2000

// scale is the fixture size for this run.
func scale() int {
	if raw := os.Getenv("LOAD_SCALE"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultScale
}

func TestMain(m *testing.M) {
	suiteURL = dbtest.MustSchemaURL("load")
	if err := prepare(suiteURL); err != nil {
		fmt.Fprintf(os.Stderr, "load: %v\n", err)
		os.Exit(1)
	}

	var err error
	pool, err = pgxpool.New(context.Background(), suiteURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load: pool: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

// prepare rolls the schema back and forward and seeds the geography, the same
// way the integration suite does — the divisions and areas these fixtures hang
// merchants and riders off are the real seeded ones, not squares invented here.
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
	if _, err := migrate.Down(ctx, db, loaded, 0); err != nil {
		return fmt.Errorf("roll back: %w", err)
	}
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

	scripts, err := migrations.Seeds()
	if err != nil {
		return err
	}
	if _, err := migrate.Seed(ctx, db, false, scripts); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	return nil
}
