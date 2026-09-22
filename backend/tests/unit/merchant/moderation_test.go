package merchant

import (
	"context"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// TestApprovalMakesAShopSearchable is the other half of the acceptance
// criterion: the status and the search index change together, because a shop
// that looks approved and cannot be found is the same bug as the reverse.
func TestApprovalMakesAShopSearchable(t *testing.T) {
	h := newHarness(t)
	merchant := h.approve(t, "usr_1")

	if merchant.Status != domain.StatusApproved {
		t.Errorf("status = %q, want approved", merchant.Status)
	}
	if !h.geo.isActive(merchant.ID) {
		t.Error("an approved shop is not searchable")
	}
	if !merchant.IsListed(h.clock.Now()) {
		t.Error("an approved shop is not listed")
	}
}

func TestSuspensionTakesAShopOutOfTheIndex(t *testing.T) {
	h := newHarness(t)
	merchant := h.approve(t, "usr_1")

	suspended, err := h.moderation.Suspend(context.Background(), "usr_admin", merchant.ID, "Repeated cancellations.")
	if err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if suspended.Status != domain.StatusSuspended {
		t.Errorf("status = %q, want suspended", suspended.Status)
	}
	if h.geo.isActive(merchant.ID) {
		t.Error("a suspended shop is still searchable")
	}
	if suspended.ReviewNote != "Repeated cancellations." {
		t.Errorf("review note = %q, want the suspension reason", suspended.ReviewNote)
	}
}

// TestReinstatingNeedsNoFreshReview: suspension is distinct from rejection
// exactly so an operator can put a shop back without sending it round the
// approval loop again.
func TestReinstatingNeedsNoFreshReview(t *testing.T) {
	h := newHarness(t)
	merchant := h.approve(t, "usr_1")

	if _, err := h.moderation.Suspend(context.Background(), "usr_admin", merchant.ID, "Under investigation."); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	reinstated, err := h.moderation.Reinstate(context.Background(), "usr_admin", merchant.ID, "")
	if err != nil {
		t.Fatalf("Reinstate: %v", err)
	}
	if reinstated.Status != domain.StatusApproved {
		t.Errorf("status = %q, want approved", reinstated.Status)
	}
	if !h.geo.isActive(merchant.ID) {
		t.Error("a reinstated shop is not searchable again")
	}
	if reinstated.ReviewNote != "" {
		t.Errorf("review note = %q, want the suspension reason cleared", reinstated.ReviewNote)
	}
}

// TestARejectionMustSayWhy: the usual cause is an unreadable licence photo, and
// the owner can fix that in a minute if we tell them.
func TestARejectionMustSayWhy(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())
	h.documentAll(t, "usr_1", domain.TypeRestaurant)
	merchant, err := h.registration.SubmitForReview(context.Background(), "usr_1")
	if err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}

	for name, call := range map[string]func(string) error{
		"reject": func(note string) error {
			_, e := h.moderation.Reject(context.Background(), "usr_admin", merchant.ID, note)
			return e
		},
		"suspend": func(note string) error {
			_, e := h.moderation.Suspend(context.Background(), "usr_admin", merchant.ID, note)
			return e
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := call("   ")
			if got := errs.CodeOf(err); got != "reason_required" {
				t.Errorf("code = %q, want reason_required", got)
			}
			if errs.KindOf(err) != errs.KindInvalid {
				t.Errorf("kind = %v, want invalid", errs.KindOf(err))
			}
		})
	}
}

// TestAnApprovalNeedsNoNote: there is nothing to explain, and demanding one
// would make an operator type something to get past a form.
func TestAnApprovalNeedsNoNote(t *testing.T) {
	h := newHarness(t)
	if merchant := h.approve(t, "usr_1"); merchant.ReviewNote != "" {
		t.Errorf("review note = %q, want empty", merchant.ReviewNote)
	}
}

func TestARejectedShopCanFixItsDocumentsAndResubmit(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())
	h.documentAll(t, "usr_1", domain.TypeRestaurant)
	merchant, err := h.registration.SubmitForReview(context.Background(), "usr_1")
	if err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}

	rejected, err := h.moderation.Reject(context.Background(), "usr_admin", merchant.ID, "The trade licence photo is unreadable.")
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if rejected.ReviewNote == "" {
		t.Error("the rejection carried no reason to show the owner")
	}
	if h.geo.isActive(merchant.ID) {
		t.Error("a rejected shop is searchable")
	}

	// The owner replaces the licence and submits again.
	if _, err := h.registration.AddDocument(context.Background(), "usr_1", application.DocumentRequest{
		Kind: "trade_licence", Number: "TRAD-2", FileURL: "/clear.png",
	}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	resubmitted, err := h.registration.SubmitForReview(context.Background(), "usr_1")
	if err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}
	if resubmitted.Status != domain.StatusPendingReview {
		t.Errorf("status = %q, want pending_review", resubmitted.Status)
	}
}

func TestADecisionThatTheWorkflowForbidsIsRefused(t *testing.T) {
	h := newHarness(t)
	merchant := h.register(t, "usr_1", validRequest())

	// A draft has not been submitted, so there is nothing to approve.
	_, err := h.moderation.Approve(context.Background(), "usr_admin", merchant.ID, "")
	if got := errs.CodeOf(err); got != "invalid_status_change" {
		t.Errorf("code = %q, want invalid_status_change", got)
	}
	if errs.KindOf(err) != errs.KindConflict {
		t.Errorf("kind = %v, want conflict", errs.KindOf(err))
	}
}

func TestADecisionOnAShopThatDoesNotExistIsNotFound(t *testing.T) {
	h := newHarness(t)

	for name, call := range map[string]func() error{
		"approve": func() error { _, e := h.moderation.Approve(context.Background(), "a", "mch_nope", ""); return e },
		"get":     func() error { _, e := h.moderation.Get(context.Background(), "mch_nope"); return e },
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			if got := errs.CodeOf(err); got != "merchant_not_found" {
				t.Errorf("code = %q, want merchant_not_found", got)
			}
			if errs.KindOf(err) != errs.KindNotFound {
				t.Errorf("kind = %v, want not_found", errs.KindOf(err))
			}
		})
	}
}

// TestEveryDecisionIsRecordedWithWhoTookIt: an approval with no name attached
// is not an audit trail.
func TestEveryDecisionIsRecordedWithWhoTookIt(t *testing.T) {
	h := newHarness(t)
	merchant := h.approve(t, "usr_1")

	if _, err := h.moderation.Suspend(context.Background(), "usr_admin2", merchant.ID, "Complaints."); err != nil {
		t.Fatalf("Suspend: %v", err)
	}

	history, err := h.moderation.History(context.Background(), merchant.ID, 0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("history = %d entries, want 3 (submit, approve, suspend)", len(history))
	}
	// Newest first.
	if history[0].To != domain.StatusSuspended || history[0].ActorUserID != "usr_admin2" ||
		history[0].Note != "Complaints." {
		t.Errorf("newest entry = %+v", history[0])
	}
	if history[2].To != domain.StatusPendingReview || history[2].ActorUserID != "usr_1" {
		t.Errorf("oldest entry = %+v", history[2])
	}
}

func TestHistoryIsPaged(t *testing.T) {
	h := newHarness(t)
	merchant := h.approve(t, "usr_1")

	history, err := h.moderation.History(context.Background(), merchant.ID, 1)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("history = %d entries, want 1", len(history))
	}

	// An absurd limit falls back to the default page rather than letting a
	// caller pull the whole table.
	if _, err := h.moderation.History(context.Background(), merchant.ID, 10_000); err != nil {
		t.Errorf("History with an absurd limit: %v", err)
	}
}

func TestModerationReportsAStoreThatIsDown(t *testing.T) {
	cases := map[string]struct {
		broken func(*memoryRepo)
		call   func(*harness, string) error
	}{
		"read": {
			func(r *memoryRepo) { r.readErr = errStore },
			func(h *harness, id string) error { _, e := h.moderation.Get(context.Background(), id); return e },
		},
		"save": {
			func(r *memoryRepo) { r.saveErr = errStore },
			func(h *harness, id string) error {
				_, e := h.moderation.Suspend(context.Background(), "a", id, "why")
				return e
			},
		},
		"audit": {
			func(r *memoryRepo) { r.eventErr = errStore },
			func(h *harness, id string) error {
				_, e := h.moderation.Suspend(context.Background(), "a", id, "why")
				return e
			},
		},
		"list": {
			func(r *memoryRepo) { r.listErr = errStore },
			func(h *harness, _ string) error {
				_, e := h.moderation.List(context.Background(), application.ListRequest{})
				return e
			},
		},
		"history": {
			func(r *memoryRepo) { r.historyErr = errStore },
			func(h *harness, id string) error { _, e := h.moderation.History(context.Background(), id, 0); return e },
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			merchant := h.approve(t, "usr_1")
			tc.broken(h.repo)

			if got := errs.KindOf(tc.call(h, merchant.ID)); got != errs.KindUnavailable {
				t.Errorf("kind = %v, want unavailable", got)
			}
		})
	}
}

// TestADecisionReportsAFailedIndexUpdate: the two writes may disagree for the
// moment between them; what must not happen is that they disagree silently and
// permanently.
func TestADecisionReportsAFailedIndexUpdate(t *testing.T) {
	h := newHarness(t)
	h.register(t, "usr_1", validRequest())
	h.documentAll(t, "usr_1", domain.TypeRestaurant)
	merchant, err := h.registration.SubmitForReview(context.Background(), "usr_1")
	if err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}
	h.geo.placeErr = errs.New(errs.KindUnavailable, "merchant_place_failed", "Try again.")

	if _, err := h.moderation.Approve(context.Background(), "usr_admin", merchant.ID, ""); errs.CodeOf(err) != "merchant_place_failed" {
		t.Errorf("code = %q, want merchant_place_failed", errs.CodeOf(err))
	}
	if len(h.repo.events) != 1 {
		t.Errorf("events = %d, want the audit entry withheld when the index did not take", len(h.repo.events))
	}
}

// ------------------------------------------------------------------ listing

func TestTheApprovalQueueFiltersByStatusTypeAndDivision(t *testing.T) {
	h := newHarness(t)

	// Three shops: an approved restaurant in Dhaka, a pending grocery in
	// Dhaka, and a pending grocery in Rangpur.
	h.approve(t, "usr_1")

	grocery := validRequest()
	grocery.Type = "grocery"
	h.register(t, "usr_2", grocery)
	h.documentAll(t, "usr_2", domain.TypeGrocery)
	if _, err := h.registration.SubmitForReview(context.Background(), "usr_2"); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}

	h.geo.area.DivisionCode = "BD-H"
	h.register(t, "usr_3", grocery)
	h.documentAll(t, "usr_3", domain.TypeGrocery)
	if _, err := h.registration.SubmitForReview(context.Background(), "usr_3"); err != nil {
		t.Fatalf("SubmitForReview: %v", err)
	}

	cases := map[string]struct {
		req  application.ListRequest
		want int
	}{
		"everything":         {application.ListRequest{}, 3},
		"pending":            {application.ListRequest{Status: "pending_review"}, 2},
		"groceries":          {application.ListRequest{Type: "grocery"}, 2},
		"Dhaka":              {application.ListRequest{DivisionCode: "BD-C"}, 2},
		"pending in Rangpur": {application.ListRequest{Status: "pending_review", DivisionCode: "BD-H"}, 1},
		"approved groceries": {application.ListRequest{Status: "approved", Type: "grocery"}, 0},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			found, err := h.moderation.List(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(found) != tc.want {
				t.Errorf("found %d shops, want %d", len(found), tc.want)
			}
		})
	}
}

func TestTheApprovalQueueRefusesFiltersItDoesNotUnderstand(t *testing.T) {
	h := newHarness(t)

	if _, err := h.moderation.List(context.Background(), application.ListRequest{Status: "banished"}); errs.CodeOf(err) != "unknown_status" {
		t.Errorf("code = %q, want unknown_status", errs.CodeOf(err))
	}
	if _, err := h.moderation.List(context.Background(), application.ListRequest{Type: "hardware"}); errs.CodeOf(err) != "unknown_merchant_type" {
		t.Errorf("code = %q, want unknown_merchant_type", errs.CodeOf(err))
	}
}

func TestTheApprovalQueueIsPaged(t *testing.T) {
	h := newHarness(t)
	for _, owner := range []string{"usr_1", "usr_2", "usr_3"} {
		h.register(t, owner, validRequest())
		h.clock.Advance(time.Minute)
	}

	first, err := h.moderation.List(context.Background(), application.ListRequest{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first page = %d, want 2", len(first))
	}

	second, err := h.moderation.List(context.Background(), application.ListRequest{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(second) != 1 {
		t.Errorf("second page = %d, want 1", len(second))
	}
	if first[0].ID == second[0].ID {
		t.Error("the two pages overlap")
	}
}

// TestAnAbsurdPageSizeFallsBackToTheDefault: an unbounded listing is the
// easiest way to make the admin console time out.
func TestAnAbsurdPageSizeFallsBackToTheDefault(t *testing.T) {
	h := newHarness(t)
	for _, limit := range []int{0, -5, 10_000} {
		if _, err := h.moderation.List(context.Background(),
			application.ListRequest{Limit: limit, Offset: -1}); err != nil {
			t.Errorf("List(limit=%d): %v", limit, err)
		}
	}
}
