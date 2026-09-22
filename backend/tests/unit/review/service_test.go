package review

import (
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
)

// TestServiceImplementsReviewContract exercises application.Service, the
// concrete type consumed structurally wherever ReviewContract is needed
// (merchant, dispatch — Appendix A), the same zero-adapter pattern used
// throughout this codebase since P13.
func TestServiceImplementsReviewContract(t *testing.T) {
	repo := &fakeReviewRepo{}
	seedReview(t, repo, domain.SubjectMerchant, "mer_1", 5)
	seedReview(t, repo, domain.SubjectPartner, "prt_1", 3)

	svc := application.NewService(application.NewRatingsUseCase(repo))

	mr, err := svc.MerchantRating(ctx(), "mer_1")
	if err != nil {
		t.Fatalf("MerchantRating: %v", err)
	}
	if mr.SubjectID != "mer_1" || mr.Average != 5 || mr.Count != 1 {
		t.Errorf("merchant rating = %+v", mr)
	}

	pr, err := svc.PartnerRating(ctx(), "prt_1")
	if err != nil {
		t.Fatalf("PartnerRating: %v", err)
	}
	if pr.SubjectID != "prt_1" || pr.Average != 3 || pr.Count != 1 {
		t.Errorf("partner rating = %+v", pr)
	}
}

func TestServiceSurfacesARatingFailure(t *testing.T) {
	repo := &fakeReviewRepo{ratingErr: errBoom}
	svc := application.NewService(application.NewRatingsUseCase(repo))

	if _, err := svc.MerchantRating(ctx(), "mer_1"); err == nil {
		t.Error("MerchantRating: want an error")
	}
	if _, err := svc.PartnerRating(ctx(), "prt_1"); err == nil {
		t.Error("PartnerRating: want an error")
	}
}
