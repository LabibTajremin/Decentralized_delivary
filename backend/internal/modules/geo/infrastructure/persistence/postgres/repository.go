// Package postgres implements the geo repository against PostGIS.
//
// Engine-specific SQL is confined to this package (05-architecture.md 2.4). The
// port it satisfies is engine-neutral; PostGIS is the supported path because
// ALG-01 and ALG-03 depend on a GiST index, which ADR 0003 records as a
// deliberate asymmetry rather than a portability claim we cannot keep.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
)

// Querier is the subset of pgxpool this repository needs. Depending on the
// interface rather than the concrete pool lets a test drive the repository with
// a transaction that rolls back.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository reads geometry from PostGIS.
type Repository struct {
	db Querier
}

// New builds a repository over a pool or transaction.
func New(db Querier) *Repository { return &Repository{db: db} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool) *Repository { return &Repository{db: pool} }

// scanPolygon turns a PostGIS ring, returned as ordered lat/lng pairs, into a
// domain polygon.
func scanPolygon(ctx context.Context, db Querier, sql string, args ...any) (domain.Polygon, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return domain.Polygon{}, fmt.Errorf("query boundary: %w", err)
	}
	defer rows.Close()

	var ring []domain.Coordinate
	for rows.Next() {
		var lat, lng float64
		if err := rows.Scan(&lat, &lng); err != nil {
			return domain.Polygon{}, fmt.Errorf("scan boundary vertex: %w", err)
		}
		c, err := domain.NewCoordinate(lat, lng)
		if err != nil {
			return domain.Polygon{}, fmt.Errorf("boundary vertex: %w", err)
		}
		ring = append(ring, c)
	}
	if err := rows.Err(); err != nil {
		return domain.Polygon{}, fmt.Errorf("read boundary: %w", err)
	}
	return domain.NewPolygon(ring)
}

// boundaryPointsSQL expands a polygon boundary into its vertices in ring order.
const boundaryPointsSQL = `
SELECT ST_Y(geom) AS lat, ST_X(geom) AS lng
FROM (
    SELECT (ST_DumpPoints(boundary::geometry)).geom AS geom,
           (ST_DumpPoints(boundary::geometry)).path AS path
    FROM %s
    WHERE code = $1
) pts
ORDER BY path`

// Divisions returns every division with its boundary.
func (r *Repository) Divisions(ctx context.Context) ([]domain.Division, error) {
	rows, err := r.db.Query(ctx, `SELECT code, name FROM geo_divisions ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("list divisions: %w", err)
	}
	type row struct {
		code, name string
	}
	var meta []row
	for rows.Next() {
		var rw row
		if err := rows.Scan(&rw.code, &rw.name); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan division: %w", err)
		}
		meta = append(meta, rw)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read divisions: %w", err)
	}

	out := make([]domain.Division, 0, len(meta))
	for _, m := range meta {
		boundary, err := scanPolygon(ctx, r.db, fmt.Sprintf(boundaryPointsSQL, "geo_divisions"), m.code)
		if err != nil {
			return nil, fmt.Errorf("division %s boundary: %w", m.code, err)
		}
		d, err := domain.NewDivision(domain.DivisionCode(m.code), m.name, boundary)
		if err != nil {
			return nil, fmt.Errorf("division %s: %w", m.code, err)
		}
		out = append(out, d)
	}
	return out, nil
}

// DivisionContaining implements ALG-03 with ST_Contains over a GiST index.
//
// The boundary test runs in the database rather than by loading polygons and
// testing in Go: division geometry is large, and shipping it to the process on
// every address lookup would dominate the cost of the query.
func (r *Repository) DivisionContaining(ctx context.Context, c domain.Coordinate) (domain.Division, error) {
	var code, name string
	err := r.db.QueryRow(ctx, `
		SELECT code, name
		FROM geo_divisions
		WHERE ST_Contains(boundary::geometry, ST_SetSRID(ST_MakePoint($1, $2), 4326))
		LIMIT 1`, c.Lng(), c.Lat()).Scan(&code, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Division{}, domain.ErrUnknownDivision
	}
	if err != nil {
		return domain.Division{}, fmt.Errorf("division containing point: %w", err)
	}

	boundary, err := scanPolygon(ctx, r.db, fmt.Sprintf(boundaryPointsSQL, "geo_divisions"), code)
	if err != nil {
		return domain.Division{}, fmt.Errorf("division %s boundary: %w", code, err)
	}
	return domain.NewDivision(domain.DivisionCode(code), name, boundary)
}

// AreaContaining returns the finest administrative area for a point.
//
// Areas may abut, so the smallest containing area wins: ordering by area puts
// a neighbourhood ahead of the upazila that encloses it, which is what config
// resolution wants (Appendix B resolves area before district).
func (r *Repository) AreaContaining(ctx context.Context, c domain.Coordinate) (domain.Area, error) {
	var code, name, district, division string
	var lat, lng float64
	err := r.db.QueryRow(ctx, `
		SELECT code, name, district_code, division_code, ST_Y(centre::geometry), ST_X(centre::geometry)
		FROM geo_areas
		WHERE ST_Contains(boundary::geometry, ST_SetSRID(ST_MakePoint($1, $2), 4326))
		ORDER BY ST_Area(boundary::geometry) ASC
		LIMIT 1`, c.Lng(), c.Lat()).Scan(&code, &name, &district, &division, &lat, &lng)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Area{}, domain.ErrUnknownDivision
	}
	if err != nil {
		return domain.Area{}, fmt.Errorf("area containing point: %w", err)
	}

	centre, err := domain.NewCoordinate(lat, lng)
	if err != nil {
		return domain.Area{}, fmt.Errorf("area %s centre: %w", code, err)
	}
	return domain.NewArea(code, name, district, domain.DivisionCode(division), centre)
}

// MerchantsWithinRadius implements ALG-01.
//
// ST_DWithin on geography uses metres and is index-assisted, giving
// O(log n + k). The division predicate is part of the WHERE clause, not a
// post-filter, so the D3 ceiling cannot be bypassed by a caller that forgets to
// check. Results are ordered by true distance, and the distance is returned so
// pricing never recomputes it.
//
// use_spheroid is false on purpose. PostGIS defaults to the WGS84 ellipsoid,
// which is ~0.4% different from the sphere that domain.Coordinate.DistanceTo
// uses — 1.7 m over 440 m. Two different answers for one distance is exactly
// what 2.9 forbids, and the integration tests assert the two agree to within a
// metre. Over a 25 km maximum radius the sphere is off by tens of metres, which
// is far inside the kilometre granularity of the fee bands (ALG-05), so
// agreement is worth more here than absolute accuracy.
func (r *Repository) MerchantsWithinRadius(
	ctx context.Context,
	centre domain.Coordinate,
	radius domain.Distance,
	division domain.DivisionCode,
	limit int,
) ([]ports.MerchantLocation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT merchant_id,
		       ST_Y(location::geometry) AS lat,
		       ST_X(location::geometry) AS lng,
		       ST_Distance(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, false) AS distance_m
		FROM geo_merchant_locations
		WHERE is_active
		  AND division_code = $3
		  AND ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $4, false)
		ORDER BY distance_m ASC
		LIMIT $5`,
		centre.Lng(), centre.Lat(), string(division), radius.Metres(), limit)
	if err != nil {
		return nil, fmt.Errorf("merchants within radius: %w", err)
	}
	defer rows.Close()

	out := make([]ports.MerchantLocation, 0, limit)
	for rows.Next() {
		var id string
		var lat, lng, distance float64
		if err := rows.Scan(&id, &lat, &lng, &distance); err != nil {
			return nil, fmt.Errorf("scan merchant: %w", err)
		}
		loc, err := domain.NewCoordinate(lat, lng)
		if err != nil {
			return nil, fmt.Errorf("merchant %s location: %w", id, err)
		}
		out = append(out, ports.MerchantLocation{
			MerchantID: id,
			Location:   loc,
			Distance:   domain.Distance(distance),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read merchants: %w", err)
	}
	return out, nil
}

// CountMerchantsWithinRadius returns only the count, for the expansion decision.
func (r *Repository) CountMerchantsWithinRadius(
	ctx context.Context,
	centre domain.Coordinate,
	radius domain.Distance,
	division domain.DivisionCode,
) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `
		SELECT count(*)
		FROM geo_merchant_locations
		WHERE is_active
		  AND division_code = $3
		  AND ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $4, false)`,
		centre.Lng(), centre.Lat(), string(division), radius.Metres()).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count merchants within radius: %w", err)
	}
	return n, nil
}
