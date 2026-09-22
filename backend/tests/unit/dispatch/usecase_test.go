package dispatch

import (
	"context"
	"testing"
	"time"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
	geocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// rig is one assembled dispatch module with every collaborator reachable.
type rig struct {
	repo   *fakeRepo
	order  *fakeOrder
	geo    *fakeGeo
	config *fakeConfig
	clock  *movableClock

	offers   *application.OfferUseCase
	partners *application.PartnerUseCase
	service  *application.Service
}

func newRig() *rig {
	repo := newRepo()
	orders := &fakeOrder{}
	geoService := &fakeGeo{area: geocontract.Area{
		AreaCode: "DHA-DHK-DHN", DistrictCode: "DHA-DHK", DivisionCode: "DHA",
	}}
	config := &fakeConfig{settings: appendixB()}
	clk := &movableClock{now: at}
	ids := &fakeIDs{}

	offers := application.NewOfferUseCase(repo, config, clk, ids)
	return &rig{
		repo: repo, order: orders, geo: geoService, config: config, clock: clk,
		offers:   offers,
		partners: application.NewPartnerUseCase(repo, orders, geoService, config, clk, ids),
		service:  application.NewService(repo, offers),
	}
}

// onShift registers a partner, puts them online and places them.
func (r *rig) onShift(t *testing.T, userID, name string, lat, lng float64, preference string) application.PartnerView {
	t.Helper()
	ctx := context.Background()
	view, err := r.partners.Register(ctx, userID, "", application.RegisterRequest{
		Name: name, Phone: "+8801711111111", Vehicle: "motorcycle",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := r.partners.SetAvailability(ctx, userID, "available", ""); err != nil {
		t.Fatalf("SetAvailability: %v", err)
	}
	if preference != "" {
		if _, err := r.partners.SetPreference(ctx, userID, preference, ""); err != nil {
			t.Fatalf("SetPreference: %v", err)
		}
	}
	if _, err := r.partners.ReportLocation(ctx, userID, lat, lng, ""); err != nil {
		t.Fatalf("ReportLocation: %v", err)
	}
	return view
}

// offerRequest is an order ready to collect near Dhanmondi.
func offerRequest(orderID string, distanceM float64) contract.OfferRequest {
	return contract.OfferRequest{
		OrderID: orderID, Code: "ABC234",
		Pickup: contract.Place{
			Name: "Star Kabab", Phone: "+8801711000001",
			SingleLine: "Road 27, Dhanmondi", Lat: 23.7455, Lng: 90.3738,
		},
		Destination: contract.Place{
			Name: "Fardin", Phone: "+8801711111111",
			SingleLine: "House 5, Dhanmondi", Lat: 23.746, Lng: 90.375,
		},
		DistanceM:    distanceM,
		AreaCode:     "DHA-DHK-DHN",
		DistrictCode: "DHA-DHK",
		DivisionCode: "DHA",
	}
}

// ------------------------------------------------------------------ partners

func TestRegisteringAsAPartner(t *testing.T) {
	r := newRig()
	ctx := context.Background()

	view, err := r.partners.Register(ctx, "USR-1", "", application.RegisterRequest{
		Name: "Rafi", Phone: "+8801711111111", Vehicle: "bicycle",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if view.Availability != "offline" || view.Preference != "any" {
		t.Fatalf("view = %+v", view)
	}
	// A new partner reads as fully reliable rather than as nobody.
	if view.AcceptancePercent != 100 {
		t.Errorf("acceptance = %d, want 100", view.AcceptancePercent)
	}

	// Registering twice is somebody tapping a button again.
	again, err := r.partners.Register(ctx, "USR-1", "", application.RegisterRequest{Name: "Rafi", Phone: "x"})
	if err != nil {
		t.Fatalf("Register again: %v", err)
	}
	if again.ID != view.ID {
		t.Fatalf("a second registration made a second partner: %s, %s", view.ID, again.ID)
	}

	if _, err := r.partners.Register(ctx, "USR-2", "", application.RegisterRequest{Phone: "x"}); errs.CodeOf(err) != "invalid_partner" {
		t.Fatalf("a nameless partner was accepted: %v", err)
	}
}

func TestAnAccountThatIsNotAPartner(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	for name, run := range map[string]func() error{
		"me":           func() error { _, err := r.partners.Me(ctx, "USR-9", ""); return err },
		"availability": func() error { _, err := r.partners.SetAvailability(ctx, "USR-9", "available", ""); return err },
		"preference":   func() error { _, err := r.partners.SetPreference(ctx, "USR-9", "short", ""); return err },
		"location":     func() error { _, err := r.partners.ReportLocation(ctx, "USR-9", 23.7, 90.4, ""); return err },
		"feed":         func() error { _, err := r.partners.Feed(ctx, "USR-9", "en"); return err },
		"jobs":         func() error { _, err := r.partners.MyJobs(ctx, "USR-9", true, 20, 0, "en"); return err },
		"accept":       func() error { _, err := r.partners.Accept(ctx, "USR-9", "JOB-1", "en"); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); errs.CodeOf(err) != "partner_not_found" {
				t.Fatalf("err = %v, want partner_not_found", err)
			}
		})
	}
}

func TestSettingAvailabilityAndPreference(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")

	view, err := r.partners.SetPreference(ctx, "USR-1", "long", "")
	if err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	if view.Preference != "long" || view.PreferenceLabel == "" {
		t.Fatalf("view = %+v", view)
	}

	if _, err := r.partners.SetPreference(ctx, "USR-1", "medium", ""); errs.CodeOf(err) != "invalid_preference" {
		t.Fatalf("err = %v", err)
	}
	if _, err := r.partners.SetAvailability(ctx, "USR-1", "busy", ""); errs.CodeOf(err) != "invalid_availability" {
		t.Fatalf("a partner declared themselves busy: %v", err)
	}
}

// ------------------------------------------------------------------ offering

func TestOfferingAJobFindsTheBestPartner(t *testing.T) {
	r := newRig()
	ctx := context.Background()

	near := r.onShift(t, "USR-NEAR", "Near", 23.7456, 90.3739, "")
	far := r.onShift(t, "USR-FAR", "Far", 23.80, 90.42, "")
	r.repo.nearby = []ports.PartnerDistance{
		{Partner: r.repo.partners[far.ID], DistanceM: 6000},
		{Partner: r.repo.partners[near.ID], DistanceM: 150},
	}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if job.Status != "offered" || job.Partner.ID != near.ID {
		t.Fatalf("job = %+v, want an offer to the nearest partner", job)
	}
	if job.Band != "short" {
		t.Errorf("band = %q, want short for 3 km", job.Band)
	}
	if job.Attempts != 1 {
		t.Errorf("attempts = %d", job.Attempts)
	}
	// The offer is counted against the partner's rate until they take it.
	if len(r.repo.offersRecorded) != 1 || r.repo.offersRecorded[0].accepted {
		t.Errorf("offers recorded = %+v", r.repo.offersRecorded)
	}
	// The partner's details travel with the job so a tracking screen needs no
	// second module.
	if job.Partner.Name != "Near" || job.Partner.Phone == "" {
		t.Errorf("partner = %+v", job.Partner)
	}
}

// Idempotent on the order id. An order that reaches `ready` twice — a retry, a
// re-delivered event — must not become two jobs for two riders.
func TestOfferingTheSameOrderTwice(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	first, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	second, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer again: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("two jobs: %s and %s", first.ID, second.ID)
	}
	if len(r.repo.jobs) != 1 {
		t.Fatalf("%d jobs were written", len(r.repo.jobs))
	}
}

// The same order twice in the same millisecond gets past the check above and is
// caught by the unique index instead. That is a conflict, not an outage: the
// job that won is the answer, and the shop is not told the delivery could not
// be arranged.
func TestTwoReadyEventsRacingEachOther(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	won, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}

	// The loser's own JobForOrder came back empty, so it wrote — and the index
	// refused it.
	r.repo.createErr = errs.New(errs.KindConflict, "job_exists", "Already out for delivery.")
	r.repo.hideJobForOrderOnce = true
	lost, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("the losing event failed: %v", err)
	}
	if lost.ID != won.ID {
		t.Fatalf("the loser got job %s, want %s", lost.ID, won.ID)
	}

	// If the re-read fails too, there is nothing honest to return but an
	// outage.
	r.repo.hideJobForOrderOnce = true
	r.repo.jobErr = errBoom
	if _, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000)); errs.CodeOf(err) != "dispatch_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

// Nobody nearby is a normal state in a rural area at two in the morning, not a
// failure. The job waits and the next round tries again.
func TestOfferingWithNobodyAround(t *testing.T) {
	r := newRig()
	job, err := r.service.Offer(context.Background(), offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if job.Status != "waiting" || job.Partner.ID != "" {
		t.Fatalf("job = %+v", job)
	}
}

// A partner who is full is not a candidate, even though the query returned
// them.
func TestOfferingSkipsPartnersWithNoRoom(t *testing.T) {
	r := newRig()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	full := r.repo.partners[rider.ID]
	full.Carrying = 3
	r.repo.partners[rider.ID] = full
	r.repo.nearby = []ports.PartnerDistance{{Partner: full, DistanceM: 200}}

	job, err := r.service.Offer(context.Background(), offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if job.Status != "waiting" {
		t.Fatalf("a full partner was offered a job: %+v", job)
	}
}

// D4 in the offering round: a long job is not put to somebody who only wants
// local work.
func TestOfferingHonoursTheDistanceChoice(t *testing.T) {
	r := newRig()
	local := r.onShift(t, "USR-LOCAL", "Local", 23.746, 90.375, "short")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[local.ID], DistanceM: 200}}

	// 12 km is a long job with Appendix B's 5 km short ceiling.
	job, err := r.service.Offer(context.Background(), offerRequest("ORD-1", 12000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if job.Band != "long" {
		t.Fatalf("band = %q", job.Band)
	}
	if job.Status != "waiting" {
		t.Fatalf("a short-distance partner was offered a long job: %+v", job)
	}
}

func TestOfferingFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("the job store cannot be read", func(t *testing.T) {
		r := newRig()
		r.repo.jobErr = errBoom
		if _, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000)); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("configuration is unreachable", func(t *testing.T) {
		r := newRig()
		r.config.err = errBoom
		if _, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000)); errs.CodeOf(err) != "dispatch_config_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	for _, key := range []string{
		cfgcontract.DispatchShortDistance, cfgcontract.DispatchLongDistance,
		cfgcontract.DispatchPartnerRadius, cfgcontract.DispatchAssignTimeout,
		cfgcontract.DispatchMaxConcurrent,
	} {
		t.Run("missing "+key, func(t *testing.T) {
			r := newRig()
			settings := appendixB()
			settings.failOn = key
			r.config.settings = settings
			if _, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000)); errs.CodeOf(err) != "dispatch_config_unavailable" {
				t.Fatalf("err = %v", err)
			}
		})
	}

	t.Run("the job cannot be written", func(t *testing.T) {
		r := newRig()
		r.repo.createErr = errBoom
		if _, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000)); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a negative distance", func(t *testing.T) {
		r := newRig()
		if _, err := r.service.Offer(ctx, offerRequest("ORD-1", -1)); err == nil {
			t.Fatal("a job with a negative distance was created")
		}
	})

	// The job exists and is on the board; failing to find somebody for it right
	// now is not a reason to tell the shop the delivery could not be arranged.
	t.Run("the candidate query fails", func(t *testing.T) {
		r := newRig()
		r.repo.availErr = errBoom
		job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
		if err != nil {
			t.Fatalf("Offer: %v", err)
		}
		if job.Status != "waiting" {
			t.Fatalf("job = %+v", job)
		}
	})

	// Likewise a counter that would not increment. A delivery is not worth
	// refusing over a statistic.
	t.Run("the offer counter fails", func(t *testing.T) {
		r := newRig()
		rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}
		r.repo.offerErr = errBoom
		job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
		if err != nil {
			t.Fatalf("Offer: %v", err)
		}
		if job.Status != "offered" {
			t.Fatalf("job = %+v", job)
		}
	})
}

// ------------------------------------------------------------------ the feed

func TestTheFeedIsOnlyWhatThisPartnerShouldSee(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "short")

	// Two jobs on the board: one short, one long.
	shortJob, err := r.service.Offer(ctx, offerRequest("ORD-SHORT", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	longJob, err := r.service.Offer(ctx, offerRequest("ORD-LONG", 12000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	r.repo.waiting = []ports.JobDistance{
		{Job: r.repo.jobs[longJob.ID], DistanceM: 100},
		{Job: r.repo.jobs[shortJob.ID], DistanceM: 800},
	}

	feed, err := r.partners.Feed(ctx, "USR-1", "en")
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(feed.Jobs) != 1 || feed.Jobs[0].ID != shortJob.ID {
		t.Fatalf("feed = %+v", feed.Jobs)
	}
	if feed.RadiusM != 7000 {
		t.Errorf("radius = %v, want the configured 7000", feed.RadiusM)
	}
	// The entry is fully composed: a rider on a motorbike reads words, not
	// numbers they have to interpret.
	entry := feed.Jobs[0]
	if entry.StatusLabel == "" || entry.BandLabel == "" || entry.Distance == "" || entry.ToPickup == "" {
		t.Errorf("entry = %+v", entry)
	}
	_ = rider
}

// Offline is a real answer, not an empty list. A partner who forgot to go
// online should be told that, not shown nothing and left to wonder.
func TestTheFeedSaysWhyItIsEmpty(t *testing.T) {
	ctx := context.Background()

	t.Run("offline", func(t *testing.T) {
		r := newRig()
		if _, err := r.partners.Register(ctx, "USR-1", "", application.RegisterRequest{
			Name: "Rafi", Phone: "+8801711111111",
		}); err != nil {
			t.Fatalf("Register: %v", err)
		}
		feed, err := r.partners.Feed(ctx, "USR-1", "en")
		if err != nil {
			t.Fatalf("Feed: %v", err)
		}
		if feed.Reason != "offline" || feed.Notice != "You are offline. Go online to see deliveries." {
			t.Fatalf("feed = %+v", feed)
		}
	})

	t.Run("at capacity", func(t *testing.T) {
		r := newRig()
		rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		full := r.repo.partners[rider.ID]
		full.Carrying = 3
		r.repo.partners[rider.ID] = full

		feed, err := r.partners.Feed(ctx, "USR-1", "en")
		if err != nil {
			t.Fatalf("Feed: %v", err)
		}
		if feed.Reason != "at_capacity" || feed.Notice == "" {
			t.Fatalf("feed = %+v", feed)
		}
	})

	t.Run("nothing nearby", func(t *testing.T) {
		r := newRig()
		r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		feed, err := r.partners.Feed(ctx, "USR-1", "en")
		if err != nil {
			t.Fatalf("Feed: %v", err)
		}
		if feed.Reason != "nothing_nearby" || feed.Notice != "There are no deliveries near you right now." {
			t.Fatalf("feed = %+v", feed)
		}
	})

	t.Run("in Bengali", func(t *testing.T) {
		r := newRig()
		r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		feed, err := r.partners.Feed(ctx, "USR-1", "")
		if err != nil {
			t.Fatalf("Feed: %v", err)
		}
		if feed.Notice != "আপনার আশেপাশে এখন কোনো ডেলিভারি নেই।" {
			t.Errorf("notice = %q", feed.Notice)
		}
	})
}

// The settings a rider works under are the ones configured where they are
// standing, not a global default.
func TestTheFeedResolvesSettingsWhereTheRiderIs(t *testing.T) {
	r := newRig()
	r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	before := len(r.config.seen)

	if _, err := r.partners.Feed(context.Background(), "USR-1", "en"); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	asked := r.config.seen[before]
	if asked.DivisionCode != "DHA" || asked.AreaCode != "DHA-DHK-DHN" {
		t.Fatalf("config was asked about %+v", asked)
	}
}

// A partner who has never reported a location gets the global defaults rather
// than an error: they have no feed to compute either, and refusing to tell them
// their own settings would be refusing them the screen that explains why.
func TestAPartnerWithNoLocation(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	if _, err := r.partners.Register(ctx, "USR-1", "", application.RegisterRequest{
		Name: "Rafi", Phone: "+8801711111111",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := r.partners.SetAvailability(ctx, "USR-1", "available", ""); err != nil {
		t.Fatalf("SetAvailability: %v", err)
	}

	feed, err := r.partners.Feed(ctx, "USR-1", "en")
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if feed.Reason != "nothing_nearby" {
		t.Fatalf("feed = %+v", feed)
	}
	// Resolved against nowhere, so config was asked about the global default.
	last := r.config.seen[len(r.config.seen)-1]
	if last.DivisionCode != "" {
		t.Errorf("config was asked about %+v", last)
	}
}

// A geo lookup that fails falls back to the global settings rather than
// refusing a rider their screen.
func TestAFailedPlacementFallsBackToTheDefaults(t *testing.T) {
	r := newRig()
	r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.geo.areaErr = errBoom

	feed, err := r.partners.Feed(context.Background(), "USR-1", "en")
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if feed.RadiusM != 7000 {
		t.Fatalf("feed = %+v", feed)
	}
}

func TestFeedFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("the waiting query fails", func(t *testing.T) {
		r := newRig()
		r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		r.repo.nearbyErr = errBoom
		if _, err := r.partners.Feed(ctx, "USR-1", "en"); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("configuration is unreachable", func(t *testing.T) {
		r := newRig()
		r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		r.config.err = errBoom
		if _, err := r.partners.Feed(ctx, "USR-1", "en"); errs.CodeOf(err) != "dispatch_config_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the partner store is broken", func(t *testing.T) {
		r := newRig()
		r.repo.partnerErr = errBoom
		if _, err := r.partners.Feed(ctx, "USR-1", "en"); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the partner cannot be written", func(t *testing.T) {
		r := newRig()
		r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		r.repo.savePartErr = errBoom
		if _, err := r.partners.ReportLocation(ctx, "USR-1", 23.7, 90.4, ""); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
		if _, err := r.partners.SetAvailability(ctx, "USR-1", "offline", ""); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
		if _, err := r.partners.SetPreference(ctx, "USR-1", "long", ""); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a brand new partner cannot be written", func(t *testing.T) {
		r := newRig()
		r.repo.savePartErr = errBoom
		if _, err := r.partners.Register(ctx, "USR-1", "", application.RegisterRequest{
			Name: "Rafi", Phone: "x",
		}); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the job list fails", func(t *testing.T) {
		r := newRig()
		r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		r.repo.listErr = errBoom
		if _, err := r.partners.MyJobs(ctx, "USR-1", true, 20, 0, "en"); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})
}

// --------------------------------------------------------------- the journey

func TestARiderTakesAJobAllTheWay(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}

	accepted, err := r.partners.Accept(ctx, "USR-1", job.ID, "en")
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if accepted.Status != "assigned" {
		t.Fatalf("job = %+v", accepted)
	}
	// Taking a job counts on the partner's record, and fills a slot.
	if r.repo.partners[rider.ID].Carrying != 1 {
		t.Errorf("carrying = %d", r.repo.partners[rider.ID].Carrying)
	}
	if last := r.repo.offersRecorded[len(r.repo.offersRecorded)-1]; !last.accepted {
		t.Errorf("the acceptance was not recorded: %+v", r.repo.offersRecorded)
	}

	collected, err := r.partners.Collect(ctx, "USR-1", job.ID, "en")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if collected.Status != "collected" {
		t.Fatalf("job = %+v", collected)
	}
	// The order moved with it, through the way in the order module left open.
	if len(r.order.advanced) != 1 || r.order.advanced[0].to != "picked_up" ||
		r.order.advanced[0].actor != "partner" {
		t.Fatalf("order advances = %+v", r.order.advanced)
	}

	delivered, err := r.partners.Deliver(ctx, "USR-1", job.ID, "en")
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if delivered.Status != "delivered" || delivered.Live {
		t.Fatalf("job = %+v", delivered)
	}
	if r.repo.partners[rider.ID].Carrying != 0 {
		t.Errorf("the slot was not freed: %d", r.repo.partners[rider.ID].Carrying)
	}
	if len(r.order.advanced) != 2 || r.order.advanced[1].to != "delivered" {
		t.Fatalf("order advances = %+v", r.order.advanced)
	}
}

// The acceptance criterion: two partners cannot be assigned the same order.
// The second one loses the compare-and-set.
func TestTwoPartnersCannotTakeTheSameJob(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	first := r.onShift(t, "USR-1", "First", 23.746, 90.375, "")
	second := r.onShift(t, "USR-2", "Second", 23.7461, 90.3751, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[first.ID], DistanceM: 200}}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}

	if _, err := r.partners.Accept(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	// The second partner was never offered it, so the domain refuses them
	// before storage is even consulted.
	if _, err := r.partners.Accept(ctx, "USR-2", job.ID, "en"); errs.CodeOf(err) != "not_your_job" {
		t.Fatalf("err = %v, want not_your_job", err)
	}
	// And they cannot move it either.
	for name, run := range map[string]func() error{
		"collect": func() error { _, err := r.partners.Collect(ctx, "USR-2", job.ID, "en"); return err },
		"deliver": func() error { _, err := r.partners.Deliver(ctx, "USR-2", job.ID, "en"); return err },
		"fail":    func() error { _, err := r.partners.Fail(ctx, "USR-2", job.ID, "why", "en"); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); errs.CodeOf(err) != "not_your_job" {
				t.Fatalf("err = %v", err)
			}
		})
	}
	_ = second
}

func TestDecliningPutsAJobBackOnTheBoard(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	declined, err := r.partners.Decline(ctx, "USR-1", job.ID, "en")
	if err != nil {
		t.Fatalf("Decline: %v", err)
	}
	if declined.Status != "waiting" {
		t.Fatalf("job = %+v", declined)
	}
	if r.repo.partners[rider.ID].Carrying != 0 {
		t.Errorf("declining filled a slot: %d", r.repo.partners[rider.ID].Carrying)
	}
}

func TestFailingADelivery(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if _, err := r.partners.Accept(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if _, err := r.partners.Fail(ctx, "USR-1", job.ID, "", "en"); errs.CodeOf(err) != "reason_required" {
		t.Fatalf("a reasonless failure was accepted: %v", err)
	}

	// Giving up before collecting is not a failed delivery: the job goes back
	// on the board and the customer's order is untouched.
	gaveUp, err := r.partners.Fail(ctx, "USR-1", job.ID, "my bike broke down", "en")
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if gaveUp.Status != "waiting" {
		t.Fatalf("job = %+v, want it back on the board", gaveUp)
	}
	if len(r.order.advanced) != 0 {
		t.Fatalf("a broken bike moved the order: %+v", r.order.advanced)
	}
	if r.repo.partners[rider.ID].Carrying != 0 {
		t.Errorf("the slot was not freed: %d", r.repo.partners[rider.ID].Carrying)
	}

	// Once the rider has the food, it is a real failure and the order says so.
	// The sweep is what puts the job back to them — nobody else is around.
	if _, err := r.offers.Sweep(ctx, 50); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if _, err := r.partners.Accept(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if _, err := r.partners.Collect(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	failed, err := r.partners.Fail(ctx, "USR-1", job.ID, "nobody at the address", "en")
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if failed.Status != "failed" || failed.Reason != "nobody at the address" {
		t.Fatalf("job = %+v", failed)
	}
	if len(r.order.advanced) != 2 || r.order.advanced[1].to != "failed" ||
		r.order.advanced[1].reason != "nobody at the address" {
		t.Fatalf("order advances = %+v", r.order.advanced)
	}
	if r.repo.partners[rider.ID].Carrying != 0 {
		t.Errorf("the slot was not freed: %d", r.repo.partners[rider.ID].Carrying)
	}
}

func TestJobActionFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("no such job", func(t *testing.T) {
		r := newRig()
		r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		if _, err := r.partners.Accept(ctx, "USR-1", "JOB-nope", "en"); errs.CodeOf(err) != "job_not_found" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the job store is broken", func(t *testing.T) {
		r := newRig()
		r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		r.repo.jobErr = errBoom
		if _, err := r.partners.Accept(ctx, "USR-1", "JOB-1", "en"); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the write loses the race", func(t *testing.T) {
		r := newRig()
		rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
		r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}
		job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
		if err != nil {
			t.Fatalf("Offer: %v", err)
		}
		r.repo.saveJobErr = errBoom
		if _, err := r.partners.Accept(ctx, "USR-1", job.ID, "en"); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})
}

// ------------------------------------------------------------------ the sweep

func TestTheSweepPutsLapsedOffersBack(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if job.Status != "offered" {
		t.Fatalf("job = %+v", job)
	}

	// Before the timeout, nothing is swept.
	swept, err := r.offers.Sweep(ctx, 50)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if swept.Expired != 0 {
		t.Fatalf("swept %d live offers", swept.Expired)
	}

	r.clock.now = at.Add(31 * time.Second)
	swept, err = r.offers.Sweep(ctx, 50)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if swept.Expired != 1 {
		t.Fatalf("expired = %d, want 1", swept.Expired)
	}
	// The second pass finds the job it just freed. The only rider nearby is the
	// one who let it lapse, and in a town with one rider they are the only way
	// it gets delivered, so it goes back to them rather than nowhere.
	if swept.Offered != 1 {
		t.Fatalf("offered = %d, want 1", swept.Offered)
	}
	if r.repo.jobs[job.ID].Status != domain.JobOffered {
		t.Fatalf("job = %+v", r.repo.jobs[job.ID])
	}
	// A lapsed offer counts against the rate exactly like a decline, because to
	// the customer it was the same thing: a rider who was asked and did not
	// come.
	if last := r.repo.offersRecorded[len(r.repo.offersRecorded)-1]; last.accepted {
		t.Errorf("offers = %+v", r.repo.offersRecorded)
	}

	// Running it again is safe.
	if _, err := r.offers.Sweep(ctx, 50); err != nil {
		t.Fatalf("Sweep again: %v", err)
	}
}

// The second pass is the only thing that re-offers a job. Without it a declined
// delivery would sit on the board forever, because a decline is the one way
// onto the board that no partner's own action takes it off again.
func TestTheSweepReoffersWaitingJobs(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rafi := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	nadia := r.onShift(t, "USR-2", "Nadia", 23.747, 90.376, "")
	r.repo.nearby = []ports.PartnerDistance{
		{Partner: r.repo.partners[rafi.ID], DistanceM: 200},
		{Partner: r.repo.partners[nadia.ID], DistanceM: 400},
	}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if _, err := r.partners.Decline(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Decline: %v", err)
	}

	swept, err := r.offers.Sweep(ctx, 50)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if swept.Offered != 1 {
		t.Fatalf("offered = %d, want 1", swept.Offered)
	}
	held := r.repo.jobs[job.ID]
	if held.Status != domain.JobOffered || held.PartnerID != nadia.ID {
		t.Fatalf("the declined job went to %+v, want %s", held, nadia.ID)
	}
	// And the hint is spent: the next round may ask Rafi again.
	if held.PassedBy != "" {
		t.Errorf("PassedBy = %q after a fresh offer", held.PassedBy)
	}
}

func TestSweepFailures(t *testing.T) {
	r := newRig()
	r.repo.lapsedErr = errBoom
	if _, err := r.offers.Sweep(context.Background(), 50); errs.CodeOf(err) != "dispatch_unavailable" {
		t.Fatalf("err = %v", err)
	}

	// The board being unreadable is the same kind of outage as the offers
	// being unreadable, and must not be reported as a quiet sweep.
	waiting := newRig()
	waiting.repo.waitingErr = errBoom
	if _, err := waiting.offers.Sweep(context.Background(), 50); errs.CodeOf(err) != "dispatch_unavailable" {
		t.Fatalf("err = %v", err)
	}

	// A job somebody accepted while the sweep was running is skipped, not
	// forced back.
	raced := newRig()
	ctx := context.Background()
	rider := raced.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	raced.repo.nearby = []ports.PartnerDistance{{Partner: raced.repo.partners[rider.ID], DistanceM: 200}}
	if _, err := raced.service.Offer(ctx, offerRequest("ORD-1", 3000)); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	raced.clock.now = at.Add(31 * time.Second)
	raced.repo.saveJobErr = errBoom
	swept, err := raced.offers.Sweep(ctx, 50)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if swept.Expired != 0 || swept.Offered != 0 {
		t.Fatalf("swept = %+v, want nothing", swept)
	}
}

// One area whose settings cannot be read must not stop the sweep for every
// other area — a job is skipped this pass and tried again on the next.
func TestTheSweepSkipsAJobItCannotPrice(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if _, err := r.partners.Decline(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Decline: %v", err)
	}

	r.config.err = errBoom
	swept, err := r.offers.Sweep(ctx, 50)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if swept.Offered != 0 {
		t.Fatalf("offered = %d with unreadable settings", swept.Offered)
	}
	if r.repo.jobs[job.ID].Status != domain.JobWaiting {
		t.Fatalf("job = %+v", r.repo.jobs[job.ID])
	}
}

// A round that cannot read the candidate pool leaves the job where it is.
func TestTheSweepSurvivesARoundThatFails(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if _, err := r.partners.Decline(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Decline: %v", err)
	}

	r.repo.availErr = errBoom
	swept, err := r.offers.Sweep(ctx, 50)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if swept.Offered != 0 {
		t.Fatalf("offered = %d with no candidate pool", swept.Offered)
	}
}

// ------------------------------------------------------------------ withdraw

func TestWithdrawingAJob(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if _, err := r.partners.Accept(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if err := r.service.Withdraw(ctx, "ORD-1", "the customer called it off"); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if r.repo.jobs[job.ID].Status != domain.JobCancelled {
		t.Fatalf("job = %+v", r.repo.jobs[job.ID])
	}
	// The rider's slot is freed: they are not carrying something that no
	// longer exists.
	if r.repo.partners[rider.ID].Carrying != 0 {
		t.Errorf("carrying = %d", r.repo.partners[rider.ID].Carrying)
	}

	// Withdrawing again is safe, and so is withdrawing an order that never had
	// a job — an order cancelled before it was ever ready.
	if err := r.service.Withdraw(ctx, "ORD-1", "again"); err != nil {
		t.Fatalf("Withdraw again: %v", err)
	}
	if err := r.service.Withdraw(ctx, "ORD-never", "nothing to do"); err != nil {
		t.Fatalf("Withdraw nothing: %v", err)
	}
}

func TestWithdrawFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("the store cannot be read", func(t *testing.T) {
		r := newRig()
		r.repo.jobErr = errBoom
		if err := r.service.Withdraw(ctx, "ORD-1", ""); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the write fails", func(t *testing.T) {
		r := newRig()
		if _, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000)); err != nil {
			t.Fatalf("Offer: %v", err)
		}
		r.repo.saveJobErr = errBoom
		if err := r.service.Withdraw(ctx, "ORD-1", ""); errs.CodeOf(err) != "dispatch_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})
}

// ----------------------------------------------------------------- contract

func TestJobForOrder(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	var api contract.DispatchContract = r.service

	// Before an order is ready there is no job, which is most of its life — so
	// a tracking screen gets "no rider yet" rather than a not-found.
	if _, found, err := api.JobForOrder(ctx, "ORD-1"); err != nil || found {
		t.Fatalf("found = %v, err = %v", found, err)
	}

	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}
	if _, err := api.Offer(ctx, offerRequest("ORD-1", 3000)); err != nil {
		t.Fatalf("Offer: %v", err)
	}

	got, found, err := api.JobForOrder(ctx, "ORD-1")
	if err != nil || !found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if got.OrderID != "ORD-1" || got.Partner.Name != "Rafi" {
		t.Fatalf("job = %+v", got)
	}

	broken := newRig()
	broken.repo.jobErr = errBoom
	if _, _, err := broken.service.JobForOrder(ctx, "ORD-1"); errs.CodeOf(err) != "dispatch_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

// PartnerOfUser is how payment's COD ledger resolves a signed-in rider to the
// partner id dispatch already uses as the actor on a "delivered" event.
func TestPartnerOfUserOverTheContract(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	var api contract.DispatchContract = r.service

	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")

	if id, found, err := api.PartnerOfUser(ctx, "USR-1"); err != nil || !found || id != rider.ID {
		t.Fatalf("id = %q, found = %v, err = %v", id, found, err)
	}

	// An account that never registered as a partner is not an error — most
	// accounts are not partners.
	if _, found, err := api.PartnerOfUser(ctx, "USR-CUSTOMER"); err != nil || found {
		t.Fatalf("found = %v, err = %v", found, err)
	}

	broken := newRig()
	broken.repo.partnerErr = errBoom
	if _, _, err := broken.service.PartnerOfUser(ctx, "USR-1"); errs.CodeOf(err) != "dispatch_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

// A job whose partner cannot be read is still a job. The caller gets the
// delivery without the rider's details rather than an outage.
func TestAJobWhosePartnerCannotBeRead(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}
	if _, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000)); err != nil {
		t.Fatalf("Offer: %v", err)
	}

	delete(r.repo.partners, rider.ID)
	got, found, err := r.service.JobForOrder(ctx, "ORD-1")
	if err != nil || !found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if got.Partner.Name != "" {
		t.Errorf("partner = %+v", got.Partner)
	}
}

func TestMyJobs(t *testing.T) {
	r := newRig()
	ctx := context.Background()
	rider := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[rider.ID], DistanceM: 200}}

	job, err := r.service.Offer(ctx, offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if _, err := r.partners.Accept(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	live, err := r.partners.MyJobs(ctx, "USR-1", true, 20, 0, "en")
	if err != nil {
		t.Fatalf("MyJobs: %v", err)
	}
	if live.Total != 1 || live.Jobs[0].ID != job.ID {
		t.Fatalf("jobs = %+v", live)
	}

	if _, err := r.partners.Collect(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if _, err := r.partners.Deliver(ctx, "USR-1", job.ID, "en"); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	stillLive, err := r.partners.MyJobs(ctx, "USR-1", true, 20, 0, "en")
	if err != nil {
		t.Fatalf("MyJobs: %v", err)
	}
	if stillLive.Total != 0 {
		t.Fatalf("a delivered job is still live: %+v", stillLive)
	}
	all, err := r.partners.MyJobs(ctx, "USR-1", false, 0, 0, "en")
	if err != nil {
		t.Fatalf("MyJobs: %v", err)
	}
	if all.Total != 1 {
		t.Fatalf("history = %+v", all)
	}
}
