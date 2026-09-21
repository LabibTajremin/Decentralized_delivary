package review

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	reviewdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	reviewpg "github.com/rootlogic-lab/delivery/backend/internal/modules/review/infrastructure/persistence/postgres"
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
		v := values[i]
		switch target := d.(type) {
		case *string:
			*target = v.(string)
		case *int:
			*target = v.(int)
		case *bool:
			*target = v.(bool)
		case *time.Time:
			*target = v.(time.Time)
		case **time.Time:
			if v == nil {
				*target = nil
				continue
			}
			tv := v.(time.Time)
			*target = &tv
		case **float64:
			if v == nil {
				*target = nil
				continue
			}
			fv := v.(float64)
			*target = &fv
		default:
			return errors.New("stub: unsupported destination type")
		}
	}
	return nil
}

type stubRow struct {
	values []any
	err    error
}

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assign(dest, r.values)
}

// stubDB scripts what the repository's queries return.
type stubDB struct {
	rowsByQuery []*stubRows
	rowByQuery  []stubRow
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

func (s *stubDB) QueryRow(context.Context, string, ...any) pgx.Row {
	if len(s.rowByQuery) > 0 {
		next := s.rowByQuery[0]
		s.rowByQuery = s.rowByQuery[1:]
		return next
	}
	return stubRow{}
}

func (s *stubDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if s.execErr != nil {
		return pgconn.CommandTag{}, s.execErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func bg() context.Context { return context.Background() }

func sampleReviewRow() []any {
	return []any{"rev_1", "ord_1", "usr_1", "merchant", "mer_1", 5, "great", time.Now()}
}

func sampleTicketRow(resolvedAt any) []any {
	return []any{"tkt_1", "ord_1", "usr_1", "damaged", "open", "", "", "", time.Now(), resolvedAt}
}

// ------------------------------------------------------------------ SaveReview

func TestSaveReviewFailure(t *testing.T) {
	repo := reviewpg.New(&stubDB{execErr: errDB})
	r := reviewdomain.Review{ID: "rev_1", OrderID: "ord_1", RaterID: "usr_1",
		Subject: reviewdomain.SubjectMerchant, SubjectID: "mer_1", Rating: 5}
	if err := repo.SaveReview(bg(), r); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveReviewSucceeds(t *testing.T) {
	repo := reviewpg.New(&stubDB{})
	r := reviewdomain.Review{ID: "rev_1", OrderID: "ord_1", RaterID: "usr_1",
		Subject: reviewdomain.SubjectMerchant, SubjectID: "mer_1", Rating: 5}
	if err := repo.SaveReview(bg(), r); err != nil {
		t.Fatalf("SaveReview: %v", err)
	}
}

// ------------------------------------------------------------------ ExistsForRater

func TestExistsForRaterFailure(t *testing.T) {
	repo := reviewpg.New(&stubDB{rowByQuery: []stubRow{{err: errDB}}})
	if _, err := repo.ExistsForRater(bg(), "ord_1", "usr_1", reviewdomain.SubjectMerchant, "mer_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

func TestExistsForRaterSucceeds(t *testing.T) {
	repo := reviewpg.New(&stubDB{rowByQuery: []stubRow{{values: []any{true}}}})
	exists, err := repo.ExistsForRater(bg(), "ord_1", "usr_1", reviewdomain.SubjectMerchant, "mer_1")
	if err != nil || !exists {
		t.Fatalf("exists = %v, err = %v", exists, err)
	}
}

// ------------------------------------------------------------------ RatingFor

func TestRatingForFailure(t *testing.T) {
	repo := reviewpg.New(&stubDB{rowByQuery: []stubRow{{err: errDB}}})
	if _, err := repo.RatingFor(bg(), reviewdomain.SubjectMerchant, "mer_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

func TestRatingForWithNoReviews(t *testing.T) {
	repo := reviewpg.New(&stubDB{rowByQuery: []stubRow{{values: []any{0, nil}}}})
	rating, err := repo.RatingFor(bg(), reviewdomain.SubjectMerchant, "mer_1")
	if err != nil || rating.Count != 0 || rating.Average != 0 {
		t.Fatalf("rating = %+v, err = %v", rating, err)
	}
}

func TestRatingForWithReviews(t *testing.T) {
	repo := reviewpg.New(&stubDB{rowByQuery: []stubRow{{values: []any{2, 4.5}}}})
	rating, err := repo.RatingFor(bg(), reviewdomain.SubjectMerchant, "mer_1")
	if err != nil || rating.Count != 2 || rating.Average != 4.5 {
		t.Fatalf("rating = %+v, err = %v", rating, err)
	}
}

// ------------------------------------------------------------------ ReviewsFor

func TestReviewsForFailures(t *testing.T) {
	broken := reviewpg.New(&stubDB{queryErr: errDB})
	if _, err := broken.ReviewsFor(bg(), reviewdomain.SubjectMerchant, "mer_1", 10); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	midScan := reviewpg.New(&stubDB{rowsByQuery: []*stubRows{
		{rows: [][]any{{}}, scanErr: errDB},
	}})
	if _, err := midScan.ReviewsFor(bg(), reviewdomain.SubjectMerchant, "mer_1", 10); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	iterFails := reviewpg.New(&stubDB{rowsByQuery: []*stubRows{{iterErr: errDB}}})
	if _, err := iterFails.ReviewsFor(bg(), reviewdomain.SubjectMerchant, "mer_1", 10); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	ok := reviewpg.New(&stubDB{rowsByQuery: []*stubRows{{rows: [][]any{sampleReviewRow()}}}})
	got, err := ok.ReviewsFor(bg(), reviewdomain.SubjectMerchant, "mer_1", 10)
	if err != nil || len(got) != 1 || got[0].ID != "rev_1" {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
}

// ------------------------------------------------------------------ SaveTicket

func TestSaveTicketFailure(t *testing.T) {
	repo := reviewpg.New(&stubDB{execErr: errDB})
	tk := reviewdomain.Ticket{ID: "tkt_1", OrderID: "ord_1", RaisedBy: "usr_1", Subject: "damaged", Status: reviewdomain.TicketOpen}
	if err := repo.SaveTicket(bg(), tk); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveTicketWithAResolution(t *testing.T) {
	repo := reviewpg.New(&stubDB{})
	tk := reviewdomain.Ticket{
		ID: "tkt_1", OrderID: "ord_1", RaisedBy: "usr_1", Subject: "damaged",
		Status: reviewdomain.TicketResolved, Resolution: reviewdomain.ResolutionRejected,
		AgentID: "adm_1", ResolvedAt: time.Now(),
	}
	if err := repo.SaveTicket(bg(), tk); err != nil {
		t.Fatalf("SaveTicket: %v", err)
	}
}

// ------------------------------------------------------------------ Ticket

func TestTicketFailure(t *testing.T) {
	repo := reviewpg.New(&stubDB{rowByQuery: []stubRow{{err: errDB}}})
	if _, _, err := repo.Ticket(bg(), "tkt_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

func TestTicketNotFound(t *testing.T) {
	repo := reviewpg.New(&stubDB{rowByQuery: []stubRow{{err: pgx.ErrNoRows}}})
	_, found, err := repo.Ticket(bg(), "tkt_missing")
	if err != nil || found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
}

func TestTicketFound(t *testing.T) {
	repo := reviewpg.New(&stubDB{rowByQuery: []stubRow{{values: sampleTicketRow(nil)}}})
	tk, found, err := repo.Ticket(bg(), "tkt_1")
	if err != nil || !found || tk.ID != "tkt_1" {
		t.Fatalf("ticket = %+v, found = %v, err = %v", tk, found, err)
	}
}

func TestTicketFoundResolved(t *testing.T) {
	repo := reviewpg.New(&stubDB{rowByQuery: []stubRow{{values: sampleTicketRow(time.Now())}}})
	tk, found, err := repo.Ticket(bg(), "tkt_1")
	if err != nil || !found || tk.ResolvedAt.IsZero() {
		t.Fatalf("ticket = %+v, found = %v, err = %v", tk, found, err)
	}
}

// ------------------------------------------------------------------ TicketsRaisedBy / OpenTickets

func TestTicketsRaisedByFailures(t *testing.T) {
	broken := reviewpg.New(&stubDB{queryErr: errDB})
	if _, err := broken.TicketsRaisedBy(bg(), "usr_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	midScan := reviewpg.New(&stubDB{rowsByQuery: []*stubRows{{rows: [][]any{{}}, scanErr: errDB}}})
	if _, err := midScan.TicketsRaisedBy(bg(), "usr_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	iterFails := reviewpg.New(&stubDB{rowsByQuery: []*stubRows{{iterErr: errDB}}})
	if _, err := iterFails.TicketsRaisedBy(bg(), "usr_1"); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	ok := reviewpg.New(&stubDB{rowsByQuery: []*stubRows{{rows: [][]any{sampleTicketRow(nil)}}}})
	got, err := ok.TicketsRaisedBy(bg(), "usr_1")
	if err != nil || len(got) != 1 || got[0].ID != "tkt_1" {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
}

func TestOpenTicketsFailure(t *testing.T) {
	repo := reviewpg.New(&stubDB{queryErr: errDB})
	if _, err := repo.OpenTickets(bg(), 10); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenTicketsSucceeds(t *testing.T) {
	repo := reviewpg.New(&stubDB{rowsByQuery: []*stubRows{{rows: [][]any{sampleTicketRow(nil)}}}})
	got, err := repo.OpenTickets(bg(), 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
}

func TestTheProductionConstructor(t *testing.T) {
	if reviewpg.NewFromPool(nil) == nil {
		t.Fatal("NewFromPool built nothing")
	}
}
