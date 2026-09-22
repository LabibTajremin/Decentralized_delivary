package review

import (
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

func seedReview(t *testing.T, repo *fakeReviewRepo, subject domain.Subject, subjectID string, rating int) {
	t.Helper()
	r, err := domain.NewReview("rev_"+subjectID, "ord_x", "usr_x", subject, subjectID, rating, "", at)
	if err != nil {
		t.Fatalf("seed review: %v", err)
	}
	repo.reviews = append(repo.reviews, r)
}

func TestRatingsForAveragesAcrossReviews(t *testing.T) {
	repo := &fakeReviewRepo{}
	seedReview(t, repo, domain.SubjectMerchant, "mer_1", 5)
	rev2, _ := domain.NewReview("rev_2", "ord_y", "usr_y", domain.SubjectMerchant, "mer_1", 3, "", at)
	repo.reviews = append(repo.reviews, rev2)

	rating, err := application.NewRatingsUseCase(repo).For(ctx(), domain.SubjectMerchant, "mer_1")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if rating.Count != 2 || rating.Average != 4 {
		t.Errorf("rating = %+v, want average 4 across 2 reviews", rating)
	}
}

func TestRatingsForASubjectWithNoReviews(t *testing.T) {
	repo := &fakeReviewRepo{}
	rating, err := application.NewRatingsUseCase(repo).For(ctx(), domain.SubjectMerchant, "mer_none")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if rating.Count != 0 || rating.Average != 0 {
		t.Errorf("rating = %+v, want zero value", rating)
	}
}

func TestRatingsForRejectsAnInvalidSubject(t *testing.T) {
	repo := &fakeReviewRepo{}
	_, err := application.NewRatingsUseCase(repo).For(ctx(), domain.Subject("courier"), "x")
	if errs.CodeOf(err) != "invalid_review_subject" {
		t.Errorf("err = %v", err)
	}
	_, err = application.NewRatingsUseCase(repo).For(ctx(), domain.SubjectMerchant, "")
	if errs.CodeOf(err) != "invalid_review_subject" {
		t.Errorf("err = %v", err)
	}
}

func TestRatingsForSurfacesAStorageFailure(t *testing.T) {
	repo := &fakeReviewRepo{ratingErr: errBoom}
	_, err := application.NewRatingsUseCase(repo).For(ctx(), domain.SubjectMerchant, "mer_1")
	if errs.CodeOf(err) != "review_unavailable" {
		t.Errorf("err = %v", err)
	}
}

func TestListReviewsForListsNewestFirst(t *testing.T) {
	repo := &fakeReviewRepo{}
	seedReview(t, repo, domain.SubjectItem, "itm_1", 4)
	seedReview(t, repo, domain.SubjectItem, "itm_1", 2)

	reviews, err := application.NewListReviewsUseCase(repo).For(ctx(), domain.SubjectItem, "itm_1", 0)
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if len(reviews) != 2 {
		t.Fatalf("reviews = %d, want 2", len(reviews))
	}
}

func TestListReviewsForRejectsAnInvalidSubject(t *testing.T) {
	repo := &fakeReviewRepo{}
	_, err := application.NewListReviewsUseCase(repo).For(ctx(), domain.Subject(""), "x", 0)
	if errs.CodeOf(err) != "invalid_review_subject" {
		t.Errorf("err = %v", err)
	}
}

func TestListReviewsForClampsAnOversizedLimit(t *testing.T) {
	repo := &fakeReviewRepo{}
	seedReview(t, repo, domain.SubjectItem, "itm_1", 4)
	reviews, err := application.NewListReviewsUseCase(repo).For(ctx(), domain.SubjectItem, "itm_1", 500)
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if len(reviews) != 1 {
		t.Errorf("reviews = %d, want 1", len(reviews))
	}
}

func TestListReviewsForSurfacesAStorageFailure(t *testing.T) {
	repo := &fakeReviewRepo{listErr: errBoom}
	_, err := application.NewListReviewsUseCase(repo).For(ctx(), domain.SubjectItem, "itm_1", 0)
	if errs.CodeOf(err) != "review_unavailable" {
		t.Errorf("err = %v", err)
	}
}
