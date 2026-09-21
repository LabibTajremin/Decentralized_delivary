package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// SubmitReviewUseCase records one customer's opinion, once the order behind
// it says they are allowed to give it.
type SubmitReviewUseCase struct {
	repo  ports.ReviewRepository
	order order.Service
	clock clock.Clock
	ids   id.Generator
}

// NewSubmitReviewUseCase wires the use case.
func NewSubmitReviewUseCase(repo ports.ReviewRepository, o order.Service, c clock.Clock, ids id.Generator) *SubmitReviewUseCase {
	return &SubmitReviewUseCase{repo: repo, order: o, clock: c, ids: ids}
}

// SubmitReviewRequest is one review as a customer submits it.
type SubmitReviewRequest struct {
	OrderID   string
	RaterID   string
	Subject   domain.Subject
	SubjectID string
	Rating    int
	Comment   string
}

// Execute checks eligibility against the order, then records the review.
//
// Eligibility, in order: the order must exist, the rater must be the
// order's own customer, the order must be delivered, and the subject must be
// something this specific order actually involved. Each rule fails with its
// own reason, because "you cannot review this" and "you cannot review yet"
// and "that is not what you ordered" are different problems with different
// fixes, and collapsing them into one generic refusal would leave a customer
// guessing which one applies.
func (uc *SubmitReviewUseCase) Execute(ctx context.Context, req SubmitReviewRequest) (domain.Review, error) {
	ord, err := uc.order.Order(ctx, req.OrderID)
	if err != nil {
		return domain.Review{}, notFoundOr(err)
	}
	if ord.CustomerID != req.RaterID {
		return domain.Review{}, errs.New(errs.KindForbidden, "not_your_order",
			"You can only review your own orders.")
	}
	if ord.Status != "delivered" {
		return domain.Review{}, errs.New(errs.KindConflict, "order_not_delivered",
			"You can review an order once it has been delivered.")
	}
	if err := verifySubject(ord, req.Subject, req.SubjectID); err != nil {
		return domain.Review{}, err
	}

	dup, err := uc.repo.ExistsForRater(ctx, req.OrderID, req.RaterID, req.Subject, req.SubjectID)
	if err != nil {
		return domain.Review{}, storageError(err)
	}
	if dup {
		return domain.Review{}, errs.New(errs.KindConflict, "already_reviewed",
			"You have already reviewed this.")
	}

	review, err := domain.NewReview(uc.ids.New("rev"), req.OrderID, req.RaterID,
		req.Subject, req.SubjectID, req.Rating, req.Comment, uc.clock.Now())
	if err != nil {
		return domain.Review{}, reviewError(err)
	}
	if err := uc.repo.SaveReview(ctx, review); err != nil {
		return domain.Review{}, storageError(err)
	}
	return review, nil
}

// verifySubject checks that subjectID names something ord actually
// involved, using only what OrderContract already carries: review depends
// on order's contract alone, not dispatch's or catalogue's (Appendix A), so
// a merchant subject is checked against the order's own MerchantID, an item
// subject against its lines' ItemID, and a partner subject against the
// ActorID on the event that actually delivered it.
func verifySubject(ord order.Order, subject domain.Subject, subjectID string) error {
	switch subject {
	case domain.SubjectMerchant:
		if ord.MerchantID != subjectID {
			return errs.New(errs.KindInvalid, "invalid_review_subject",
				"That is not the shop this order was placed with.")
		}
		return nil
	case domain.SubjectItem:
		for _, l := range ord.Lines {
			if l.ItemID == subjectID {
				return nil
			}
		}
		return errs.New(errs.KindInvalid, "invalid_review_subject",
			"That item was not part of this order.")
	case domain.SubjectPartner:
		for _, e := range ord.Events {
			if e.Status == "delivered" && e.Actor == "partner" && e.ActorID == subjectID {
				return nil
			}
		}
		return errs.New(errs.KindInvalid, "invalid_review_subject",
			"That partner did not deliver this order.")
	default:
		return errs.New(errs.KindInvalid, "invalid_review_subject",
			"That is not something a review can be about.")
	}
}
