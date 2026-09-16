package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	dispatchports "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application/ports"
	dispatchdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
	dispatchpg "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/infrastructure/persistence/postgres"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// withDispatch runs a test inside a transaction that is always rolled back.
func withDispatch(t *testing.T, fn func(ctx context.Context, tx pgx.Tx, repo *dispatchpg.Repository)) {
	t.Helper()
	ctx := context.Background()
	conn := connect(t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	fn(ctx, tx, dispatchpg.New(tx))
}

// dispatchClock is a fixed instant the whole file shares, truncated because
// Postgres keeps microseconds and Go keeps nanoseconds.
func dispatchClock() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func samplePartner(ctx context.Context, t *testing.T, repo *dispatchpg.Repository,
	id, userID, name string, lat, lng float64,
) dispatchdomain.Partner {
	t.Helper()
	p, err := dispatchdomain.NewPartner(id, userID, name, "+8801711111111", "motorcycle")
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}
	p, err = p.WithAvailability(dispatchdomain.AvailabilityAvailable)
	if err != nil {
		t.Fatalf("WithAvailability: %v", err)
	}
	p = p.At(lat, lng)
	if err := repo.SavePartner(ctx, p); err != nil {
		t.Fatalf("SavePartner: %v", err)
	}
	return p
}

func sampleJob(t *testing.T, id, orderID string, distanceM float64, band dispatchdomain.Band) dispatchdomain.Job {
	t.Helper()
	job, err := dispatchdomain.NewJob(id, orderID, "ABC234",
		dispatchdomain.Place{
			Name: "Star Kabab", Phone: "+8801711000001",
			SingleLine: "Road 27, Dhanmondi", Lat: 23.7455, Lng: 90.3738,
		},
		dispatchdomain.Place{
			Name: "Fardin", Phone: "+8801711111111",
			SingleLine: "House 5, Dhanmondi", Lat: 23.746, Lng: 90.375,
		},
		distanceM, band,
		dispatchdomain.Placement{
			AreaCode: "DHA-DHK-DHN", DistrictCode: "DHA-DHK", DivisionCode: "DHA",
		},
		dispatchClock())
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	return job
}

// A partner round-trips with their location and their counters intact. The
// counters matter: they are what the acceptance rate is computed from, and
// SavePartner must not be able to reset them by writing a stale copy.
func TestAPartnerRoundTrips(t *testing.T) {
	withDispatch(t, func(ctx context.Context, _ pgx.Tx, repo *dispatchpg.Repository) {
		saved := samplePartner(ctx, t, repo, "prt_1", "usr_1", "Rafi", 23.746, 90.375)

		got, err := repo.Partner(ctx, saved.ID)
		if err != nil {
			t.Fatalf("Partner: %v", err)
		}
		if got.Name != "Rafi" || got.Availability != dispatchdomain.AvailabilityAvailable {
			t.Fatalf("partner = %+v", got)
		}
		// PostGIS stores the point as x=lng, y=lat. Reading it back the other
		// way round would put every rider in the Indian Ocean.
		if int(got.Lat*1000) != 23746 || int(got.Lng*1000) != 90375 {
			t.Fatalf("pin = %f, %f", got.Lat, got.Lng)
		}

		byUser, found, err := repo.PartnerOfUser(ctx, "usr_1")
		if err != nil || !found || byUser.ID != saved.ID {
			t.Fatalf("PartnerOfUser = %+v, %v, %v", byUser, found, err)
		}
		_, found, err = repo.PartnerOfUser(ctx, "usr_nobody")
		if err != nil || found {
			t.Fatalf("an account with no partner record = %v, %v", found, err)
		}

		if _, err := repo.Partner(ctx, "prt_missing"); errs.CodeOf(err) != "partner_not_found" {
			t.Fatalf("err = %v", err)
		}
	})
}

// RecordOffer is a counter increment rather than a read-modify-write, so two
// offers made at the same moment cannot lose each other.
func TestTheAcceptanceCountersAreIncrements(t *testing.T) {
	withDispatch(t, func(ctx context.Context, _ pgx.Tx, repo *dispatchpg.Repository) {
		partner := samplePartner(ctx, t, repo, "prt_1", "usr_1", "Rafi", 23.746, 90.375)

		for range 3 {
			if err := repo.RecordOffer(ctx, partner.ID, false); err != nil {
				t.Fatalf("RecordOffer: %v", err)
			}
		}
		if err := repo.RecordOffer(ctx, partner.ID, true); err != nil {
			t.Fatalf("RecordOffer: %v", err)
		}

		// A save in between must not roll the counters back, which is the bug
		// a read-modify-write would have.
		moved := partner.At(23.75, 90.38)
		if err := repo.SavePartner(ctx, moved); err != nil {
			t.Fatalf("SavePartner: %v", err)
		}

		got, err := repo.Partner(ctx, partner.ID)
		if err != nil {
			t.Fatalf("Partner: %v", err)
		}
		if got.Offered != 3 || got.Accepted != 1 {
			t.Fatalf("counters = %d offered, %d accepted", got.Offered, got.Accepted)
		}
	})
}

// The candidate pool is a spatial query with D4 in its own WHERE clause: a
// partner who has chosen short runs is not a candidate for a long one, and a
// partner outside the radius is not a candidate at all.
func TestTheCandidatePoolIsSpatialAndHonoursD4(t *testing.T) {
	withDispatch(t, func(ctx context.Context, _ pgx.Tx, repo *dispatchpg.Repository) {
		pickup := dispatchdomain.Place{Lat: 23.7455, Lng: 90.3738}

		near := samplePartner(ctx, t, repo, "prt_near", "usr_near", "Near", 23.7456, 90.3739)
		far := samplePartner(ctx, t, repo, "prt_far", "usr_far", "Far", 23.85, 90.48)
		shortOnly := samplePartner(ctx, t, repo, "prt_short", "usr_short", "Short", 23.7457, 90.3740)
		updated, err := shortOnly.WithPreference(dispatchdomain.PreferenceShort)
		if err != nil {
			t.Fatalf("WithPreference: %v", err)
		}
		if err := repo.SavePartner(ctx, updated); err != nil {
			t.Fatalf("SavePartner: %v", err)
		}
		offline := samplePartner(ctx, t, repo, "prt_off", "usr_off", "Off", 23.7458, 90.3741)
		gone, err := offline.WithAvailability(dispatchdomain.AvailabilityOffline)
		if err != nil {
			t.Fatalf("WithAvailability: %v", err)
		}
		if err := repo.SavePartner(ctx, gone); err != nil {
			t.Fatalf("SavePartner: %v", err)
		}

		shortPool, err := repo.AvailableWithin(ctx, pickup, 3000, dispatchdomain.BandShort, 50)
		if err != nil {
			t.Fatalf("AvailableWithin: %v", err)
		}
		ids := map[string]bool{}
		for _, c := range shortPool {
			ids[c.Partner.ID] = true
		}
		if !ids[near.ID] || !ids[shortOnly.ID] {
			t.Fatalf("short pool = %v, want both nearby riders", ids)
		}
		if ids[far.ID] {
			t.Errorf("a rider 12 km away was in a 3 km pool")
		}
		if ids[gone.ID] {
			t.Errorf("an offline rider was in the pool")
		}

		longPool, err := repo.AvailableWithin(ctx, pickup, 3000, dispatchdomain.BandLong, 50)
		if err != nil {
			t.Fatalf("AvailableWithin: %v", err)
		}
		for _, c := range longPool {
			if c.Partner.ID == shortOnly.ID {
				t.Fatalf("a short-runs-only rider was offered a long job (D4)")
			}
		}

		// The pool comes back nearest first, which is what ALG-04 weights most.
		if len(shortPool) >= 2 && shortPool[0].DistanceM > shortPool[1].DistanceM {
			t.Errorf("pool = %+v, want it ordered by distance", shortPool)
		}
	})
}

// A job round-trips with both ends, its placement and its band. The placement
// is the sweeper's only way to know which area's settings govern the job.
func TestAJobRoundTrips(t *testing.T) {
	withDispatch(t, func(ctx context.Context, _ pgx.Tx, repo *dispatchpg.Repository) {
		job := sampleJob(t, "job_1", "ord_1", 3000, dispatchdomain.BandShort)
		if err := repo.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob: %v", err)
		}

		got, err := repo.Job(ctx, job.ID)
		if err != nil {
			t.Fatalf("Job: %v", err)
		}
		if got.Status != dispatchdomain.JobWaiting || got.PartnerID != "" {
			t.Fatalf("job = %+v", got)
		}
		if got.Placement.AreaCode != "DHA-DHK-DHN" || got.Placement.DivisionCode != "DHA" {
			t.Fatalf("placement = %+v", got.Placement)
		}
		if got.Pickup.Name != "Star Kabab" || int(got.Destination.Lat*1000) != 23746 {
			t.Fatalf("ends = %+v / %+v", got.Pickup, got.Destination)
		}
		// A waiting job has no offer clock, and that must read back as the zero
		// time rather than as an epoch timestamp.
		if !got.OfferedAt.IsZero() || !got.OfferExpiresAt.IsZero() {
			t.Fatalf("a waiting job has an offer clock: %+v", got)
		}

		byOrder, found, err := repo.JobForOrder(ctx, "ord_1")
		if err != nil || !found || byOrder.ID != job.ID {
			t.Fatalf("JobForOrder = %+v, %v, %v", byOrder, found, err)
		}
		_, found, err = repo.JobForOrder(ctx, "ord_none")
		if err != nil || found {
			t.Fatalf("an order with no job = %v, %v", found, err)
		}

		if _, err := repo.Job(ctx, "job_missing"); errs.CodeOf(err) != "job_not_found" {
			t.Fatalf("err = %v", err)
		}
	})
}

// One job per order, by a unique index rather than a check: an order that
// becomes ready twice must not become two jobs for two riders.
func TestAnOrderCannotHaveTwoJobs(t *testing.T) {
	withDispatch(t, func(ctx context.Context, _ pgx.Tx, repo *dispatchpg.Repository) {
		if err := repo.CreateJob(ctx, sampleJob(t, "job_1", "ord_1", 3000, dispatchdomain.BandShort)); err != nil {
			t.Fatalf("CreateJob: %v", err)
		}
		err := repo.CreateJob(ctx, sampleJob(t, "job_2", "ord_1", 3000, dispatchdomain.BandShort))
		if errs.CodeOf(err) != "job_exists" {
			t.Fatalf("a second job for one order = %v", err)
		}
	})
}

// The phase's third acceptance criterion: two partners cannot be assigned the
// same order. The compare-and-set is in the UPDATE's own WHERE clause, so the
// second write finds nothing to update rather than overwriting the first.
func TestTwoPartnersCannotTakeTheSameJob(t *testing.T) {
	withDispatch(t, func(ctx context.Context, _ pgx.Tx, repo *dispatchpg.Repository) {
		rafi := samplePartner(ctx, t, repo, "prt_1", "usr_1", "Rafi", 23.746, 90.375)
		nadia := samplePartner(ctx, t, repo, "prt_2", "usr_2", "Nadia", 23.747, 90.376)

		job := sampleJob(t, "job_1", "ord_1", 3000, dispatchdomain.BandShort)
		if err := repo.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob: %v", err)
		}

		now := dispatchClock()
		first := job
		if err := first.Offer(rafi.ID, now, time.Minute); err != nil {
			t.Fatalf("Offer: %v", err)
		}
		if err := repo.SaveJob(ctx, first, dispatchdomain.JobWaiting); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}

		// Nadia's client read the job while it was still waiting.
		second := job
		if err := second.Offer(nadia.ID, now, time.Minute); err != nil {
			t.Fatalf("Offer: %v", err)
		}
		err := repo.SaveJob(ctx, second, dispatchdomain.JobWaiting)
		if errs.CodeOf(err) != "job_moved" {
			t.Fatalf("the second write = %v, want a conflict", err)
		}

		held, err := repo.Job(ctx, job.ID)
		if err != nil {
			t.Fatalf("Job: %v", err)
		}
		if held.PartnerID != rafi.ID {
			t.Fatalf("the job went to %q, want %q", held.PartnerID, rafi.ID)
		}
		if held.OfferExpiresAt.IsZero() {
			t.Fatalf("an offered job has no clock: %+v", held)
		}
	})
}

// A declined job records who passed on it, so the next round can ask somebody
// else. The hint has to survive the round trip or it is not a hint at all.
func TestADeclineIsRemembered(t *testing.T) {
	withDispatch(t, func(ctx context.Context, _ pgx.Tx, repo *dispatchpg.Repository) {
		rafi := samplePartner(ctx, t, repo, "prt_1", "usr_1", "Rafi", 23.746, 90.375)
		job := sampleJob(t, "job_1", "ord_1", 3000, dispatchdomain.BandShort)
		if err := repo.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob: %v", err)
		}

		now := dispatchClock()
		if err := job.Offer(rafi.ID, now, time.Minute); err != nil {
			t.Fatalf("Offer: %v", err)
		}
		if err := repo.SaveJob(ctx, job, dispatchdomain.JobWaiting); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}
		if err := job.Decline(rafi.ID, now); err != nil {
			t.Fatalf("Decline: %v", err)
		}
		if err := repo.SaveJob(ctx, job, dispatchdomain.JobOffered); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}

		got, err := repo.Job(ctx, job.ID)
		if err != nil {
			t.Fatalf("Job: %v", err)
		}
		if got.Status != dispatchdomain.JobWaiting || got.PassedBy != rafi.ID {
			t.Fatalf("job = %+v, want it waiting with Rafi recorded as having passed", got)
		}
		// Attempts survives too: an order nobody wants is an operational fact
		// somebody has to be able to see.
		if got.Attempts != 1 {
			t.Errorf("attempts = %d, want 1", got.Attempts)
		}
	})
}

// The feed is a spatial query over waiting jobs, nearest pickup first (ALG-08).
func TestTheFeedQueryIsSpatial(t *testing.T) {
	withDispatch(t, func(ctx context.Context, _ pgx.Tx, repo *dispatchpg.Repository) {
		nearJob := sampleJob(t, "job_near", "ord_near", 3000, dispatchdomain.BandShort)
		farJob := sampleJob(t, "job_far", "ord_far", 3000, dispatchdomain.BandShort)
		farJob.Pickup.Lat, farJob.Pickup.Lng = 23.7500, 90.3800
		for _, j := range []dispatchdomain.Job{nearJob, farJob} {
			if err := repo.CreateJob(ctx, j); err != nil {
				t.Fatalf("CreateJob: %v", err)
			}
		}

		at := dispatchdomain.Place{Lat: 23.7455, Lng: 90.3738}
		feed, err := repo.WaitingNear(ctx, at, 3000, 20)
		if err != nil {
			t.Fatalf("WaitingNear: %v", err)
		}
		if len(feed) != 2 {
			t.Fatalf("feed = %+v, want both waiting jobs", feed)
		}
		if feed[0].Job.ID != nearJob.ID {
			t.Fatalf("feed[0] = %s, want the nearer pickup first", feed[0].Job.ID)
		}
		if feed[0].DistanceM > feed[1].DistanceM {
			t.Errorf("distances = %f, %f", feed[0].DistanceM, feed[1].DistanceM)
		}

		// A tight radius excludes the far pickup rather than merely ranking it
		// lower, because a rider cannot be shown work they cannot reach.
		tight, err := repo.WaitingNear(ctx, at, 100, 20)
		if err != nil {
			t.Fatalf("WaitingNear: %v", err)
		}
		if len(tight) != 1 || tight[0].Job.ID != nearJob.ID {
			t.Fatalf("tight feed = %+v", tight)
		}
	})
}

// The sweeper's two queries: offers that have run out, and the board.
func TestTheSweeperQueries(t *testing.T) {
	withDispatch(t, func(ctx context.Context, _ pgx.Tx, repo *dispatchpg.Repository) {
		rafi := samplePartner(ctx, t, repo, "prt_1", "usr_1", "Rafi", 23.746, 90.375)

		lapsing := sampleJob(t, "job_lapsing", "ord_lapsing", 3000, dispatchdomain.BandShort)
		live := sampleJob(t, "job_live", "ord_live", 3000, dispatchdomain.BandShort)
		waiting := sampleJob(t, "job_waiting", "ord_waiting", 3000, dispatchdomain.BandShort)
		for _, j := range []dispatchdomain.Job{lapsing, live, waiting} {
			if err := repo.CreateJob(ctx, j); err != nil {
				t.Fatalf("CreateJob: %v", err)
			}
		}

		past := dispatchClock().Add(-2 * time.Minute)
		if err := lapsing.Offer(rafi.ID, past, time.Minute); err != nil {
			t.Fatalf("Offer: %v", err)
		}
		if err := repo.SaveJob(ctx, lapsing, dispatchdomain.JobWaiting); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}
		if err := live.Offer(rafi.ID, dispatchClock(), time.Hour); err != nil {
			t.Fatalf("Offer: %v", err)
		}
		if err := repo.SaveJob(ctx, live, dispatchdomain.JobWaiting); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}

		lapsed, err := repo.LapsedOffers(ctx, 50)
		if err != nil {
			t.Fatalf("LapsedOffers: %v", err)
		}
		if len(lapsed) != 1 || lapsed[0].ID != lapsing.ID {
			t.Fatalf("lapsed = %+v, want only the expired offer", lapsed)
		}

		board, err := repo.WaitingJobs(ctx, 50)
		if err != nil {
			t.Fatalf("WaitingJobs: %v", err)
		}
		if len(board) != 1 || board[0].ID != waiting.ID {
			t.Fatalf("board = %+v, want only the waiting job", board)
		}
		// The board carries its placement, or the sweeper cannot resolve which
		// area's settings govern a re-offer.
		if board[0].Placement.AreaCode != "DHA-DHK-DHN" {
			t.Fatalf("placement = %+v", board[0].Placement)
		}
	})
}

// A partner's own work, filtered and paged.
func TestListingAPartnersJobs(t *testing.T) {
	withDispatch(t, func(ctx context.Context, _ pgx.Tx, repo *dispatchpg.Repository) {
		rafi := samplePartner(ctx, t, repo, "prt_1", "usr_1", "Rafi", 23.746, 90.375)
		now := dispatchClock()

		for i, id := range []string{"job_1", "job_2", "job_3"} {
			job := sampleJob(t, id, "ord_"+id, 3000, dispatchdomain.BandShort)
			job.CreatedAt = now.Add(time.Duration(i) * time.Second)
			job.UpdatedAt = job.CreatedAt
			if err := repo.CreateJob(ctx, job); err != nil {
				t.Fatalf("CreateJob: %v", err)
			}
			if err := job.Offer(rafi.ID, now, time.Hour); err != nil {
				t.Fatalf("Offer: %v", err)
			}
			if err := job.Accept(rafi.ID, now); err != nil {
				t.Fatalf("Accept: %v", err)
			}
			if id == "job_1" {
				if err := job.Collect(rafi.ID, now); err != nil {
					t.Fatalf("Collect: %v", err)
				}
				if err := job.Deliver(rafi.ID, now); err != nil {
					t.Fatalf("Deliver: %v", err)
				}
			}
			if err := repo.SaveJob(ctx, job, dispatchdomain.JobWaiting); err != nil {
				t.Fatalf("SaveJob: %v", err)
			}
		}

		live, total, err := repo.Jobs(ctx, dispatchports.JobFilter{
			PartnerID: rafi.ID, LiveOnly: true, Limit: 10,
		})
		if err != nil {
			t.Fatalf("Jobs: %v", err)
		}
		if total != 2 || len(live) != 2 {
			t.Fatalf("live = %d of %d, want 2", len(live), total)
		}
		// Newest first, so the job a rider is working on now is at the top.
		if live[0].ID != "job_3" {
			t.Fatalf("live[0] = %s, want job_3", live[0].ID)
		}

		all, total, err := repo.Jobs(ctx, dispatchports.JobFilter{
			PartnerID: rafi.ID, Limit: 2, Offset: 2,
		})
		if err != nil {
			t.Fatalf("Jobs: %v", err)
		}
		if total != 3 || len(all) != 1 {
			t.Fatalf("page = %d of %d, want the last of three", len(all), total)
		}

		delivered, total, err := repo.Jobs(ctx, dispatchports.JobFilter{
			PartnerID: rafi.ID,
			Statuses:  []dispatchdomain.JobStatus{dispatchdomain.JobDelivered},
			Limit:     10,
		})
		if err != nil {
			t.Fatalf("Jobs: %v", err)
		}
		if total != 1 || delivered[0].ID != "job_1" {
			t.Fatalf("delivered = %+v", delivered)
		}

		none, total, err := repo.Jobs(ctx, dispatchports.JobFilter{PartnerID: "prt_nobody", Limit: 10})
		if err != nil || total != 0 || none != nil {
			t.Fatalf("a partner with no jobs = %+v, %d, %v", none, total, err)
		}
	})
}
