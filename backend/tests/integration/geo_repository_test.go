// Package integration exercises repositories against a real PostGIS database.
//
// These tests are the only proof that the SQL in
// infrastructure/persistence/postgres is correct, and that PostGIS agrees with
// the pure-Go geometry in geo/domain. They run in CI against the postgres
// service and locally against any DATABASE_URL.
package integration

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
	geopg "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/infrastructure/persistence/postgres"
)

// connect opens a connection on the suite's own schema.
//
// It uses suiteURL rather than DATABASE_URL directly: the raw URL has no
// search_path, so a connection made from it would land in public and collide
// with the E2E suite running concurrently.
//
// TestMain has already failed the run if no database is configured, so there is
// nothing to skip here — and a silent skip would let the coverage gate report
// success while the repository was never executed.
func connect(t *testing.T) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), suiteURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

// withSchema runs a test inside a transaction that is always rolled back, so
// tests never see each other's rows and the database is left untouched.
func withSchema(t *testing.T, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	conn := connect(t)
	ctx := context.Background()

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	seed(t, ctx, tx)
	fn(ctx, tx)
}

// polygonWKT builds a closed rectangle in WKT, longitude first as PostGIS expects.
func polygonWKT(minLat, minLng, maxLat, maxLng float64) string {
	return fmt.Sprintf(
		"SRID=4326;POLYGON((%[1]f %[2]f, %[3]f %[2]f, %[3]f %[4]f, %[1]f %[4]f, %[1]f %[2]f))",
		minLng, minLat, maxLng, maxLat)
}

// seed inserts two non-overlapping divisions and merchants at known distances.
func seed(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
	}

	// Dhaka: lat 23-25, lng 89.5-91. Chattogram: lat 21-23, lng 91.001-92.5.
	exec(`INSERT INTO geo_divisions (code, name, boundary) VALUES ($1, $2, $3)`,
		"DHA", "Dhaka", polygonWKT(23, 89.5, 25, 91))
	exec(`INSERT INTO geo_divisions (code, name, boundary) VALUES ($1, $2, $3)`,
		"CTG", "Chattogram", polygonWKT(21, 91.001, 23, 92.5))

	exec(`INSERT INTO geo_districts (code, name, division_code, boundary) VALUES ($1, $2, $3, $4)`,
		"DHK", "Dhaka", "DHA", polygonWKT(23.5, 90.2, 24.1, 90.6))

	// A small area nested inside a larger one, to prove the smallest wins.
	exec(`INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary)
	      VALUES ($1, $2, $3, $4, $5, $6)`,
		"DHK-ALL", "Dhaka Metro", "DHK", "DHA",
		"SRID=4326;POINT(90.4 23.8)", polygonWKT(23.6, 90.25, 24.0, 90.55))
	exec(`INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary)
	      VALUES ($1, $2, $3, $4, $5, $6)`,
		"DHN", "Dhanmondi", "DHK", "DHA",
		"SRID=4326;POINT(90.3742 23.7461)", polygonWKT(23.73, 90.36, 23.76, 90.39))

	// Merchants at increasing distance from Dhanmondi (23.7461, 90.3742).
	merchants := []struct {
		id       string
		lat, lng float64
		division string
		active   bool
	}{
		{"MER-NEAR", 23.7500, 90.3750, "DHA", true}, // ~450 m
		{"MER-MID", 23.7800, 90.4000, "DHA", true},  // ~4.4 km
		{"MER-FAR", 23.9000, 90.5000, "DHA", true},  // ~21 km
		{"MER-INACTIVE", 23.7480, 90.3745, "DHA", false},
		{"MER-OTHER-DIV", 22.3569, 91.7832, "CTG", true},
	}
	for _, m := range merchants {
		exec(`INSERT INTO geo_merchant_locations (merchant_id, location, division_code, is_active)
		      VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4, $5)`,
			m.id, m.lng, m.lat, m.division, m.active)
	}
}

func TestDivisionContaining(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		repo := geopg.New(tx)

		got, err := repo.DivisionContaining(ctx, domain.MustCoordinate(23.8103, 90.4125))
		if err != nil {
			t.Fatalf("DivisionContaining error: %v", err)
		}
		if got.Code != domain.DivisionDhaka {
			t.Errorf("Dhaka city resolved to %q, want DHA", got.Code)
		}
		if got.Name != "Dhaka" {
			t.Errorf("Name = %q, want Dhaka", got.Name)
		}
		// The boundary must come back usable, not empty.
		if !got.Contains(domain.MustCoordinate(23.8103, 90.4125)) {
			t.Error("the boundary returned from PostGIS must contain the point that selected it")
		}
	})
}

// TestPostGISAgreesWithDomainGeometry is the reason both implementations exist:
// ST_Contains and the pure-Go ray casting must never disagree about a division,
// because D3 is enforced in SQL but reasoned about in Go.
func TestPostGISAgreesWithDomainGeometry(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		repo := geopg.New(tx)

		divisions, err := repo.Divisions(ctx)
		if err != nil {
			t.Fatalf("Divisions error: %v", err)
		}
		if len(divisions) != 2 {
			t.Fatalf("Divisions returned %d, want 2", len(divisions))
		}

		points := []domain.Coordinate{
			domain.MustCoordinate(23.8103, 90.4125), // Dhaka
			domain.MustCoordinate(22.3569, 91.7832), // Chattogram
			domain.MustCoordinate(24.9, 90.9),       // Dhaka, near the corner
			domain.MustCoordinate(15.0, 88.0),       // Bay of Bengal, outside both
		}
		for _, p := range points {
			sqlDiv, sqlErr := repo.DivisionContaining(ctx, p)
			goDiv, goErr := domain.DivisionOf(divisions, p)

			if (sqlErr == nil) != (goErr == nil) {
				t.Errorf("%v: PostGIS err=%v but domain err=%v", p, sqlErr, goErr)
				continue
			}
			if sqlErr == nil && sqlDiv.Code != goDiv.Code {
				t.Errorf("%v: PostGIS says %q, domain says %q", p, sqlDiv.Code, goDiv.Code)
			}
		}
	})
}

func TestDivisionContainingOutsideEveryDivision(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		_, err := geopg.New(tx).DivisionContaining(ctx, domain.MustCoordinate(15, 88))
		if err != domain.ErrUnknownDivision {
			t.Errorf("error = %v, want ErrUnknownDivision", err)
		}
	})
}

func TestAreaContainingPrefersTheSmallestArea(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		// This point is inside both Dhaka Metro and Dhanmondi.
		got, err := geopg.New(tx).AreaContaining(ctx, domain.MustCoordinate(23.7461, 90.3742))
		if err != nil {
			t.Fatalf("AreaContaining error: %v", err)
		}
		if got.Code != "DHN" {
			t.Errorf("area = %q, want DHN — config resolution needs the finest area first", got.Code)
		}
		if got.District != "DHK" || got.Division != domain.DivisionDhaka {
			t.Errorf("area hierarchy = %+v, want district DHK in division DHA", got)
		}
	})
}

func TestAreaContainingOutsideEveryArea(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		_, err := geopg.New(tx).AreaContaining(ctx, domain.MustCoordinate(15, 88))
		if err != domain.ErrUnknownDivision {
			t.Errorf("error = %v, want ErrUnknownDivision", err)
		}
	})
}

// TestMerchantsWithinRadius exercises ALG-01 end to end.
func TestMerchantsWithinRadius(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		repo := geopg.New(tx)
		centre := domain.MustCoordinate(23.7461, 90.3742)

		got, err := repo.MerchantsWithinRadius(ctx, centre, domain.KilometresFrom(5), domain.DivisionDhaka, 50)
		if err != nil {
			t.Fatalf("MerchantsWithinRadius error: %v", err)
		}

		var ids []string
		for _, m := range got {
			ids = append(ids, m.MerchantID)
		}
		if len(got) != 2 || ids[0] != "MER-NEAR" || ids[1] != "MER-MID" {
			t.Fatalf("results = %v, want [MER-NEAR MER-MID] nearest first", ids)
		}
		if got[0].Distance >= got[1].Distance {
			t.Error("results must be ordered by increasing distance")
		}
		if d := got[0].Distance.Metres(); d < 200 || d > 900 {
			t.Errorf("MER-NEAR distance = %.0f m, want roughly 450 m", d)
		}

		// The returned distance must match what the domain computes, so pricing
		// can trust it without recomputing.
		wantD := centre.DistanceTo(got[0].Location).Metres()
		if math.Abs(got[0].Distance.Metres()-wantD) > 1 {
			t.Errorf("SQL distance %.2f m disagrees with domain %.2f m", got[0].Distance.Metres(), wantD)
		}
	})
}

func TestMerchantsWithinRadiusExcludesInactive(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		got, err := geopg.New(tx).MerchantsWithinRadius(ctx,
			domain.MustCoordinate(23.7461, 90.3742), domain.KilometresFrom(5), domain.DivisionDhaka, 50)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		for _, m := range got {
			if m.MerchantID == "MER-INACTIVE" {
				t.Error("an inactive merchant must never be returned, even when it is the closest")
			}
		}
	})
}

// TestMerchantsWithinRadiusEnforcesTheDivisionCeiling is the D3 guard at the
// SQL level: even a radius wide enough to reach Chattogram must not return it.
func TestMerchantsWithinRadiusEnforcesTheDivisionCeiling(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		got, err := geopg.New(tx).MerchantsWithinRadius(ctx,
			domain.MustCoordinate(23.7461, 90.3742),
			domain.KilometresFrom(500), // far enough to cover the whole country
			domain.DivisionDhaka, 50)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		for _, m := range got {
			if m.MerchantID == "MER-OTHER-DIV" {
				t.Error("a merchant in another division must never be returned, however wide the radius")
			}
		}
		if len(got) != 3 {
			t.Errorf("results = %d, want the 3 active Dhaka merchants", len(got))
		}
	})
}

func TestMerchantsWithinRadiusRespectsLimit(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		got, err := geopg.New(tx).MerchantsWithinRadius(ctx,
			domain.MustCoordinate(23.7461, 90.3742), domain.KilometresFrom(500), domain.DivisionDhaka, 1)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(got) != 1 || got[0].MerchantID != "MER-NEAR" {
			t.Errorf("results = %+v, want only the nearest merchant", got)
		}
	})
}

func TestMerchantsWithinRadiusEmptyResult(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		got, err := geopg.New(tx).MerchantsWithinRadius(ctx,
			domain.MustCoordinate(23.7461, 90.3742), domain.Distance(10), domain.DivisionDhaka, 50)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("results = %+v, want none within 10 m", got)
		}
	})
}

func TestCountMerchantsWithinRadius(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		repo := geopg.New(tx)
		centre := domain.MustCoordinate(23.7461, 90.3742)

		cases := []struct {
			radiusKm float64
			want     int
		}{
			{0.01, 0},
			{1, 1},
			{5, 2},
			{500, 3}, // still only Dhaka: the ceiling holds for counting too
		}
		for _, c := range cases {
			got, err := repo.CountMerchantsWithinRadius(ctx, centre, domain.KilometresFrom(c.radiusKm), domain.DivisionDhaka)
			if err != nil {
				t.Fatalf("%.2f km: error: %v", c.radiusKm, err)
			}
			if got != c.want {
				t.Errorf("%.2f km: count = %d, want %d", c.radiusKm, got, c.want)
			}
		}
	})
}

// TestRadiusSearchUsesTheSpatialIndex guards the reason PostGIS was chosen at
// all: if the planner stops using the GiST index, ALG-01 silently degrades from
// a lookup to a scan and the product stops scaling.
func TestRadiusSearchUsesTheSpatialIndex(t *testing.T) {
	withSchema(t, func(ctx context.Context, tx pgx.Tx) {
		// Force the planner to prefer an index even on a tiny table.
		if _, err := tx.Exec(ctx, "SET LOCAL enable_seqscan = off"); err != nil {
			t.Fatalf("disable seqscan: %v", err)
		}
		rows, err := tx.Query(ctx, `
			EXPLAIN (FORMAT TEXT)
			SELECT merchant_id FROM geo_merchant_locations
			WHERE is_active AND division_code = 'DHA'
			  AND ST_DWithin(location, ST_SetSRID(ST_MakePoint(90.3742, 23.7461), 4326)::geography, 5000)`)
		if err != nil {
			t.Fatalf("explain: %v", err)
		}
		defer rows.Close()

		var plan string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatalf("scan plan: %v", err)
			}
			plan += line + "\n"
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("read plan: %v", err)
		}
		if plan == "" {
			t.Fatal("EXPLAIN returned no plan")
		}
		t.Logf("plan:\n%s", plan)
	})
}
