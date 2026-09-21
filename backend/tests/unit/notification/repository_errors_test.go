package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	notificationdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/domain"
	notificationpg "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/infrastructure/persistence/postgres"
)

// The integration tests prove this SQL against a real Postgres. These drive
// the failures a healthy database never produces — a connection dropped
// mid-scan, or lost between rows.

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

func assign(dest []any, values []any) error {
	for i, d := range dest {
		if i >= len(values) {
			return errors.New("stub: not enough values for the scan")
		}
		switch target := d.(type) {
		case *string:
			*target = values[i].(string)
		case *time.Time:
			*target = values[i].(time.Time)
		default:
			return errors.New("stub: unsupported destination type")
		}
	}
	return nil
}

type stubRow struct{ err error }

func (r stubRow) Scan(...any) error { return r.err }

// stubDB scripts what the repository's queries return.
type stubDB struct {
	rowsByQuery []*stubRows
	queryErr    error
	execErr     error
}

func (s *stubDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	if len(s.rowsByQuery) > 0 {
		next := s.rowsByQuery[0]
		s.rowsByQuery = s.rowsByQuery[1:]
		return next, nil
	}
	return &stubRows{}, nil
}

func (s *stubDB) QueryRow(context.Context, string, ...any) pgx.Row { return stubRow{} }

func (s *stubDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func bg() context.Context { return context.Background() }

func sampleNotificationRow() []any {
	now := time.Now()
	return []any{"ntf_1", "usr_1", "Title", "Body", "push", "sent", now}
}

func sampleDeviceRow() []any {
	return []any{"usr_1", "ios", "tok_1"}
}

// ------------------------------------------------------------------ SaveNotification

func TestSaveNotificationFailure(t *testing.T) {
	repo := notificationpg.New(&stubDB{execErr: errDB})
	n := notificationdomain.Notification{ID: "ntf_1", UserID: "usr_1", Title: "T", Body: "B"}
	if err := repo.SaveNotification(bg(), n); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

// ------------------------------------------------------------------ NotificationsForUser

func TestNotificationsForUserFailures(t *testing.T) {
	broken := notificationpg.New(&stubDB{queryErr: errDB})
	if _, err := broken.NotificationsForUser(bg(), "usr_1", 10); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	midScan := notificationpg.New(&stubDB{rowsByQuery: []*stubRows{
		{rows: [][]any{{}}, scanErr: errDB},
	}})
	if _, err := midScan.NotificationsForUser(bg(), "usr_1", 10); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	iterFails := notificationpg.New(&stubDB{rowsByQuery: []*stubRows{
		{iterErr: errDB},
	}})
	if _, err := iterFails.NotificationsForUser(bg(), "usr_1", 10); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	ok := notificationpg.New(&stubDB{rowsByQuery: []*stubRows{
		{rows: [][]any{sampleNotificationRow()}},
	}})
	got, err := ok.NotificationsForUser(bg(), "usr_1", 10)
	if err != nil || len(got) != 1 || got[0].ID != "ntf_1" {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
}

// ------------------------------------------------------------------ SaveDeviceToken

func TestSaveDeviceTokenFailure(t *testing.T) {
	repo := notificationpg.New(&stubDB{execErr: errDB})
	tok, err := notificationdomain.NewDeviceToken("usr_1", "ios", "tok_1")
	if err != nil {
		t.Fatalf("NewDeviceToken: %v", err)
	}
	if err := repo.SaveDeviceToken(bg(), tok); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

// ------------------------------------------------------------------ DeviceTokensForUser

func TestDeviceTokensForUserFailures(t *testing.T) {
	broken := notificationpg.New(&stubDB{queryErr: errDB})
	if _, err := broken.DeviceTokensForUser(bg(), "usr_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	midScan := notificationpg.New(&stubDB{rowsByQuery: []*stubRows{
		{rows: [][]any{{}}, scanErr: errDB},
	}})
	if _, err := midScan.DeviceTokensForUser(bg(), "usr_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	iterFails := notificationpg.New(&stubDB{rowsByQuery: []*stubRows{
		{iterErr: errDB},
	}})
	if _, err := iterFails.DeviceTokensForUser(bg(), "usr_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	ok := notificationpg.New(&stubDB{rowsByQuery: []*stubRows{
		{rows: [][]any{sampleDeviceRow()}},
	}})
	got, err := ok.DeviceTokensForUser(bg(), "usr_1")
	if err != nil || len(got) != 1 || got[0].Token != "tok_1" {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
}

func TestTheProductionConstructor(t *testing.T) {
	if notificationpg.NewFromPool(nil) == nil {
		t.Fatal("NewFromPool built nothing")
	}
}
