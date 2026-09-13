package platform

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/migrate"
)

// These tests drive the migrator against a scripted database. The integration
// suite proves the SQL runs against real Postgres; what matters here is the
// decision-making: what gets applied, what gets skipped, and what a partial
// failure leaves behind. A migrator that silently skips work is worse than one
// that fails, so every skip and every failure path is asserted.

var errDB = errors.New("connection refused")

// fakeDB scripts Exec and QueryVersions.
type fakeDB struct {
	execs    []string
	failOn   func(sql string, call int) error
	versions map[int]bool
	versErr  error
	versCall int
}

func (f *fakeDB) Exec(_ context.Context, sql string, _ ...any) (int64, error) {
	call := len(f.execs)
	f.execs = append(f.execs, sql)
	if f.failOn != nil {
		if err := f.failOn(sql, call); err != nil {
			return 0, err
		}
	}
	return 1, nil
}

func (f *fakeDB) QueryVersions(context.Context) (map[int]bool, error) {
	f.versCall++
	if f.versErr != nil {
		return nil, f.versErr
	}
	if f.versions == nil {
		return map[int]bool{}, nil
	}
	return f.versions, nil
}

// ranStatement reports whether any executed statement contained the fragment.
func (f *fakeDB) ranStatement(fragment string) bool {
	for _, s := range f.execs {
		if strings.Contains(s, fragment) {
			return true
		}
	}
	return false
}

func failWhen(fragment string) func(string, int) error {
	return func(sql string, _ int) error {
		if strings.Contains(sql, fragment) {
			return errDB
		}
		return nil
	}
}

func ctx() context.Context { return context.Background() }

func twoMigrations() fstest.MapFS {
	return fstest.MapFS{
		"migrations/0001_geo.up.sql":      {Data: []byte("CREATE TABLE geo ();")},
		"migrations/0001_geo.down.sql":    {Data: []byte("DROP TABLE geo;")},
		"migrations/0002_orders.up.sql":   {Data: []byte("CREATE TABLE orders ();")},
		"migrations/0002_orders.down.sql": {Data: []byte("DROP TABLE orders;")},
		"migrations/README.md":            {Data: []byte("not a migration")},
		"migrations/nested/keep.txt":      {Data: []byte("a directory entry")},
	}
}

func mustLoad(t *testing.T) []migrate.Migration {
	t.Helper()
	got, err := migrate.Load(twoMigrations(), "migrations")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return got
}

func TestLoadPairsUpsWithDownsInVersionOrder(t *testing.T) {
	got := mustLoad(t)
	if len(got) != 2 {
		t.Fatalf("loaded %d migrations, want 2", len(got))
	}
	if got[0].Version != 1 || got[1].Version != 2 {
		t.Errorf("versions = %d,%d, want 1,2", got[0].Version, got[1].Version)
	}
	if got[0].Name != "geo" {
		t.Errorf("name = %q, want geo", got[0].Name)
	}
	if got[0].Up != "CREATE TABLE geo ();" || got[0].Down != "DROP TABLE geo;" {
		t.Errorf("bodies not paired: up=%q down=%q", got[0].Up, got[0].Down)
	}
}

func TestLoadReportsAMissingDirectory(t *testing.T) {
	if _, err := migrate.Load(fstest.MapFS{}, "nowhere"); err == nil {
		t.Fatal("Load must fail when the directory does not exist")
	}
}

func TestLoadReportsAnEmptyDirectory(t *testing.T) {
	fsys := fstest.MapFS{"migrations/README.md": {Data: []byte("nothing here")}}
	_, err := migrate.Load(fsys, "migrations")
	if !errors.Is(err, migrate.ErrNoMigrations) {
		t.Errorf("error = %v, want ErrNoMigrations", err)
	}
}

func TestLoadRejectsAVersionTooLargeToParse(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/99999999999999999999999_huge.up.sql":   {Data: []byte("")},
		"migrations/99999999999999999999999_huge.down.sql": {Data: []byte("")},
	}
	_, err := migrate.Load(fsys, "migrations")
	if err == nil || !strings.Contains(err.Error(), "bad version") {
		t.Errorf("error = %v, want a bad version report", err)
	}
}

func TestLoadRejectsAMissingUp(t *testing.T) {
	fsys := fstest.MapFS{"migrations/0001_geo.down.sql": {Data: []byte("DROP TABLE geo;")}}
	_, err := migrate.Load(fsys, "migrations")
	if err == nil || !strings.Contains(err.Error(), "missing .up.sql") {
		t.Errorf("error = %v, want a missing up report", err)
	}
}

// TestLoadRejectsAMissingDown guards the rule that every migration is
// reversible. Discovering an irreversible migration during a rollback is too
// late, so it fails at load time instead.
func TestLoadRejectsAMissingDown(t *testing.T) {
	fsys := fstest.MapFS{"migrations/0001_geo.up.sql": {Data: []byte("CREATE TABLE geo ();")}}
	_, err := migrate.Load(fsys, "migrations")
	if err == nil || !strings.Contains(err.Error(), "missing .down.sql") {
		t.Errorf("error = %v, want a missing down report", err)
	}
}

// unreadableFS lists a file that cannot be opened, which is what a permission
// problem or a truncated embed looks like.
type unreadableFS struct{ inner fs.FS }

func (u unreadableFS) Open(string) (fs.File, error) { return nil, fs.ErrPermission }

func (u unreadableFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(u.inner, name)
}

func TestLoadSurfacesAnUnreadableFile(t *testing.T) {
	_, err := migrate.Load(unreadableFS{inner: twoMigrations()}, "migrations")
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("error = %v, want the read failure wrapped", err)
	}
}

func TestUpAppliesEveryMigrationToAnEmptyDatabase(t *testing.T) {
	db := &fakeDB{}
	res, err := migrate.Up(ctx(), db, mustLoad(t))
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if len(res.Applied) != 2 || res.Applied[0] != 1 || res.Applied[1] != 2 {
		t.Errorf("applied = %v, want [1 2]", res.Applied)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("skipped = %v, want none", res.Skipped)
	}
	if !db.ranStatement("CREATE TABLE IF NOT EXISTS schema_migrations") {
		t.Error("Up must bootstrap schema_migrations")
	}
	if !db.ranStatement("CREATE TABLE geo ();") || !db.ranStatement("CREATE TABLE orders ();") {
		t.Errorf("both migration bodies must run, ran %v", db.execs)
	}
}

// TestUpAppliesOnlyWhatIsMissing is the upgrade case: a database already
// carrying 0001 must get 0002 and nothing else.
func TestUpAppliesOnlyWhatIsMissing(t *testing.T) {
	db := &fakeDB{versions: map[int]bool{1: false}}
	res, err := migrate.Up(ctx(), db, mustLoad(t))
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if len(res.Applied) != 1 || res.Applied[0] != 2 {
		t.Errorf("applied = %v, want [2]", res.Applied)
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != 1 {
		t.Errorf("skipped = %v, want [1]", res.Skipped)
	}
	if db.ranStatement("CREATE TABLE geo ();") {
		t.Error("an already-applied migration must not run again")
	}
}

// TestUpOnAnUpToDateDatabaseIsANoOp is the property that makes it safe to run
// on every deploy.
func TestUpOnAnUpToDateDatabaseIsANoOp(t *testing.T) {
	db := &fakeDB{versions: map[int]bool{1: false, 2: false}}
	res, err := migrate.Up(ctx(), db, mustLoad(t))
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if len(res.Applied) != 0 {
		t.Errorf("applied = %v, want none", res.Applied)
	}
	if db.ranStatement("CREATE TABLE") && db.ranStatement("orders") {
		t.Errorf("no migration body may run, ran %v", db.execs)
	}
}

func TestUpRefusesAnEmptySet(t *testing.T) {
	if _, err := migrate.Up(ctx(), &fakeDB{}, nil); !errors.Is(err, migrate.ErrNoMigrations) {
		t.Errorf("error = %v, want ErrNoMigrations", err)
	}
}

func TestUpSurfacesAVersionTableFailure(t *testing.T) {
	db := &fakeDB{failOn: failWhen("CREATE TABLE IF NOT EXISTS schema_migrations")}
	if _, err := migrate.Up(ctx(), db, mustLoad(t)); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestUpSurfacesAVersionReadFailure(t *testing.T) {
	db := &fakeDB{versErr: errDB}
	if _, err := migrate.Up(ctx(), db, mustLoad(t)); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

// TestUpStopsOnADirtyVersion: a half-applied migration means the schema is in
// an unknown state. Applying more on top would compound the damage.
func TestUpStopsOnADirtyVersion(t *testing.T) {
	db := &fakeDB{versions: map[int]bool{1: true}}
	_, err := migrate.Up(ctx(), db, mustLoad(t))
	if !errors.Is(err, migrate.ErrDirtyVersion) {
		t.Fatalf("error = %v, want ErrDirtyVersion", err)
	}
	if !strings.Contains(err.Error(), "version 1") {
		t.Errorf("error = %v, want the offending version named", err)
	}
}

func TestUpSurfacesAMarkFailure(t *testing.T) {
	db := &fakeDB{failOn: failWhen("INSERT INTO schema_migrations")}
	res, err := migrate.Up(ctx(), db, mustLoad(t))
	if !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
	if len(res.Applied) != 0 {
		t.Errorf("applied = %v, want none reported on failure", res.Applied)
	}
}

// TestUpLeavesTheVersionDirtyWhenTheBodyFails is the point of the dirty flag:
// the failure is recorded in the database, not only in the deploy log.
func TestUpLeavesTheVersionDirtyWhenTheBodyFails(t *testing.T) {
	db := &fakeDB{failOn: failWhen("CREATE TABLE geo ();")}
	_, err := migrate.Up(ctx(), db, mustLoad(t))
	if !errors.Is(err, errDB) {
		t.Fatalf("error = %v, want the failure wrapped", err)
	}
	if !db.ranStatement("INSERT INTO schema_migrations") {
		t.Error("the version must be marked dirty before the body runs")
	}
	if db.ranStatement("UPDATE schema_migrations") {
		t.Error("a failed migration must not be marked clean")
	}
	if db.ranStatement("CREATE TABLE orders ();") {
		t.Error("a failure must stop the run, not continue to the next migration")
	}
}

func TestUpSurfacesAFinaliseFailure(t *testing.T) {
	db := &fakeDB{failOn: failWhen("UPDATE schema_migrations")}
	if _, err := migrate.Up(ctx(), db, mustLoad(t)); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestDownRollsBackNewestFirst(t *testing.T) {
	db := &fakeDB{versions: map[int]bool{1: false, 2: false}}
	res, err := migrate.Down(ctx(), db, mustLoad(t), 0)
	if err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(res.Applied) != 2 || res.Applied[0] != 2 || res.Applied[1] != 1 {
		t.Errorf("rolled back = %v, want [2 1]", res.Applied)
	}
	first, second := -1, -1
	for i, s := range db.execs {
		if strings.Contains(s, "DROP TABLE orders;") {
			first = i
		}
		if strings.Contains(s, "DROP TABLE geo;") {
			second = i
		}
	}
	if first == -1 || second == -1 || first > second {
		t.Errorf("orders must be dropped before geo, got %d then %d", first, second)
	}
}

func TestDownHonoursTheStepLimit(t *testing.T) {
	db := &fakeDB{versions: map[int]bool{1: false, 2: false}}
	res, err := migrate.Down(ctx(), db, mustLoad(t), 1)
	if err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(res.Applied) != 1 || res.Applied[0] != 2 {
		t.Errorf("rolled back = %v, want [2]", res.Applied)
	}
	if db.ranStatement("DROP TABLE geo;") {
		t.Error("the step limit must stop the rollback")
	}
}

func TestDownSkipsWhatWasNeverApplied(t *testing.T) {
	db := &fakeDB{versions: map[int]bool{2: false}}
	res, err := migrate.Down(ctx(), db, mustLoad(t), 0)
	if err != nil {
		t.Fatalf("Down: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != 1 {
		t.Errorf("skipped = %v, want [1]", res.Skipped)
	}
}

func TestDownSurfacesAVersionTableFailure(t *testing.T) {
	db := &fakeDB{failOn: failWhen("CREATE TABLE IF NOT EXISTS schema_migrations")}
	if _, err := migrate.Down(ctx(), db, mustLoad(t), 0); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestDownSurfacesAVersionReadFailure(t *testing.T) {
	db := &fakeDB{versErr: errDB}
	if _, err := migrate.Down(ctx(), db, mustLoad(t), 0); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestDownSurfacesARollbackFailure(t *testing.T) {
	db := &fakeDB{versions: map[int]bool{1: false, 2: false}, failOn: failWhen("DROP TABLE orders;")}
	_, err := migrate.Down(ctx(), db, mustLoad(t), 0)
	if !errors.Is(err, errDB) || !strings.Contains(err.Error(), "0002_orders") {
		t.Errorf("error = %v, want the failing migration named", err)
	}
}

func TestDownSurfacesAVersionClearFailure(t *testing.T) {
	db := &fakeDB{versions: map[int]bool{1: false, 2: false}, failOn: failWhen("DELETE FROM schema_migrations")}
	if _, err := migrate.Down(ctx(), db, mustLoad(t), 0); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

// TestDownDoesNotMutateTheCallersSlice: Down sorts newest-first internally, and
// a caller that runs Down then Up must still get ascending order.
func TestDownDoesNotMutateTheCallersSlice(t *testing.T) {
	loaded := mustLoad(t)
	db := &fakeDB{versions: map[int]bool{1: false, 2: false}}
	if _, err := migrate.Down(ctx(), db, loaded, 0); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if loaded[0].Version != 1 || loaded[1].Version != 2 {
		t.Errorf("caller slice reordered to %d,%d", loaded[0].Version, loaded[1].Version)
	}
}

func TestDescribeReportsAnUpToDateDatabase(t *testing.T) {
	got := migrate.Describe(migrate.Result{})
	if !strings.Contains(got, "already up to date") {
		t.Errorf("Describe = %q", got)
	}
}

func TestDescribeNamesWhatWasApplied(t *testing.T) {
	got := migrate.Describe(migrate.Result{Applied: []int{1, 2}})
	if !strings.Contains(got, "applied 2 migration(s)") || !strings.Contains(got, "0001") || !strings.Contains(got, "0002") {
		t.Errorf("Describe = %q", got)
	}
}

func TestDescribeCountsSkips(t *testing.T) {
	got := migrate.Describe(migrate.Result{Applied: []int{2}, Skipped: []int{1}})
	if !strings.Contains(got, "skipped 1 already applied") {
		t.Errorf("Describe = %q", got)
	}
}
