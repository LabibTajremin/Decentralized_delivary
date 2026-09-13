// Package migrations embeds the SQL that defines the schema and the demo data.
//
// Embedding rather than reading from disk means a deployed binary carries its
// own migrations: there is no "the image shipped without the sql directory"
// failure mode, and the migrations that run are exactly the ones that were
// built and tested together.
//
// Schema and demo data are embedded into two separate filesystems on purpose.
// A deploy that runs `migrate up` reads only `schema`, so demo data cannot ride
// along with a schema change even by accident.
package migrations

import (
	"embed"
	"io/fs"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/migrate"
)

//go:embed *.up.sql *.down.sql
var schema embed.FS

//go:embed seed/*.sql
var seed embed.FS

// SchemaFS exposes the migration files.
func SchemaFS() fs.FS { return schema }

// SeedFS exposes the demo-data files.
func SeedFS() fs.FS { return seed }

// Load parses the embedded migrations into version order.
func Load() ([]migrate.Migration, error) { return migrate.Load(schema, ".") }

// Seeds parses the embedded demo data into file order.
func Seeds() ([]migrate.Script, error) { return migrate.LoadScripts(seed, "seed") }
