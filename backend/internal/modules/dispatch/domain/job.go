package domain

import (
	"errors"
	"time"
)

// JobStatus is where a delivery is in its own life.
//
// Deliberately not the order's status. An order is the customer's view of the
// thing; a job is the platform's view of getting it moved, and they diverge:
// an order sitting at `ready` may have been offered to four partners and
// declined by three, none of which is a fact the customer needs.
type JobStatus string

// The job lifecycle.
const (
	// JobWaiting is a job nobody holds. It appears in every eligible partner's
	// feed until somebody takes it.
	JobWaiting JobStatus = "waiting"
	// JobOffered is a job put to one partner, with a clock running
	// (dispatch.assignment_timeout).
	JobOffered JobStatus = "offered"
	// JobAssigned is a partner having accepted it.
	JobAssigned JobStatus = "assigned"
	// JobCollected is the rider having the goods.
	JobCollected JobStatus = "collected"
	// JobDelivered is done, and terminal.
	JobDelivered JobStatus = "delivered"
	// JobFailed is a delivery that could not be completed, and terminal.
	JobFailed JobStatus = "failed"
	// JobCancelled is the order behind it having gone away, and terminal.
	JobCancelled JobStatus = "cancelled"
)

// IsTerminal reports whether a job has finished.
func (s JobStatus) IsTerminal() bool {
	return s == JobDelivered || s == JobFailed || s == JobCancelled
}

// IsLive reports whether a job is still going.
func (s JobStatus) IsLive() bool { return !s.IsTerminal() && s != "" }

// The rules a job holds itself to.
var (
	ErrNoOrder          = errors.New("a job must be about an order")
	ErrJobIllegalMove   = errors.New("a job cannot move that way")
	ErrJobNotYours      = errors.New("that job is not yours")
	ErrJobFinished      = errors.New("that job has already finished")
	ErrJobUnassigned    = errors.New("that job has nobody on it")
	ErrOfferExpired     = errors.New("that offer has expired")
	ErrNegativeDistance = errors.New("a distance cannot be negative")
	ErrNoFailureReason  = errors.New("a failed delivery needs a reason")
)

// Place is one end of a delivery.
type Place struct {
	Name       string
	Phone      string
	SingleLine string
	Lat        float64
	Lng        float64
}

// Placement is where a job is, in administrative terms.
//
// Carried on the job because the settings that govern it — the offer timeout,
// the candidate radius — are per-area (D2), and the sweeper that re-offers a
// job later has only the job to go on.
type Placement struct {
	AreaCode     string
	DistrictCode string
	DivisionCode string
}

// Job is one delivery to be made.
type Job struct {
	ID      string
	OrderID string
	// Code is the order's short reference, carried so a rider and a shopkeeper
	// can say the same six characters to each other.
	Code string

	Status JobStatus
	// PartnerID is who holds it — the partner it is offered to, or the one who
	// accepted. Empty while waiting.
	PartnerID string

	Pickup      Place
	Destination Place
	DistanceM   float64
	Band        Band
	Placement   Placement

	// PassedBy is the last partner who had this job and did not take it —
	// declined it, or let the offer run out. The next round skips them, so a
	// rider who has already said no is not asked the same question twice
	// while somebody else is standing by. Cleared on the next offer.
	PassedBy string

	// Reason is why a job ended the way it did — the order was cancelled, the
	// customer was not in. Kept because a failed delivery is the start of a
	// support conversation, and "failed" on its own starts it badly.
	Reason string
	// Attempts is how many partners have been offered this and not taken it.
	// An order nobody wants is an operational fact somebody has to see.
	Attempts int
	// OfferedAt and OfferExpiresAt bound the current offer.
	OfferedAt      time.Time
	OfferExpiresAt time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewJob opens a delivery for an order that is ready to collect.
func NewJob(
	id, orderID, code string,
	pickup, destination Place,
	distanceM float64, band Band, placement Placement,
	now time.Time,
) (Job, error) {
	switch {
	case orderID == "":
		return Job{}, ErrNoOrder
	case distanceM < 0:
		return Job{}, ErrNegativeDistance
	}
	return Job{
		ID: id, OrderID: orderID, Code: code,
		Status:      JobWaiting,
		Pickup:      pickup,
		Destination: destination,
		DistanceM:   distanceM,
		Band:        band,
		Placement:   placement,
		CreatedAt:   now, UpdatedAt: now,
	}, nil
}

// Offer puts a job to one partner, with a clock running.
//
// The clock is the point. A partner who opens the app, sees an offer and puts
// the phone in their pocket must not hold a customer's dinner hostage; the
// offer lapses and the job goes back into everybody's feed.
func (j *Job) Offer(partnerID string, at time.Time, timeout time.Duration) error {
	if j.Status.IsTerminal() {
		return ErrJobFinished
	}
	if j.Status != JobWaiting && !j.OfferLapsed(at) {
		return ErrJobIllegalMove
	}
	j.Status = JobOffered
	j.PartnerID = partnerID
	j.PassedBy = ""
	j.OfferedAt = at
	j.OfferExpiresAt = at.Add(timeout)
	j.Attempts++
	j.UpdatedAt = at
	return nil
}

// OfferLapsed reports whether the current offer has run out.
func (j Job) OfferLapsed(at time.Time) bool {
	return j.Status == JobOffered && !at.Before(j.OfferExpiresAt)
}

// Accept is a partner taking the job they were offered.
func (j *Job) Accept(partnerID string, at time.Time) error {
	if err := j.heldBy(partnerID); err != nil {
		return err
	}
	if j.Status != JobOffered {
		return ErrJobIllegalMove
	}
	if j.OfferLapsed(at) {
		return ErrOfferExpired
	}
	j.Status = JobAssigned
	j.OfferExpiresAt = time.Time{}
	j.UpdatedAt = at
	return nil
}

// Decline is a partner passing. The job goes back to waiting and the next
// round finds somebody else.
func (j *Job) Decline(partnerID string, at time.Time) error {
	if err := j.heldBy(partnerID); err != nil {
		return err
	}
	if j.Status != JobOffered {
		return ErrJobIllegalMove
	}
	return j.release(at)
}

// Lapse expires an offer nobody answered.
func (j *Job) Lapse(at time.Time) error {
	if !j.OfferLapsed(at) {
		return ErrJobIllegalMove
	}
	return j.release(at)
}

// release returns a job to the pool.
func (j *Job) release(at time.Time) error {
	j.PassedBy = j.PartnerID
	j.Status = JobWaiting
	j.PartnerID = ""
	j.OfferedAt = time.Time{}
	j.OfferExpiresAt = time.Time{}
	j.UpdatedAt = at
	return nil
}

// Collect is the rider having the goods.
func (j *Job) Collect(partnerID string, at time.Time) error {
	return j.move(partnerID, JobAssigned, JobCollected, at)
}

// Deliver is the rider having handed them over.
func (j *Job) Deliver(partnerID string, at time.Time) error {
	return j.move(partnerID, JobCollected, JobDelivered, at)
}

// Fail is a rider giving up on a delivery — nobody at the address, an accident.
//
// What that means depends on whether they have the goods. Before collection the
// food is still on the shop's counter, so there is nothing to fail: the job
// goes back on the board for somebody else and the customer's order carries on.
// After collection the rider is holding somebody's dinner and the delivery
// really has failed.
func (j *Job) Fail(partnerID, reason string, at time.Time) error {
	if err := j.heldBy(partnerID); err != nil {
		return err
	}
	if reason == "" {
		// A rider abandoning a delivery owes the customer and the shop an
		// explanation. It is the only thing either of them will have to go on.
		return ErrNoFailureReason
	}
	switch j.Status {
	case JobAssigned:
		j.Reason = reason
		return j.release(at)
	case JobCollected:
		j.Status = JobFailed
		j.Reason = reason
		j.UpdatedAt = at
		return nil
	default:
		return ErrJobIllegalMove
	}
}

// Cancel takes a job off the board because the order behind it went away.
//
// No partner id: this is the platform acting, not a person. A rider holding a
// cancelled job finds out because it leaves their list.
func (j *Job) Cancel(reason string, at time.Time) error {
	if j.Status.IsTerminal() {
		return ErrJobFinished
	}
	j.Status = JobCancelled
	j.Reason = reason
	j.UpdatedAt = at
	return nil
}

// move applies a straightforward transition by the partner holding the job.
func (j *Job) move(partnerID string, from, to JobStatus, at time.Time) error {
	if err := j.heldBy(partnerID); err != nil {
		return err
	}
	if j.Status != from {
		return ErrJobIllegalMove
	}
	j.Status = to
	j.UpdatedAt = at
	return nil
}

// heldBy checks that this partner is the one who has the job.
//
// The acceptance criterion "two partners cannot be assigned the same order" has
// two halves. This is the one in the domain: a partner who is not holding a job
// cannot move it. The other half is a compare-and-set in the repository, which
// is what stops two partners accepting the same waiting job in the same
// millisecond.
func (j Job) heldBy(partnerID string) error {
	switch {
	case j.Status.IsTerminal():
		return ErrJobFinished
	case j.PartnerID == "":
		return ErrJobUnassigned
	case j.PartnerID != partnerID:
		return ErrJobNotYours
	}
	return nil
}
