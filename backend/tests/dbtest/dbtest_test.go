package dbtest_test

import (
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/tests/dbtest"
)

// The schema name goes into DDL unparameterised, because an identifier cannot
// be a bind parameter. That makes the validator a security boundary, not a
// convenience, so it is tested directly.
func TestSchemaURLRejectsAnythingThatCouldCarrySQL(t *testing.T) {
	for _, name := range []string{
		"",
		"public; DROP SCHEMA public CASCADE",
		`e2e"`,
		"e2e-suite",
		"E2E",
		"1st",
		"pg_catalog",
		strings.Repeat("a", 64),
	} {
		if _, err := dbtest.SchemaURL("postgres://ignored/db", name); err == nil {
			t.Errorf("SchemaURL accepted %q", name)
		}
	}
}
