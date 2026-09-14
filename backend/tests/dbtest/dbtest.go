// Package dbtest gives each test suite its own isolated schema.
//
// Go runs the test binaries for different packages concurrently, so the
// integration and E2E suites reach the database at the same time. Sharing one
// schema means one suite drops the tables the other has just created — which
// looks exactly like a flake and is not one.
//
// Each suite gets a Postgres schema of its own instead. PostGIS itself stays in
// public and is reached through the search path, so no suite needs superuser
// rights to install an extension, and migrations keep using unqualified table
// names.
package dbtest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ErrNoDatabaseURL says the environment is not configured for database tests.
//
// These suites fail rather than skip when it is missing: a skipped integration
// test is indistinguishable from a passing one in CI output, and the whole
// point of the suite is that the SQL is exercised.
const ErrNoDatabaseURL = "DATABASE_URL is not set. These tests need a real PostGIS database;\n" +
	"run ./scripts/dev-postgres.sh, or `docker compose up -d postgres`, and export DATABASE_URL."

// SchemaURL returns a connection string scoped to its own schema, creating the
// schema if it does not exist.
//
// The returned URL sets search_path to "<schema>,public": new tables land in the
// suite's schema, while PostGIS functions and types resolve from public.
func SchemaURL(baseURL, schema string) (string, error) {
	if !validSchema(schema) {
		return "", fmt.Errorf("dbtest: %q is not a usable schema name", schema)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, baseURL)
	if err != nil {
		return "", fmt.Errorf("dbtest: connect: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	// The name is validated above rather than parameterised, because an
	// identifier cannot be a bind parameter in DDL.
	if _, err := conn.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS `+schema); err != nil {
		return "", fmt.Errorf("dbtest: create schema %s: %w", schema, err)
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("dbtest: parse DATABASE_URL: %w", err)
	}
	q := parsed.Query()
	q.Set("search_path", schema+",public")
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

// MustSchemaURL is SchemaURL for a TestMain, which has no *testing.T to fail.
func MustSchemaURL(schema string) string {
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		fmt.Fprintln(os.Stderr, ErrNoDatabaseURL)
		os.Exit(1)
	}
	scoped, err := SchemaURL(base, schema)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return scoped
}

// validSchema allows only plain lower-case identifiers, which is every name
// these suites use and nothing that could carry SQL.
func validSchema(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return !strings.HasPrefix(name, "pg_")
}
