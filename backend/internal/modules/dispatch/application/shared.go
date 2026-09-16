// Package application holds dispatch's use cases: putting a delivery on a
// board, choosing who to offer it to, and following it to the door.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/domain"
	cfg "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// rules is the dispatch configuration in force somewhere, read once.
//
// Five values that have to agree with each other: a job classified against one
// area's short-distance ceiling and offered to partners found with another
// area's radius would be a job nobody sensible was asked about.
type rules struct {
	shortMaxM     float64
	longMaxM      float64
	partnerRadius float64
	offerTimeout  time.Duration
	maxConcurrent int
}

// resolveRules reads the five settings for a placement.
func resolveRules(ctx context.Context, service cfg.Service, placement cfg.Placement) (rules, error) {
	settings, err := service.Settings(ctx, placement)
	if err != nil {
		return rules{}, configError(err)
	}

	shortMax, err := settings.Int(cfg.ShortDistanceMax)
	if err != nil {
		return rules{}, configError(err)
	}
	longMax, err := settings.Int(cfg.LongDistanceMax)
	if err != nil {
		return rules{}, configError(err)
	}
	radius, err := settings.Int(cfg.PartnerRadius)
	if err != nil {
		return rules{}, configError(err)
	}
	timeout, err := settings.Int(cfg.AssignTimeout)
	if err != nil {
		return rules{}, configError(err)
	}
	concurrent, err := settings.Int(cfg.MaxConcurrent)
	if err != nil {
		return rules{}, configError(err)
	}

	return rules{
		shortMaxM:     float64(shortMax),
		longMaxM:      float64(longMax),
		partnerRadius: float64(radius),
		offerTimeout:  time.Duration(timeout) * time.Second,
		maxConcurrent: int(concurrent),
	}, nil
}

// jobError turns a domain rule into a failure a rider can act on.
func jobError(err error) error {
	switch {
	case errors.Is(err, domain.ErrJobFinished):
		return errs.Wrap(err, errs.KindConflict, "job_finished",
			"That delivery has already finished.")
	case errors.Is(err, domain.ErrJobNotYours), errors.Is(err, domain.ErrJobUnassigned):
		// One message for both. A rider who is not on a job does not need to
		// know whether somebody else is — and telling them would say something
		// about another partner's work.
		return errs.Wrap(err, errs.KindConflict, "not_your_job",
			"Somebody else has taken that delivery.")
	case errors.Is(err, domain.ErrOfferExpired):
		return errs.Wrap(err, errs.KindConflict, "offer_expired",
			"That offer has expired. It has gone back to the list.")
	case errors.Is(err, domain.ErrNoFailureReason):
		return errs.Wrap(err, errs.KindInvalid, "reason_required",
			"Please say what went wrong with this delivery.")
	default:
		// ErrJobIllegalMove and anything else the job aggregate could grow.
		// A move the job refused is a move the job refused, whatever the
		// reason is called.
		return errs.Wrap(err, errs.KindConflict, "illegal_move",
			"That delivery cannot be changed that way.")
	}
}

// partnerError turns a partner rule into a failure somebody can act on.
//
// Only the rules a person can break by asking for something are here.
// ErrNotWorking and ErrAtCapacity are read as facts rather than raised as
// failures — they decide who is a candidate for a job, and a rider is never
// told "you are at capacity" because they never asked for the job that would
// have said so.
func partnerError(err error) error {
	switch {
	case errors.Is(err, domain.ErrBadPreference):
		return errs.Wrap(err, errs.KindInvalid, "invalid_preference",
			"Choose short-distance, long-distance, or both.")
	case errors.Is(err, domain.ErrBadAvailability):
		return errs.Wrap(err, errs.KindInvalid, "invalid_availability",
			"You can go online or offline.")
	default:
		// ErrNoName, ErrNoPhone, ErrNoUser and anything else the partner
		// aggregate could grow: the details given were not enough.
		return errs.Wrap(err, errs.KindInvalid, "invalid_partner",
			"Please check your name and phone number.")
	}
}

// storageError reports a job or a partner we could not read or write.
func storageError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "dispatch_unavailable",
		"We could not reach the delivery system just now. Please try again.")
}

// configError reports configuration that could not be read. A dispatch round
// that silently fell back to a hard-coded radius would be a division quietly
// offering its jobs to the wrong people.
func configError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "dispatch_config_unavailable",
		"We could not work out who to offer that delivery to. Please try again.")
}

// notFound is one job or partner that is not there — or not the caller's,
// which is the same answer on purpose.
func notFound(what string) error {
	if what == "partner" {
		return errs.New(errs.KindNotFound, "partner_not_found",
			"You are not registered as a delivery partner.")
	}
	return errs.New(errs.KindNotFound, "job_not_found", "We could not find that delivery.")
}
