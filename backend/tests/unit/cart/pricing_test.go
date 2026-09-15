package cart

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/contract"
)

// The cart's receipt. The cart owns the goods and pricing owns everything after
// them, so what is asserted here is the seam: the subtotal the cart computed
// reaching pricing, and pricing's answer reaching the screen unaltered.

func TestACartWithAnAddressIsPriced(t *testing.T) {
	r := withBurger()
	add(t, r, "ITM-burger", 2) // ৳250 each → ৳500 of goods
	r.discovery.reach = contract.Reach{Reachable: true, DivisionCode: "DHA", DistanceM: 2400}

	view, err := r.carts.SetAddress(context.Background(), "USR-1", "ADR-1", 23.7, 90.4, "en")
	if err != nil {
		t.Fatalf("SetAddress: %v", err)
	}
	if view.Pricing == nil {
		t.Fatal("a reachable cart was not priced")
	}
	// ৳500 of goods clears the ৳500 free-delivery threshold.
	if !view.Pricing.FreeDelivery || view.Pricing.Delivery.Minor != 0 {
		t.Fatalf("pricing = %+v", view.Pricing)
	}
	if view.Pricing.Total.Minor != 50000 || view.Pricing.Subtotal.Minor != view.Subtotal.Minor {
		t.Fatalf("the cart's subtotal did not reach pricing: cart %d, quote %+v",
			view.Subtotal.Minor, view.Pricing)
	}
	if view.Pricing.DistanceM != 2400 {
		t.Errorf("distance = %v, want the one discovery measured", view.Pricing.DistanceM)
	}
	// The receipt is composed, not assembled by the client.
	if len(view.Pricing.Rows) == 0 || view.Pricing.Rows[0].Label == "" {
		t.Errorf("rows = %+v", view.Pricing.Rows)
	}
	// One tariff read per render, not one per line.
	if len(r.pricing.seen) != 1 {
		t.Errorf("pricing was asked %d times for one cart", len(r.pricing.seen))
	}
	if r.pricing.seen[0].DivisionCode != "DHA" {
		t.Errorf("pricing was asked about %+v", r.pricing.seen[0])
	}
}

// Below the threshold, the customer pays — and is told how much more would
// change that, without ever holding the threshold themselves (2.9).
func TestACheapCartPaysForDelivery(t *testing.T) {
	r := withBurger()
	add(t, r, "ITM-burger", 1) // ৳250
	r.discovery.reach = contract.Reach{Reachable: true, DivisionCode: "DHA", DistanceM: 2400}

	view, err := r.carts.SetAddress(context.Background(), "USR-1", "ADR-1", 23.7, 90.4, "en")
	if err != nil {
		t.Fatalf("SetAddress: %v", err)
	}
	// ৳40 + 3 × ৳10 = ৳70.
	if view.Pricing.Delivery.Minor != 7000 || view.Pricing.Total.Minor != 32000 {
		t.Fatalf("pricing = %+v", view.Pricing)
	}
	if view.Pricing.AwayFromFreeDelivery.Minor != 25000 || view.Pricing.Notice == "" {
		t.Errorf("pricing = %+v", view.Pricing)
	}
}

// D2 reaching the cart: a shop the customer had to widen their search to find
// costs more to deliver from, and the receipt says by how much.
func TestAnExpandedCartCarriesTheSurcharge(t *testing.T) {
	r := withBurger()
	add(t, r, "ITM-burger", 1)
	r.discovery.reach = contract.Reach{
		Reachable: true, DivisionCode: "DHA", DistanceM: 8000,
		RequiredLevel: 1, Expanded: true,
	}

	view, err := r.carts.SetAddress(context.Background(), "USR-1", "ADR-1", 23.7, 90.4, "en")
	if err != nil {
		t.Fatalf("SetAddress: %v", err)
	}
	if !view.Pricing.Expanded || view.Pricing.ExpansionSurcharge.Minor != 6000 {
		t.Fatalf("pricing = %+v", view.Pricing)
	}
	found := false
	for _, row := range view.Pricing.Rows {
		if row.Key == "expansion_surcharge" {
			found = true
		}
	}
	if !found {
		t.Errorf("the receipt has no surcharge row: %+v", view.Pricing.Rows)
	}
}

// A delivery charge for a journey that cannot happen is a number the customer
// would reasonably take for a promise.
func TestAnUnreachableOrAddresslessCartIsNotPriced(t *testing.T) {
	t.Run("no address", func(t *testing.T) {
		r := withBurger()
		view := add(t, r, "ITM-burger", 1)
		if view.Pricing != nil {
			t.Fatalf("a cart with no address was priced: %+v", view.Pricing)
		}
		if len(r.pricing.seen) != 0 {
			t.Errorf("pricing was consulted anyway: %+v", r.pricing.seen)
		}
	})

	t.Run("another division", func(t *testing.T) {
		r := withBurger()
		add(t, r, "ITM-burger", 1)
		r.discovery.reach = contract.Reach{Reason: contract.ReasonOutsideDivision}
		view, err := r.carts.SetAddress(context.Background(), "USR-1", "ADR-1", 22.3, 91.8, "en")
		if err != nil {
			t.Fatalf("SetAddress: %v", err)
		}
		if view.Pricing != nil {
			t.Fatalf("an unreachable cart was priced: %+v", view.Pricing)
		}
		if view.Blocker != string(domain.BlockerOutsideDivision) {
			t.Errorf("blocker = %q", view.Blocker)
		}
	})
}

func TestAFailedQuoteFailsTheRead(t *testing.T) {
	r := withBurger()
	add(t, r, "ITM-burger", 1)
	if _, err := r.carts.SetAddress(context.Background(), "USR-1", "ADR-1", 23.7, 90.4, "en"); err != nil {
		t.Fatalf("SetAddress: %v", err)
	}

	// A cart shown without its total is a cart the customer cannot decide
	// about. Failing the read is the honest answer.
	r.pricing.err = errBoom
	if _, _, err := r.carts.Current(context.Background(), "USR-1", "en"); err == nil {
		t.Fatal("a cart was rendered with no total")
	}

	// And a tariff that resolves but cannot answer.
	r.pricing.err = nil
	broken := appendixB()
	broken.ratios["pricing.expansion_multiplier"] = 0.5
	r.pricing.settings = broken
	if _, _, err := r.carts.Current(context.Background(), "USR-1", "en"); err == nil {
		t.Fatal("a cart was priced against an impossible tariff")
	}
}

// The quote crosses the contract order will call, restated in the cart's own
// primitives because a contract may not import another module's contract.
func TestTheContractCarriesTheQuote(t *testing.T) {
	r := withBurger()
	add(t, r, "ITM-burger", 1)
	r.discovery.reach = contract.Reach{Reachable: true, DivisionCode: "DHA", DistanceM: 2400}
	if _, err := r.carts.SetAddress(context.Background(), "USR-1", "ADR-1", 23.7, 90.4, ""); err != nil {
		t.Fatalf("SetAddress: %v", err)
	}

	got, found, err := r.service.Current(context.Background(), "USR-1")
	if err != nil || !found {
		t.Fatalf("Current: found = %v, err = %v", found, err)
	}
	if got.Delivery == nil {
		t.Fatal("the contract carried no quote")
	}
	if got.Delivery.Fee.Minor != 7000 || got.Delivery.Total.Minor != 32000 {
		t.Fatalf("delivery = %+v", got.Delivery)
	}
	if got.Delivery.Fee.Display == "" {
		t.Error("money crossed the contract with no rendered string")
	}
	if got.Delivery.DistanceM != 2400 || got.Delivery.Expanded || got.Delivery.FreeDelivery {
		t.Errorf("delivery = %+v", got.Delivery)
	}
}

// A distance that cannot be priced. Unreachable while geo is the only source of
// distances — it never returns a negative one — but the cart takes the number
// on trust from another module, and a cart that showed a total worked out from
// nonsense would be worse than one that refused.
func TestANonsensicalDistanceFailsTheQuote(t *testing.T) {
	r := withBurger()
	add(t, r, "ITM-burger", 1)
	r.discovery.reach = contract.Reach{Reachable: true, DivisionCode: "DHA", DistanceM: -1}

	if _, err := r.carts.SetAddress(context.Background(), "USR-1", "ADR-1", 23.7, 90.4, "en"); err == nil {
		t.Fatal("a cart was priced over a negative distance")
	}
}
