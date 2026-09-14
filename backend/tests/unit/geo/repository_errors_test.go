package geo

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
	geopg "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/infrastructure/persistence/postgres"
)

// The integration tests prove the SQL is right against a real PostGIS. These
// tests drive the failure branches that a healthy database never produces —
// a dropped connection mid-scan, or a row the database should not have been
// able to store. Repository depends on the Querier interface precisely so
// these paths are reachable without breaking a real server.

var errQuery = errors.New("connection reset by peer")

// stubRow returns a fixed error or values from QueryRow.
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

// stubRows is a canned pgx.Rows.
type stubRows struct {
	rows    [][]any
	idx     int
	scanErr error
	iterErr error
	closed  bool
}

func (s *stubRows) Close()                                       { s.closed = true }
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

// assign copies canned values into scan destinations.
func assign(dest []any, values []any) error {
	if len(values) < len(dest) {
		return errors.New("stub: not enough values")
	}
	for i, d := range dest {
		switch target := d.(type) {
		case *string:
			*target = values[i].(string)
		case *float64:
			*target = values[i].(float64)
		case *int:
			*target = values[i].(int)
		default:
			return errors.New("stub: unsupported destination type")
		}
	}
	return nil
}

// stubQuerier scripts Query and QueryRow results in call order.
type stubQuerier struct {
	queryResults []*stubRows
	queryErrs    []error
	rowResults   []stubRow
	queryCalls   int
	rowCalls     int

	// execErr is what Exec returns; the write paths have no rows to script.
	execErr error
}

func (q *stubQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, q.execErr
}

func (q *stubQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	i := q.queryCalls
	q.queryCalls++
	if i < len(q.queryErrs) && q.queryErrs[i] != nil {
		return nil, q.queryErrs[i]
	}
	if i < len(q.queryResults) {
		return q.queryResults[i], nil
	}
	return &stubRows{}, nil
}

func (q *stubQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	i := q.rowCalls
	q.rowCalls++
	if i < len(q.rowResults) {
		return q.rowResults[i]
	}
	return stubRow{err: pgx.ErrNoRows}
}

func ctx() context.Context { return context.Background() }

func TestDivisionsSurfacesQueryFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{queryErrs: []error{errQuery}})
	if _, err := repo.Divisions(ctx()); !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the query failure wrapped", err)
	}
}

func TestDivisionsSurfacesScanFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{
		queryResults: []*stubRows{{rows: [][]any{{"DHA", "Dhaka"}}, scanErr: errQuery}},
	})
	if _, err := repo.Divisions(ctx()); !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the scan failure wrapped", err)
	}
}

func TestDivisionsSurfacesIterationFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{
		queryResults: []*stubRows{{iterErr: errQuery}},
	})
	if _, err := repo.Divisions(ctx()); !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the iteration failure wrapped", err)
	}
}

func TestDivisionsRejectsUnusableBoundary(t *testing.T) {
	// The metadata query succeeds, then the boundary comes back degenerate.
	repo := geopg.New(&stubQuerier{
		queryResults: []*stubRows{
			{rows: [][]any{{"DHA", "Dhaka"}}},
			{rows: [][]any{{23.0, 90.0}}}, // one vertex is not a polygon
		},
	})
	if _, err := repo.Divisions(ctx()); !errors.Is(err, domain.ErrDegeneratePolygon) {
		t.Errorf("error = %v, want ErrDegeneratePolygon", err)
	}
}

func TestDivisionsRejectsImpossibleVertex(t *testing.T) {
	repo := geopg.New(&stubQuerier{
		queryResults: []*stubRows{
			{rows: [][]any{{"DHA", "Dhaka"}}},
			{rows: [][]any{{999.0, 90.0}}}, // a latitude the database should never hold
		},
	})
	if _, err := repo.Divisions(ctx()); !errors.Is(err, domain.ErrLatitudeOutOfRange) {
		t.Errorf("error = %v, want the bad vertex rejected", err)
	}
}

func TestDivisionsRejectsUnknownDivisionCode(t *testing.T) {
	repo := geopg.New(&stubQuerier{
		queryResults: []*stubRows{
			{rows: [][]any{{"ZZZ", "Atlantis"}}},
			{rows: [][]any{{0.0, 0.0}, {0.0, 1.0}, {1.0, 1.0}}},
		},
	})
	if _, err := repo.Divisions(ctx()); !errors.Is(err, domain.ErrEmptyCode) {
		t.Errorf("error = %v, want an unknown division code rejected", err)
	}
}

func TestDivisionContainingSurfacesQueryFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{rowResults: []stubRow{{err: errQuery}}})
	if _, err := repo.DivisionContaining(ctx(), domain.MustCoordinate(23.8, 90.4)); !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the query failure wrapped", err)
	}
}

func TestDivisionContainingSurfacesBoundaryFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{
		rowResults: []stubRow{{values: []any{"DHA", "Dhaka"}}},
		queryErrs:  []error{errQuery},
	})
	if _, err := repo.DivisionContaining(ctx(), domain.MustCoordinate(23.8, 90.4)); !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the boundary query failure wrapped", err)
	}
}

func TestAreaContainingSurfacesQueryFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{rowResults: []stubRow{{err: errQuery}}})
	if _, err := repo.AreaContaining(ctx(), domain.MustCoordinate(23.8, 90.4)); !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the query failure wrapped", err)
	}
}

func TestAreaContainingRejectsImpossibleCentre(t *testing.T) {
	repo := geopg.New(&stubQuerier{
		rowResults: []stubRow{{values: []any{"DHN", "Dhanmondi", "DHK", "DHA", 999.0, 90.0}}},
	})
	if _, err := repo.AreaContaining(ctx(), domain.MustCoordinate(23.8, 90.4)); !errors.Is(err, domain.ErrLatitudeOutOfRange) {
		t.Errorf("error = %v, want the bad centre rejected", err)
	}
}

func TestMerchantsWithinRadiusSurfacesQueryFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{queryErrs: []error{errQuery}})
	_, err := repo.MerchantsWithinRadius(ctx(), domain.MustCoordinate(23.8, 90.4),
		domain.KilometresFrom(5), domain.DivisionDhaka, 10)
	if !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the query failure wrapped", err)
	}
}

func TestMerchantsWithinRadiusSurfacesScanFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{
		queryResults: []*stubRows{{rows: [][]any{{"MER-1", 23.8, 90.4, 100.0}}, scanErr: errQuery}},
	})
	_, err := repo.MerchantsWithinRadius(ctx(), domain.MustCoordinate(23.8, 90.4),
		domain.KilometresFrom(5), domain.DivisionDhaka, 10)
	if !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the scan failure wrapped", err)
	}
}

func TestMerchantsWithinRadiusRejectsImpossibleLocation(t *testing.T) {
	repo := geopg.New(&stubQuerier{
		queryResults: []*stubRows{{rows: [][]any{{"MER-1", 999.0, 90.4, 100.0}}}},
	})
	_, err := repo.MerchantsWithinRadius(ctx(), domain.MustCoordinate(23.8, 90.4),
		domain.KilometresFrom(5), domain.DivisionDhaka, 10)
	if !errors.Is(err, domain.ErrLatitudeOutOfRange) {
		t.Errorf("error = %v, want the impossible location rejected", err)
	}
}

func TestMerchantsWithinRadiusSurfacesIterationFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{queryResults: []*stubRows{{iterErr: errQuery}}})
	_, err := repo.MerchantsWithinRadius(ctx(), domain.MustCoordinate(23.8, 90.4),
		domain.KilometresFrom(5), domain.DivisionDhaka, 10)
	if !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the iteration failure wrapped", err)
	}
}

func TestCountMerchantsWithinRadiusSurfacesFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{rowResults: []stubRow{{err: errQuery}}})
	_, err := repo.CountMerchantsWithinRadius(ctx(), domain.MustCoordinate(23.8, 90.4),
		domain.KilometresFrom(5), domain.DivisionDhaka)
	if !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the count failure wrapped", err)
	}
}

// TestNewFromPoolIsWired guards the production constructor, which the E2E
// wiring uses and no other test touches.
func TestNewFromPoolIsWired(t *testing.T) {
	if repo := geopg.NewFromPool(nil); repo == nil {
		t.Error("NewFromPool must return a repository")
	}
}

func TestBoundaryScanSurfacesIterationFailure(t *testing.T) {
	// Metadata reads fine, then the connection drops while streaming vertices.
	repo := geopg.New(&stubQuerier{
		queryResults: []*stubRows{
			{rows: [][]any{{"DHA", "Dhaka"}}},
			{iterErr: errQuery},
		},
	})
	if _, err := repo.Divisions(ctx()); !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the boundary iteration failure wrapped", err)
	}
}

func TestBoundaryScanSurfacesVertexScanFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{
		queryResults: []*stubRows{
			{rows: [][]any{{"DHA", "Dhaka"}}},
			{rows: [][]any{{23.0, 90.0}}, scanErr: errQuery},
		},
	})
	if _, err := repo.Divisions(ctx()); !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the vertex scan failure wrapped", err)
	}
}

// TestPublishingAMerchantSurfacesAWriteFailure. A publish that failed quietly
// would leave an approved shop with no point in the index — visible in the
// admin console and unfindable by every customer.
func TestPublishingAMerchantSurfacesAWriteFailure(t *testing.T) {
	repo := geopg.New(&stubQuerier{execErr: errQuery})

	err := repo.UpsertMerchantLocation(context.Background(), ports.MerchantPoint{
		MerchantID: "mch_1",
		Location:   domain.MustCoordinate(23.7461, 90.3742),
		Division:   domain.DivisionDhaka,
		AreaCode:   "DHN",
	})
	if !errors.Is(err, errQuery) {
		t.Errorf("error = %v, want the failure wrapped", err)
	}

	if err := repo.DeleteMerchantLocation(context.Background(), "mch_1"); !errors.Is(err, errQuery) {
		t.Errorf("delete: error = %v, want the failure wrapped", err)
	}
}
