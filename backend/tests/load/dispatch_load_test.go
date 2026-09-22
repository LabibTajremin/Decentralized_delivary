package load

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	dispatchdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
	dispatchpg "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/infrastructure/persistence/postgres"
)

// pickup is where every assignment round in this file starts from: the same
// centre the discovery fixture uses, because a rider pool and a shop pool that
// sat in different cities would make the radius queries meaningless.
var pickup = dispatchdomain.Place{
	Name:       "লোড টেস্ট দোকান",
	Phone:      "+8801711000000",
	SingleLine: "House 12, Road 7, Dhanmondi",
	Lat:        centreLat,
	Lng:        centreLng,
}

// riderSpread is how far riders scatter, in degrees — about 8 km, tighter
// than the shops because a candidate pool is drawn from a dispatch radius and
// not from a division.
const riderSpread = 0.07

var (
	riderFixture sync.Once
	riderErr     error
)

// seedPartnersAtScale fills delivery_partners with n riders on shift around
// the pickup, with a spread of loads, acceptance histories and D4 preferences
// — so ALG-04 has something to actually order rather than n identical rows
// whose ranking is arbitrary.
func seedPartnersAtScale(t *testing.T, n int) {
	t.Helper()
	riderFixture.Do(func() { riderErr = writePartnerFixture(n) })
	if riderErr != nil {
		t.Fatalf("partner fixture: %v", riderErr)
	}
	analyse(t, "delivery_partners")
}

func writePartnerFixture(n int) error {
	random := rand.New(rand.NewPCG(20260922, 20200519))
	preferences := []string{"any", "short", "long"}

	rows := make([]string, 0, n)
	for i := 0; i < n; i++ {
		lat := centreLat + (random.Float64()*2-1)*riderSpread
		lng := centreLng + (random.Float64()*2-1)*riderSpread
		// Offered and accepted are counters, so the acceptance rate is a real
		// ratio rather than a stored float: a rider who has been asked twenty
		// times and taken five is 25%.
		offered := 10 + i%40
		accepted := offered * (i % 5) / 4
		rows = append(rows, fmt.Sprintf(
			"('PTR-LOAD-%06d', 'USR-PTR-LOAD-%06d', 'রাইডার %d', '+8801711000000', "+
				"'motorcycle', 'available', '%s', "+
				"ST_GeogFromText('SRID=4326;POINT(%.6f %.6f)'), %d, %d, %d)",
			i, i, i, preferences[i%len(preferences)], lng, lat,
			i%3, offered, accepted))
	}

	if _, err := pool.Exec(context.Background(), `
		INSERT INTO delivery_partners (id, user_id, name, phone, vehicle,
		                               availability, preference, pin,
		                               carrying, offered, accepted)
		VALUES `+strings.Join(rows, ",")+`
		ON CONFLICT (id) DO NOTHING`); err != nil {
		return fmt.Errorf("seed partners: %w", err)
	}
	return nil
}

// TestALG04CandidatePoolStaysAnIndexLookupAtScale is the dispatch half of the
// question this package asks of discovery.
//
// The candidate-pool query runs once per assignment round, and an assignment
// round runs every time any shop anywhere says a bag is ready. If the planner
// stops using the partial GiST index, every one of those scans the whole
// rider table — and it is the busiest hour, when the table is largest, that
// the scan appears in.
func TestALG04CandidatePoolStaysAnIndexLookupAtScale(t *testing.T) {
	seedPartnersAtScale(t, scale())

	plan := explain(t, `
		SELECT id, ST_Distance(pin, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, false)
		FROM delivery_partners
		WHERE availability = 'available'
		  AND pin IS NOT NULL
		  AND ST_DWithin(pin, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3, false)
		  AND (preference = 'any' OR preference = $4 OR $4 = 'beyond')
		ORDER BY 2
		LIMIT $5`,
		pickup.Lng, pickup.Lat, 5000.0, "short", 50)

	t.Logf("ALG-04 candidate-pool plan at %d riders:\n%s", scale(), plan)
	if !strings.Contains(plan, "delivery_partners_available_idx") {
		t.Errorf("the planner is not using the partial spatial index any more:\n%s", plan)
	}
	if strings.Contains(plan, "Seq Scan on delivery_partners") {
		t.Errorf("the candidate pool has become a sequential scan:\n%s", plan)
	}
}

// TestALG04DrawsACandidatePoolUnderLoad measures the query an assignment
// round waits for, with several rounds in flight at once — which is what a
// dinner rush is.
func TestALG04DrawsACandidatePoolUnderLoad(t *testing.T) {
	seedPartnersAtScale(t, scale())

	repo := dispatchpg.New(pool)
	pooled, err := repo.AvailableWithin(
		context.Background(), pickup, 5000, dispatchdomain.BandShort, 50)
	if err != nil {
		t.Fatalf("candidate pool: %v", err)
	}
	if len(pooled) == 0 {
		t.Fatal("the fixture put no available riders within 5 km of the pickup")
	}
	t.Logf("5 km of the pickup holds %d candidates (capped at 50) of %d riders",
		len(pooled), scale())

	// The nearest candidate really is nearest: the ORDER BY is the query's,
	// and a change that quietly dropped it would leave the assigner scoring a
	// pool that is no longer the closest fifty.
	for i := 1; i < len(pooled); i++ {
		if pooled[i].DistanceM < pooled[i-1].DistanceM {
			t.Fatalf("candidate %d is nearer than candidate %d", i, i-1)
		}
	}

	timings := measure(t, 16, 25, func(ctx context.Context) error {
		_, err := repo.AvailableWithin(ctx, pickup, 5000, dispatchdomain.BandShort, 50)
		return err
	})
	timings.report(t, fmt.Sprintf("ALG-04 candidate pool at %d riders", scale()))

	if p95 := timings.percentile(0.95); p95 > 250*time.Millisecond {
		t.Errorf("p95 = %v, over the 250ms ceiling", p95)
	}
}

// TestALG04OrdersItsCandidatesInLogLinearTime is the only test in this package
// with no database in it, because the claim it checks has none either.
//
// ALG-04 is documented as O(n log n) to build and O(log n) to take the next
// candidate. The way that claim breaks in practice is not a slow heap; it is
// somebody replacing the heap with a sort inside a loop, or re-scoring every
// candidate on every Next. Either turns it quadratic, and neither shows up in
// a correctness test.
//
// So the shape is measured rather than the wall clock: build a pool, then a
// pool eight times larger, and require the cost to grow by far less than the
// sixty-four times a quadratic algorithm would cost. The bound is loose
// enough that a shared CI runner cannot fail it by being busy, and tight
// enough that O(n²) cannot pass it.
func TestALG04OrdersItsCandidatesInLogLinearTime(t *testing.T) {
	const radiusM = 5000

	small := scale()
	large := small * 8

	// Warm the code paths first, so the small measurement is not paying for
	// this goroutine's first allocations.
	drain(candidates(small/10, radiusM), radiusM)

	smallTime := drain(candidates(small, radiusM), radiusM)
	largeTime := drain(candidates(large, radiusM), radiusM)

	t.Logf("ALG-04 ordering: %d candidates in %v, %d candidates in %v",
		small, smallTime, large, largeTime)

	// n log n over an eight-fold increase is about 8 × (log 8n / log n) —
	// under ten. Quadratic would be sixty-four. Twenty-five sits between them
	// with room on both sides.
	ratio := float64(largeTime) / math.Max(float64(smallTime), 1)
	if ratio > 25 {
		t.Errorf("eight times the candidates cost %.1f times the work; "+
			"that is not O(n log n)", ratio)
	}
}

// candidates builds a pool of n riders spread across the radius, with varying
// load and acceptance so the scores differ.
func candidates(n int, radiusM float64) []dispatchdomain.Candidate {
	random := rand.New(rand.NewPCG(20260923, 20200519))
	out := make([]dispatchdomain.Candidate, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, dispatchdomain.Candidate{
			PartnerID:      fmt.Sprintf("PTR-%06d", i),
			DistanceM:      random.Float64() * radiusM,
			Carrying:       i % 3,
			MaxConcurrent:  3,
			AcceptanceRate: random.Float64(),
		})
	}
	return out
}

// drain builds the assigner and takes every candidate out of it, which is the
// worst case a round can reach: every rider offered to and every one
// declining.
func drain(pool []dispatchdomain.Candidate, radiusM float64) time.Duration {
	start := time.Now()
	assigner := dispatchdomain.NewAssigner(pool, radiusM)
	var previous float64
	for {
		candidate, ok := assigner.Next()
		if !ok {
			break
		}
		// Reading the score back keeps the loop from being optimised into
		// nothing, and asserts the one property the heap exists for: they
		// come out best-first.
		score := dispatchdomain.Score(candidate, radiusM)
		if score < previous {
			panic("the assigner handed out a worse candidate before a better one")
		}
		previous = score
	}
	return time.Since(start)
}
