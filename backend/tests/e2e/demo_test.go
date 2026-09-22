package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

// The seeded demo accounts, as docs/demo.md publishes them. Written out here
// rather than read from the seed so that changing a number in the seed and
// forgetting the documentation fails the build.
const (
	demoCustomerPhone = "01700000001"
	demoMerchantPhone = "01700000011"
	demoPartnerPhone  = "01700000021"
	demoAdminPhone    = "01700000031"

	// The shop 0005_merchant.sql already gave USR-DEMO-0001, which
	// 0009_demo_accounts.sql gives a phone number.
	demoMerchantShop = "MER-DEMO-0001"
)

// demoSecondCustomerPhone is the other seeded customer, in Mirpur.
const demoSecondCustomerPhone = "01700000002"

// demoPhones is every number the seed publishes, in the order docs/demo.md
// lists them. Only used to reset their rate limits between tests.
var demoPhones = []string{
	demoCustomerPhone, demoSecondCustomerPhone,
	demoMerchantPhone, "01700000012",
	demoPartnerPhone, "01700000022",
	demoAdminPhone,
}

// startDemoAPI runs the real binary as a demo deployment.
func startDemoAPI(t *testing.T) (string, func()) {
	t.Helper()
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Fatal("REDIS_URL is not set; the demo tests need a real Redis")
	}
	dbURL := dbtestSchemaURL(t)
	seedDemoData(t, dbURL)
	clearDemoRateLimits(t, redisURL)

	return startAPI(t, buildAPI(t),
		"DATABASE_URL="+dbURL, "REDIS_URL="+redisURL, "DEMO_MODE=true")
}

// clearDemoRateLimits resets the OTP request counters for the demo numbers.
//
// Redis is the one store these tests share and nothing resets: seedDemoData
// rebuilds the schema for every test, but the rate limiter lives in Redis with
// an hour-long window, so a handful of tests signing in as the same demo
// number exhaust `auth.otp_requests_per_hour` and the rest fail with 429 for
// reasons that have nothing to do with what they assert.
//
// It is worth knowing that this is not only a test problem. A published demo
// shares its numbers among everyone looking at it, and five sign-ins an hour
// is not many — docs/demo.md tells an operator to raise the limit, which is an
// ordinary admin setting rather than anything demo mode changes.
func clearDemoRateLimits(t *testing.T, redisURL string) {
	t.Helper()
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("parse REDIS_URL: %v", err)
	}
	client := goredis.NewClient(opts)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	for _, phone := range demoPhones {
		// The key the limiter builds: "rate:" + the caller's key, and
		// identity's key is "otp_request:" + the number in E.164.
		key := "rate:otp_request:+88" + phone
		if err := client.Del(ctx, key).Err(); err != nil {
			t.Fatalf("clear %s: %v", key, err)
		}
	}
}

// addressBook is what the address list answers.
type addressBook struct {
	Addresses []addressPayload `json:"addresses"`
}

// challengeBody is what a code request answers.
type challengeBody struct {
	Phone       string `json:"phone"`
	ExpiresIn   int64  `json:"expires_in"`
	ResendAfter int64  `json:"resend_after"`
	DemoCode    string `json:"demo_code"`
}

// demoSignIn signs in the way a demo visitor does: ask for a code, read it off
// the response, verify it.
//
// No log tail. That is the point — a published demo has no terminal anybody
// can watch, which is the whole reason DEMO_MODE exists.
func demoSignIn(t *testing.T, base, phone, role string) tokens {
	t.Helper()
	var challenge challengeBody
	if status := postJSON(t, base+"/v1/auth/otp/request",
		fmt.Sprintf(`{"phone":%q}`, phone), "", &challenge); status != http.StatusOK {
		t.Fatalf("request code for %s: status = %d", phone, status)
	}
	if challenge.DemoCode == "" {
		t.Fatalf("no code was revealed for %s, so nobody can sign in", phone)
	}

	var pair tokens
	if status := postJSON(t, base+"/v1/auth/otp/verify", fmt.Sprintf(
		`{"phone":%q,"code":%q,"role":%q,"device":"demo"}`,
		phone, challenge.DemoCode, role), "", &pair); status != http.StatusOK {
		t.Fatalf("verify %s as %s: status = %d", phone, role, status)
	}
	return pair
}

// TestADemoRevealsItsCodeAndAnOrdinaryDeploymentDoesNot is the pair of
// assertions demo mode stands on.
//
// The first is what makes a public demonstration possible: a visitor types a
// number nobody owns and is shown the code, because there is no handset to
// send it to. The second is what keeps that from being a hole — the same
// binary, the same request, without DEMO_MODE, reveals nothing.
func TestADemoRevealsItsCodeAndAnOrdinaryDeploymentDoesNot(t *testing.T) {
	t.Run("demo mode reveals it", func(t *testing.T) {
		base, stop := startDemoAPI(t)
		defer stop()

		phone := uniquePhone(t)
		var challenge challengeBody
		if status := postJSON(t, base+"/v1/auth/otp/request",
			fmt.Sprintf(`{"phone":%q}`, phone), "", &challenge); status != http.StatusOK {
			t.Fatalf("status = %d", status)
		}
		if len(challenge.DemoCode) != 6 {
			t.Fatalf("demo_code = %q, want six digits", challenge.DemoCode)
		}
		// Everything else is unchanged: the number is still masked, and the
		// countdown is still the server's.
		if challenge.Phone == phone || challenge.ExpiresIn == 0 {
			t.Errorf("challenge = %+v", challenge)
		}
	})

	t.Run("an ordinary deployment does not", func(t *testing.T) {
		base, _, stop := startAuthAPI(t)
		defer stop()

		var challenge challengeBody
		if status := postJSON(t, base+"/v1/auth/otp/request",
			fmt.Sprintf(`{"phone":%q}`, uniquePhone(t)), "", &challenge); status != http.StatusOK {
			t.Fatalf("status = %d", status)
		}
		if challenge.DemoCode != "" {
			t.Fatalf("a deployment that really sends an SMS also handed the code back: %q",
				challenge.DemoCode)
		}
	})
}

// TestTheRevealedCodeIsAnOrdinaryCode. Demo mode changes how a code reaches
// the person asking for it and nothing else — so a wrong code is still wrong,
// and the real one still works exactly once.
func TestTheRevealedCodeIsAnOrdinaryCode(t *testing.T) {
	base, stop := startDemoAPI(t)
	defer stop()

	phone := uniquePhone(t)
	var challenge challengeBody
	if status := postJSON(t, base+"/v1/auth/otp/request",
		fmt.Sprintf(`{"phone":%q}`, phone), "", &challenge); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}

	wrong := fmt.Sprintf(`{"phone":%q,"code":"000000","role":"customer","device":"d"}`, phone)
	if status := postJSON(t, base+"/v1/auth/otp/verify", wrong, "", nil); status == http.StatusOK {
		t.Fatal("a wrong code was accepted on a demo deployment")
	}

	right := fmt.Sprintf(`{"phone":%q,"code":%q,"role":"customer","device":"d"}`,
		phone, challenge.DemoCode)
	var pair tokens
	if status := postJSON(t, base+"/v1/auth/otp/verify", right, "", &pair); status != http.StatusOK {
		t.Fatalf("the revealed code did not verify: status = %d", status)
	}
	if pair.AccessToken == "" {
		t.Fatal("no token was issued")
	}

	// Once. The code is consumed on the way through, demo or not.
	if status := postJSON(t, base+"/v1/auth/otp/verify", right, "", nil); status == http.StatusOK {
		t.Error("the same code verified twice")
	}
}

// TestEveryDemoAccountLandsSomewhereUseful is what the seed is for.
//
// A demo whose sign-ins work but drop everybody into an empty app is not a
// demo. Each of these four accounts has to arrive already owning the thing
// their app is about, and this walks all four.
func TestEveryDemoAccountLandsSomewhereUseful(t *testing.T) {
	base, stop := startDemoAPI(t)
	defer stop()

	t.Run("the customer has an address to order to", func(t *testing.T) {
		customer := demoSignIn(t, base, demoCustomerPhone, "customer")
		var book addressBook
		if status := requestAs(t, http.MethodGet, base+"/v1/me/addresses", "",
			customer.AccessToken, &book); status != http.StatusOK {
			t.Fatalf("addresses: status = %d", status)
		}
		addresses := book.Addresses
		if len(addresses) == 0 {
			t.Fatal("the demo customer has no saved address, so Home has nothing to search from")
		}
		if !addresses[0].IsDefault || addresses[0].AreaCode == "" {
			t.Errorf("address = %+v", addresses[0])
		}

		// And that address really can reach a shop, which is the thing a
		// visitor will try first.
		found := discover(t, base, addresses[0].Lat, addresses[0].Lng, "lang=en")
		if found.Total == 0 {
			t.Fatal("the demo address has no shops near it")
		}
	})

	t.Run("the merchant already owns an approved, open shop", func(t *testing.T) {
		owner := demoSignIn(t, base, demoMerchantPhone, "merchant")
		var shop merchantPayload
		if status := requestAs(t, http.MethodGet, base+"/v1/merchants/me", "",
			owner.AccessToken, &shop); status != http.StatusOK {
			t.Fatalf("merchants/me: status = %d", status)
		}
		if shop.ID != demoMerchantShop {
			t.Fatalf("the demo merchant owns %q, want %s", shop.ID, demoMerchantShop)
		}
		if shop.Status != "approved" {
			t.Fatalf("the demo shop is %q, so its owner sees the registration form", shop.Status)
		}
		// Open around the clock, so the demo works at any hour. The other
		// twelve seeded shops keep realistic hours on purpose.
		if !shop.IsOpenNow {
			t.Error("the demo shop is closed, so nothing can be ordered from it right now")
		}

		// And it has something to sell.
		var menu struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		if status := getJSON(t, base+"/v1/catalogue/"+shop.ID+"/menu", &menu); status != http.StatusOK ||
			len(menu.Items) == 0 {
			t.Fatalf("menu: status = %d, items = %d", status, len(menu.Items))
		}
	})

	t.Run("the partner is registered and off shift", func(t *testing.T) {
		rider := demoSignIn(t, base, demoPartnerPhone, "partner")
		var partner partnerPayload
		if status := requestAs(t, http.MethodGet, base+"/v1/partner", "",
			rider.AccessToken, &partner); status != http.StatusOK {
			t.Fatalf("partner: status = %d", status)
		}
		if partner.Name == "" || partner.AvailabilityLabel == "" {
			t.Fatalf("partner = %+v", partner)
		}
		// Off shift on purpose: going on shift is the first thing a rider
		// does and the step a demo most needs to show. Seeding them online
		// would hide it.
		if partner.Availability != "offline" {
			t.Errorf("the demo rider starts %q, want offline", partner.Availability)
		}
		if partner.AcceptancePercent != 100 {
			t.Errorf("a rider who has never been offered anything reads as %d%%",
				partner.AcceptancePercent)
		}
	})

	t.Run("the admin can reach the admin surface", func(t *testing.T) {
		admin := demoSignIn(t, base, demoAdminPhone, "admin")
		if status := getJSONAs(t, base+"/v1/config/effective?area=DHK-DHM",
			admin.AccessToken, nil); status != http.StatusOK {
			t.Errorf("config: status = %d", status)
		}
		if status := getJSONAs(t, base+"/v1/admin/merchants?status=approved",
			admin.AccessToken, nil); status != http.StatusOK {
			t.Errorf("approval queue: status = %d", status)
		}
	})
}

// TestADemoCanPayForAnOrderWithoutAGateway walks the prepaid path a demo
// visitor sees, which is the one the manual gateway would otherwise leave
// stuck at "pending" forever.
func TestADemoCanPayForAnOrderWithoutAGateway(t *testing.T) {
	base, stop := startDemoAPI(t)
	defer stop()

	customer := demoSignIn(t, base, demoCustomerPhone, "customer")

	var book addressBook
	if status := requestAs(t, http.MethodGet, base+"/v1/me/addresses", "",
		customer.AccessToken, &book); status != http.StatusOK || len(book.Addresses) == 0 {
		t.Fatalf("addresses: status = %d, count = %d", status, len(book.Addresses))
	}
	address := book.Addresses[0]

	itemID := demoOrderableItem(t, base, demoMerchantShop)
	if status := requestAs(t, http.MethodPost, base+"/v1/cart/items", fmt.Sprintf(
		`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":1}`, demoMerchantShop, itemID),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("fill cart: status = %d", status)
	}
	if status := requestAs(t, http.MethodPut, base+"/v1/cart/address", fmt.Sprintf(
		`{"address_id":%q,"lat":%f,"lng":%f}`, address.ID, address.Lat, address.Lng),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("bind address: status = %d", status)
	}

	var order orderResponse
	if status := requestAs(t, http.MethodPost, base+"/v1/orders", fmt.Sprintf(
		`{"address_id":%q,"payment_method":"online"}`, address.ID),
		customer.AccessToken, &order); status != http.StatusCreated {
		t.Fatalf("place order: status = %d", status)
	}
	if order.Status != "pending_payment" {
		t.Fatalf("a prepaid order started at %q", order.Status)
	}

	var checkout struct {
		PaymentID      string `json:"payment_id"`
		DemoCompletion bool   `json:"demo_completion"`
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/payments/checkout",
		fmt.Sprintf(`{"order_id":%q}`, order.ID), customer.AccessToken, &checkout); status != http.StatusOK {
		t.Fatalf("checkout: status = %d", status)
	}
	// The flag is the whole mechanism. The app shows its button because the
	// server said so, and calls the route below only then.
	if !checkout.DemoCompletion {
		t.Fatal("a demo checkout did not offer completion, so the app shows no way to pay")
	}

	// Somebody else's payment is not theirs to settle, even on a demo.
	stranger := demoSignIn(t, base, demoSecondCustomerPhone, "customer")
	if status := requestAs(t, http.MethodPost, base+"/v1/payments/manual/complete",
		fmt.Sprintf(`{"payment_id":%q,"succeeded":true}`, checkout.PaymentID),
		stranger.AccessToken, nil); status != http.StatusNotFound {
		t.Errorf("a stranger completing the payment: status = %d, want 404", status)
	}

	if status := requestAs(t, http.MethodPost, base+"/v1/payments/manual/complete",
		fmt.Sprintf(`{"payment_id":%q,"succeeded":true}`, checkout.PaymentID),
		customer.AccessToken, nil); status != http.StatusNoContent {
		t.Fatalf("complete: status = %d", status)
	}

	if status := requestAs(t, http.MethodGet, base+"/v1/orders/"+order.ID, "",
		customer.AccessToken, &order); status != http.StatusOK {
		t.Fatalf("re-read order: status = %d", status)
	}
	if order.Status != "placed" {
		t.Fatalf("after completing the payment the order is %q, want placed", order.Status)
	}
}

// demoOrderableItem finds an item on a shop's menu that can be added to a cart
// without choosing anything.
//
// Not simply the first item: the demo restaurant's first dish has a required
// size, and a test that picked it would be asserting about option validation
// rather than about payment.
func demoOrderableItem(t *testing.T, base, merchantID string) string {
	t.Helper()
	var menu struct {
		Items []struct {
			ID        string `json:"id"`
			Orderable bool   `json:"orderable"`
			Variants  []struct {
				Required bool `json:"required"`
			} `json:"variant_groups"`
			Addons []struct {
				Required bool `json:"required"`
			} `json:"addon_groups"`
		} `json:"items"`
	}
	if status := getJSON(t, base+"/v1/catalogue/"+merchantID+"/menu", &menu); status != http.StatusOK {
		t.Fatalf("menu: status = %d", status)
	}
	for _, item := range menu.Items {
		if !item.Orderable {
			continue
		}
		required := false
		for _, g := range item.Variants {
			required = required || g.Required
		}
		for _, g := range item.Addons {
			required = required || g.Required
		}
		if !required {
			return item.ID
		}
	}
	t.Fatalf("no item on %s can be ordered without choosing an option", merchantID)
	return ""
}
