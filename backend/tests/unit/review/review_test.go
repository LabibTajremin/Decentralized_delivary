package review

import (
	"testing"

	ordercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/order/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/review/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

func newSubmit(repo *fakeReviewRepo, ord *fakeOrder) *application.SubmitReviewUseCase {
	return application.NewSubmitReviewUseCase(repo, ord, fakeClock{}, &fakeIDs{})
}

func deliveredOrder() orderx.Order {
	return orderx.Order{
		ID: "ord_1", CustomerID: "usr_1", MerchantID: "mer_1", Status: "delivered",
		Lines: []ordercontract.Line{{Name: "Biryani", ItemID: "itm_1"}},
		Events: []ordercontract.Event{
			{Status: "delivered", Actor: "partner", ActorID: "prt_1"},
		},
	}
}

func TestSubmitReviewRecordsAMerchantReview(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{}

	review, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectMerchant, SubjectID: "mer_1", Rating: 5, Comment: "great",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if review.SubjectID != "mer_1" || review.Rating != 5 {
		t.Errorf("review = %+v", review)
	}
	if len(repo.reviews) != 1 {
		t.Errorf("stored reviews = %d, want 1", len(repo.reviews))
	}
}

func TestSubmitReviewRecordsAnItemReview(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{}

	if _, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectItem, SubjectID: "itm_1", Rating: 4,
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestSubmitReviewRecordsAPartnerReview(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{}

	if _, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectPartner, SubjectID: "prt_1", Rating: 3,
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

func TestSubmitReviewRejectsSomeoneElsesOrder(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{}

	_, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_2",
		Subject: domain.SubjectMerchant, SubjectID: "mer_1", Rating: 5,
	})
	if errs.CodeOf(err) != "not_your_order" {
		t.Errorf("err = %v", err)
	}
}

func TestSubmitReviewRejectsAnUndeliveredOrder(t *testing.T) {
	ord := newFakeOrder()
	o := deliveredOrder()
	o.Status = "placed"
	ord.orders["ord_1"] = o
	repo := &fakeReviewRepo{}

	_, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectMerchant, SubjectID: "mer_1", Rating: 5,
	})
	if errs.CodeOf(err) != "order_not_delivered" {
		t.Errorf("err = %v", err)
	}
}

func TestSubmitReviewRejectsAWrongMerchant(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{}

	_, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectMerchant, SubjectID: "mer_2", Rating: 5,
	})
	if errs.CodeOf(err) != "invalid_review_subject" {
		t.Errorf("err = %v", err)
	}
}

func TestSubmitReviewRejectsAnItemNotInTheOrder(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{}

	_, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectItem, SubjectID: "itm_9", Rating: 5,
	})
	if errs.CodeOf(err) != "invalid_review_subject" {
		t.Errorf("err = %v", err)
	}
}

func TestSubmitReviewRejectsAPartnerWhoDidNotDeliverIt(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{}

	_, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectPartner, SubjectID: "prt_9", Rating: 5,
	})
	if errs.CodeOf(err) != "invalid_review_subject" {
		t.Errorf("err = %v", err)
	}
}

func TestSubmitReviewRejectsAnUnknownSubject(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{}

	_, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.Subject("courier"), SubjectID: "mer_1", Rating: 5,
	})
	if errs.CodeOf(err) != "invalid_review_subject" {
		t.Errorf("err = %v", err)
	}
}

func TestSubmitReviewRejectsADuplicate(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{}
	sub := newSubmit(repo, ord)
	req := application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectMerchant, SubjectID: "mer_1", Rating: 5,
	}
	if _, err := sub.Execute(ctx(), req); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	if _, err := sub.Execute(ctx(), req); errs.CodeOf(err) != "already_reviewed" {
		t.Errorf("err = %v", err)
	}
}

func TestSubmitReviewRejectsAnInvalidRating(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{}

	_, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectMerchant, SubjectID: "mer_1", Rating: 9,
	})
	if errs.CodeOf(err) != "invalid_rating" {
		t.Errorf("err = %v", err)
	}
}

func TestSubmitReviewSurfacesANotFoundOrder(t *testing.T) {
	ord := newFakeOrder()
	repo := &fakeReviewRepo{}

	_, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_missing", RaterID: "usr_1",
		Subject: domain.SubjectMerchant, SubjectID: "mer_1", Rating: 5,
	})
	if errs.CodeOf(err) != "not_found" {
		t.Errorf("err = %v", err)
	}
}

func TestSubmitReviewSurfacesAnOrderStorageFailure(t *testing.T) {
	ord := newFakeOrder()
	ord.err = errBoom
	repo := &fakeReviewRepo{}

	_, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectMerchant, SubjectID: "mer_1", Rating: 5,
	})
	if errs.CodeOf(err) != "review_unavailable" {
		t.Errorf("err = %v", err)
	}
}

func TestSubmitReviewSurfacesAnExistsCheckFailure(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{existsErr: errBoom}

	_, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectMerchant, SubjectID: "mer_1", Rating: 5,
	})
	if errs.CodeOf(err) != "review_unavailable" {
		t.Errorf("err = %v", err)
	}
}

func TestSubmitReviewSurfacesASaveFailure(t *testing.T) {
	ord := newFakeOrder()
	ord.orders["ord_1"] = deliveredOrder()
	repo := &fakeReviewRepo{saveErr: errBoom}

	_, err := newSubmit(repo, ord).Execute(ctx(), application.SubmitReviewRequest{
		OrderID: "ord_1", RaterID: "usr_1",
		Subject: domain.SubjectMerchant, SubjectID: "mer_1", Rating: 5,
	})
	if errs.CodeOf(err) != "review_unavailable" {
		t.Errorf("err = %v", err)
	}
}
