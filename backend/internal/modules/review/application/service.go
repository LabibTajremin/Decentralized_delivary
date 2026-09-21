package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
)

// Service implements contract.ReviewContract.
type Service struct {
	ratings *RatingsUseCase
}

// NewService wires the public service.
func NewService(ratings *RatingsUseCase) *Service { return &Service{ratings: ratings} }

// MerchantRating returns a merchant's aggregate rating.
func (s *Service) MerchantRating(ctx context.Context, merchantID string) (contract.Rating, error) {
	return s.ratingFor(ctx, domain.SubjectMerchant, merchantID)
}

// PartnerRating returns a delivery partner's aggregate rating.
func (s *Service) PartnerRating(ctx context.Context, partnerID string) (contract.Rating, error) {
	return s.ratingFor(ctx, domain.SubjectPartner, partnerID)
}

func (s *Service) ratingFor(ctx context.Context, subject domain.Subject, subjectID string) (contract.Rating, error) {
	rating, err := s.ratings.For(ctx, subject, subjectID)
	if err != nil {
		return contract.Rating{}, err
	}
	return contract.Rating{SubjectID: rating.SubjectID, Average: rating.Average, Count: rating.Count}, nil
}
