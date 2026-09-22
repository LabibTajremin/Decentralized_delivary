// Package application holds review's use cases: submitting a review, reading
// a subject's rating and reviews, and raising and resolving support tickets.
package application

import (
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// reviewError turns a domain rule about a review's own shape into a failure
// a caller can act on.
//
// ErrInvalidRating is the only error NewReview can still return by the time
// SubmitReviewUseCase.Execute calls it: the order lookup and rater already
// succeeded, and verifySubject has already refused an invalid subject or a
// subject id that does not belong to the order, so nothing else reaches
// here.
func reviewError(err error) error {
	return errs.Wrap(err, errs.KindInvalid, "invalid_rating", "A rating must be between 1 and 5.")
}

// ticketError turns a domain rule about a ticket into a failure a caller can
// act on.
func ticketError(err error) error {
	switch {
	case errors.Is(err, domain.ErrTicketClosed):
		return errs.Wrap(err, errs.KindConflict, "ticket_already_resolved", "That ticket has already been resolved.")
	case errors.Is(err, domain.ErrNoResolution):
		return errs.Wrap(err, errs.KindInvalid, "invalid_resolution", "That is not a way to resolve a ticket.")
	default:
		// ErrNoTicketOrder, ErrNoRaisedBy, ErrNoSubject (from RaiseTicketUseCase)
		// and ErrNoAgent (from ResolveTicketUseCase) all land here: none of
		// them get a more specific message because a client that reaches this
		// far with an empty required field is malformed, not a person with a
		// decision to make about it.
		return errs.Wrap(err, errs.KindInvalid, "invalid_request", "We could not process that.")
	}
}

// storageError reports review storage we could not read or write.
func storageError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "review_unavailable",
		"We could not reach reviews just now. Please try again.")
}

// refundError reports that a ticket's refund could not go through payment.
func refundError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "refund_failed",
		"We could not process the refund just now. Please try again.")
}

// notFound is the response for an order or ticket that does not exist.
func notFound() error {
	return errs.New(errs.KindNotFound, "not_found", "That could not be found.")
}

// notFoundOr passes a genuine "no such order" through as 404 and reports
// everything else as order being unreachable — a caller asking "was this
// order delivered" should not get an ambiguous error for a database outage.
func notFoundOr(err error) error {
	if errs.CodeOf(err) == "order_not_found" {
		return notFound()
	}
	return errs.Wrap(err, errs.KindUnavailable, "review_unavailable",
		"We could not reach reviews just now. Please try again.")
}
