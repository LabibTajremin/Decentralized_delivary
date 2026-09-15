package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// These drive a customer filling a cart through the real binary against real
// Postgres and the real seeded menus. The phase's two acceptance criteria —
// single-merchant enforcement, and a cart that invalidates when the address
// changes — are checked here at the only level that proves them: the API a
// Flutter app will actually call.

type cartLineBody struct {
	ID        string `json:"id"`
	TargetID  string `json:"target_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Issue     string `json:"issue"`
	IssueText string `json:"issue_text"`
	Orderable bool   `json:"orderable"`
	UnitPrice struct {
		Minor   int64  `json:"minor"`
		Display string `json:"display"`
	} `json:"unit_price"`
	LineTotal struct {
		Minor int64 `json:"minor"`
	} `json:"line_total"`
}

type cartResponse struct {
	ID           string         `json:"id"`
	MerchantID   string         `json:"merchant_id"`
	MerchantName string         `json:"merchant_name"`
	AddressID    string         `json:"address_id"`
	Lines        []cartLineBody `json:"lines"`
	Count        int            `json:"count"`
	Subtotal     struct {
		Minor   int64  `json:"minor"`
		Display string `json:"display"`
	} `json:"subtotal"`
	Orderable   bool   `json:"orderable"`
	Blocker     string `json:"blocker"`
	BlockerText string `json:"blocker_text"`
}

// seededMenuItem finds an orderable item with no required options, from a
// seeded shop of the given type near a point.
func seededMenuItem(t *testing.T, base, merchantID string) (itemID string, minor int64) {
	t.Helper()
	var menu struct {
		Items []struct {
			ID        string `json:"id"`
			Orderable bool   `json:"orderable"`
			Price     struct {
				Minor int64 `json:"minor"`
			} `json:"price"`
			VariantGroups []struct {
				Required bool `json:"required"`
			} `json:"variant_groups"`
			AddOnGroups []struct {
				Required bool `json:"required"`
			} `json:"addon_groups"`
		} `json:"items"`
	}
	if status := getJSON(t, base+"/v1/catalogue/"+merchantID+"/menu", &menu); status != http.StatusOK {
		t.Fatalf("menu for %s: status = %d", merchantID, status)
	}
	for _, item := range menu.Items {
		if !item.Orderable {
			continue
		}
		required := false
		for _, group := range item.VariantGroups {
			if group.Required {
				required = true
			}
		}
		for _, group := range item.AddOnGroups {
			if group.Required {
				required = true
			}
		}
		if !required {
			return item.ID, item.Price.Minor
		}
	}
	t.Fatalf("%s has no orderable item without a required variant", merchantID)
	return "", 0
}

// twoNearbyShops returns two different seeded shops near Dhanmondi.
func twoNearbyShops(t *testing.T, base string) (string, string) {
	t.Helper()
	got := discover(t, base, dhanmondiLat, dhanmondiLng, "level=2&limit=50")
	if len(got.Merchants) < 2 {
		t.Fatalf("only %d shops near Dhanmondi; this test needs two", len(got.Merchants))
	}
	return got.Merchants[0].ID, got.Merchants[1].ID
}

func cartCall(t *testing.T, method, url, body, token string, into any) int {
	t.Helper()
	return requestAs(t, method, url, body, token, into)
}

// The whole journey: open a cart, price it from the menu, give it an address,
// change a quantity, take a line out.
func TestACustomerFillsACart(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer := signInAs(t, base, tail, uniquePhone(t), "customer phone", "customer")
	shopID, _ := twoNearbyShops(t, base)
	itemID, priceMinor := seededMenuItem(t, base, shopID)

	var cart cartResponse
	body := fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":2}`, shopID, itemID)
	if status := cartCall(t, http.MethodPost, base+"/v1/cart/items?lang=en", body, customer.AccessToken, &cart); status != http.StatusOK {
		t.Fatalf("add: status = %d", status)
	}
	if cart.MerchantID != shopID || cart.Count != 2 {
		t.Fatalf("cart = %+v", cart)
	}
	// The price came from the seeded menu, not from the request.
	if cart.Lines[0].UnitPrice.Minor != priceMinor || cart.Subtotal.Minor != priceMinor*2 {
		t.Fatalf("prices = %+v (menu says %d)", cart.Lines[0], priceMinor)
	}
	if cart.Subtotal.Display == "" || cart.Lines[0].UnitPrice.Display == "" {
		t.Error("money crossed the wire with no rendered string")
	}
	if cart.Blocker != "no_address" || cart.Orderable {
		t.Errorf("a cart with no address: blocker = %q, orderable = %v", cart.Blocker, cart.Orderable)
	}
	lineID := cart.Lines[0].ID

	// Adding the same thing again bumps the quantity rather than making a
	// second line.
	var again cartResponse
	if status := cartCall(t, http.MethodPost, base+"/v1/cart/items", body, customer.AccessToken, &again); status != http.StatusOK {
		t.Fatalf("add again: status = %d", status)
	}
	if len(again.Lines) != 1 || again.Lines[0].Quantity != 4 {
		t.Fatalf("a repeat add made %d lines: %+v", len(again.Lines), again.Lines)
	}

	var placed cartResponse
	address := fmt.Sprintf(`{"address_id":"ADR-E2E","lat":%f,"lng":%f}`, dhanmondiLat, dhanmondiLng)
	if status := cartCall(t, http.MethodPut, base+"/v1/cart/address?lang=en", address, customer.AccessToken, &placed); status != http.StatusOK {
		t.Fatalf("address: status = %d", status)
	}
	if placed.AddressID != "ADR-E2E" {
		t.Fatalf("cart = %+v", placed)
	}
	// Seeded shops keep 09:00–22:00 hours, so the cart is orderable only
	// inside them. Either answer is correct; what must not happen is a blocker
	// about the address.
	if placed.Blocker == "outside_division" || placed.Blocker == "beyond_radius" {
		t.Fatalf("a shop found by a Dhanmondi search was unreachable from Dhanmondi: %+v", placed)
	}

	var changed cartResponse
	if status := cartCall(t, http.MethodPut, base+"/v1/cart/lines/"+lineID, `{"quantity":1}`, customer.AccessToken, &changed); status != http.StatusOK {
		t.Fatalf("quantity: status = %d", status)
	}
	if changed.Count != 1 || changed.Subtotal.Minor != priceMinor {
		t.Fatalf("after the change: %+v", changed)
	}

	var emptied cartResponse
	if status := cartCall(t, http.MethodDelete, base+"/v1/cart/lines/"+lineID, "", customer.AccessToken, &emptied); status != http.StatusOK {
		t.Fatalf("remove: status = %d", status)
	}
	if len(emptied.Lines) != 0 || emptied.Blocker != "empty" {
		t.Fatalf("after the removal: %+v", emptied)
	}

	if status := cartCall(t, http.MethodDelete, base+"/v1/cart", "", customer.AccessToken, nil); status != http.StatusNoContent {
		t.Fatalf("clear: status = %d", status)
	}
}

// The first acceptance criterion. An order is collected by one rider from one
// counter, so a cart spanning two shops is two orders wearing one button.
func TestSingleMerchantEnforcementHolds(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer := signInAs(t, base, tail, uniquePhone(t), "customer phone", "customer")
	firstShop, secondShop := twoNearbyShops(t, base)
	firstItem, _ := seededMenuItem(t, base, firstShop)
	secondItem, secondPrice := seededMenuItem(t, base, secondShop)

	if status := cartCall(t, http.MethodPost, base+"/v1/cart/items",
		fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":1}`, firstShop, firstItem),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("first add: status = %d", status)
	}

	var refused struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	status := cartCall(t, http.MethodPost, base+"/v1/cart/items",
		fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":1}`, secondShop, secondItem),
		customer.AccessToken, &refused)
	if status != http.StatusConflict {
		t.Fatalf("a second shop returned %d, want 409", status)
	}
	if refused.Error.Code != "different_merchant" || refused.Error.Message == "" {
		t.Fatalf("error = %+v", refused.Error)
	}

	// The refused add changed nothing.
	var unchanged cartResponse
	if status := cartCall(t, http.MethodGet, base+"/v1/cart", "", customer.AccessToken, &unchanged); status != http.StatusOK {
		t.Fatalf("get: status = %d", status)
	}
	if unchanged.MerchantID != firstShop || len(unchanged.Lines) != 1 {
		t.Fatalf("the refused add altered the cart: %+v", unchanged)
	}

	// Replace takes the destructive step deliberately.
	var replaced cartResponse
	if status := cartCall(t, http.MethodPost, base+"/v1/cart/replace",
		fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":1}`, secondShop, secondItem),
		customer.AccessToken, &replaced); status != http.StatusOK {
		t.Fatalf("replace: status = %d", status)
	}
	if replaced.MerchantID != secondShop || len(replaced.Lines) != 1 ||
		replaced.Lines[0].TargetID != secondItem || replaced.Subtotal.Minor != secondPrice {
		t.Fatalf("replaced = %+v", replaced)
	}
}

// The second acceptance criterion, and D3 through two modules: the cart asks
// discovery, discovery asks geo, and the customer finds out while the cart is
// still small.
func TestACartInvalidatesWhenTheAddressLeavesTheDivision(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer := signInAs(t, base, tail, uniquePhone(t), "customer phone", "customer")
	shopID, _ := twoNearbyShops(t, base)
	itemID, _ := seededMenuItem(t, base, shopID)

	if status := cartCall(t, http.MethodPost, base+"/v1/cart/items",
		fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":1}`, shopID, itemID),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("add: status = %d", status)
	}

	var near cartResponse
	if status := cartCall(t, http.MethodPut, base+"/v1/cart/address?lang=en",
		fmt.Sprintf(`{"address_id":"ADR-HOME","lat":%f,"lng":%f}`, dhanmondiLat, dhanmondiLng),
		customer.AccessToken, &near); status != http.StatusOK {
		t.Fatalf("home address: status = %d", status)
	}
	if near.Blocker == "outside_division" {
		t.Fatalf("a Dhanmondi address was outside the division of a Dhanmondi shop: %+v", near)
	}

	// The customer switches to an address in Chattogram. D3 makes that
	// permanent — no expansion level reaches across a division boundary.
	var away cartResponse
	if status := cartCall(t, http.MethodPut, base+"/v1/cart/address?lang=en",
		fmt.Sprintf(`{"address_id":"ADR-CTG","lat":%f,"lng":%f}`, agrabadLat, agrabadLng),
		customer.AccessToken, &away); status != http.StatusOK {
		t.Fatalf("far address: status = %d", status)
	}
	if away.Orderable || away.Blocker != "outside_division" {
		t.Fatalf("a cross-division address left the cart orderable: %+v", away)
	}
	if away.BlockerText == "" {
		t.Error("no sentence explaining the block")
	}
	// The cart itself survives: the customer can switch the address back.
	if len(away.Lines) != 1 {
		t.Errorf("the cart was emptied rather than blocked: %+v", away.Lines)
	}

	var back cartResponse
	if status := cartCall(t, http.MethodPut, base+"/v1/cart/address?lang=en",
		fmt.Sprintf(`{"address_id":"ADR-HOME","lat":%f,"lng":%f}`, dhanmondiLat, dhanmondiLng),
		customer.AccessToken, &back); status != http.StatusOK {
		t.Fatalf("back home: status = %d", status)
	}
	if back.Blocker == "outside_division" {
		t.Fatalf("the block did not lift when the address came back: %+v", back)
	}
}

// One cart per customer, and it is the caller's own. The user id comes from
// the token, so there is no id to pass and no cart to reach.
func TestACartIsPrivateToItsOwner(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	first := signInAs(t, base, tail, uniquePhone(t), "first phone", "customer")
	second := signInAs(t, base, tail, uniquePhone(t), "second phone", "customer")

	shopID, _ := twoNearbyShops(t, base)
	itemID, _ := seededMenuItem(t, base, shopID)

	if status := cartCall(t, http.MethodPost, base+"/v1/cart/items",
		fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":3}`, shopID, itemID),
		first.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("add: status = %d", status)
	}

	var theirs cartResponse
	if status := cartCall(t, http.MethodGet, base+"/v1/cart", "", second.AccessToken, &theirs); status != http.StatusOK {
		t.Fatalf("get: status = %d", status)
	}
	if len(theirs.Lines) != 0 || theirs.Count != 0 {
		t.Fatalf("a second customer saw the first one's cart: %+v", theirs)
	}

	// And nothing at all without a token.
	if status := getJSON(t, base+"/v1/cart", nil); status != http.StatusUnauthorized {
		t.Fatalf("an anonymous cart read returned %d, want 401", status)
	}
}
