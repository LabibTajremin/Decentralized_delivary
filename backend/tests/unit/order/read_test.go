package order

import (
	"context"
	"testing"

	cartcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// placeMany puts n orders on the books for one customer at one shop.
func placeMany(t *testing.T, r *rig, n int) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		r.cart.cart = readyCart()
		ids = append(ids, place(t, r, "").ID)
	}
	return ids
}

func TestACustomerSeesTheirOwnOrders(t *testing.T) {
	r := newRig()
	placeMany(t, r, 3)

	got, err := r.reads.OfCustomer(context.Background(), "USR-1", application.ListQuery{Lang: "en"})
	if err != nil {
		t.Fatalf("OfCustomer: %v", err)
	}
	if got.Total != 3 || len(got.Orders) != 3 {
		t.Fatalf("got %d of %d", len(got.Orders), got.Total)
	}
	for _, view := range got.Orders {
		if view.StatusLabel == "" || len(view.Receipt) == 0 {
			t.Errorf("a list entry is not composed: %+v", view)
		}
	}

	// And nobody else's.
	empty, err := r.reads.OfCustomer(context.Background(), "USR-2", application.ListQuery{})
	if err != nil {
		t.Fatalf("OfCustomer: %v", err)
	}
	if empty.Total != 0 {
		t.Fatalf("another customer saw %d orders", empty.Total)
	}
}

func TestAShopSeesItsOwnQueue(t *testing.T) {
	r := newRig()
	placeMany(t, r, 2)

	got, err := r.reads.OfMerchant(context.Background(), "MER-1", application.ListQuery{})
	if err != nil {
		t.Fatalf("OfMerchant: %v", err)
	}
	if got.Total != 2 {
		t.Fatalf("total = %d", got.Total)
	}
	other, err := r.reads.OfMerchant(context.Background(), "MER-2", application.ListQuery{})
	if err != nil {
		t.Fatalf("OfMerchant: %v", err)
	}
	if other.Total != 0 {
		t.Fatalf("another shop saw %d orders", other.Total)
	}
}

// The current-orders tab.
func TestTheLiveFilter(t *testing.T) {
	r := newRig()
	ids := placeMany(t, r, 3)
	move(t, r, ids[0], domain.StatusCancelled, "", customer())

	live, err := r.reads.OfCustomer(context.Background(), "USR-1", application.ListQuery{LiveOnly: true})
	if err != nil {
		t.Fatalf("OfCustomer: %v", err)
	}
	if live.Total != 2 {
		t.Fatalf("live = %d, want 2", live.Total)
	}
	all, err := r.reads.OfCustomer(context.Background(), "USR-1", application.ListQuery{})
	if err != nil {
		t.Fatalf("OfCustomer: %v", err)
	}
	if all.Total != 3 {
		t.Fatalf("all = %d, want 3", all.Total)
	}
}

func TestListPaging(t *testing.T) {
	r := newRig()
	placeMany(t, r, 5)
	ctx := context.Background()

	cases := []struct {
		name          string
		limit, offset int
		wantCount     int
	}{
		{"a default page", 0, 0, 5},
		{"a first page of two", 2, 0, 2},
		{"a second page of two", 2, 2, 2},
		{"past the end", 2, 9, 0},
		// A client asking for a thousand wants "as many as you will give me".
		{"an absurd limit is clamped", 1000, 0, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.reads.OfCustomer(ctx, "USR-1", application.ListQuery{
				Limit: tc.limit, Offset: tc.offset,
			})
			if err != nil {
				t.Fatalf("OfCustomer: %v", err)
			}
			if len(got.Orders) != tc.wantCount {
				t.Fatalf("got %d entries, want %d", len(got.Orders), tc.wantCount)
			}
			// The total is the whole count, not the page.
			if got.Total != 5 {
				t.Errorf("total = %d, want 5", got.Total)
			}
		})
	}
}

// A shop reading an order sees the shop's next actions, and no cancellation
// countdown — that is the customer's, and offering it to a shop would suggest
// they can use it.
func TestTheSameOrderLooksDifferentToDifferentPeople(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")
	ctx := context.Background()

	theirs, err := r.reads.One(ctx, placed.ID, customer())
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	if len(theirs.NextActions) != 1 || theirs.NextActions[0] != "cancelled" {
		t.Fatalf("customer sees %v", theirs.NextActions)
	}
	if !theirs.Cancel.Allowed {
		t.Errorf("cancel = %+v", theirs.Cancel)
	}

	shops, err := r.reads.One(ctx, placed.ID, shopkeeper())
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	if len(shops.NextActions) != 2 {
		t.Fatalf("shop sees %v", shops.NextActions)
	}
	if shops.Cancel.Allowed || shops.Cancel.SecondsLeft != 0 {
		t.Errorf("a shop was offered the customer's countdown: %+v", shops.Cancel)
	}

	// An admin sees everything and is bound by nothing but the state machine.
	adminView, err := r.reads.One(ctx, placed.ID, operator())
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	if len(adminView.NextActions) != 3 {
		t.Errorf("admin sees %v", adminView.NextActions)
	}
}

// Bengali is the default, because the audience reads Bengali (1.4).
func TestTheOrderScreenSpeaksBengaliByDefault(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")

	view, err := r.reads.One(context.Background(), placed.ID,
		application.Caller{Actor: domain.ActorCustomer, ID: "USR-1"})
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	if view.StatusLabel != "দোকানে পাঠানো হয়েছে" {
		t.Errorf("status label = %q", view.StatusLabel)
	}
	if view.Receipt[0].Label != "পণ্যের মোট" {
		t.Errorf("receipt label = %q", view.Receipt[0].Label)
	}
}

// Every status has a sentence in both languages. A missing one shows the
// customer a blank line where the most important word on the screen should be.
func TestEveryStatusHasASentence(t *testing.T) {
	r := newRig()
	statuses := []domain.Status{
		domain.StatusPendingPayment, domain.StatusPlaced, domain.StatusAccepted,
		domain.StatusPreparing, domain.StatusReady, domain.StatusPickedUp,
		domain.StatusDelivered, domain.StatusCancelled, domain.StatusRejected,
		domain.StatusFailed,
	}
	seen := map[string]bool{}
	for _, status := range statuses {
		r.cart.cart = readyCart()
		placed := place(t, r, "")
		order, _ := r.repo.Order(context.Background(), placed.ID)
		order.Status = status
		r.repo.orders[placed.ID] = order

		for _, lang := range []string{"", "en"} {
			view, err := r.reads.One(context.Background(), placed.ID,
				application.Caller{Actor: domain.ActorCustomer, ID: "USR-1", Lang: lang})
			if err != nil {
				t.Fatalf("One: %v", err)
			}
			if view.StatusLabel == "" {
				t.Errorf("%s in %q has no sentence", status, lang)
			}
			seen[view.StatusLabel] = true
		}
	}
	// Twenty distinct sentences: ten statuses in two languages, none reused.
	if len(seen) != len(statuses)*2 {
		t.Errorf("%d distinct sentences for %d statuses in two languages", len(seen), len(statuses))
	}
}

// The free-delivery row appears on a receipt that earned it, and the receipt is
// rebuilt from what was frozen rather than re-quoted.
func TestTheFrozenReceipt(t *testing.T) {
	r := newRig()
	free := place(t, r, "") // ৳600 of goods clears the threshold

	view, err := r.reads.One(context.Background(), free.ID, customer())
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	keys := map[string]bool{}
	for _, row := range view.Receipt {
		keys[row.Key] = true
		if row.Label == "" || row.Amount.Display == "" {
			t.Errorf("row %q is not composed: %+v", row.Key, row)
		}
	}
	if !keys["free_delivery"] {
		t.Errorf("receipt = %+v", view.Receipt)
	}

	// A cheaper order gets no such row.
	cart := readyCart()
	cart.Subtotal = cartcontract.Money{Minor: 25000}
	r.cart.cart = cart
	paid := place(t, r, "")
	view, err = r.reads.One(context.Background(), paid.ID, customer())
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	for _, row := range view.Receipt {
		if row.Key == "free_delivery" {
			t.Errorf("a paid delivery got a waiver row: %+v", view.Receipt)
		}
	}
}

func TestReadFailures(t *testing.T) {
	r := newRig()
	r.repo.listErr = errBoom
	ctx := context.Background()

	if _, err := r.reads.OfCustomer(ctx, "USR-1", application.ListQuery{}); errs.CodeOf(err) != "order_unavailable" {
		t.Fatalf("customer list: err = %v", err)
	}
	if _, err := r.reads.OfMerchant(ctx, "MER-1", application.ListQuery{}); errs.CodeOf(err) != "order_unavailable" {
		t.Fatalf("shop list: err = %v", err)
	}

	broken := newRig()
	broken.repo.readErr = errBoom
	if _, err := broken.reads.One(ctx, "ORD-1", customer()); errs.CodeOf(err) != "order_unavailable" {
		t.Fatalf("read: err = %v", err)
	}
}

// A read whose cancellation window cannot be resolved still returns the order.
// The countdown is the one part that can be wrong without the screen being
// useless.
func TestAnUnreadableWindowStillReturnsTheOrder(t *testing.T) {
	r := newRig()
	placed := place(t, r, "")
	r.config.err = errBoom

	view, err := r.reads.One(context.Background(), placed.ID, customer())
	if err != nil {
		t.Fatalf("One: %v", err)
	}
	if view.ID != placed.ID || view.Cancel.Allowed {
		t.Fatalf("view = %+v", view)
	}
}
