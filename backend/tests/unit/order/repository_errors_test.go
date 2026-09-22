package order

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	orderports "github.com/rootlogic-lab/delivery/backend/internal/modules/order/application/ports"
	orderdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	orderpg "github.com/rootlogic-lab/delivery/backend/internal/modules/order/infrastructure/persistence/postgres"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// The integration tests prove the SQL against a real Postgres. These drive the
// failures a healthy database never produces — a connection dropped between the
// order row and its lines, a transaction that will not start, a commit that
// fails after the lines were written. Getting them wrong means an order the
// customer paid for and the shop never saw, which is the one outcome the
// transaction exists to prevent.

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
		case *int:
			*target = values[i].(int)
		case *int64:
			*target = values[i].(int64)
		case *float64:
			*target = values[i].(float64)
		case *bool:
			*target = values[i].(bool)
		case *time.Time:
			*target = values[i].(time.Time)
		default:
			return errors.New("stub: unsupported destination type")
		}
	}
	return nil
}

// stubDB scripts query and row results, failing a chosen query by index so a
// test can reach the third read of a four-query listing.
type stubDB struct {
	rowsByQuery []*stubRows
	rowByQuery  []stubRow
	row         stubRow
	queryErr    error
	execErr     error

	failQueryAt int
	queries     int
	rowQueries  int
}

func (s *stubDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	s.queries++
	if s.queryErr != nil && (s.failQueryAt == 0 || s.queries == s.failQueryAt) {
		return nil, s.queryErr
	}
	if s.queries <= len(s.rowsByQuery) && s.rowsByQuery[s.queries-1] != nil {
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
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

// stubTx fails on a chosen statement, and can report that an UPDATE matched
// nothing — which is how the concurrency guard reports a lost race.
type stubTx struct {
	pgx.Tx
	failOn     string
	noRowsOn   string
	commitErr  error
	rolledBack bool
}

func (s *stubTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if s.failOn != "" && strings.Contains(sql, s.failOn) {
		return pgconn.CommandTag{}, errDB
	}
	if s.noRowsOn != "" && strings.Contains(sql, s.noRowsOn) {
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

// storedOrder is a well-formed order row, in the projection order the
// repository reads.
func storedOrder() []any {
	return []any{
		"ord_1", "ABC234", "usr_1", "mch_1", "placed", "cash",
		int64(50000), int64(7000), int64(0), int64(57000),
		false, false, 0, 2400.0,
		"adr_1", "Home", "Fardin", "+8801711111111",
		"House 5", "", "House 5, Dhanmondi", 23.746, 90.375,
		"DHK-DHM", "Dhanmondi", "",
		"Star Kabab", "+8801711000001", "Road 27", 23.7455, 90.3738,
		time.Unix(0, 0).UTC(), time.Unix(0, 0).UTC(),
	}
}

func storedLine() []any {
	return []any{"ord_1", "oln_1", "item", "itm_1", "Kacchi", int64(35000), 2, ""}
}

func storedEvent() []any {
	return []any{"ord_1", "oev_1", "placed", "customer", "usr_1", "", time.Unix(0, 0).UTC()}
}

// sampleOrder is a well-formed order for the write paths.
func sampleOrder(t *testing.T) orderdomain.Order {
	t.Helper()
	order, err := orderdomain.NewOrder(orderdomain.Draft{
		ID: "ord_1", EventID: "oev_1", Code: "ABC234",
		CustomerID: "usr_1", MerchantID: "mch_1",
		Payment: orderdomain.PaymentCash,
		Lines: []orderdomain.Line{{
			ID: "oln_1", Kind: "item", TargetID: "itm_1", Name: "Kacchi",
			UnitPrice: money.Taka(35000), Quantity: 2,
			Options: []orderdomain.Option{
				{GroupID: "grp", OptionID: "opt", Name: "Full", Price: money.Taka(5000)},
			},
		}},
		Charges:     orderdomain.Charges{Subtotal: money.Taka(80000), Total: money.Taka(87000)},
		Destination: orderdomain.Destination{AddressID: "adr_1"},
		Now:         time.Unix(0, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	return order
}

// ------------------------------------------------------------------ tests

func TestCreateFailures(t *testing.T) {
	order := sampleOrder(t)
	cases := []struct {
		name     string
		beginner *stubBeginner
	}{
		{"the transaction will not start", &stubBeginner{beginErr: errDB}},
		{"the order row will not write", &stubBeginner{tx: &stubTx{failOn: "INSERT INTO orders"}}},
		{"a line will not write", &stubBeginner{tx: &stubTx{failOn: "INSERT INTO order_lines"}}},
		{"an option will not write", &stubBeginner{tx: &stubTx{failOn: "INSERT INTO order_line_options"}}},
		{"the first event will not write", &stubBeginner{tx: &stubTx{failOn: "INSERT INTO order_events"}}},
		{"the commit fails", &stubBeginner{tx: &stubTx{commitErr: errDB}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := orderpg.New(&stubDB{}, tc.beginner)
			if err := repo.Create(bg(), order); !errors.Is(err, errDB) {
				t.Fatalf("err = %v, want the database failure", err)
			}
			// Everything rolls back together. An order written without its
			// lines is an order the shop cannot make.
			if tc.beginner.tx != nil && !tc.beginner.tx.rolledBack {
				t.Error("the transaction was not rolled back")
			}
		})
	}
}

func TestOrderReadFailures(t *testing.T) {
	t.Run("no such order", func(t *testing.T) {
		repo := orderpg.New(&stubDB{row: stubRow{err: pgx.ErrNoRows}}, &stubBeginner{})
		_, err := repo.Order(bg(), "ord_1")
		if !errs.Is(err, errs.KindNotFound) {
			t.Fatalf("err = %v, want a not-found", err)
		}
	})

	t.Run("the order row cannot be read", func(t *testing.T) {
		repo := orderpg.New(&stubDB{row: stubRow{err: errDB}}, &stubBeginner{})
		if _, err := repo.Order(bg(), "ord_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the lines query fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row: stubRow{values: storedOrder()}, queryErr: errDB, failQueryAt: 1,
		}, &stubBeginner{})
		if _, err := repo.Order(bg(), "ord_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a line fails mid-scan", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row:         stubRow{values: storedOrder()},
			rowsByQuery: []*stubRows{{rows: [][]any{storedLine()}, scanErr: errDB}},
		}, &stubBeginner{})
		if _, err := repo.Order(bg(), "ord_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the line iteration fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row:         stubRow{values: storedOrder()},
			rowsByQuery: []*stubRows{{iterErr: errDB}},
		}, &stubBeginner{})
		if _, err := repo.Order(bg(), "ord_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	// The lines came back and the connection dropped before the options did.
	// An order rendered from those lines would be missing every choice the
	// customer made, at every choice's price.
	t.Run("the options query fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row:         stubRow{values: storedOrder()},
			rowsByQuery: []*stubRows{{rows: [][]any{storedLine()}}},
			queryErr:    errDB, failQueryAt: 2,
		}, &stubBeginner{})
		if _, err := repo.Order(bg(), "ord_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("an option fails mid-scan", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row: stubRow{values: storedOrder()},
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedLine()}},
				{rows: [][]any{{"oln_1", "grp", "opt", "Full", int64(5000)}}, scanErr: errDB},
			},
		}, &stubBeginner{})
		if _, err := repo.Order(bg(), "ord_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the option iteration fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row: stubRow{values: storedOrder()},
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedLine()}},
				{iterErr: errDB},
			},
		}, &stubBeginner{})
		if _, err := repo.Order(bg(), "ord_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	// An option whose line is not in the set just read. Impossible through the
	// join the repository uses; skipped rather than indexed blindly, because a
	// panic here would be a 500 on an order screen.
	t.Run("an orphan option row is skipped", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row: stubRow{values: storedOrder()},
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedLine()}},
				{rows: [][]any{{"oln_nowhere", "grp", "opt", "Full", int64(5000)}}},
				{rows: [][]any{storedEvent()}},
			},
		}, &stubBeginner{})
		got, err := repo.Order(bg(), "ord_1")
		if err != nil {
			t.Fatalf("Order: %v", err)
		}
		if len(got.Lines) != 1 || len(got.Lines[0].Options) != 0 {
			t.Fatalf("lines = %+v", got.Lines)
		}
	})

	t.Run("the events query fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row: stubRow{values: storedOrder()},
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedLine()}},
				{},
			},
			queryErr: errDB, failQueryAt: 3,
		}, &stubBeginner{})
		if _, err := repo.Order(bg(), "ord_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("an event fails mid-scan", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row: stubRow{values: storedOrder()},
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedLine()}},
				{},
				{rows: [][]any{storedEvent()}, scanErr: errDB},
			},
		}, &stubBeginner{})
		if _, err := repo.Order(bg(), "ord_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the event iteration fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row: stubRow{values: storedOrder()},
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedLine()}},
				{},
				{iterErr: errDB},
			},
		}, &stubBeginner{})
		if _, err := repo.Order(bg(), "ord_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	// An order with no lines does not run the options query at all.
	t.Run("an order with no lines", func(t *testing.T) {
		db := &stubDB{row: stubRow{values: storedOrder()}}
		repo := orderpg.New(db, &stubBeginner{})
		got, err := repo.Order(bg(), "ord_1")
		if err != nil {
			t.Fatalf("Order: %v", err)
		}
		if len(got.Lines) != 0 {
			t.Fatalf("lines = %+v", got.Lines)
		}
	})
}

func TestListFailures(t *testing.T) {
	counted := stubRow{values: []any{2}}

	t.Run("the count query fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{row: stubRow{err: errDB}}, &stubBeginner{})
		if _, _, err := repo.Orders(bg(), orderports.Filter{CustomerID: "usr_1", Limit: 10}); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	// Nothing to list is not an error, and does not run the page query.
	t.Run("an empty result", func(t *testing.T) {
		db := &stubDB{row: stubRow{values: []any{0}}}
		repo := orderpg.New(db, &stubBeginner{})
		orders, total, err := repo.Orders(bg(), orderports.Filter{CustomerID: "usr_1", Limit: 10})
		if err != nil || total != 0 || len(orders) != 0 {
			t.Fatalf("orders = %+v, total = %d, err = %v", orders, total, err)
		}
		if db.queries != 0 {
			t.Errorf("%d page queries were run for an empty result", db.queries)
		}
	})

	t.Run("the page query fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{row: counted, queryErr: errDB, failQueryAt: 1}, &stubBeginner{})
		if _, _, err := repo.Orders(bg(), orderports.Filter{CustomerID: "usr_1", Limit: 10}); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("an order fails mid-scan", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row:         counted,
			rowsByQuery: []*stubRows{{rows: [][]any{storedOrder()}, scanErr: errDB}},
		}, &stubBeginner{})
		if _, _, err := repo.Orders(bg(), orderports.Filter{CustomerID: "usr_1", Limit: 10}); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the page iteration fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row:         counted,
			rowsByQuery: []*stubRows{{iterErr: errDB}},
		}, &stubBeginner{})
		if _, _, err := repo.Orders(bg(), orderports.Filter{CustomerID: "usr_1", Limit: 10}); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	// A count that disagrees with the page — rows deleted between the two
	// queries — is reported as the count, with nothing to show. Better than
	// pretending the count was wrong.
	t.Run("a count with no rows behind it", func(t *testing.T) {
		repo := orderpg.New(&stubDB{row: counted}, &stubBeginner{})
		orders, total, err := repo.Orders(bg(), orderports.Filter{MerchantID: "mch_1", Limit: 10})
		if err != nil {
			t.Fatalf("Orders: %v", err)
		}
		if total != 2 || len(orders) != 0 {
			t.Fatalf("orders = %+v, total = %d", orders, total)
		}
	})

	t.Run("the lines sweep fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row:         counted,
			rowsByQuery: []*stubRows{{rows: [][]any{storedOrder()}}},
			queryErr:    errDB, failQueryAt: 2,
		}, &stubBeginner{})
		if _, _, err := repo.Orders(bg(), orderports.Filter{CustomerID: "usr_1", Limit: 10}); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the events sweep fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			row: counted,
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedOrder()}},
				{rows: [][]any{storedLine()}},
				{},
			},
			queryErr: errDB, failQueryAt: 4,
		}, &stubBeginner{})
		if _, _, err := repo.Orders(bg(), orderports.Filter{CustomerID: "usr_1", Limit: 10}); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	// A status filter contributes its own placeholders, which is the branch
	// that gets the numbering wrong if anything does.
	t.Run("a status filter", func(t *testing.T) {
		db := &stubDB{row: stubRow{values: []any{0}}}
		repo := orderpg.New(db, &stubBeginner{})
		if _, _, err := repo.Orders(bg(), orderports.Filter{
			MerchantID: "mch_1", LiveOnly: true, Limit: 10,
			Statuses: []orderdomain.Status{orderdomain.StatusPlaced, orderdomain.StatusAccepted},
		}); err != nil {
			t.Fatalf("Orders: %v", err)
		}
	})
}

func TestAppendTransitionFailures(t *testing.T) {
	event := orderdomain.Event{
		ID: "oev_2", Status: orderdomain.StatusAccepted,
		Actor: orderdomain.ActorMerchant, At: time.Unix(0, 0).UTC(),
	}

	t.Run("the transaction will not start", func(t *testing.T) {
		repo := orderpg.New(&stubDB{}, &stubBeginner{beginErr: errDB})
		if err := repo.AppendTransition(bg(), "ord_1", orderdomain.StatusPlaced, event); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the update fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{failOn: "UPDATE orders"}})
		if err := repo.AppendTransition(bg(), "ord_1", orderdomain.StatusPlaced, event); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	// The concurrency guard: the order is no longer where the caller thought
	// it was, so the UPDATE matched nothing and the second rider is told.
	t.Run("the order moved underneath", func(t *testing.T) {
		repo := orderpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{noRowsOn: "UPDATE orders"}})
		err := repo.AppendTransition(bg(), "ord_1", orderdomain.StatusPlaced, event)
		if errs.CodeOf(err) != "order_moved" {
			t.Fatalf("err = %v, want order_moved", err)
		}
	})

	t.Run("the event will not write", func(t *testing.T) {
		repo := orderpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{failOn: "INSERT INTO order_events"}})
		if err := repo.AppendTransition(bg(), "ord_1", orderdomain.StatusPlaced, event); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the commit fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{}, &stubBeginner{tx: &stubTx{commitErr: errDB}})
		if err := repo.AppendTransition(bg(), "ord_1", orderdomain.StatusPlaced, event); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestIdempotencyKeyFailures(t *testing.T) {
	t.Run("the key lookup fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{row: stubRow{err: errDB}}, &stubBeginner{})
		if _, _, err := repo.ByIdempotencyKey(bg(), "usr_1", "k"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	// The key resolves to an order that has since gone. Reported rather than
	// treated as "no key": a mapping pointing at nothing is a fault, not a
	// fresh request.
	t.Run("the key points at a missing order", func(t *testing.T) {
		repo := orderpg.New(&stubDB{
			rowByQuery: []stubRow{
				{values: []any{"ord_gone"}},
				{err: pgx.ErrNoRows},
			},
		}, &stubBeginner{})
		if _, _, err := repo.ByIdempotencyKey(bg(), "usr_1", "k"); err == nil {
			t.Fatal("a dangling key was treated as no key")
		}
	})

	t.Run("the claim fails", func(t *testing.T) {
		repo := orderpg.New(&stubDB{execErr: errDB}, &stubBeginner{})
		if err := repo.ClaimIdempotencyKey(bg(), "usr_1", "k", "ord_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})
}
