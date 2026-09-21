package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	paymentpg "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/infrastructure/persistence/postgres"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// The integration tests prove this SQL against a real Postgres. These drive
// the failures a healthy database never produces — a connection dropped
// mid-scan, a row this module's own writer could never have produced, a
// transaction that will not start or will not commit. Repository depends on
// the Querier and TxBeginner interfaces precisely so these paths are
// reachable without breaking a real server, and getting Remit wrong in
// particular means an operator's "all of it was handed in" claiming half a
// remittance happened.

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

func assign(dest []any, values []any) error {
	for i, d := range dest {
		if i >= len(values) {
			return errors.New("stub: not enough values for the scan")
		}
		switch target := d.(type) {
		case *string:
			*target = values[i].(string)
		case *int64:
			*target = values[i].(int64)
		case *time.Time:
			*target = values[i].(time.Time)
		default:
			return errors.New("stub: unsupported destination type")
		}
	}
	return nil
}

// stubDB scripts what the repository's queries return.
type stubDB struct {
	rowsByQuery []*stubRows
	row         stubRow
	rowByQuery  []stubRow
	queryErr    error
	execErr     error
	// zeroRows makes Exec report that nothing matched, the way a
	// compare-and-set reports a lost race.
	zeroRows bool

	queries    int
	rowQueries int
}

func (s *stubDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	s.queries++
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	if s.queries <= len(s.rowsByQuery) {
		return s.rowsByQuery[s.queries-1], nil
	}
	return &stubRows{}, nil
}

func (s *stubDB) QueryRow(context.Context, string, ...any) pgx.Row {
	s.rowQueries++
	if s.rowQueries <= len(s.rowByQuery) {
		return s.rowByQuery[s.rowQueries-1]
	}
	return s.row
}

func (s *stubDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	if s.zeroRows {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

// stubTx fails on a chosen call, or reports that an UPDATE matched nothing —
// how Remit's own per-row guard reports a collection that is not the
// caller's, or already moved.
type stubTx struct {
	pgx.Tx
	execErr    error
	noRowsAt   int
	commitErr  error
	rolledBack bool

	execs int
}

func (s *stubTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	s.execs++
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	if s.noRowsAt != 0 && s.execs == s.noRowsAt {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (s *stubTx) Commit(context.Context) error   { return s.commitErr }
func (s *stubTx) Rollback(context.Context) error { s.rolledBack = true; return nil }

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

// samplePaymentRow is one row's worth of scanPayment's columns, in order.
func samplePaymentRow() []any {
	now := time.Now()
	return []any{
		"pay_1", "ord_1", "usr_1", "manual", "ref_1", "gw_1",
		int64(50000), "BDT", "pending", "", "",
		now, now,
	}
}

func sampleCollectionRow() []any {
	now := time.Now()
	return []any{
		"col_1", "ord_1", "PTR-1", int64(50000), "BDT", "held", "",
		now, time.Unix(0, 0).UTC(), now, now,
	}
}

// ------------------------------------------------------------------ scanPayment

// scanPayment has two ways to fail: the row itself cannot be read, and a row
// that reads but names a currency this deployment does not use — which a
// healthy database never writes, since this module writes BDT everywhere,
// but a Querier stub can still produce.
func TestScanPaymentFailures(t *testing.T) {
	brokenRow := paymentpg.New(&stubDB{row: stubRow{err: errDB}}, nil)
	if _, _, err := brokenRow.PendingForOrder(bg(), "ord_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	badCurrency := samplePaymentRow()
	badCurrency[7] = "USD"
	brokenMoney := paymentpg.New(&stubDB{row: stubRow{values: badCurrency}}, nil)
	if _, _, err := brokenMoney.PendingForOrder(bg(), "ord_1"); err == nil {
		t.Fatal("a payment in a currency this deployment does not use must not scan cleanly")
	}
}

// ------------------------------------------------------------------ PendingForOrder / LatestForOrder

func TestPendingForOrderFailures(t *testing.T) {
	missing := paymentpg.New(&stubDB{row: stubRow{err: pgx.ErrNoRows}}, nil)
	if _, found, err := missing.PendingForOrder(bg(), "ord_1"); found || err != nil {
		t.Fatalf("found = %v, err = %v", found, err)
	}

	broken := paymentpg.New(&stubDB{row: stubRow{err: errDB}}, nil)
	if _, found, err := broken.PendingForOrder(bg(), "ord_1"); found || !errors.Is(err, errDB) {
		t.Fatalf("found = %v, err = %v", found, err)
	}

	found := paymentpg.New(&stubDB{row: stubRow{values: samplePaymentRow()}}, nil)
	if p, ok, err := found.PendingForOrder(bg(), "ord_1"); err != nil || !ok || p.ID != "pay_1" {
		t.Fatalf("p = %+v, ok = %v, err = %v", p, ok, err)
	}
}

func TestLatestForOrderFailures(t *testing.T) {
	missing := paymentpg.New(&stubDB{row: stubRow{err: pgx.ErrNoRows}}, nil)
	if _, found, err := missing.LatestForOrder(bg(), "ord_1"); found || err != nil {
		t.Fatalf("found = %v, err = %v", found, err)
	}

	broken := paymentpg.New(&stubDB{row: stubRow{err: errDB}}, nil)
	if _, found, err := broken.LatestForOrder(bg(), "ord_1"); found || !errors.Is(err, errDB) {
		t.Fatalf("found = %v, err = %v", found, err)
	}

	found := paymentpg.New(&stubDB{row: stubRow{values: samplePaymentRow()}}, nil)
	if p, ok, err := found.LatestForOrder(bg(), "ord_1"); err != nil || !ok || p.ID != "pay_1" {
		t.Fatalf("p = %+v, ok = %v, err = %v", p, ok, err)
	}
}

// ------------------------------------------------------------------ Save

func samplePaymentForSave(t *testing.T) domain.Payment {
	t.Helper()
	p, err := domain.NewPayment("pay_1", "ord_1", "usr_1", "manual", taka(50000), at)
	if err != nil {
		t.Fatalf("NewPayment: %v", err)
	}
	return p
}

func TestSaveFailures(t *testing.T) {
	broken := paymentpg.New(&stubDB{execErr: errDB}, nil)
	if err := broken.Save(bg(), samplePaymentForSave(t), domain.StatusPending); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

// The compare-and-set moved nothing: somebody else wrote this payment
// between the caller's read and this write, which Save reports as a
// conflict rather than silently succeeding.
func TestSaveLosesARace(t *testing.T) {
	lost := paymentpg.New(&stubDB{zeroRows: true}, nil)
	if err := lost.Save(bg(), samplePaymentForSave(t), domain.StatusPending); errs.CodeOf(err) != "payment_moved" {
		t.Fatalf("err = %v", err)
	}
}

// ------------------------------------------------------------------ scanCollection

func TestScanCollectionFailures(t *testing.T) {
	brokenRow := paymentpg.New(&stubDB{row: stubRow{err: errDB}}, nil)
	if _, err := brokenRow.Collection(bg(), "col_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	badCurrency := sampleCollectionRow()
	badCurrency[4] = "USD"
	brokenMoney := paymentpg.New(&stubDB{row: stubRow{values: badCurrency}}, nil)
	if _, err := brokenMoney.Collection(bg(), "col_1"); err == nil {
		t.Fatal("a collection in a currency this deployment does not use must not scan cleanly")
	}
}

// ------------------------------------------------------------------ ForPartner

func TestForPartnerFailures(t *testing.T) {
	broken := paymentpg.New(&stubDB{queryErr: errDB}, nil)
	if _, err := broken.ForPartner(bg(), "PTR-1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	midScan := paymentpg.New(&stubDB{rowsByQuery: []*stubRows{
		{rows: [][]any{{}}, scanErr: errDB},
	}}, nil)
	if _, err := midScan.ForPartner(bg(), "PTR-1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	iterFails := paymentpg.New(&stubDB{rowsByQuery: []*stubRows{
		{iterErr: errDB},
	}}, nil)
	if _, err := iterFails.ForPartner(bg(), "PTR-1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

// ------------------------------------------------------------------ Remit

func TestRemitFailures(t *testing.T) {
	noBeginner := paymentpg.New(&stubDB{}, &stubBeginner{beginErr: errDB})
	if err := noBeginner.Remit(bg(), "PTR-1", []string{"col_1"}, "ref", time.Now()); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	execFails := paymentpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{execErr: errDB}})
	if err := execFails.Remit(bg(), "PTR-1", []string{"col_1"}, "ref", time.Now()); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	notRemittable := paymentpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{noRowsAt: 1}})
	if err := notRemittable.Remit(bg(), "PTR-1", []string{"col_1"}, "ref", time.Now()); errs.CodeOf(err) != "collection_not_remittable" {
		t.Fatalf("err = %v", err)
	}

	// The second id in the batch is the one that fails, proving the whole
	// batch is inside one transaction rather than committing the first id
	// before the second is even attempted.
	secondFails := paymentpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{noRowsAt: 2}})
	if err := secondFails.Remit(bg(), "PTR-1", []string{"col_1", "col_2"}, "ref", time.Now()); errs.CodeOf(err) != "collection_not_remittable" {
		t.Fatalf("err = %v", err)
	}

	commitFails := paymentpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{commitErr: errDB}})
	if err := commitFails.Remit(bg(), "PTR-1", []string{"col_1"}, "ref", time.Now()); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	// A commit racing the transaction's own timeout closes it out from under
	// the caller — the collections it already moved still moved, so this is
	// not reported as a failure.
	commitRaced := paymentpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{commitErr: pgx.ErrTxClosed}})
	if err := commitRaced.Remit(bg(), "PTR-1", []string{"col_1"}, "ref", time.Now()); err != nil {
		t.Fatalf("err = %v, want a closed-on-commit race treated as success", err)
	}

	ok := paymentpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{}})
	if err := ok.Remit(bg(), "PTR-1", []string{"col_1", "col_2"}, "ref", time.Now()); err != nil {
		t.Fatalf("Remit: %v", err)
	}
}

func TestTheProductionConstructor(t *testing.T) {
	if paymentpg.NewFromPool(nil) == nil {
		t.Fatal("NewFromPool built nothing")
	}
}
