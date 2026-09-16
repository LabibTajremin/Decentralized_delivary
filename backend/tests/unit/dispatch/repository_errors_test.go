package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
	dispatchpg "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/infrastructure/persistence/postgres"
)

// The integration tests prove this SQL against a real PostGIS. These drive the
// failures a healthy database never produces — a connection dropped mid-scan, a
// row the database should not have been able to store. Repository depends on
// the Querier interface precisely so these paths are reachable without breaking
// a real server, and they matter: a rider whose feed silently swallowed a
// dropped connection would be shown "no work nearby" in a city full of it.

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

// stubDB scripts what the repository's queries return.
type stubDB struct {
	rowsByQuery []*stubRows
	row         stubRow
	rowByQuery  []stubRow
	queryErr    error
	execErr     error

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
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func ctx() context.Context { return context.Background() }

// ------------------------------------------------------------------ partners

// A partner who is not there and a database that is not there are different
// answers. Returning "not a partner" for a dropped connection would sign a
// rider out of their own job mid-shift.
func TestAPartnerLookupTellsMissingFromBroken(t *testing.T) {
	broken := dispatchpg.New(&stubDB{row: stubRow{err: errDB}})
	if _, found, err := broken.PartnerOfUser(ctx(), "usr_1"); found || !errors.Is(err, errDB) {
		t.Fatalf("found = %v, err = %v", found, err)
	}

	missing := dispatchpg.New(&stubDB{row: stubRow{err: pgx.ErrNoRows}})
	if _, found, err := missing.PartnerOfUser(ctx(), "usr_1"); found || err != nil {
		t.Fatalf("found = %v, err = %v", found, err)
	}
}

func TestTheCandidatePoolFailures(t *testing.T) {
	at := domain.Place{Lat: 23.746, Lng: 90.375}

	broken := dispatchpg.New(&stubDB{queryErr: errDB})
	if _, err := broken.AvailableWithin(ctx(), at, 3000, domain.BandShort, 50); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	// A row that cannot be read is not an empty pool: a job would be left on
	// the board with riders standing next to it.
	mid := dispatchpg.New(&stubDB{rowsByQuery: []*stubRows{
		{rows: [][]any{{}}, scanErr: errDB},
	}})
	if _, err := mid.AvailableWithin(ctx(), at, 3000, domain.BandShort, 50); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

// ------------------------------------------------------------------ jobs

func TestAJobLookupTellsMissingFromBroken(t *testing.T) {
	broken := dispatchpg.New(&stubDB{row: stubRow{err: errDB}})
	if _, found, err := broken.JobForOrder(ctx(), "ord_1"); found || !errors.Is(err, errDB) {
		t.Fatalf("found = %v, err = %v", found, err)
	}

	missing := dispatchpg.New(&stubDB{row: stubRow{err: pgx.ErrNoRows}})
	if _, found, err := missing.JobForOrder(ctx(), "ord_1"); found || err != nil {
		t.Fatalf("found = %v, err = %v", found, err)
	}
}

func TestListingJobsFailures(t *testing.T) {
	filter := ports.JobFilter{PartnerID: "prt_1", Limit: 10}

	count := dispatchpg.New(&stubDB{row: stubRow{err: errDB}})
	if _, _, err := count.Jobs(ctx(), filter); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	// The count succeeded and the page did not, which is the shape that would
	// otherwise show a rider "3 deliveries" above an empty list.
	page := dispatchpg.New(&stubDB{row: stubRow{values: []any{2}}, queryErr: errDB})
	if _, _, err := page.Jobs(ctx(), filter); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	mid := dispatchpg.New(&stubDB{
		row:         stubRow{values: []any{1}},
		rowsByQuery: []*stubRows{{rows: [][]any{{}}, scanErr: errDB}},
	})
	if _, _, err := mid.Jobs(ctx(), filter); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	// A filter with no partner in it must not return everybody's jobs. The
	// count comes back zero and nothing is read at all.
	empty := dispatchpg.New(&stubDB{row: stubRow{values: []any{0}}})
	jobs, total, err := empty.Jobs(ctx(), ports.JobFilter{Limit: 10})
	if err != nil || total != 0 || jobs != nil {
		t.Fatalf("jobs = %+v, total = %d, err = %v", jobs, total, err)
	}
}

func TestSavingAJobFailure(t *testing.T) {
	repo := dispatchpg.New(&stubDB{execErr: errDB})
	job := domain.Job{ID: "job_1", Status: domain.JobWaiting}
	if err := repo.SaveJob(ctx(), job, domain.JobWaiting); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

func TestTheFeedQueryFailures(t *testing.T) {
	at := domain.Place{Lat: 23.746, Lng: 90.375}

	broken := dispatchpg.New(&stubDB{queryErr: errDB})
	if _, err := broken.WaitingNear(ctx(), at, 3000, 20); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}

	mid := dispatchpg.New(&stubDB{rowsByQuery: []*stubRows{
		{rows: [][]any{{}}, scanErr: errDB},
	}})
	if _, err := mid.WaitingNear(ctx(), at, 3000, 20); !errors.Is(err, errDB) {
		t.Fatalf("err = %v", err)
	}
}

func TestTheSweeperQueryFailures(t *testing.T) {
	for name, run := range map[string]func(*dispatchpg.Repository) error{
		"lapsed offers": func(r *dispatchpg.Repository) error {
			_, err := r.LapsedOffers(ctx(), 50)
			return err
		},
		"the board": func(r *dispatchpg.Repository) error {
			_, err := r.WaitingJobs(ctx(), 50)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			broken := dispatchpg.New(&stubDB{queryErr: errDB})
			if err := run(broken); !errors.Is(err, errDB) {
				t.Fatalf("err = %v", err)
			}
			mid := dispatchpg.New(&stubDB{rowsByQuery: []*stubRows{
				{rows: [][]any{{}}, scanErr: errDB},
			}})
			if err := run(mid); !errors.Is(err, errDB) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// NewFromPool is what the running binary uses; New is what these tests use.
// Building one proves the two constructors have not drifted apart.
func TestTheProductionConstructor(t *testing.T) {
	if dispatchpg.NewFromPool(nil) == nil {
		t.Fatal("NewFromPool built nothing")
	}
}
