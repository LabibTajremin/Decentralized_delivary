package review

import (
	"context"
	"errors"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/review/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

var errBoom = errors.New("boom")

var at = time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

func ctx() context.Context { return context.Background() }

// fakeReviewRepo is review's own storage, in memory.
type fakeReviewRepo struct {
	reviews   []domain.Review
	saveErr   error
	existsErr error
	ratingErr error
	listErr   error
}

func (f *fakeReviewRepo) SaveReview(_ context.Context, r domain.Review) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.reviews = append(f.reviews, r)
	return nil
}

func (f *fakeReviewRepo) ExistsForRater(_ context.Context, orderID, raterID string, subject domain.Subject, subjectID string) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	for _, r := range f.reviews {
		if r.OrderID == orderID && r.RaterID == raterID && r.Subject == subject && r.SubjectID == subjectID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeReviewRepo) RatingFor(_ context.Context, subject domain.Subject, subjectID string) (domain.Rating, error) {
	if f.ratingErr != nil {
		return domain.Rating{}, f.ratingErr
	}
	rating := domain.Rating{Subject: subject, SubjectID: subjectID}
	var total int
	for _, r := range f.reviews {
		if r.Subject == subject && r.SubjectID == subjectID {
			total += r.Rating
			rating.Count++
		}
	}
	if rating.Count > 0 {
		rating.Average = float64(total) / float64(rating.Count)
	}
	return rating, nil
}

func (f *fakeReviewRepo) ReviewsFor(_ context.Context, subject domain.Subject, subjectID string, limit int) ([]domain.Review, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.Review
	for i := len(f.reviews) - 1; i >= 0; i-- {
		r := f.reviews[i]
		if r.Subject == subject && r.SubjectID == subjectID {
			out = append(out, r)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// fakeTicketRepo is review's ticket storage, in memory.
type fakeTicketRepo struct {
	tickets []domain.Ticket
	saveErr error
	getErr  error
	listErr error
	openErr error
}

func (f *fakeTicketRepo) SaveTicket(_ context.Context, t domain.Ticket) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	for i, existing := range f.tickets {
		if existing.ID == t.ID {
			f.tickets[i] = t
			return nil
		}
	}
	f.tickets = append(f.tickets, t)
	return nil
}

func (f *fakeTicketRepo) Ticket(_ context.Context, id string) (domain.Ticket, bool, error) {
	if f.getErr != nil {
		return domain.Ticket{}, false, f.getErr
	}
	for _, t := range f.tickets {
		if t.ID == id {
			return t, true, nil
		}
	}
	return domain.Ticket{}, false, nil
}

func (f *fakeTicketRepo) TicketsRaisedBy(_ context.Context, userID string) ([]domain.Ticket, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.Ticket
	for _, t := range f.tickets {
		if t.RaisedBy == userID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeTicketRepo) OpenTickets(_ context.Context, limit int) ([]domain.Ticket, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	var out []domain.Ticket
	for _, t := range f.tickets {
		if t.Status == domain.TicketOpen {
			out = append(out, t)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// fakeOrder is review's view of order.
type fakeOrder struct {
	orders map[string]orderx.Order
	err    error
}

func newFakeOrder() *fakeOrder { return &fakeOrder{orders: map[string]orderx.Order{}} }

func (f *fakeOrder) Order(_ context.Context, orderID string) (orderx.Order, error) {
	if f.err != nil {
		return orderx.Order{}, f.err
	}
	o, ok := f.orders[orderID]
	if !ok {
		return orderx.Order{}, notFoundErr
	}
	return o, nil
}

// notFoundErr mirrors order's own "order_not_found" code, which
// notFoundOr in the application package reads to decide between a genuine
// 404 and a storage failure.
var notFoundErr = errs.New(errs.KindNotFound, "order_not_found", "We could not find that order.")

// fakePayment is review's view of payment.
type fakePayment struct {
	err     error
	refunds []refundCall
}

type refundCall struct{ orderID, reason string }

func (f *fakePayment) Refund(_ context.Context, orderID, reason string) error {
	f.refunds = append(f.refunds, refundCall{orderID, reason})
	return f.err
}

// fakeIDs mints predictable ids.
type fakeIDs struct{ n int }

func (f *fakeIDs) New(prefix string) string {
	f.n++
	return prefix + "_test"
}

var _ id.Generator = (*fakeIDs)(nil)

// fakeClock is a fixed clock.
type fakeClock struct{}

func (fakeClock) Now() time.Time { return at }
