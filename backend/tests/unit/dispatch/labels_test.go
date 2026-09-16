package dispatch

import (
	"context"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Every string a rider's screen shows is composed here, in both languages
// (2.9, 1.4). These walk each label through both, because a half-translated
// screen — an English status over a Bengali distance — is what happens when
// only one language is ever exercised.

// ascii reports whether a rendered label is plain English text. Bengali is the
// default everywhere (1.4), so a label with no non-ASCII rune in it is either
// English or a translation somebody forgot.
func ascii(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// seedJob puts a job in whatever state a label needs, without walking the whole
// lifecycle to get there. The transitions themselves are tested in job_test.go;
// what is under test here is the rendering.
func seedJob(r *rig, id string, status domain.JobStatus, band domain.Band, partnerID string) domain.Job {
	job := domain.Job{
		ID: id, OrderID: "ORD-" + id, Code: "ABC234",
		Status: status, PartnerID: partnerID, Band: band,
		Pickup:      domain.Place{Name: "Star Kabab", Lat: 23.7455, Lng: 90.3738},
		Destination: domain.Place{Name: "Fardin", Lat: 23.746, Lng: 90.375},
		DistanceM:   3000,
		CreatedAt:   at, UpdatedAt: at,
	}
	r.repo.jobs[id] = job
	return job
}

func TestEveryJobLabelIsRenderedInBothLanguages(t *testing.T) {
	ctx := context.Background()
	statuses := []domain.JobStatus{
		domain.JobWaiting, domain.JobOffered, domain.JobAssigned,
		domain.JobCollected, domain.JobDelivered, domain.JobFailed, domain.JobCancelled,
	}
	bands := []domain.Band{domain.BandShort, domain.BandLong, domain.BandBeyond}

	for _, lang := range []string{"bn", "en"} {
		r := newRig()
		rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		for i, status := range statuses {
			seedJob(r, string(rune('A'+i)), status, bands[i%len(bands)], rider.ID)
		}

		list, err := r.partners.MyJobs(ctx, "USR-1", false, 50, 0, lang)
		if err != nil {
			t.Fatalf("MyJobs: %v", err)
		}
		if list.Total != len(statuses) {
			t.Fatalf("%s: %d jobs, want %d", lang, list.Total, len(statuses))
		}
		for _, job := range list.Jobs {
			if job.StatusLabel == "" || job.BandLabel == "" || job.Distance == "" {
				t.Fatalf("%s: job = %+v", lang, job)
			}
			if ascii(job.StatusLabel) != (lang == "en") {
				t.Fatalf("%s produced %q", lang, job.StatusLabel)
			}
		}
	}
}

func TestEveryPartnerLabelIsRenderedInBothLanguages(t *testing.T) {
	ctx := context.Background()
	for _, lang := range []string{"bn", "en"} {
		for _, preference := range []string{"short", "long", "any"} {
			r := newRig()
			r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, preference)

			online, err := r.partners.Me(ctx, "USR-1", lang)
			if err != nil {
				t.Fatalf("Me: %v", err)
			}
			if online.AvailabilityLabel == "" || online.PreferenceLabel == "" {
				t.Fatalf("%s/%s: partner = %+v", lang, preference, online)
			}
			if ascii(online.PreferenceLabel) != (lang == "en") {
				t.Fatalf("%s produced %q", lang, online.PreferenceLabel)
			}

			offline, err := r.partners.SetAvailability(ctx, "USR-1", "offline", lang)
			if err != nil {
				t.Fatalf("SetAvailability: %v", err)
			}
			if offline.AvailabilityLabel == online.AvailabilityLabel {
				t.Fatalf("offline reads the same as available: %q", offline.AvailabilityLabel)
			}
		}
	}
}

// Busy is not something a partner declares, so the only way to see its label is
// to fill their hands.
func TestTheBusyLabel(t *testing.T) {
	ctx := context.Background()
	for _, lang := range []string{"bn", "en"} {
		r := newRig()
		rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		busy := r.repo.partners[rider.ID]
		busy.Availability = domain.AvailabilityBusy
		r.repo.partners[rider.ID] = busy

		view, err := r.partners.Me(ctx, "USR-1", lang)
		if err != nil {
			t.Fatalf("Me: %v", err)
		}
		if view.Availability != "busy" || view.AvailabilityLabel == "" {
			t.Fatalf("%s: partner = %+v", lang, view)
		}
	}
}

// The three reasons a feed is empty are three different sentences, in both
// languages. "Nothing here" and "you are offline" are different facts.
func TestEveryEmptyFeedReason(t *testing.T) {
	ctx := context.Background()
	seen := map[string]bool{}
	for _, lang := range []string{"bn", "en"} {
		r := newRig()
		rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")

		nothing, err := r.partners.Feed(ctx, "USR-1", lang)
		if err != nil {
			t.Fatalf("Feed: %v", err)
		}
		if nothing.Reason != "nothing_nearby" || nothing.Notice == "" {
			t.Fatalf("%s: feed = %+v", lang, nothing)
		}
		seen[nothing.Notice] = true

		full := r.repo.partners[rider.ID]
		full.Carrying = 3
		r.repo.partners[rider.ID] = full
		atCapacity, err := r.partners.Feed(ctx, "USR-1", lang)
		if err != nil {
			t.Fatalf("Feed: %v", err)
		}
		if atCapacity.Reason != "at_capacity" || atCapacity.Notice == "" {
			t.Fatalf("%s: feed = %+v", lang, atCapacity)
		}
		seen[atCapacity.Notice] = true

		if _, err := r.partners.SetAvailability(ctx, "USR-1", "offline", lang); err != nil {
			t.Fatalf("SetAvailability: %v", err)
		}
		offline, err := r.partners.Feed(ctx, "USR-1", lang)
		if err != nil {
			t.Fatalf("Feed: %v", err)
		}
		if offline.Reason != "offline" || offline.Notice == "" {
			t.Fatalf("%s: feed = %+v", lang, offline)
		}
		seen[offline.Notice] = true
	}
	if len(seen) != 6 {
		t.Fatalf("three reasons in two languages produced %d sentences: %v", len(seen), seen)
	}
}

// Distances are rendered for a rider on a motorbike: coarse, and in Bengali
// digits unless English was asked for.
func TestDistancesAreRenderedForARider(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		distanceM  float64
		bengali    string
		english    string
		wantAnswer bool
	}{
		{0, "", "", false},
		{40, "১০০ মিটার", "100 m", true},
		{450, "৫০০ মিটার", "500 m", true},
		{3000, "৩.০ কিমি", "3.0 km", true},
	}
	for _, tc := range cases {
		for _, lang := range []string{"bn", "en"} {
			r := newRig()
			rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
			job := seedJob(r, "JOB-D", domain.JobWaiting, domain.BandShort, rider.ID)
			job.DistanceM = tc.distanceM
			r.repo.jobs[job.ID] = job

			list, err := r.partners.MyJobs(ctx, "USR-1", false, 50, 0, lang)
			if err != nil {
				t.Fatalf("MyJobs: %v", err)
			}
			want := tc.bengali
			if lang == "en" {
				want = tc.english
			}
			if got := list.Jobs[0].Distance; got != want {
				t.Fatalf("%.0f m in %s = %q, want %q", tc.distanceM, lang, got, want)
			}
		}
	}
}

// An open offer carries a countdown composed from the server's clock, because a
// rider's phone clock is wrong often enough to expire an offer at the wrong
// moment. Anything that is not an open offer carries none.
func TestTheOfferCountdown(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")

	open := seedJob(r, "JOB-OPEN", domain.JobOffered, domain.BandShort, rider.ID)
	open.OfferedAt = at
	open.OfferExpiresAt = at.Add(30 * time.Second)
	open.UpdatedAt = at
	r.repo.jobs[open.ID] = open

	list, err := r.partners.MyJobs(ctx, "USR-1", true, 50, 0, "en")
	if err != nil {
		t.Fatalf("MyJobs: %v", err)
	}
	if list.Jobs[0].SecondsLeft != 30 {
		t.Fatalf("seconds left = %d, want 30", list.Jobs[0].SecondsLeft)
	}

	// An offer whose clock has already run out counts down to zero rather than
	// to a negative number a screen would have to interpret.
	expired := r.repo.jobs[open.ID]
	expired.UpdatedAt = at.Add(time.Minute)
	r.repo.jobs[open.ID] = expired
	list, err = r.partners.MyJobs(ctx, "USR-1", true, 50, 0, "en")
	if err != nil {
		t.Fatalf("MyJobs: %v", err)
	}
	if list.Jobs[0].SecondsLeft != 0 {
		t.Fatalf("seconds left = %d, want 0", list.Jobs[0].SecondsLeft)
	}
}

// A page is clamped rather than refused: a rider whose app asks for a thousand
// jobs gets fifty, not an error.
func TestThePageIsClamped(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	for i := range 3 {
		seedJob(r, string(rune('A'+i)), domain.JobDelivered, domain.BandShort, rider.ID)
	}

	for _, limit := range []int{0, 1000} {
		list, err := r.partners.MyJobs(ctx, "USR-1", false, limit, 0, "en")
		if err != nil {
			t.Fatalf("MyJobs: %v", err)
		}
		if list.Total != 3 {
			t.Fatalf("limit %d returned %d of %d", limit, len(list.Jobs), list.Total)
		}
	}
}

// Registering when the store cannot say whether this account is already a
// partner must not create a second one.
func TestRegisteringWithABrokenStore(t *testing.T) {
	r := newRig()
	r.repo.partnerErr = errBoom
	if _, err := r.partners.Register(context.Background(), "USR-1", "", application.RegisterRequest{
		Name: "Rafi", Phone: "+8801711111111",
	}); err == nil {
		t.Fatal("a broken store registered a partner")
	}
}

// Score is the one piece of ALG-04 a caller can hand anything to, so it clamps
// its terms rather than trusting them.
func TestScoreClampsItsTerms(t *testing.T) {
	radius := 7000.0
	// An acceptance rate above 1 would otherwise make the term negative and
	// let a partner score better than the best possible.
	impossible := domain.Score(domain.Candidate{
		DistanceM: 0, Carrying: 0, MaxConcurrent: 3, AcceptanceRate: 1.5,
	}, radius)
	perfect := domain.Score(domain.Candidate{
		DistanceM: 0, Carrying: 0, MaxConcurrent: 3, AcceptanceRate: 1,
	}, radius)
	if impossible != perfect {
		t.Fatalf("an impossible acceptance rate scored %f, want %f", impossible, perfect)
	}

	// And one beyond the radius is no worse than one exactly at it.
	far := domain.Score(domain.Candidate{DistanceM: 99000, MaxConcurrent: 3}, radius)
	edge := domain.Score(domain.Candidate{DistanceM: radius, MaxConcurrent: 3}, radius)
	if far != edge {
		t.Fatalf("beyond the radius scored %f, at the radius %f", far, edge)
	}
}

// Candidates left over after a round are a fact the caller can act on: "nobody
// nearby" and "everybody nearby already said no" are different situations.
func TestTheAssignerReportsWhatIsLeft(t *testing.T) {
	assigner := domain.NewAssigner([]domain.Candidate{
		{PartnerID: "PTR-2", DistanceM: 400, MaxConcurrent: 3, AcceptanceRate: 1},
		{PartnerID: "PTR-1", DistanceM: 200, MaxConcurrent: 3, AcceptanceRate: 1},
	}, 7000)
	if assigner.Remaining() != 2 {
		t.Fatalf("remaining = %d", assigner.Remaining())
	}
	first, ok := assigner.Next()
	if !ok || first.PartnerID != "PTR-1" {
		t.Fatalf("first = %+v, %v", first, ok)
	}
	if assigner.Remaining() != 1 {
		t.Fatalf("remaining = %d", assigner.Remaining())
	}
	if _, ok := assigner.Next(); !ok {
		t.Fatal("the second candidate was not handed out")
	}
	if _, ok := assigner.Next(); ok {
		t.Fatal("an empty assigner handed out a candidate")
	}
}

// ------------------------------------------------------- the awkward corners

// A job that moved between the sweeper's query and its loop — the order behind
// it was cancelled while the sweep was running — is skipped, not forced back
// onto somebody's phone.
func TestTheSweepSkipsAJobThatMovedUnderIt(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}
	r.repo.board = []domain.Job{seedJob(r, "JOB-GONE", domain.JobCancelled, domain.BandShort, "")}

	swept, err := r.offers.Sweep(ctx, 50)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if swept.Offered != 0 {
		t.Fatalf("a cancelled job was offered to somebody: %+v", swept)
	}
}

// The compare-and-set losing a race during a re-offer is not a failure either:
// the job is where somebody else put it.
func TestASweptOfferThatLosesTheRace(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}
	seedJob(r, "JOB-RACE", domain.JobWaiting, domain.BandShort, "")

	r.repo.saveJobErr = errBoom
	swept, err := r.offers.Sweep(ctx, 50)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if swept.Offered != 0 {
		t.Fatalf("swept = %+v", swept)
	}
}

// Withdrawing frees the rider who was holding the job — and a withdrawal that
// cannot be written is reported, because the order is gone and a rider still
// driving to the shop is the outcome this prevents.
func TestWithdrawingFreesTheRider(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if _, err := r.partners.Accept(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	// Carrying their limit, so they read as busy until this job goes away.
	busy := r.repo.partners[rider.ID]
	busy.Availability = domain.AvailabilityBusy
	r.repo.partners[rider.ID] = busy

	if err := r.service.Withdraw(ctx, "ORD-1", "the customer cancelled"); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	freed := r.repo.partners[rider.ID]
	if freed.Carrying != 0 || freed.Availability != domain.AvailabilityAvailable {
		t.Fatalf("partner = %+v, want a free rider", freed)
	}
}

// A rider record that cannot be read leaves the withdrawal standing: the job is
// off the board, which is the part the customer can see.
func TestWithdrawingWhenTheRiderCannotBeRead(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if _, err := r.partners.Accept(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	r.repo.partnerErr = errBoom
	if err := r.service.Withdraw(ctx, "ORD-1", "the customer cancelled"); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if r.repo.jobs[job.ID].Status != domain.JobCancelled {
		t.Fatalf("job = %+v", r.repo.jobs[job.ID])
	}
}

// A withdrawal the store refuses is reported rather than swallowed.
func TestWithdrawingAgainstABrokenStore(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}
	if _, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000)); err != nil {
		t.Fatalf("Offer: %v", err)
	}

	r.repo.saveJobErr = errBoom
	if err := r.service.Withdraw(ctx, "ORD-1", "the customer cancelled"); err == nil {
		t.Fatal("a withdrawal that was never written reported success")
	}
}

// The two conflicts a rider meets on a job they do hold: one that has already
// finished, and an offer whose clock ran out while their phone was in a pocket.
func TestTheConflictsARiderIsTold(t *testing.T) {
	ctx := context.Background()

	finished := newRig()
	rider := finished.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	seedJob(finished, "JOB-DONE", domain.JobDelivered, domain.BandShort, rider.ID)
	if _, err := finished.partners.Accept(ctx, "USR-1", "JOB-DONE", "en"); errs.CodeOf(err) != "job_finished" {
		t.Fatalf("err = %v, want job_finished", err)
	}

	expired := newRig()
	late := expired.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	expired.repo.nearby = []ports.PartnerDistance{{Partner: expired.repo.partners[late.ID], DistanceM: 200}}
	job, err := expired.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	expired.clock.now = at.Add(31 * time.Second)
	if _, err := expired.partners.Accept(ctx, "USR-1", job.ID, "en"); errs.CodeOf(err) != "offer_expired" {
		t.Fatalf("err = %v, want offer_expired", err)
	}
}
