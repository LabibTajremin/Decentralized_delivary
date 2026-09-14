package user

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/domain"
	userpg "github.com/rootlogic-lab/delivery/backend/internal/modules/user/infrastructure/persistence/postgres"
)

// The integration tests prove the SQL against real PostgreSQL. These drive the
// failures a healthy database never produces — a connection dropped mid-scan,
// a transaction that will not start. Getting them wrong means a half-written
// address book, which is the one outcome the transaction exists to prevent.

var errDB = errors.New("connection reset by peer")

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
	values := s.rows[s.idx-1]
	for i, d := range dest {
		switch target := d.(type) {
		case *string:
			*target = values[i].(string)
		case *float64:
			*target = values[i].(float64)
		case *bool:
			*target = values[i].(bool)
		default:
			return errors.New("stub: unsupported destination type")
		}
	}
	return nil
}

type stubDB struct {
	rows     *stubRows
	queryErr error
	execErr  error
}

func (s *stubDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	if s.rows == nil {
		return &stubRows{}, nil
	}
	return s.rows, nil
}

func (s *stubDB) QueryRow(context.Context, string, ...any) pgx.Row { return stubRow{} }

func (s *stubDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

type stubRow struct{}

func (stubRow) Scan(...any) error { return pgx.ErrNoRows }

// stubTx fails on a chosen statement.
type stubTx struct {
	pgx.Tx
	failOn    string
	calls     int
	commitErr error
	rolledBk  bool
}

func (s *stubTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	s.calls++
	if s.failOn != "" && contains(sql, s.failOn) {
		return pgconn.CommandTag{}, errDB
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
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

func TestListingAddressesSurfacesAQueryFailure(t *testing.T) {
	repo := userpg.New(&stubDB{queryErr: errDB}, &stubBeginner{})
	if _, err := repo.Addresses(bg(), "usr_1"); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
}

func TestListingAddressesSurfacesAScanFailure(t *testing.T) {
	db := &stubDB{rows: &stubRows{rows: [][]any{{"adr_1"}}, scanErr: errDB}}
	if _, err := userpg.New(db, &stubBeginner{}).Addresses(bg(), "usr_1"); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the scan failure wrapped", err)
	}
}

// A connection dropped mid-stream yields a short, error-free-looking list.
// Treating that as complete would silently hide a customer's addresses — and,
// worse, could look like an empty address book at checkout.
func TestListingAddressesSurfacesAnIterationFailure(t *testing.T) {
	db := &stubDB{rows: &stubRows{iterErr: errDB}}
	if _, err := userpg.New(db, &stubBeginner{}).Addresses(bg(), "usr_1"); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the iteration failure wrapped", err)
	}
}

func TestSavingAddressesSurfacesABeginFailure(t *testing.T) {
	repo := userpg.New(&stubDB{}, &stubBeginner{beginErr: errDB})
	if err := repo.SaveAddresses(bg(), "usr_1", nil); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the begin failure wrapped", err)
	}
}

// TestAFailureClearingDefaultsRollsBack: if the flag cannot be cleared, the
// inserts that follow would hit the one-default index, and a partial write
// would leave the address book with two defaults or none.
func TestAFailureClearingDefaultsRollsBack(t *testing.T) {
	tx := &stubTx{failOn: "SET is_default = FALSE"}
	repo := userpg.New(&stubDB{}, &stubBeginner{tx: tx})

	if err := repo.SaveAddresses(bg(), "usr_1", []domain.Address{{ID: "adr_1"}}); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
	if !tx.rolledBk {
		t.Error("a failed write must roll back")
	}
}

func TestAFailureSavingOneAddressRollsBackTheRest(t *testing.T) {
	tx := &stubTx{failOn: "INSERT INTO user_addresses"}
	repo := userpg.New(&stubDB{}, &stubBeginner{tx: tx})

	if err := repo.SaveAddresses(bg(), "usr_1", []domain.Address{{ID: "adr_1"}}); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}
	if !tx.rolledBk {
		t.Error("a failed write must roll back")
	}
}

func TestSavingAddressesSurfacesACommitFailure(t *testing.T) {
	tx := &stubTx{commitErr: errDB}
	repo := userpg.New(&stubDB{}, &stubBeginner{tx: tx})
	if err := repo.SaveAddresses(bg(), "usr_1", nil); !errors.Is(err, errDB) {
		t.Errorf("error = %v, want the commit failure wrapped", err)
	}
}

// A transaction already closed by the server is not worth reporting: the work
// is done and nothing is pending.
func TestACommitOnAnAlreadyClosedTransactionIsNotAnError(t *testing.T) {
	tx := &stubTx{commitErr: pgx.ErrTxClosed}
	repo := userpg.New(&stubDB{}, &stubBeginner{tx: tx})
	if err := repo.SaveAddresses(bg(), "usr_1", nil); err != nil {
		t.Errorf("error = %v, want an already-closed transaction tolerated", err)
	}
}

func TestProfileAndAddressReadsSurfaceFailures(t *testing.T) {
	repo := userpg.New(&stubDB{execErr: errDB}, &stubBeginner{})

	profile, err := domain.NewProfile("usr_1", "A", "", domain.LanguageBengali)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	if err := repo.SaveProfile(bg(), profile); !errors.Is(err, errDB) {
		t.Errorf("save profile = %v", err)
	}
	if err := repo.DeleteAddress(bg(), "usr_1", "adr_1"); !errors.Is(err, errDB) {
		t.Errorf("delete address = %v", err)
	}
}
