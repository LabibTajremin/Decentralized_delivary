package platform

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/migrate"
	"github.com/rootlogic-lab/delivery/backend/migrations"
)

func demoScripts() []migrate.Script {
	return []migrate.Script{
		{Name: "seed/0001_geo.sql", SQL: "INSERT INTO geo_divisions VALUES ('DHA');"},
		{Name: "seed/0002_more.sql", SQL: "INSERT INTO geo_areas VALUES ('DHK-DHM');"},
	}
}

// TestSeedRefusesProduction is the guard that matters most here. Demo merchants
// in a production database are visible to real customers and feed the radius
// algorithms; deleting the rows afterwards does not undo that.
func TestSeedRefusesProduction(t *testing.T) {
	db := &fakeDB{}
	_, err := migrate.Seed(ctx(), db, true, demoScripts())
	if !errors.Is(err, migrate.ErrSeedInProduction) {
		t.Fatalf("error = %v, want ErrSeedInProduction", err)
	}
	if len(db.execs) != 0 {
		t.Errorf("nothing may run against a production database, ran %v", db.execs)
	}
}

func TestSeedRunsEveryScriptInOrder(t *testing.T) {
	db := &fakeDB{}
	ran, err := migrate.Seed(ctx(), db, false, demoScripts())
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if len(ran) != 2 || ran[0] != "seed/0001_geo.sql" || ran[1] != "seed/0002_more.sql" {
		t.Errorf("ran = %v, want both scripts in order", ran)
	}
	if len(db.execs) != 2 || !strings.Contains(db.execs[0], "geo_divisions") {
		t.Errorf("statements = %v", db.execs)
	}
}

// TestSeedRecordsNothingInSchemaMigrations: demo data is not schema. A seeded
// database must still look un-seeded to the migrator, or `up` would start
// skipping real migrations.
func TestSeedRecordsNothingInSchemaMigrations(t *testing.T) {
	db := &fakeDB{}
	if _, err := migrate.Seed(ctx(), db, false, demoScripts()); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if db.ranStatement("schema_migrations") {
		t.Errorf("seeding must not touch schema_migrations, ran %v", db.execs)
	}
}

func TestSeedRefusesAnEmptySet(t *testing.T) {
	if _, err := migrate.Seed(ctx(), &fakeDB{}, false, nil); err == nil {
		t.Error("Seed must report an empty script set rather than claim success")
	}
}

func TestSeedStopsAtTheFirstFailure(t *testing.T) {
	db := &fakeDB{failOn: failWhen("geo_areas")}
	ran, err := migrate.Seed(ctx(), db, false, demoScripts())
	if !errors.Is(err, errDB) || !strings.Contains(err.Error(), "0002_more") {
		t.Errorf("error = %v, want the failing script named", err)
	}
	if len(ran) != 1 {
		t.Errorf("ran = %v, want only the script that succeeded", ran)
	}
}

func TestStatusSeparatesAppliedFromPending(t *testing.T) {
	db := &fakeDB{versions: map[int]bool{1: false}}
	got, err := migrate.Status(ctx(), db, mustLoad(t))
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d statuses, want 2", len(got))
	}
	if !got[0].Applied || got[0].Dirty {
		t.Errorf("0001 = %+v, want applied and clean", got[0])
	}
	if got[1].Applied {
		t.Errorf("0002 = %+v, want pending", got[1])
	}
}

func TestStatusReportsADirtyVersion(t *testing.T) {
	db := &fakeDB{versions: map[int]bool{1: true}}
	got, err := migrate.Status(ctx(), db, mustLoad(t))
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !got[0].Dirty {
		t.Errorf("0001 = %+v, want dirty", got[0])
	}
	if !strings.Contains(migrate.DescribeStatus(got), "DIRTY") {
		t.Errorf("a dirty version must be visible in the report: %q", migrate.DescribeStatus(got))
	}
}

func TestStatusSurfacesAVersionTableFailure(t *testing.T) {
	db := &fakeDB{failOn: failWhen("CREATE TABLE IF NOT EXISTS schema_migrations")}
	if _, err := migrate.Status(ctx(), db, mustLoad(t)); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestStatusSurfacesAVersionReadFailure(t *testing.T) {
	db := &fakeDB{versErr: errDB}
	if _, err := migrate.Status(ctx(), db, mustLoad(t)); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestDescribeStatusReportsAnEmptySet(t *testing.T) {
	if got := migrate.DescribeStatus(nil); !strings.Contains(got, "no migrations") {
		t.Errorf("DescribeStatus = %q", got)
	}
}

func TestDescribeStatusNamesEveryMigrationOnItsOwnLine(t *testing.T) {
	got := migrate.DescribeStatus([]migrate.VersionStatus{
		{Version: 1, Name: "geo", Applied: true},
		{Version: 2, Name: "orders"},
	})
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "0001") || !strings.Contains(lines[0], "applied") || !strings.Contains(lines[0], "geo") {
		t.Errorf("line = %q", lines[0])
	}
	if !strings.Contains(lines[1], "pending") {
		t.Errorf("line = %q", lines[1])
	}
}

// The embedded SQL is what actually ships. A build that embeds nothing still
// compiles, so these assert the files are really in the binary.

func TestEmbeddedMigrationsLoad(t *testing.T) {
	loaded, err := migrations.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) == 0 {
		t.Fatal("the binary embeds no migrations")
	}
	for _, m := range loaded {
		if strings.TrimSpace(m.Up) == "" {
			t.Errorf("migration %04d has an empty up", m.Version)
		}
		if strings.TrimSpace(m.Down) == "" {
			t.Errorf("migration %04d has an empty down", m.Version)
		}
	}
}

func TestEmbeddedSeedScriptsAreReadable(t *testing.T) {
	scripts, err := migrations.Seeds()
	if err != nil {
		t.Fatalf("Seeds: %v", err)
	}
	if len(scripts) == 0 {
		t.Fatal("the binary embeds no demo data")
	}
	for _, s := range scripts {
		if strings.TrimSpace(s.SQL) == "" {
			t.Errorf("seed %s is empty", s.Name)
		}
	}
}

func TestLoadScriptsReadsSqlInFilenameOrder(t *testing.T) {
	fsys := fstest.MapFS{
		"seed/0002_merchants.sql": {Data: []byte("INSERT INTO m;")},
		"seed/0001_geo.sql":       {Data: []byte("INSERT INTO g;")},
		"seed/README.md":          {Data: []byte("not sql")},
		"seed/nested/x.sql":       {Data: []byte("a directory entry")},
	}
	got, err := migrate.LoadScripts(fsys, "seed")
	if err != nil {
		t.Fatalf("LoadScripts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("loaded %d scripts, want 2: %+v", len(got), got)
	}
	if got[0].Name != "seed/0001_geo.sql" || got[1].Name != "seed/0002_merchants.sql" {
		t.Errorf("order = %s, %s", got[0].Name, got[1].Name)
	}
	if got[0].SQL != "INSERT INTO g;" {
		t.Errorf("body = %q", got[0].SQL)
	}
}

func TestLoadScriptsReportsAMissingDirectory(t *testing.T) {
	if _, err := migrate.LoadScripts(fstest.MapFS{}, "nowhere"); err == nil {
		t.Error("LoadScripts must fail when the directory does not exist")
	}
}

func TestLoadScriptsSurfacesAnUnreadableFile(t *testing.T) {
	fsys := unreadableFS{inner: fstest.MapFS{"seed/0001_geo.sql": {Data: []byte("INSERT INTO g;")}}}
	_, err := migrate.LoadScripts(fsys, "seed")
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("error = %v, want the read failure wrapped", err)
	}
}

// TestSeedIsNotEmbeddedAsAMigration is the structural half of the production
// guard: even a deploy that only ever runs `up` cannot apply demo data,
// because the schema FS does not contain it.
func TestSeedIsNotEmbeddedAsAMigration(t *testing.T) {
	loaded, err := migrations.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, m := range loaded {
		if strings.Contains(m.Up, "MER-DEMO") {
			t.Errorf("migration %04d_%s carries demo data", m.Version, m.Name)
		}
	}
}

func TestSchemaAndSeedFilesystemsAreDistinct(t *testing.T) {
	if _, err := migrations.SchemaFS().Open("seed/0001_geo.sql"); err == nil {
		t.Error("the schema filesystem must not expose demo data")
	}
	if _, err := migrations.SeedFS().Open("0001_geo.up.sql"); err == nil {
		t.Error("the seed filesystem must not expose migrations")
	}
}
