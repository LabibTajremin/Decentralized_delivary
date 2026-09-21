// Package ports declares what review's use cases need from the outside
// world. Every implementation lives in infrastructure/.
package ports

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
)

// ReviewRepository stores reviews and answers the two questions a screen
// needs about a subject's reviews: the aggregate, and the list.
type ReviewRepository interface {
	SaveReview(ctx context.Context, r domain.Review) error

	// ExistsForRater reports whether this rater already reviewed this
	// subject from this order — one opinion per person per thing, not one
	// per tap of a submit button.
	ExistsForRater(ctx context.Context, orderID, raterID string, subject domain.Subject, subjectID string) (bool, error)

	// RatingFor reduces every review of one subject to a mean and a count.
	// A subject with no reviews yet is not an error — it returns a zero
	// Rating, the way a shop that just opened has no rating rather than a
	// broken one.
	RatingFor(ctx context.Context, subject domain.Subject, subjectID string) (domain.Rating, error)

	// ReviewsFor lists a subject's reviews, newest first.
	ReviewsFor(ctx context.Context, subject domain.Subject, subjectID string, limit int) ([]domain.Review, error)
}

// TicketRepository stores support tickets.
type TicketRepository interface {
	SaveTicket(ctx context.Context, t domain.Ticket) error
	Ticket(ctx context.Context, id string) (domain.Ticket, bool, error)

	// TicketsRaisedBy lists a customer's own tickets, newest first.
	TicketsRaisedBy(ctx context.Context, userID string) ([]domain.Ticket, error)

	// OpenTickets lists tickets no agent has resolved yet, oldest first —
	// the queue a support agent works down.
	OpenTickets(ctx context.Context, limit int) ([]domain.Ticket, error)
}
