package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// RatingsUseCase serves one subject's aggregate rating.
type RatingsUseCase struct {
	repo ports.ReviewRepository
}

// NewRatingsUseCase wires the use case.
func NewRatingsUseCase(repo ports.ReviewRepository) *RatingsUseCase {
	return &RatingsUseCase{repo: repo}
}

// For returns subjectID's mean rating and how many reviews it rests on.
func (uc *RatingsUseCase) For(ctx context.Context, subject domain.Subject, subjectID string) (domain.Rating, error) {
	if !subject.Valid() || subjectID == "" {
		return domain.Rating{}, errs.New(errs.KindInvalid, "invalid_review_subject",
			"That is not something reviews are kept for.")
	}
	rating, err := uc.repo.RatingFor(ctx, subject, subjectID)
	if err != nil {
		return domain.Rating{}, storageError(err)
	}
	return rating, nil
}

// ListReviewsUseCase serves one subject's own reviews.
type ListReviewsUseCase struct {
	repo ports.ReviewRepository
}

// NewListReviewsUseCase wires the use case.
func NewListReviewsUseCase(repo ports.ReviewRepository) *ListReviewsUseCase {
	return &ListReviewsUseCase{repo: repo}
}

// For lists a subject's reviews, newest first, capped the same way a
// notification inbox is: an unbounded request is not a person reading
// reviews, it is a scrape.
func (uc *ListReviewsUseCase) For(ctx context.Context, subject domain.Subject, subjectID string, limit int) ([]domain.Review, error) {
	if !subject.Valid() || subjectID == "" {
		return nil, errs.New(errs.KindInvalid, "invalid_review_subject",
			"That is not something reviews are kept for.")
	}
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	reviews, err := uc.repo.ReviewsFor(ctx, subject, subjectID, limit)
	if err != nil {
		return nil, storageError(err)
	}
	return reviews, nil
}
