package cart

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	cartdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	cartpg "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/infrastructure/persistence/postgres"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// The integration tests prove the SQL against a real Postgres. These drive the
// failures a healthy database never produces — a connection dropped between the
// lines query and the options query, a transaction that will not start, a
// commit that fails after the lines were written. Getting them wrong means a
// cart saved with half its options, which is the one outcome the transaction
// exists to prevent.

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
		case *time.Time:
			*target = values[i].(time.Time)
		default:
			return errors.New("stub: unsupported destination type")
		}
	}
	return nil
}

// stubDB scripts query and row results, failing a chosen query by index so a
// test can reach the second read of a two-query method.
type stubDB struct {
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
	if s.queries <= len(s.rowsByQuery) && s.rowsByQuery[s.queries-1] != nil {
		return s.rowsByQuery[s.queries-1], nil
	}
	return &stubRows{}, nil
}

func (s *stubDB) QueryRow(context.Context, string, ...any) pgx.Row { return s.row }

func (s *stubDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	return pgconn.NewCommandTag("DELETE 1"), nil
}

// stubTx fails on a chosen statement.
type stubTx struct {
	pgx.Tx
	failOn     string
	commitErr  error
	rolledBack bool
}

func (s *stubTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if s.failOn != "" && strings.Contains(sql, s.failOn) {
		return pgconn.CommandTag{}, errDB
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
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

// storedCart is a well-formed cart row, in the projection order the repository
// reads.
func storedCart() []any {
	return []any{"crt_1", "usr_1", "mch_1", "adr_1", 23.7465, 90.3760, time.Unix(0, 0).UTC()}
}

// storedLine is a well-formed line row, likewise.
func storedLine(id string) []any {
	return []any{id, "item", "itm_1", "Burger", int64(25000), 2, ""}
}

// ------------------------------------------------------------------ tests

func TestOfUserFailures(t *testing.T) {
	t.Run("the cart row cannot be read", func(t *testing.T) {
		repo := cartpg.New(&stubDB{row: stubRow{err: errDB}}, &stubBeginner{})
		if _, _, err := repo.OfUser(bg(), "usr_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the lines query fails", func(t *testing.T) {
		repo := cartpg.New(&stubDB{
			row: stubRow{values: storedCart()}, queryErr: errDB, failQueryAt: 1,
		}, &stubBeginner{})
		if _, _, err := repo.OfUser(bg(), "usr_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a line fails mid-scan", func(t *testing.T) {
		repo := cartpg.New(&stubDB{
			row: stubRow{values: storedCart()},
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedLine("cln_1")}, scanErr: errDB},
			},
		}, &stubBeginner{})
		if _, _, err := repo.OfUser(bg(), "usr_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the line iteration fails", func(t *testing.T) {
		repo := cartpg.New(&stubDB{
			row:         stubRow{values: storedCart()},
			rowsByQuery: []*stubRows{{iterErr: errDB}},
		}, &stubBeginner{})
		if _, _, err := repo.OfUser(bg(), "usr_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	// The interesting one: the lines came back and the connection dropped
	// before the options did. A cart rendered from those lines would be missing
	// every variant the customer chose, at every variant's price.
	t.Run("the options query fails", func(t *testing.T) {
		repo := cartpg.New(&stubDB{
			row: stubRow{values: storedCart()},
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedLine("cln_1")}},
			},
			queryErr: errDB, failQueryAt: 2,
		}, &stubBeginner{})
		if _, _, err := repo.OfUser(bg(), "usr_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("an option fails mid-scan", func(t *testing.T) {
		repo := cartpg.New(&stubDB{
			row: stubRow{values: storedCart()},
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedLine("cln_1")}},
				{rows: [][]any{{"cln_1", "grp", "opt", "Large", int64(5000)}}, scanErr: errDB},
			},
		}, &stubBeginner{})
		if _, _, err := repo.OfUser(bg(), "usr_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the option iteration fails", func(t *testing.T) {
		repo := cartpg.New(&stubDB{
			row: stubRow{values: storedCart()},
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedLine("cln_1")}},
				{iterErr: errDB},
			},
		}, &stubBeginner{})
		if _, _, err := repo.OfUser(bg(), "usr_1"); !errors.Is(err, errDB) {
			t.Fatalf("err = %v", err)
		}
	})

	// An option row whose line is not in the set just read. Impossible through
	// the join the repository uses; skipped rather than indexed blindly,
	// because a panic here would be a 500 on a cart screen.
	t.Run("an orphan option row is skipped", func(t *testing.T) {
		repo := cartpg.New(&stubDB{
			row: stubRow{values: storedCart()},
			rowsByQuery: []*stubRows{
				{rows: [][]any{storedLine("cln_1")}},
				{rows: [][]any{{"cln_nowhere", "grp", "opt", "Large", int64(5000)}}},
			},
		}, &stubBeginner{})
		cart, found, err := repo.OfUser(bg(), "usr_1")
		if err != nil || !found {
			t.Fatalf("found = %v, err = %v", found, err)
		}
		if len(cart.Lines) != 1 || len(cart.Lines[0].Options) != 0 {
			t.Fatalf("cart = %+v", cart.Lines)
		}
	})

	// A cart with no lines does not run the options query at all.
	t.Run("an empty cart", func(t *testing.T) {
		db := &stubDB{row: stubRow{values: storedCart()}}
		repo := cartpg.New(db, &stubBeginner{})
		cart, found, err := repo.OfUser(bg(), "usr_1")
		if err != nil || !found {
			t.Fatalf("found = %v, err = %v", found, err)
		}
		if len(cart.Lines) != 0 {
			t.Fatalf("lines = %+v", cart.Lines)
		}
		if db.queries != 1 {
			t.Errorf("%d queries for an empty cart, want one", db.queries)
		}
	})
}

func TestSaveFailures(t *testing.T) {
	cart, err := cartdomain.NewCart("crt_1", "usr_1", "mch_1", time.Unix(0, 0))
	if err != nil {
		t.Fatalf("NewCart: %v", err)
	}
	line := cartdomain.Line{
		ID: "cln_1", Kind: cartdomain.KindItem, TargetID: "itm_1", Name: "Burger",
		UnitPrice: money.Taka(25000), Quantity: 1,
		Options: []cartdomain.Option{
			{GroupID: "grp", OptionID: "opt", Name: "Large", Price: money.Taka(5000)},
		},
	}
	if err := cart.Add("mch_1", line); err != nil {
		t.Fatalf("Add: %v", err)
	}

	cases := []struct {
		name     string
		beginner *stubBeginner
	}{
		{"the transaction will not start", &stubBeginner{beginErr: errDB}},
		{"the cart row will not write", &stubBeginner{tx: &stubTx{failOn: "INSERT INTO carts"}}},
		{"the old lines will not clear", &stubBeginner{tx: &stubTx{failOn: "DELETE FROM cart_lines"}}},
		{"a line will not write", &stubBeginner{tx: &stubTx{failOn: "INSERT INTO cart_lines"}}},
		{"an option will not write", &stubBeginner{tx: &stubTx{failOn: "INSERT INTO cart_line_options"}}},
		{"the commit fails", &stubBeginner{tx: &stubTx{commitErr: errDB}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := cartpg.New(&stubDB{}, tc.beginner)
			if err := repo.Save(bg(), cart); !errors.Is(err, errDB) {
				t.Fatalf("err = %v, want the database failure", err)
			}
			// Everything rolls back together. A cart saved with its lines and
			// not its options would charge the customer for a size they did
			// not choose.
			if tc.beginner.tx != nil && !tc.beginner.tx.rolledBack {
				t.Error("the transaction was not rolled back")
			}
		})
	}
}

func TestDeleteFailure(t *testing.T) {
	repo := cartpg.New(&stubDB{execErr: errDB}, &stubBeginner{})
	if err := repo.Delete(bg(), "crt_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

// TestNewFromPoolIsWired guards the production constructor, which only the API
// binary calls.
func TestNewFromPoolIsWired(t *testing.T) {
	if repo := cartpg.NewFromPool(nil); repo == nil {
		t.Error("NewFromPool must return a repository")
	}
}
