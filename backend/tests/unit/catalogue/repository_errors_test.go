package catalogue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	catpg "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/infrastructure/persistence/postgres"
)

// The integration tests prove the SQL against a real Postgres. These drive the
// failures a healthy database never produces — a connection dropped mid-scan, a
// transaction that will not start, a row holding a shop type or unit we
// retired. Getting them wrong means a menu saved with half its option groups,
// which is the one outcome the transaction exists to prevent.

var errDB = errors.New("connection reset by peer")

// ------------------------------------------------------------------ stubs

type stubRows struct {
	rows    [][]any
	idx     int
	scanErr error
	iterErr error
}

func (s *stubRows) Close()                                       {}
func (s *stubRows) Err() error                                   { return s.iterErr }
func (s *stubRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (s *stubRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (s *stubRows) Values() ([]any, error)                       { return nil, nil }
func (s *stubRows) RawValues() [][]byte                          { return nil }
func (s *stubRows) Conn() *pgx.Conn                              { return nil }
func (s *stubRows) TypeMap() *pgtype.Map                         { return pgtype.NewMap() }

func (s *stubRows) Next() bool {
	if s.idx >= len(s.rows) {
		return false
	}
	s.idx++
	return true
}

func (s *stubRows) Scan(dest ...any) error {
	if s.scanErr != nil {
		return s.scanErr
	}
	return assign(dest, s.rows[s.idx-1])
}

type stubRow struct {
	err    error
	values []any
}

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assign(dest, r.values)
}

// assign copies scripted values into scan destinations.
func assign(dest []any, values []any) error {
	for i, d := range dest {
		if i >= len(values) {
			return errors.New("stub: not enough values for the scan")
		}
		switch target := d.(type) {
		case *string:
			*target = values[i].(string)
		case *int:
			*target = values[i].(int)
		case *int64:
			*target = values[i].(int64)
		case *bool:
			*target = values[i].(bool)
		case *[]byte:
			*target = values[i].([]byte)
		case **string:
			*target = values[i].(*string)
		case **int64:
			*target = values[i].(*int64)
		case **bool:
			*target = values[i].(*bool)
		case *time.Time:
			*target = values[i].(time.Time)
		default:
			return errors.New("stub: unsupported destination type")
		}
	}
	return nil
}

// stubDB scripts query and row results, and can fail a chosen query by index so
// a test can reach the second or third read of a multi-query method.
//
// Reading a menu is several queries deep — items, then their variant groups,
// then their add-on groups — and the interesting failures are in the later
// ones. rowsByQuery gives each its own result set so a test can hand back good
// items and then a result that fails mid-scan, which is what a connection
// dropped halfway through a menu read actually looks like.
type stubDB struct {
	rows        *stubRows
	rowsByQuery []*stubRows
	row         stubRow
	queryErr    error
	execErr     error

	// failQueryAt fails only the nth query (1-based). Zero fails every one.
	failQueryAt int
	queries     int
}

func (s *stubDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	s.queries++
	if s.queryErr != nil && (s.failQueryAt == 0 || s.queries == s.failQueryAt) {
		return nil, s.queryErr
	}
	if s.queries <= len(s.rowsByQuery) {
		if scripted := s.rowsByQuery[s.queries-1]; scripted != nil {
			return scripted, nil
		}
		return &stubRows{}, nil
	}
	if s.rows == nil {
		return &stubRows{}, nil
	}
	return s.rows, nil
}

func (s *stubDB) QueryRow(context.Context, string, ...any) pgx.Row { return s.row }

func (s *stubDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

// stubTx fails on a chosen statement.
type stubTx struct {
	pgx.Tx
	failOn    string
	commitErr error
	rolledBk  bool
}

func (s *stubTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if s.failOn != "" && contains(sql, s.failOn) {
		return pgconn.CommandTag{}, errDB
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (s *stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil }
func (s *stubTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return stubRow{err: pgx.ErrNoRows}
}
func (s *stubTx) Commit(context.Context) error   { return s.commitErr }
func (s *stubTx) Rollback(context.Context) error { s.rolledBk = true; return nil }

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

type stubBeginner struct {
	tx       *stubTx
	beginErr error
}

func (s *stubBeginner) Begin(context.Context) (pgx.Tx, error) {
	if s.beginErr != nil {
		return nil, s.beginErr
	}
	return s.tx, nil
}

func bg() context.Context { return context.Background() }

// storedItem is a well-formed item row, in the projection order the repository
// reads.
func storedItem(kind, unit string, availability []byte) []any {
	return []any{
		"itm_1", "mch_1", "cat_1", kind, "Kacchi", "", "",
		int64(35_000), false, 0,
		unit, "", "", "", "", false,
		false, 0, true, availability, 0,
	}
}

// storedCombo is a well-formed combo row.
func storedCombo(kind string, availability []byte) []any {
	return []any{
		"cmb_1", "mch_1", kind, "Meal", "", "",
		int64(38_000), true, availability, 0,
	}
}

// ------------------------------------------------------------- categories

func TestAMissingCategoryIsNotFound(t *testing.T) {
	repo := catpg.New(&stubDB{row: stubRow{err: pgx.ErrNoRows}}, &stubBeginner{})
	if _, err := repo.Category(bg(), "mch_1", "cat_1"); !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Errorf("error = %v, want ErrCategoryNotFound", err)
	}
}

func TestCategoryReadsSurfaceTheirFailures(t *testing.T) {
	row := []any{"cat_1", "mch_1", "Biryani", 1, true}

	cases := map[string]*stubDB{
		"read one":       {row: stubRow{err: errDB}},
		"list query":     {queryErr: errDB},
		"list scan":      {rows: &stubRows{rows: [][]any{row}, scanErr: errDB}},
		"list iteration": {rows: &stubRows{iterErr: errDB}},
	}

	for name, db := range cases {
		t.Run(name, func(t *testing.T) {
			repo := catpg.New(db, &stubBeginner{})
			var err error
			if name == "read one" {
				_, err = repo.Category(bg(), "mch_1", "cat_1")
			} else {
				_, err = repo.Categories(bg(), "mch_1")
			}
			if !errors.Is(err, errDB) {
				t.Errorf("error = %v, want the failure wrapped", err)
			}
		})
	}
}

func TestSimpleCategoryWritesSurfaceTheirFailures(t *testing.T) {
	repo := catpg.New(&stubDB{execErr: errDB}, &stubBeginner{})

	if err := repo.SaveCategory(bg(), domain.Category{ID: "cat_1"}); !errors.Is(err, errDB) {
		t.Errorf("save: error = %v", err)
	}
	if err := repo.DeleteCategory(bg(), "mch_1", "cat_1"); !errors.Is(err, errDB) {
		t.Errorf("delete: error = %v", err)
	}
	if _, err := repo.CountItemsInCategory(bg(), "mch_1", "cat_1"); err == nil {
		t.Error("count did not report a failure")
	}
}

// ------------------------------------------------------------------- items

func TestAMissingItemIsNotFound(t *testing.T) {
	repo := catpg.New(&stubDB{row: stubRow{err: pgx.ErrNoRows}}, &stubBeginner{})
	if _, err := repo.Item(bg(), "mch_1", "itm_1"); !errors.Is(err, domain.ErrItemNotFound) {
		t.Errorf("error = %v, want ErrItemNotFound", err)
	}
}

// TestARowWithAShopTypeOrUnitWeRetiredFailsLoudly: silently defaulting would
// give an item rules its shop does not have.
func TestARowWithAShopTypeOrUnitWeRetiredFailsLoudly(t *testing.T) {
	cases := map[string]struct {
		row  []any
		want error
	}{
		"unknown shop type": {storedItem("hardware", "", []byte(`{}`)), domain.ErrUnknownMerchantType},
		"unknown unit":      {storedItem("grocery", "furlong", []byte(`{}`)), domain.ErrUnknownUnit},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := catpg.New(&stubDB{row: stubRow{values: tc.row}}, &stubBeginner{})
			if _, err := repo.Item(bg(), "mch_1", "itm_1"); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestARowWithUnreadableAvailabilityFailsLoudly(t *testing.T) {
	for name, availability := range map[string][]byte{
		"not json":     []byte(`not json`),
		"bad schedule": []byte(`{"1":["seven to eleven"]}`),
	} {
		t.Run(name, func(t *testing.T) {
			repo := catpg.New(&stubDB{row: stubRow{values: storedItem("restaurant", "", availability)}}, &stubBeginner{})
			if _, err := repo.Item(bg(), "mch_1", "itm_1"); err == nil {
				t.Error("an unreadable availability was accepted")
			}
		})
	}
}

func TestItemReadsSurfaceTheirFailures(t *testing.T) {
	good := storedItem("restaurant", "", []byte(`{}`))

	cases := map[string]struct {
		db   *stubDB
		call func(*catpg.Repository) error
	}{
		"list query": {
			&stubDB{queryErr: errDB, failQueryAt: 1},
			func(r *catpg.Repository) error { _, e := r.Items(bg(), ports.ItemFilter{Limit: 10}); return e },
		},
		"list scan": {
			&stubDB{rows: &stubRows{rows: [][]any{good}, scanErr: errDB}},
			func(r *catpg.Repository) error { _, e := r.Items(bg(), ports.ItemFilter{Limit: 10}); return e },
		},
		"list iteration": {
			&stubDB{rows: &stubRows{iterErr: errDB}},
			func(r *catpg.Repository) error { _, e := r.Items(bg(), ports.ItemFilter{Limit: 10}); return e },
		},
		"read one": {
			&stubDB{row: stubRow{err: errDB}},
			func(r *catpg.Repository) error { _, e := r.Item(bg(), "mch_1", "itm_1"); return e },
		},
		"by id": {
			&stubDB{queryErr: errDB, failQueryAt: 1},
			func(r *catpg.Repository) error { _, e := r.ItemsByID(bg(), "mch_1", []string{"itm_1"}); return e },
		},
		"by id scan": {
			&stubDB{rows: &stubRows{rows: [][]any{good}, scanErr: errDB}},
			func(r *catpg.Repository) error { _, e := r.ItemsByID(bg(), "mch_1", []string{"itm_1"}); return e },
		},
		"by id iteration": {
			&stubDB{rows: &stubRows{iterErr: errDB}},
			func(r *catpg.Repository) error { _, e := r.ItemsByID(bg(), "mch_1", []string{"itm_1"}); return e },
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := tc.call(catpg.New(tc.db, &stubBeginner{})); !errors.Is(err, errDB) {
				t.Errorf("error = %v, want the failure wrapped", err)
			}
		})
	}
}

// TestAskingForNoItemsCostsNoQuery, which is the empty-batch case a cart with
// nothing in it produces.
func TestAskingForNoItemsCostsNoQuery(t *testing.T) {
	db := &stubDB{}
	items, err := catpg.New(db, &stubBeginner{}).ItemsByID(bg(), "mch_1", nil)
	if err != nil {
		t.Fatalf("ItemsByID: %v", err)
	}
	if len(items) != 0 || db.queries != 0 {
		t.Errorf("items = %d after %d queries, want neither", len(items), db.queries)
	}
}

// TestLoadingOptionGroupsSurfacesItsFailures. The item rows read cleanly and
// the follow-up group queries fail, which is the branch a single query error
// would not reach.
func TestLoadingOptionGroupsSurfacesItsFailures(t *testing.T) {
	good := storedItem("restaurant", "", []byte(`{}`))

	for name, failAt := range map[string]int{"variant groups": 2, "add-on groups": 3} {
		t.Run(name, func(t *testing.T) {
			db := &stubDB{
				rows:        &stubRows{rows: [][]any{good}},
				queryErr:    errDB,
				failQueryAt: failAt,
			}
			if _, err := catpg.New(db, &stubBeginner{}).Items(bg(), ports.ItemFilter{Limit: 10}); !errors.Is(err, errDB) {
				t.Errorf("error = %v, want the failure wrapped", err)
			}
		})
	}
}

func TestSimpleItemWritesSurfaceTheirFailures(t *testing.T) {
	repo := catpg.New(&stubDB{execErr: errDB}, &stubBeginner{})
	if err := repo.DeleteItem(bg(), "mch_1", "itm_1"); !errors.Is(err, errDB) {
		t.Errorf("error = %v", err)
	}
}

func TestAnItemWriteSurfacesABeginFailure(t *testing.T) {
	repo := catpg.New(&stubDB{}, &stubBeginner{beginErr: errDB})

	if err := repo.SaveItem(bg(), domain.Item{ID: "itm_1"}); !errors.Is(err, errDB) {
		t.Errorf("SaveItem: error = %v", err)
	}
	if err := repo.SaveItems(bg(), []domain.Item{{ID: "itm_1"}}); !errors.Is(err, errDB) {
		t.Errorf("SaveItems: error = %v", err)
	}
	if err := repo.SaveCombo(bg(), domain.Combo{ID: "cmb_1"}); !errors.Is(err, errDB) {
		t.Errorf("SaveCombo: error = %v", err)
	}
}

// TestAFailureAnywhereInAnItemWriteRollsBackTheWhole. An item and its option
// groups are one thing: a window where the price changed and the sizes did not
// is a window where a customer is charged for a size that no longer exists.
func TestAFailureAnywhereInAnItemWriteRollsBackTheWhole(t *testing.T) {
	item := domain.Item{
		ID: "itm_1", MerchantID: "mch_1", MerchantType: domain.Restaurant,
		VariantGroups: []domain.VariantGroup{{
			ID: "vgr_1", Name: "Size",
			Options: []domain.VariantOption{{ID: "vop_1", Name: "Large"}},
		}},
		AddOnGroups: []domain.AddOnGroup{{
			ID: "agr_1", Name: "Extras",
			Options: []domain.AddOn{{ID: "aop_1", Name: "Cheese"}},
		}},
	}

	statements := []string{
		"INSERT INTO catalogue_items",
		"DELETE FROM catalogue_variant_groups",
		"INSERT INTO catalogue_variant_groups",
		"DELETE FROM catalogue_variant_options",
		"INSERT INTO catalogue_variant_options",
		"DELETE FROM catalogue_addon_groups",
		"INSERT INTO catalogue_addon_groups",
		"DELETE FROM catalogue_addon_options",
		"INSERT INTO catalogue_addon_options",
	}

	for _, statement := range statements {
		t.Run(statement, func(t *testing.T) {
			tx := &stubTx{failOn: statement}
			repo := catpg.New(&stubDB{}, &stubBeginner{tx: tx})

			if err := repo.SaveItem(bg(), item); !errors.Is(err, errDB) {
				t.Errorf("error = %v, want the failure wrapped", err)
			}
			if !tx.rolledBk {
				t.Error("a failed write must roll back")
			}
		})
	}
}

// TestABulkWriteRollsBackEveryItem, not only the one that failed.
func TestABulkWriteRollsBackEveryItem(t *testing.T) {
	tx := &stubTx{failOn: "INSERT INTO catalogue_items"}
	repo := catpg.New(&stubDB{}, &stubBeginner{tx: tx})

	err := repo.SaveItems(bg(), []domain.Item{
		{ID: "itm_1", MerchantType: domain.Grocery},
		{ID: "itm_2", MerchantType: domain.Grocery},
	})
	if !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
	if !tx.rolledBk {
		t.Error("a failed bulk write must roll back")
	}
}

func TestAnItemWriteSurfacesACommitFailure(t *testing.T) {
	repo := catpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{commitErr: errDB}})
	if err := repo.SaveItem(bg(), domain.Item{ID: "itm_1"}); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the commit failure wrapped", err)
	}
}

// A transaction already closed by the server is not worth reporting: the work
// is done and nothing is pending.
func TestACommitOnAnAlreadyClosedTransactionIsNotAnError(t *testing.T) {
	repo := catpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{commitErr: pgx.ErrTxClosed}})
	if err := repo.SaveItem(bg(), domain.Item{ID: "itm_1"}); err != nil {
		t.Errorf("error = %v, want an already-closed transaction tolerated", err)
	}
}

// ------------------------------------------------------------------ combos

func TestAMissingComboIsNotFound(t *testing.T) {
	repo := catpg.New(&stubDB{row: stubRow{err: pgx.ErrNoRows}}, &stubBeginner{})
	if _, err := repo.Combo(bg(), "mch_1", "cmb_1"); !errors.Is(err, domain.ErrComboNotFound) {
		t.Errorf("error = %v, want ErrComboNotFound", err)
	}
}

func TestAComboRowWeCannotReadFailsLoudly(t *testing.T) {
	cases := map[string][]any{
		"unknown shop type":       storedCombo("hardware", []byte(`{}`)),
		"unreadable availability": storedCombo("restaurant", []byte(`not json`)),
		"impossible availability": storedCombo("restaurant", []byte(`{"1":["noon to three"]}`)),
	}

	for name, row := range cases {
		t.Run(name, func(t *testing.T) {
			repo := catpg.New(&stubDB{row: stubRow{values: row}}, &stubBeginner{})
			if _, err := repo.Combo(bg(), "mch_1", "cmb_1"); err == nil {
				t.Error("a broken combo row was accepted")
			}
		})
	}
}

func TestComboReadsSurfaceTheirFailures(t *testing.T) {
	good := storedCombo("restaurant", []byte(`{}`))

	cases := map[string]struct {
		db   *stubDB
		call func(*catpg.Repository) error
	}{
		"list query": {
			&stubDB{queryErr: errDB, failQueryAt: 1},
			func(r *catpg.Repository) error { _, e := r.Combos(bg(), "mch_1", false); return e },
		},
		"list scan": {
			&stubDB{rows: &stubRows{rows: [][]any{good}, scanErr: errDB}},
			func(r *catpg.Repository) error { _, e := r.Combos(bg(), "mch_1", true); return e },
		},
		"list iteration": {
			&stubDB{rows: &stubRows{iterErr: errDB}},
			func(r *catpg.Repository) error { _, e := r.Combos(bg(), "mch_1", false); return e },
		},
		"read one": {
			&stubDB{row: stubRow{err: errDB}},
			func(r *catpg.Repository) error { _, e := r.Combo(bg(), "mch_1", "cmb_1"); return e },
		},
		"lines query": {
			&stubDB{rows: &stubRows{rows: [][]any{good}}, queryErr: errDB, failQueryAt: 2},
			func(r *catpg.Repository) error { _, e := r.Combos(bg(), "mch_1", false); return e },
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := tc.call(catpg.New(tc.db, &stubBeginner{})); !errors.Is(err, errDB) {
				t.Errorf("error = %v, want the failure wrapped", err)
			}
		})
	}
}

// TestReadingComboLinesSurfacesItsOwnFailures drives the line query, which runs
// after the combo rows are closed — a different branch from a failure reading
// the combos themselves.
func TestReadingComboLinesSurfacesItsOwnFailures(t *testing.T) {
	combos := &stubRows{rows: [][]any{storedCombo("restaurant", []byte(`{}`))}}

	cases := map[string]*stubRows{
		"scan fails":      {rows: [][]any{{"cmb_1", "itm_1", 1}}, scanErr: errDB},
		"iteration fails": {iterErr: errDB},
	}

	for name, lines := range cases {
		t.Run(name, func(t *testing.T) {
			combos.idx = 0
			db := &stubDB{rowsByQuery: []*stubRows{combos, lines}}
			if _, err := catpg.New(db, &stubBeginner{}).Combos(bg(), "mch_1", false); !errors.Is(err, errDB) {
				t.Errorf("error = %v, want the failure wrapped", err)
			}
		})
	}
}

// TestReadingOptionGroupsSurfacesItsOwnFailures, for the two queries that run
// after the item rows are closed.
func TestReadingOptionGroupsSurfacesItsOwnFailures(t *testing.T) {
	// A group row as the join returns it: the group's columns, then the
	// option's, which are NULL-able.
	optionID, optionName := "vop_1", "Large"
	priceDelta := int64(15_000)
	available := true
	groupRow := []any{
		"itm_1", "vgr_1", "Size", false, 0, 1, 0,
		&optionID, &optionName, &priceDelta, &available,
	}
	addOnRow := []any{
		"itm_1", "agr_1", "Extras", 0, 1, 0,
		&optionID, &optionName, &priceDelta, &available,
	}

	cases := map[string]struct {
		variantRows *stubRows
		addOnRows   *stubRows
	}{
		"variant scan fails": {
			&stubRows{rows: [][]any{groupRow}, scanErr: errDB}, &stubRows{},
		},
		"variant iteration fails": {
			&stubRows{iterErr: errDB}, &stubRows{},
		},
		"add-on scan fails": {
			&stubRows{}, &stubRows{rows: [][]any{addOnRow}, scanErr: errDB},
		},
		"add-on iteration fails": {
			&stubRows{}, &stubRows{iterErr: errDB},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			items := &stubRows{rows: [][]any{storedItem("restaurant", "", []byte(`{}`))}}
			db := &stubDB{rowsByQuery: []*stubRows{items, tc.variantRows, tc.addOnRows}}

			if _, err := catpg.New(db, &stubBeginner{}).Items(bg(), ports.ItemFilter{Limit: 10}); !errors.Is(err, errDB) {
				t.Errorf("error = %v, want the failure wrapped", err)
			}
		})
	}
}

func TestAComboWriteRollsBackOnAnyFailure(t *testing.T) {
	combo := domain.Combo{
		ID: "cmb_1", MerchantID: "mch_1", MerchantType: domain.Restaurant,
		Lines: []domain.ComboLine{{ItemID: "itm_1", Quantity: 1}},
	}

	for _, statement := range []string{
		"INSERT INTO catalogue_combos",
		"DELETE FROM catalogue_combo_lines",
		"INSERT INTO catalogue_combo_lines",
	} {
		t.Run(statement, func(t *testing.T) {
			tx := &stubTx{failOn: statement}
			repo := catpg.New(&stubDB{}, &stubBeginner{tx: tx})

			if err := repo.SaveCombo(bg(), combo); !errors.Is(err, errDB) {
				t.Errorf("error = %v, want the failure wrapped", err)
			}
			if !tx.rolledBk {
				t.Error("a failed write must roll back")
			}
		})
	}
}

func TestDeletingAComboSurfacesItsFailure(t *testing.T) {
	repo := catpg.New(&stubDB{execErr: errDB}, &stubBeginner{})
	if err := repo.DeleteCombo(bg(), "mch_1", "cmb_1"); !errors.Is(err, errDB) {
		t.Errorf("error = %v", err)
	}
}

// TestNewFromPoolIsWired guards the production constructor, which only the API
// binary calls and which nothing else would notice if it returned nil.
func TestNewFromPoolIsWired(t *testing.T) {
	if repo := catpg.NewFromPool(nil); repo == nil {
		t.Error("NewFromPool must return a repository")
	}
}

// TestReadingOneItemSurfacesAFailureLoadingItsOptions. The item row reads
// cleanly and the follow-up group query fails, which is a different branch from
// the same failure on a listing.
func TestReadingOneItemSurfacesAFailureLoadingItsOptions(t *testing.T) {
	db := &stubDB{
		row:         stubRow{values: storedItem("restaurant", "", []byte(`{}`))},
		queryErr:    errDB,
		failQueryAt: 1,
	}
	if _, err := catpg.New(db, &stubBeginner{}).Item(bg(), "mch_1", "itm_1"); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

// TestReadingOneComboSurfacesAFailureLoadingItsLines, the same shape for a
// bundle.
func TestReadingOneComboSurfacesAFailureLoadingItsLines(t *testing.T) {
	db := &stubDB{
		row:         stubRow{values: storedCombo("restaurant", []byte(`{}`))},
		queryErr:    errDB,
		failQueryAt: 1,
	}
	if _, err := catpg.New(db, &stubBeginner{}).Combo(bg(), "mch_1", "cmb_1"); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

// TestLoadingLinesForNoCombosCostsNoQuery, the empty case a shop with no
// bundles produces on every menu read.
func TestLoadingLinesForNoCombosCostsNoQuery(t *testing.T) {
	db := &stubDB{}
	combos, err := catpg.New(db, &stubBeginner{}).Combos(bg(), "mch_1", false)
	if err != nil {
		t.Fatalf("Combos: %v", err)
	}
	if len(combos) != 0 || db.queries != 1 {
		t.Errorf("combos = %d after %d queries, want one query and no lines fetch",
			len(combos), db.queries)
	}
}
