package merchant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	merchantpg "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/infrastructure/persistence/postgres"
)

// The integration tests prove the SQL against a real PostGIS. These drive the
// failures a healthy database never produces — a connection dropped mid-scan, a
// transaction that will not start, a row holding a status we retired. Getting
// them wrong means a merchant saved with half its documents, which is the one
// outcome the transaction exists to prevent.

var errDB = errors.New("connection reset by peer")

// ---------------------------------------------------------------- stubs

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

// stubRow returns one scripted row, or an error.
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
		case *float64:
			*target = values[i].(float64)
		case *bool:
			*target = values[i].(bool)
		case *[]byte:
			*target = values[i].([]byte)
		case **time.Time:
			*target = values[i].(*time.Time)
		case *time.Time:
			*target = values[i].(time.Time)
		default:
			return errors.New("stub: unsupported destination type")
		}
	}
	return nil
}

// stubDB scripts one query result and one row result.
type stubDB struct {
	rows     *stubRows
	row      stubRow
	queryErr error
	execErr  error

	// queryErrAfter lets a test fail the *second* query — the document read
	// that follows a successful merchant read.
	queryErrAfter int
	queries       int
}

func (s *stubDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	s.queries++
	if s.queryErr != nil && (s.queryErrAfter == 0 || s.queries > s.queryErrAfter) {
		return nil, s.queryErr
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
	failWith  error
	commitErr error
	rolledBk  bool
}

func (s *stubTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if s.failOn != "" && contains(sql, s.failOn) {
		if s.failWith != nil {
			return pgconn.CommandTag{}, s.failWith
		}
		return pgconn.CommandTag{}, errDB
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (s *stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &stubRows{}, nil
}
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

// storedRow is a well-formed merchant row, in the projection order the
// repository reads.
func storedRow(status, kind string, hours []byte, holidayUntil *time.Time) []any {
	return []any{
		"mch_1", "usr_1", "Nurjahan", kind, status, "+8801712345678", "", "",
		"12/A", "", 23.75, 90.39,
		"BD-C-DHA-01", "Dhanmondi", "BD-C-DHA", "BD-C",
		hours, holidayUntil != nil, holidayUntil, "", "",
		time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
	}
}

// ------------------------------------------------------------------- reads

func TestAMissingMerchantIsNotFound(t *testing.T) {
	repo := merchantpg.New(&stubDB{row: stubRow{err: pgx.ErrNoRows}}, &stubBeginner{})

	if _, err := repo.Merchant(bg(), "mch_1"); !errors.Is(err, domain.ErrMerchantNotFound) {
		t.Errorf("Merchant: error = %v, want ErrMerchantNotFound", err)
	}
	if _, err := repo.ByOwner(bg(), "usr_1"); !errors.Is(err, domain.ErrMerchantNotFound) {
		t.Errorf("ByOwner: error = %v, want ErrMerchantNotFound", err)
	}
}

func TestAReadSurfacesAConnectionFailure(t *testing.T) {
	repo := merchantpg.New(&stubDB{row: stubRow{err: errDB}}, &stubBeginner{})
	if _, err := repo.Merchant(bg(), "mch_1"); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

// TestARowWithAStatusWeRetiredFailsLoudly: silently defaulting it would put a
// shop into a state nobody chose — possibly a visible one.
func TestARowWithAStatusOrTypeWeDoNotKnowFailsLoudly(t *testing.T) {
	hours := []byte(`{"1":["09:00-22:00"]}`)

	cases := map[string]struct {
		row  []any
		want error
	}{
		"unknown status": {storedRow("banished", "restaurant", hours, nil), domain.ErrUnknownStatus},
		"unknown type":   {storedRow("approved", "hardware", hours, nil), domain.ErrUnknownType},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := merchantpg.New(&stubDB{row: stubRow{values: tc.row}}, &stubBeginner{})
			if _, err := repo.Merchant(bg(), "mch_1"); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestARowWithUnreadableHoursFailsLoudly(t *testing.T) {
	cases := map[string][]byte{
		"not json":     []byte(`not json`),
		"bad schedule": []byte(`{"1":["nine to five"]}`),
	}
	for name, hours := range cases {
		t.Run(name, func(t *testing.T) {
			repo := merchantpg.New(
				&stubDB{row: stubRow{values: storedRow("approved", "restaurant", hours, nil)}},
				&stubBeginner{})
			if _, err := repo.Merchant(bg(), "mch_1"); err == nil {
				t.Error("an unreadable schedule was accepted")
			}
		})
	}
}

// TestAHolidayDateIsNormalisedToUTC: every timestamp the backend compares is
// UTC, and a row read back in the server's local zone would compare wrongly at
// the boundary.
func TestAHolidayDateIsNormalisedToUTC(t *testing.T) {
	until := time.Date(2026, time.March, 10, 6, 0, 0, 0, time.FixedZone("+06", 6*60*60))
	repo := merchantpg.New(
		&stubDB{row: stubRow{values: storedRow("approved", "restaurant", []byte(`{}`), &until)}},
		&stubBeginner{})

	merchant, err := repo.Merchant(bg(), "mch_1")
	if err != nil {
		t.Fatalf("Merchant: %v", err)
	}
	if merchant.Holiday.Until.Location() != time.UTC {
		t.Errorf("until is in %v, want UTC", merchant.Holiday.Until.Location())
	}
	if !merchant.Holiday.Until.Equal(until) {
		t.Errorf("until = %v, want the same instant", merchant.Holiday.Until)
	}
}

func TestReadingDocumentsSurfacesItsFailures(t *testing.T) {
	good := storedRow("approved", "restaurant", []byte(`{}`), nil)
	uploaded := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)

	cases := map[string]struct {
		db *stubDB
	}{
		"query fails": {&stubDB{row: stubRow{values: good}, queryErr: errDB}},
		"scan fails": {&stubDB{
			row:  stubRow{values: good},
			rows: &stubRows{rows: [][]any{{"trade_licence", "N", "/f.png", uploaded}}, scanErr: errDB},
		}},
		"iteration fails": {&stubDB{row: stubRow{values: good}, rows: &stubRows{iterErr: errDB}}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := merchantpg.New(tc.db, &stubBeginner{}).Merchant(bg(), "mch_1"); !errors.Is(err, errDB) {
				t.Errorf("error = %v, want the failure wrapped", err)
			}
		})
	}
}

// TestADocumentOfAKindWeRetiredFailsLoudly: counting an unrecognised row as a
// supplied licence would let a shop through review on paperwork nobody read.
func TestADocumentOfAKindWeRetiredFailsLoudly(t *testing.T) {
	uploaded := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	db := &stubDB{
		row:  stubRow{values: storedRow("approved", "restaurant", []byte(`{}`), nil)},
		rows: &stubRows{rows: [][]any{{"passport", "P-1", "/f.png", uploaded}}},
	}
	if _, err := merchantpg.New(db, &stubBeginner{}).Merchant(bg(), "mch_1"); !errors.Is(err, domain.ErrUnknownDocumentKind) {
		t.Errorf("error = %v, want ErrUnknownDocumentKind", err)
	}
}

// ------------------------------------------------------------------ writes

func TestAWriteSurfacesABeginFailure(t *testing.T) {
	repo := merchantpg.New(&stubDB{}, &stubBeginner{beginErr: errDB})

	if err := repo.Create(bg(), domain.Merchant{ID: "mch_1"}); !errors.Is(err, errDB) {
		t.Errorf("Create: error = %v", err)
	}
	if err := repo.Save(bg(), domain.Merchant{ID: "mch_1"}); !errors.Is(err, errDB) {
		t.Errorf("Save: error = %v", err)
	}
}

// TestASecondRegistrationForOneOwnerIsRecognised: two registrations racing both
// pass an application-level check, so the constraint is what decides — and the
// repository must translate it rather than surfacing a raw SQLSTATE.
func TestASecondRegistrationForOneOwnerIsRecognised(t *testing.T) {
	tx := &stubTx{
		failOn:   "INSERT INTO merchants",
		failWith: &pgconn.PgError{Code: "23505", ConstraintName: "merchants_owner_user_id_key"},
	}
	repo := merchantpg.New(&stubDB{}, &stubBeginner{tx: tx})

	if err := repo.Create(bg(), domain.Merchant{ID: "mch_1"}); !errors.Is(err, domain.ErrAlreadyRegistered) {
		t.Errorf("error = %v, want ErrAlreadyRegistered", err)
	}
	if !tx.rolledBk {
		t.Error("a failed insert must roll back")
	}
}

// TestAnotherConstraintIsNotMistakenForASecondRegistration: telling an owner
// "this account already has a shop" when the real problem is something else
// would send them to support with the wrong story.
func TestAnotherConstraintIsNotMistakenForASecondRegistration(t *testing.T) {
	tx := &stubTx{
		failOn:   "INSERT INTO merchants",
		failWith: &pgconn.PgError{Code: "23503", ConstraintName: "merchants_division_fkey"},
	}
	repo := merchantpg.New(&stubDB{}, &stubBeginner{tx: tx})

	err := repo.Create(bg(), domain.Merchant{ID: "mch_1"})
	if errors.Is(err, domain.ErrAlreadyRegistered) {
		t.Errorf("a foreign-key failure was reported as a duplicate registration: %v", err)
	}
	if err == nil {
		t.Error("a foreign-key failure was not reported at all")
	}
}

func TestAFailureWritingDocumentsRollsBackTheMerchant(t *testing.T) {
	for _, failOn := range []string{"DELETE FROM merchant_documents", "INSERT INTO merchant_documents"} {
		t.Run(failOn, func(t *testing.T) {
			tx := &stubTx{failOn: failOn}
			repo := merchantpg.New(&stubDB{}, &stubBeginner{tx: tx})

			merchant := domain.Merchant{
				ID:        "mch_1",
				Documents: []domain.Document{{Kind: domain.DocTradeLicence, Number: "N", FileURL: "/f.png"}},
			}
			if err := repo.Save(bg(), merchant); !errors.Is(err, errDB) {
				t.Errorf("error = %v, want the failure wrapped", err)
			}
			if !tx.rolledBk {
				t.Error("a failed write must roll back")
			}
		})
	}
}

func TestAWriteSurfacesACommitFailure(t *testing.T) {
	repo := merchantpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{commitErr: errDB}})
	if err := repo.Save(bg(), domain.Merchant{ID: "mch_1"}); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the commit failure wrapped", err)
	}
}

// A transaction already closed by the server is not worth reporting: the work
// is done and nothing is pending.
func TestACommitOnAnAlreadyClosedTransactionIsNotAnError(t *testing.T) {
	repo := merchantpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{commitErr: pgx.ErrTxClosed}})
	if err := repo.Save(bg(), domain.Merchant{ID: "mch_1"}); err != nil {
		t.Errorf("error = %v, want an already-closed transaction tolerated", err)
	}
}

func TestSimpleWritesSurfaceTheirFailures(t *testing.T) {
	repo := merchantpg.New(&stubDB{execErr: errDB}, &stubBeginner{})

	if err := repo.Delete(bg(), "mch_1"); !errors.Is(err, errDB) {
		t.Errorf("Delete: error = %v", err)
	}
	if err := repo.RecordStatusChange(bg(), ports.StatusChange{ID: "mse_1"}); !errors.Is(err, errDB) {
		t.Errorf("RecordStatusChange: error = %v", err)
	}
}

// --------------------------------------------------------------- listings

func TestListingSurfacesItsFailures(t *testing.T) {
	good := storedRow("approved", "restaurant", []byte(`{}`), nil)

	cases := map[string]struct {
		db   *stubDB
		want error
	}{
		"query fails":     {&stubDB{queryErr: errDB}, errDB},
		"scan fails":      {&stubDB{rows: &stubRows{rows: [][]any{good}, scanErr: errDB}}, errDB},
		"iteration fails": {&stubDB{rows: &stubRows{iterErr: errDB}}, errDB},
		// The merchant rows read cleanly and the follow-up document query
		// fails, which is the branch a single query error would not reach.
		"document read fails": {
			&stubDB{rows: &stubRows{rows: [][]any{good}}, queryErr: errDB, queryErrAfter: 1},
			errDB,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := merchantpg.New(tc.db, &stubBeginner{}).List(bg(), ports.Filter{Limit: 10}); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestTheFilterIsParameterisedRatherThanInterpolated: an admin console that
// concatenated an operator-supplied division code into SQL would be an
// injection. Every clause is a placeholder, so a filter value can only ever be
// data.
func TestTheFilterIsParameterisedRatherThanInterpolated(t *testing.T) {
	db := &stubDB{rows: &stubRows{}}
	repo := merchantpg.New(db, &stubBeginner{})

	_, err := repo.List(bg(), ports.Filter{
		Status:       domain.StatusApproved,
		Type:         domain.TypeGrocery,
		DivisionCode: "BD-C'; DROP TABLE merchants; --",
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if db.queries != 1 {
		t.Errorf("queries = %d, want 1", db.queries)
	}
}

func TestReadingHistorySurfacesItsFailures(t *testing.T) {
	at := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	row := []any{"mse_1", "mch_1", "draft", "pending_review", "usr_1", "", at}

	cases := map[string]*stubDB{
		"query fails":     {queryErr: errDB},
		"scan fails":      {rows: &stubRows{rows: [][]any{row}, scanErr: errDB}},
		"iteration fails": {rows: &stubRows{iterErr: errDB}},
	}

	for name, db := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := merchantpg.New(db, &stubBeginner{}).StatusHistory(bg(), "mch_1", 10); !errors.Is(err, errDB) {
				t.Errorf("error = %v, want the failure wrapped", err)
			}
		})
	}
}

// TestHistoryKeepsAStatusWeRetired: history is a record of what happened, and a
// status we have since dropped must still be readable rather than failing the
// whole page.
func TestHistoryKeepsAStatusWeRetired(t *testing.T) {
	at := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.FixedZone("+06", 6*60*60))
	db := &stubDB{rows: &stubRows{rows: [][]any{
		{"mse_1", "mch_1", "probation", "approved", "usr_admin", "note", at},
	}}}

	events, err := merchantpg.New(db, &stubBeginner{}).StatusHistory(bg(), "mch_1", 10)
	if err != nil {
		t.Fatalf("StatusHistory: %v", err)
	}
	if len(events) != 1 || events[0].From != domain.Status("probation") {
		t.Errorf("events = %+v", events)
	}
	if events[0].At.Location() != time.UTC {
		t.Errorf("at is in %v, want UTC", events[0].At.Location())
	}
}

// TestSavingSurfacesAFailureOnTheMerchantRow: Save and Create write the same
// row through the same statement but translate failures differently — Create
// recognises the one-shop-per-owner constraint, Save has no such case — so the
// failure path has to be exercised on both.
func TestSavingSurfacesAFailureOnTheMerchantRow(t *testing.T) {
	tx := &stubTx{failOn: "INSERT INTO merchants"}
	repo := merchantpg.New(&stubDB{}, &stubBeginner{tx: tx})

	if err := repo.Save(bg(), domain.Merchant{ID: "mch_1"}); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
	if !tx.rolledBk {
		t.Error("a failed write must roll back")
	}
}

// TestNewFromPoolIsWired guards the production constructor, which only the API
// binary calls and which nothing else would notice if it returned nil.
func TestNewFromPoolIsWired(t *testing.T) {
	if repo := merchantpg.NewFromPool(nil); repo == nil {
		t.Error("NewFromPool must return a repository")
	}
}
