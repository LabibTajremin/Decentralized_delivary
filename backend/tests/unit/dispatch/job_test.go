package dispatch

import (
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
)

const timeout = 30 * time.Second

func job(t *testing.T) domain.Job {
	t.Helper()
	return waitingJob(t, "JOB-1", domain.BandShort)
}

func TestNewJobRefusesAnImpossibleDelivery(t *testing.T) {
	_, err := domain.NewJob("JOB-1", "", "CODE12", domain.Place{}, domain.Place{}, 100, domain.BandShort, domain.Placement{}, at)
	if !errors.Is(err, domain.ErrNoOrder) {
		t.Errorf("a job about nothing was accepted: %v", err)
	}
	_, err = domain.NewJob("JOB-1", "ORD-1", "CODE12", domain.Place{}, domain.Place{}, -1, domain.BandShort, domain.Placement{}, at)
	if !errors.Is(err, domain.ErrNegativeDistance) {
		t.Errorf("a negative distance was accepted: %v", err)
	}
}

// The whole path, through the one partner who holds it.
func TestAJobGoesAllTheWay(t *testing.T) {
	j := job(t)
	if j.Status != domain.JobWaiting || !j.Status.IsLive() {
		t.Fatalf("job = %+v", j)
	}

	if err := j.Offer("PTR-1", at, timeout); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if j.Status != domain.JobOffered || j.PartnerID != "PTR-1" || j.Attempts != 1 {
		t.Fatalf("job = %+v", j)
	}
	if !j.OfferExpiresAt.Equal(at.Add(timeout)) {
		t.Errorf("expires at %v", j.OfferExpiresAt)
	}

	if err := j.Accept("PTR-1", at.Add(time.Second)); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if j.Status != domain.JobAssigned || !j.OfferExpiresAt.IsZero() {
		t.Fatalf("job = %+v", j)
	}

	if err := j.Collect("PTR-1", at.Add(2*time.Minute)); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if err := j.Deliver("PTR-1", at.Add(20*time.Minute)); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if j.Status != domain.JobDelivered || j.Status.IsLive() {
		t.Fatalf("job = %+v", j)
	}
}

// The other half of "two partners cannot be assigned the same order": a partner
// who is not holding a job cannot move it.
func TestOnlyThePartnerHoldingAJobCanMoveIt(t *testing.T) {
	j := job(t)
	if err := j.Offer("PTR-1", at, timeout); err != nil {
		t.Fatalf("Offer: %v", err)
	}

	for _, move := range []struct {
		name string
		run  func(string) error
	}{
		{"accept", func(id string) error { return j.Accept(id, at) }},
		{"decline", func(id string) error { return j.Decline(id, at) }},
		{"collect", func(id string) error { return j.Collect(id, at) }},
		{"deliver", func(id string) error { return j.Deliver(id, at) }},
		{"fail", func(id string) error { return j.Fail(id, "nobody in", at) }},
	} {
		t.Run(move.name+" by somebody else", func(t *testing.T) {
			if err := move.run("PTR-2"); !errors.Is(err, domain.ErrJobNotYours) {
				t.Fatalf("err = %v, want ErrJobNotYours", err)
			}
		})
	}

	// And a job nobody holds cannot be moved at all.
	waiting := job(t)
	if err := waiting.Accept("PTR-1", at); !errors.Is(err, domain.ErrJobUnassigned) {
		t.Errorf("err = %v, want ErrJobUnassigned", err)
	}
}

// A partner who opens the app, sees an offer and puts the phone in their pocket
// must not hold a customer's dinner hostage.
func TestAnOfferLapses(t *testing.T) {
	j := job(t)
	if err := j.Offer("PTR-1", at, timeout); err != nil {
		t.Fatalf("Offer: %v", err)
	}

	if j.OfferLapsed(at.Add(29 * time.Second)) {
		t.Error("an offer lapsed early")
	}
	// Exactly at the deadline it is gone: a rider who taps at the instant it
	// expires gets the same answer as one who taps a second later, which is
	// the only version a countdown can be honest about.
	if !j.OfferLapsed(at.Add(timeout)) {
		t.Error("an offer at its deadline was still open")
	}

	if err := j.Accept("PTR-1", at.Add(timeout)); !errors.Is(err, domain.ErrOfferExpired) {
		t.Fatalf("err = %v, want ErrOfferExpired", err)
	}

	if err := j.Lapse(at.Add(timeout)); err != nil {
		t.Fatalf("Lapse: %v", err)
	}
	if j.Status != domain.JobWaiting || j.PartnerID != "" || !j.OfferedAt.IsZero() {
		t.Fatalf("job = %+v", j)
	}
	// The attempt is still counted. An order nobody wants is an operational
	// fact somebody has to see.
	if j.Attempts != 1 {
		t.Errorf("attempts = %d", j.Attempts)
	}

	// Lapsing something that has not lapsed is refused.
	fresh := job(t)
	if err := fresh.Lapse(at); !errors.Is(err, domain.ErrJobIllegalMove) {
		t.Errorf("err = %v", err)
	}
}

// A lapsed offer can be handed to somebody else without clearing it first,
// which is what the sweeper racing the next round looks like.
func TestALapsedOfferCanBeReoffered(t *testing.T) {
	j := job(t)
	if err := j.Offer("PTR-1", at, timeout); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if err := j.Offer("PTR-2", at.Add(timeout), timeout); err != nil {
		t.Fatalf("re-offer: %v", err)
	}
	if j.PartnerID != "PTR-2" || j.Attempts != 2 {
		t.Fatalf("job = %+v", j)
	}

	// But a live offer cannot be taken from under the partner holding it.
	live := job(t)
	if err := live.Offer("PTR-1", at, timeout); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if err := live.Offer("PTR-2", at.Add(time.Second), timeout); !errors.Is(err, domain.ErrJobIllegalMove) {
		t.Fatalf("a live offer was reassigned: %v", err)
	}
}

func TestDecline(t *testing.T) {
	j := job(t)
	if err := j.Offer("PTR-1", at, timeout); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if err := j.Decline("PTR-1", at.Add(time.Second)); err != nil {
		t.Fatalf("Decline: %v", err)
	}
	if j.Status != domain.JobWaiting || j.PartnerID != "" {
		t.Fatalf("job = %+v", j)
	}

	// Declining something that was never offered to you is refused.
	if err := j.Decline("PTR-1", at); !errors.Is(err, domain.ErrJobUnassigned) {
		t.Errorf("err = %v", err)
	}
}

// A rider abandoning a delivery owes the customer and the shop an explanation.
func TestFailNeedsAReason(t *testing.T) {
	j := job(t)
	if err := j.Offer("PTR-1", at, timeout); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if err := j.Accept("PTR-1", at); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if err := j.Fail("PTR-1", "", at); !errors.Is(err, domain.ErrNoFailureReason) {
		t.Fatalf("err = %v, want ErrNoFailureReason", err)
	}
	// Before collection the food is still on the shop's counter, so giving up
	// puts the job back on the board rather than ending the order.
	if err := j.Fail("PTR-1", "my bike broke down", at); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if j.Status != domain.JobWaiting || j.Reason != "my bike broke down" {
		t.Fatalf("job = %+v, want it back on the board", j)
	}
	if j.PassedBy != "PTR-1" {
		t.Errorf("the rider who gave up was not recorded: %+v", j)
	}

	// After collection the rider has somebody's dinner, and the delivery
	// really has failed.
	collected := job(t)
	if err := collected.Offer("PTR-1", at, timeout); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if err := collected.Accept("PTR-1", at); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if err := collected.Collect("PTR-1", at); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if err := collected.Fail("PTR-1", "accident", at); err != nil {
		t.Fatalf("Fail after collect: %v", err)
	}
	if collected.Status != domain.JobFailed || collected.Reason != "accident" {
		t.Fatalf("job = %+v", collected)
	}
}

// Declining is answering an offer. A rider who has already accepted cannot
// decline their way out of a job they are holding — they give it up, which is
// a different move with a reason attached.
func TestDecliningSomethingAlreadyAccepted(t *testing.T) {
	j := job(t)
	if err := j.Offer("PTR-1", at, timeout); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if err := j.Accept("PTR-1", at); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if err := j.Decline("PTR-1", at); !errors.Is(err, domain.ErrJobIllegalMove) {
		t.Fatalf("err = %v, want ErrJobIllegalMove", err)
	}
	if j.Status != domain.JobAssigned {
		t.Fatalf("job = %+v", j)
	}
}

func TestIllegalJobMoves(t *testing.T) {
	cases := []struct {
		name  string
		build func(*testing.T) domain.Job
		run   func(*domain.Job) error
		want  error
	}{
		{"collecting something not accepted", func(t *testing.T) domain.Job {
			j := job(t)
			if err := j.Offer("PTR-1", at, timeout); err != nil {
				t.Fatalf("Offer: %v", err)
			}
			return j
		}, func(j *domain.Job) error { return j.Collect("PTR-1", at) }, domain.ErrJobIllegalMove},

		{"delivering something not collected", func(t *testing.T) domain.Job {
			j := job(t)
			if err := j.Offer("PTR-1", at, timeout); err != nil {
				t.Fatalf("Offer: %v", err)
			}
			if err := j.Accept("PTR-1", at); err != nil {
				t.Fatalf("Accept: %v", err)
			}
			return j
		}, func(j *domain.Job) error { return j.Deliver("PTR-1", at) }, domain.ErrJobIllegalMove},

		{"accepting something already accepted", func(t *testing.T) domain.Job {
			j := job(t)
			if err := j.Offer("PTR-1", at, timeout); err != nil {
				t.Fatalf("Offer: %v", err)
			}
			if err := j.Accept("PTR-1", at); err != nil {
				t.Fatalf("Accept: %v", err)
			}
			return j
		}, func(j *domain.Job) error { return j.Accept("PTR-1", at) }, domain.ErrJobIllegalMove},

		{"failing something nobody has collected", func(t *testing.T) domain.Job {
			j := job(t)
			if err := j.Offer("PTR-1", at, timeout); err != nil {
				t.Fatalf("Offer: %v", err)
			}
			return j
		}, func(j *domain.Job) error { return j.Fail("PTR-1", "why", at) }, domain.ErrJobIllegalMove},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := tc.build(t)
			if err := tc.run(&j); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestAFinishedJobCannotBeMoved(t *testing.T) {
	deliver := func(t *testing.T) domain.Job {
		t.Helper()
		j := job(t)
		if err := j.Offer("PTR-1", at, timeout); err != nil {
			t.Fatalf("Offer: %v", err)
		}
		if err := j.Accept("PTR-1", at); err != nil {
			t.Fatalf("Accept: %v", err)
		}
		if err := j.Collect("PTR-1", at); err != nil {
			t.Fatalf("Collect: %v", err)
		}
		if err := j.Deliver("PTR-1", at); err != nil {
			t.Fatalf("Deliver: %v", err)
		}
		return j
	}

	j := deliver(t)
	for _, run := range []func() error{
		func() error { return j.Accept("PTR-1", at) },
		func() error { return j.Collect("PTR-1", at) },
		func() error { return j.Deliver("PTR-1", at) },
		func() error { return j.Fail("PTR-1", "why", at) },
		func() error { return j.Offer("PTR-2", at, timeout) },
		func() error { return j.Cancel("why", at) },
	} {
		if err := run(); !errors.Is(err, domain.ErrJobFinished) {
			t.Errorf("err = %v, want ErrJobFinished", err)
		}
	}
	for _, status := range []domain.JobStatus{
		domain.JobDelivered, domain.JobFailed, domain.JobCancelled,
	} {
		if !status.IsTerminal() || status.IsLive() {
			t.Errorf("%s: terminal = %v, live = %v", status, status.IsTerminal(), status.IsLive())
		}
	}
	// The zero status is neither, which is what an unset field should be.
	if domain.JobStatus("").IsLive() {
		t.Error("the zero status reported itself live")
	}
}

// Cancelling takes a job off the board because the order behind it went away.
// No partner id: this is the platform acting, not a person.
func TestCancel(t *testing.T) {
	j := job(t)
	if err := j.Offer("PTR-1", at, timeout); err != nil {
		t.Fatalf("Offer: %v", err)
	}
	if err := j.Cancel("the customer called it off", at); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if j.Status != domain.JobCancelled || j.Reason != "the customer called it off" {
		t.Fatalf("job = %+v", j)
	}
}
