package load

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	geodomain "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
	geopg "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/infrastructure/persistence/postgres"
)

// The centre every search in this file runs from: Dhanmondi, the same point
// the rest of the suite uses.
const (
	centreLat = 23.7455
	centreLng = 90.3738
)

// spread is how far either side of the centre the fixture scatters shops, in
// degrees — about 20 km, so a search at the smallest radius finds a handful
// and one at the ceiling finds most of them. Uniform over a box rather than
// clustered: a cluster would flatter the index by making every query either
// hit everything or nothing.
const spread = 0.18

// expansionRadii are ALG-02's steps, smallest first. The real step list comes
// from configuration; these are the shape of it, and what matters here is that
// an expansion is several queries rather than one.
var expansionRadii = []float64{1000, 2000, 3000, 5000, 8000, 12000, 25000}

// seedMerchantsAtScale fills geo_merchant_locations and merchants with n
// approved, open, searchable shops around the centre.
//
// Written as two multi-row INSERTs rather than through the registration API:
// a shop registered properly costs an OTP, a document upload each and an
// admin approval, and two thousand of those is twenty minutes of HTTP to
// produce a fixture whose only job is to be numerous. What the API path does
// correctly is proven by the merchant suite; what this needs is rows.
func seedMerchantsAtScale(t *testing.T, n int) {
	t.Helper()
	merchantFixture.Do(func() { fixtureErr = writeMerchantFixture(n) })
	if fixtureErr != nil {
		t.Fatalf("merchant fixture: %v", fixtureErr)
	}
	analyse(t, "geo_merchant_locations")
	analyse(t, "merchants")
}

// merchantFixture makes the fixture once per test binary. Four tests in this
// file want the same thousands of rows, and writing them four times would be
// three-quarters of the suite's runtime spent on INSERTs.
// It also has to report its failure to every caller rather than only to the
// test that happened to run first: a sync.Once that fails inside t.Fatalf
// leaves the others passing against an empty table, which is the worst of
// both worlds.
var (
	merchantFixture sync.Once
	fixtureErr      error
)

func writeMerchantFixture(n int) error {
	ctx := context.Background()

	// A fixed seed, so a plan that changes between runs changed because the
	// code changed.
	random := rand.New(rand.NewPCG(20260921, 20200519))

	locations := make([]string, 0, n)
	merchants := make([]string, 0, n)
	hours := `{"0":["00:00-24:00"],"1":["00:00-24:00"],"2":["00:00-24:00"],` +
		`"3":["00:00-24:00"],"4":["00:00-24:00"],"5":["00:00-24:00"],"6":["00:00-24:00"]}`

	for i := 0; i < n; i++ {
		lat := centreLat + (random.Float64()*2-1)*spread
		lng := centreLng + (random.Float64()*2-1)*spread
		id := fmt.Sprintf("MER-LOAD-%06d", i)
		point := fmt.Sprintf("ST_GeogFromText('SRID=4326;POINT(%.6f %.6f)')", lng, lat)

		locations = append(locations, fmt.Sprintf(
			"('%s', %s, 'DHA', NULL, TRUE)", id, point))
		merchants = append(merchants, fmt.Sprintf(
			"('%s', '%s', 'লোড টেস্ট দোকান %d', 'restaurant', 'approved', '+8801711000000', "+
				"'House %d', %s, 'DHK', 'DHA', '%s'::jsonb)",
			id, "USR-LOAD-"+id, i, i%200+1, point, hours))
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active)
		VALUES `+strings.Join(locations, ",")+`
		ON CONFLICT (merchant_id) DO NOTHING`); err != nil {
		return fmt.Errorf("seed locations: %w", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO merchants (id, owner_user_id, name, type, status, phone,
		                       line1, pin, district_code, division_code, hours)
		VALUES `+strings.Join(merchants, ",")+`
		ON CONFLICT (id) DO NOTHING`); err != nil {
		return fmt.Errorf("seed merchants: %w", err)
	}
	return nil
}

// TestALG01StaysAnIndexLookupWhenThePlannerIsLeftAlone is the assertion this
// whole package exists for.
//
// The integration suite proves the GiST index *can* serve ALG-01, but it has
// to switch sequential scans off to make the planner want one over eleven
// rows. That leaves the question this test answers: with thousands of shops
// and nothing forced, does Postgres choose the index by itself? If it ever
// stops, ALG-01 degrades from O(log n + k) to a scan of every shop in the
// division on every home screen, and the failure mode is not an error — it is
// a product that gets slower as it succeeds.
func TestALG01StaysAnIndexLookupWhenThePlannerIsLeftAlone(t *testing.T) {
	seedMerchantsAtScale(t, scale())

	plan := explain(t, `
		SELECT merchant_id,
		       ST_Distance(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, false) AS distance_m
		FROM geo_merchant_locations
		WHERE is_active
		  AND division_code = $3
		  AND ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $4, false)
		ORDER BY distance_m ASC
		LIMIT $5`,
		centreLng, centreLat, "DHA", 3000.0, 60)

	t.Logf("ALG-01 plan at %d shops:\n%s", scale(), plan)
	if !strings.Contains(plan, "geo_merchants_location_idx") {
		t.Errorf("the planner is not using the spatial index any more:\n%s", plan)
	}
	if strings.Contains(plan, "Seq Scan on geo_merchant_locations") {
		t.Errorf("ALG-01 has become a sequential scan:\n%s", plan)
	}
}

// TestALG01AnswersOneRadiusSearchUnderLoad measures the query a customer's
// home screen waits for, sixteen at a time.
//
// The ceiling is generous on purpose. This is a shared CI container, and a
// tight bound here would fail for reasons that have nothing to do with the
// product. What it catches is the order-of-magnitude regression — a dropped
// index, a query that started sorting the whole division — not a slow
// afternoon.
func TestALG01AnswersOneRadiusSearchUnderLoad(t *testing.T) {
	seedMerchantsAtScale(t, scale())

	repo := geopg.New(pool)
	centre, err := geodomain.NewCoordinate(centreLat, centreLng)
	if err != nil {
		t.Fatalf("centre: %v", err)
	}

	found, err := repo.MerchantsWithinRadius(
		context.Background(), centre, geodomain.Distance(3000), "DHA", 60)
	if err != nil {
		t.Fatalf("radius search: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("the fixture put no shops inside 3 km of the centre")
	}
	t.Logf("3 km of the centre holds %d of %d shops", len(found), scale())

	timings := measure(t, 16, 25, func(ctx context.Context) error {
		_, err := repo.MerchantsWithinRadius(ctx, centre, geodomain.Distance(3000), "DHA", 60)
		return err
	})
	timings.report(t, fmt.Sprintf("ALG-01 radius search at %d shops", scale()))

	if p95 := timings.percentile(0.95); p95 > 250*time.Millisecond {
		t.Errorf("p95 = %v, over the 250ms ceiling", p95)
	}
}

// TestALG02ExpansionCostsItsStepsAndNoMore walks the whole expansion ladder
// the way a search with too few results does.
//
// ALG-02 is O(s·log n) for s steps, and the thing that makes that claim true
// is that each step is its own indexed count rather than a re-read of
// everything found so far. The counts are asserted to rise, which is what
// proves the steps are really widening the same search rather than repeating
// it, and the whole ladder is then timed as one unit — because a customer in
// a thin area waits for all of it, not for one step.
func TestALG02ExpansionCostsItsStepsAndNoMore(t *testing.T) {
	seedMerchantsAtScale(t, scale())

	repo := geopg.New(pool)
	centre, err := geodomain.NewCoordinate(centreLat, centreLng)
	if err != nil {
		t.Fatalf("centre: %v", err)
	}

	previous := -1
	for _, radius := range expansionRadii {
		count, err := repo.CountMerchantsWithinRadius(
			context.Background(), centre, geodomain.Distance(radius), "DHA")
		if err != nil {
			t.Fatalf("count at %.0fm: %v", radius, err)
		}
		if count < previous {
			t.Errorf("widening to %.0fm found %d, fewer than the %d inside it",
				radius, count, previous)
		}
		t.Logf("within %6.0fm: %d shops", radius, count)
		previous = count
	}

	timings := measure(t, 8, 10, func(ctx context.Context) error {
		for _, radius := range expansionRadii {
			if _, err := repo.CountMerchantsWithinRadius(
				ctx, centre, geodomain.Distance(radius), "DHA"); err != nil {
				return err
			}
		}
		return nil
	})
	timings.report(t, fmt.Sprintf("ALG-02 full %d-step expansion at %d shops",
		len(expansionRadii), scale()))

	if p95 := timings.percentile(0.95); p95 > time.Second {
		t.Errorf("p95 = %v for a whole expansion, over the 1s ceiling", p95)
	}
}

// TestTheDivisionCeilingIsNotSomethingScaleCanDefeat. D3 is the one invariant
// no setting may disable, and it is enforced inside the query rather than
// after it. At the largest radius in the ladder — 25 km, which reaches well
// past Dhaka — a search must still return nothing from another division, and
// that has to remain true when there are thousands of rows for a mistake to
// hide in.
func TestTheDivisionCeilingIsNotSomethingScaleCanDefeat(t *testing.T) {
	seedMerchantsAtScale(t, scale())

	repo := geopg.New(pool)
	centre, err := geodomain.NewCoordinate(centreLat, centreLng)
	if err != nil {
		t.Fatalf("centre: %v", err)
	}

	inDhaka, err := repo.CountMerchantsWithinRadius(
		context.Background(), centre, geodomain.Distance(25000), "DHA")
	if err != nil {
		t.Fatalf("count in DHA: %v", err)
	}
	if inDhaka == 0 {
		t.Fatal("the fixture is not visible to the search at all")
	}

	// Chattogram is 240 km away, so no radius in the ladder can legitimately
	// reach it — but the assertion that matters is the division predicate, not
	// the distance: asked as Chattogram, the same centre returns nobody.
	elsewhere, err := repo.CountMerchantsWithinRadius(
		context.Background(), centre, geodomain.Distance(25000), "CTG")
	if err != nil {
		t.Fatalf("count in CTG: %v", err)
	}
	if elsewhere != 0 {
		t.Errorf("a Chattogram search found %d Dhaka shops at scale", elsewhere)
	}
}
