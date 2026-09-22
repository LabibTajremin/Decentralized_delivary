// Package dispatch holds the unit tests for the dispatch module.
//
// The three things this phase promises are a feed bounded by the partner's own
// radius, D4's distance choice honoured, and two partners never holding the
// same order. The first two are decided here in the domain; the third has a
// half here and a half in the repository's compare-and-set.
package dispatch

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
)

var at = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// ------------------------------------------------------------------ D4

func TestBandForPlacesADistance(t *testing.T) {
	// Appendix B: 5 km short, 25 km long.
	const shortMax, longMax = 5000, 25000
	cases := []struct {
		distanceM float64
		want      domain.Band
	}{
		{0, domain.BandShort},
		{4999, domain.BandShort},
		{5000, domain.BandShort}, // the ceiling is inclusive
		{5001, domain.BandLong},
		{25000, domain.BandLong},
		{25001, domain.BandBeyond},
		{90000, domain.BandBeyond},
	}
	for _, tc := range cases {
		if got := domain.BandFor(tc.distanceM, shortMax, longMax); got != tc.want {
			t.Errorf("BandFor(%v) = %q, want %q", tc.distanceM, got, tc.want)
		}
	}
}

// D4, stated as a table: a partner is shown only the distances they chose.
func TestAPreferenceAcceptsOnlyItsOwnBands(t *testing.T) {
	cases := []struct {
		preference domain.Preference
		band       domain.Band
		want       bool
	}{
		{domain.PreferenceShort, domain.BandShort, true},
		{domain.PreferenceShort, domain.BandLong, false},
		{domain.PreferenceLong, domain.BandShort, false},
		{domain.PreferenceLong, domain.BandLong, true},
		{domain.PreferenceAny, domain.BandShort, true},
		{domain.PreferenceAny, domain.BandLong, true},

		// A job past the long ceiling is offered to everyone rather than to no
		// one. The alternative is an order that reaches `ready` and sits there
		// because nobody's preference covers it — worse for the customer, the
		// shop, and the partner who would happily have taken it.
		{domain.PreferenceShort, domain.BandBeyond, true},
		{domain.PreferenceLong, domain.BandBeyond, true},
		{domain.PreferenceAny, domain.BandBeyond, true},
	}
	for _, tc := range cases {
		if got := tc.preference.Accepts(tc.band); got != tc.want {
			t.Errorf("%s accepts %s = %v, want %v", tc.preference, tc.band, got, tc.want)
		}
	}
}

// ------------------------------------------------------------- partners

func partner(t *testing.T) domain.Partner {
	t.Helper()
	p, err := domain.NewPartner("PTR-1", "USR-1", "Rafi", "+8801711111111", "motorcycle")
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}
	return p
}

// A partner starts offline and taking any distance. Offline because signing up
// is not the same as starting a shift.
func TestANewPartnerStartsOffShift(t *testing.T) {
	p := partner(t)
	if p.Availability != domain.AvailabilityOffline || p.Preference != domain.PreferenceAny {
		t.Fatalf("partner = %+v", p)
	}
	if err := p.CanTake(3); !errors.Is(err, domain.ErrNotWorking) {
		t.Errorf("an offline partner could be offered work: %v", err)
	}
}

func TestNewPartnerRefusesAnIncompleteSignUp(t *testing.T) {
	cases := []struct {
		name                  string
		userID, person, phone string
		want                  error
	}{
		{"no account", "", "Rafi", "+8801711111111", domain.ErrNoUser},
		{"no name", "USR-1", "", "+8801711111111", domain.ErrNoName},
		{"no phone", "USR-1", "Rafi", "", domain.ErrNoPhone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := domain.NewPartner("PTR-1", tc.userID, tc.person, tc.phone, ""); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestAvailabilityAndPreference(t *testing.T) {
	p := partner(t)

	online, err := p.WithAvailability(domain.AvailabilityAvailable)
	if err != nil {
		t.Fatalf("WithAvailability: %v", err)
	}
	if online.CanTake(3) != nil {
		t.Error("an available partner could not take work")
	}

	// Busy is what being at the limit is called, not something a partner
	// declares. Letting somebody set it by hand would give them a way to stay
	// in the pool while refusing every offer.
	if _, err := p.WithAvailability(domain.AvailabilityBusy); !errors.Is(err, domain.ErrBadAvailability) {
		t.Errorf("a partner declared themselves busy: %v", err)
	}
	if _, err := p.WithAvailability("sleeping"); !errors.Is(err, domain.ErrBadAvailability) {
		t.Errorf("err = %v", err)
	}

	for _, pref := range []domain.Preference{domain.PreferenceShort, domain.PreferenceLong, domain.PreferenceAny} {
		updated, prefErr := p.WithPreference(pref)
		if prefErr != nil || updated.Preference != pref {
			t.Errorf("%s: %+v, %v", pref, updated, prefErr)
		}
	}
	if _, err := p.WithPreference("medium"); !errors.Is(err, domain.ErrBadPreference) {
		t.Errorf("err = %v", err)
	}
}

func TestCapacity(t *testing.T) {
	p, err := partner(t).WithAvailability(domain.AvailabilityAvailable)
	if err != nil {
		t.Fatalf("WithAvailability: %v", err)
	}

	p.Carrying = 2
	if p.CanTake(3) != nil {
		t.Error("a partner with room could not take work")
	}
	p.Carrying = 3
	if err := p.CanTake(3); !errors.Is(err, domain.ErrAtCapacity) {
		t.Errorf("err = %v, want ErrAtCapacity", err)
	}
	// A limit of zero switches the cap off rather than refusing everything,
	// for the same reason a zero free-delivery threshold switches that
	// promotion off: nobody configures "no work at all" by clearing a field.
	if p.CanTake(0) != nil {
		t.Error("a zero limit refused a partner")
	}
}

// A partner with no offers yet scores as fully reliable. The alternative
// punishes somebody for being new, which is the surest way to have no new
// partners: their first offer would go to the bottom of every heap.
func TestAcceptanceRate(t *testing.T) {
	p := partner(t)
	if got := p.AcceptanceRate(); got != 1 {
		t.Errorf("a new partner rates %v, want 1", got)
	}

	p.Offered, p.Accepted = 4, 3
	if got := p.AcceptanceRate(); got != 0.75 {
		t.Errorf("rate = %v, want 0.75", got)
	}

	p.Offered, p.Accepted = 2, 0
	if got := p.AcceptanceRate(); got != 0 {
		t.Errorf("rate = %v, want 0", got)
	}

	// Counters that disagree — an accepted count above the offered one, which
	// only a bug produces — clamp rather than scoring above one and outranking
	// everybody honest.
	p.Offered, p.Accepted = 2, 5
	if got := p.AcceptanceRate(); got != 1 {
		t.Errorf("rate = %v, want a clamped 1", got)
	}
}

func TestAt(t *testing.T) {
	p := partner(t).At(23.746, 90.375)
	if p.Lat != 23.746 || p.Lng != 90.375 {
		t.Fatalf("partner = %+v", p)
	}
}

// --------------------------------------------------------------- scoring

func TestScoreWeighsTheThreeSignals(t *testing.T) {
	// A partner at the pickup, carrying nothing, who accepts everything.
	best := domain.Candidate{
		PartnerID: "a", DistanceM: 0, Carrying: 0, MaxConcurrent: 3, AcceptanceRate: 1,
	}
	if got := domain.Score(best, 7000); got != 0 {
		t.Errorf("the best candidate scored %v, want 0", got)
	}

	// One at the edge of the radius, full, who accepts nothing.
	worst := domain.Candidate{
		PartnerID: "b", DistanceM: 7000, Carrying: 3, MaxConcurrent: 3, AcceptanceRate: 0,
	}
	if got := domain.Score(worst, 7000); got != 1 {
		t.Errorf("the worst candidate scored %v, want 1", got)
	}

	// A zero radius cannot normalise a distance; scoring every candidate the
	// same on that term is better than dividing by zero.
	noRadius := domain.Score(domain.Candidate{
		PartnerID: "c", DistanceM: 5000, Carrying: 3, MaxConcurrent: 3, AcceptanceRate: 0,
	}, 0)
	if noRadius != 0.25+0.20 {
		t.Errorf("with a zero radius the proximity term did not drop out: %v", noRadius)
	}

	// A zero concurrent limit likewise.
	noLimit := domain.Score(domain.Candidate{
		PartnerID: "d", DistanceM: 0, Carrying: 9, MaxConcurrent: 0, AcceptanceRate: 1,
	}, 7000)
	if noLimit != 0 {
		t.Errorf("with no limit the load term did not drop out: %v", noLimit)
	}

	// Signals out of range are clamped rather than trusted.
	wild := domain.Score(domain.Candidate{
		PartnerID: "e", DistanceM: 99999, Carrying: 99, MaxConcurrent: 3, AcceptanceRate: -5,
	}, 7000)
	if wild != 1 {
		t.Errorf("out-of-range signals were not clamped: %v", wild)
	}
}

// ALG-04: a min-heap, handing out the best remaining candidate one at a time.
func TestTheAssignerHandsOutTheBestFirst(t *testing.T) {
	candidates := []domain.Candidate{
		{PartnerID: "far", DistanceM: 6000, MaxConcurrent: 3, AcceptanceRate: 1},
		{PartnerID: "near-unreliable", DistanceM: 500, MaxConcurrent: 3, AcceptanceRate: 0},
		{PartnerID: "near", DistanceM: 500, MaxConcurrent: 3, AcceptanceRate: 1},
		{PartnerID: "middling", DistanceM: 3000, MaxConcurrent: 3, AcceptanceRate: 1},
	}
	assigner := domain.NewAssigner(candidates, 7000)
	if assigner.Remaining() != 4 {
		t.Fatalf("remaining = %d", assigner.Remaining())
	}

	want := []string{"near", "middling", "near-unreliable", "far"}
	for i, expected := range want {
		got, ok := assigner.Next()
		if !ok {
			t.Fatalf("ran out after %d", i)
		}
		if got.PartnerID != expected {
			t.Fatalf("position %d is %q, want %q", i, got.PartnerID, expected)
		}
	}
	if _, ok := assigner.Next(); ok {
		t.Error("the assigner produced a fifth candidate")
	}
	if assigner.Remaining() != 0 {
		t.Errorf("remaining = %d", assigner.Remaining())
	}
}

// Two equally-scored partners come out in the same order every time. Without
// it, two dispatch rounds a second apart could offer the same job to two
// different people.
func TestTheAssignerBreaksTiesDeterministically(t *testing.T) {
	for i := 0; i < 5; i++ {
		assigner := domain.NewAssigner([]domain.Candidate{
			{PartnerID: "b", DistanceM: 1000, MaxConcurrent: 3, AcceptanceRate: 1},
			{PartnerID: "a", DistanceM: 1000, MaxConcurrent: 3, AcceptanceRate: 1},
			{PartnerID: "c", DistanceM: 1000, MaxConcurrent: 3, AcceptanceRate: 1},
		}, 7000)
		for _, want := range []string{"a", "b", "c"} {
			got, _ := assigner.Next()
			if got.PartnerID != want {
				t.Fatalf("run %d: got %q, want %q", i, got.PartnerID, want)
			}
		}
	}
}

func TestAnEmptyAssigner(t *testing.T) {
	assigner := domain.NewAssigner(nil, 7000)
	if _, ok := assigner.Next(); ok {
		t.Error("an empty assigner produced a candidate")
	}
}

// ------------------------------------------------------------------ feed

func waitingJob(t *testing.T, id string, band domain.Band) domain.Job {
	t.Helper()
	job, err := domain.NewJob(id, "ORD-"+id, "CODE12",
		domain.Place{Name: "Shop", Lat: 23.74, Lng: 90.37},
		domain.Place{Name: "Home", Lat: 23.75, Lng: 90.38},
		3000, band, domain.Placement{AreaCode: "AREA-1"}, at)
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	return job
}

// ALG-08: the feed is D4's filter plus a distance ordering plus a bound.
func TestTheFeedHonoursThePreferenceAndTheBound(t *testing.T) {
	rider, err := partner(t).WithPreference(domain.PreferenceShort)
	if err != nil {
		t.Fatalf("WithPreference: %v", err)
	}

	entries := []domain.FeedEntry{
		{Job: waitingJob(t, "far-short", domain.BandShort), DistanceToPickupM: 4000},
		{Job: waitingJob(t, "long", domain.BandLong), DistanceToPickupM: 100},
		{Job: waitingJob(t, "near-short", domain.BandShort), DistanceToPickupM: 200},
		{Job: waitingJob(t, "beyond", domain.BandBeyond), DistanceToPickupM: 300},
	}

	got := domain.BuildFeed(rider, entries)
	if len(got) != 3 {
		t.Fatalf("feed = %d entries, want 3 (the long one is filtered out)", len(got))
	}
	// Nearest pickup first, and the beyond-ceiling job survives the filter.
	want := []string{"near-short", "beyond", "far-short"}
	for i, id := range want {
		if got[i].Job.ID != id {
			t.Fatalf("position %d is %q, want %q", i, got[i].Job.ID, id)
		}
	}
}

// A job somebody already holds is not on offer.
func TestTheFeedOnlyShowsWaitingJobs(t *testing.T) {
	rider := partner(t)
	taken := waitingJob(t, "taken", domain.BandShort)
	if err := taken.Offer("PTR-2", at, time.Minute); err != nil {
		t.Fatalf("Offer: %v", err)
	}

	got := domain.BuildFeed(rider, []domain.FeedEntry{
		{Job: taken, DistanceToPickupM: 100},
		{Job: waitingJob(t, "free", domain.BandShort), DistanceToPickupM: 500},
	})
	if len(got) != 1 || got[0].Job.ID != "free" {
		t.Fatalf("feed = %+v", got)
	}
}

// The bound is the point as much as the ordering. A partner on a low-end phone
// in a busy area could otherwise be handed three hundred jobs.
func TestTheFeedIsBounded(t *testing.T) {
	rider := partner(t)
	entries := make([]domain.FeedEntry, 0, domain.FeedLimit*3)
	for i := 0; i < domain.FeedLimit*3; i++ {
		entries = append(entries, domain.FeedEntry{
			Job:               waitingJob(t, "job-"+strconv.Itoa(i), domain.BandShort),
			DistanceToPickupM: float64(i * 10),
		})
	}
	got := domain.BuildFeed(rider, entries)
	if len(got) != domain.FeedLimit {
		t.Fatalf("feed = %d entries, want %d", len(got), domain.FeedLimit)
	}
	// And it kept the nearest ones, not the first ones it happened to see.
	if got[0].Job.ID != "job-0" || got[domain.FeedLimit-1].Job.ID != "job-"+strconv.Itoa(domain.FeedLimit-1) {
		t.Errorf("the bound kept the wrong entries: %q … %q",
			got[0].Job.ID, got[domain.FeedLimit-1].Job.ID)
	}
}

// A partner refreshing twice sees the same list in the same order, rather than
// two jobs swapping places under their thumb.
func TestTheFeedIsStableOnTies(t *testing.T) {
	rider := partner(t)
	entries := []domain.FeedEntry{
		{Job: waitingJob(t, "c", domain.BandShort), DistanceToPickupM: 500},
		{Job: waitingJob(t, "a", domain.BandShort), DistanceToPickupM: 500},
		{Job: waitingJob(t, "b", domain.BandShort), DistanceToPickupM: 500},
	}
	for i := 0; i < 5; i++ {
		got := domain.BuildFeed(rider, entries)
		if got[0].Job.ID != "a" || got[1].Job.ID != "b" || got[2].Job.ID != "c" {
			t.Fatalf("run %d: %q, %q, %q", i, got[0].Job.ID, got[1].Job.ID, got[2].Job.ID)
		}
	}
}
