package e2e

import (
	"testing"

	"github.com/rootlogic-lab/delivery/backend/tests/dbtest"
)

// dbtestSchemaURL returns the E2E suite's own schema URL.
//
// Package test binaries run concurrently and this suite rebuilds the schema for
// every test, so sharing one with the integration suite would mean each
// destroys the other's tables.
func dbtestSchemaURL(t *testing.T) string {
	t.Helper()
	return dbtest.MustSchemaURL("e2e")
}
