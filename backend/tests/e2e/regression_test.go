package e2e

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"testing"
)

// regressionWebhookSecret is what the API under test is told to sign payment
// callbacks with. The value is the test's, not a default the binary supplies:
// signing a webhook here with a secret the operator set is the only way to
// prove the deployment contract rather than a convenience the code invented
// for itself.
const regressionWebhookSecret = "regression-webhook-secret-not-a-real-one"

// startWholeProductAPI runs the real binary with everything the whole-product
// walk touches: real Postgres on its own schema, real Redis, a payment webhook
// secret, and a tracking interval short enough that a test does not wait out
// the production default.
func startWholeProductAPI(t *testing.T) (string, *logTail, func()) {
	t.Helper()
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Fatal("REDIS_URL is not set; the regression walk needs a real Redis")
	}
	dbURL := dbtestSchemaURL(t)
	seedDemoData(t, dbURL)

	tail := newLogTail(t)
	base, stop := startAPIWithOutput(t, buildAPI(t), tail,
		"DATABASE_URL="+dbURL,
		"REDIS_URL="+redisURL,
		"PAYMENT_WEBHOOK_SECRET="+regressionWebhookSecret,
		"TRACKING_STREAM_INTERVAL=50ms",
	)
	return base, tail, stop
}

// signWebhook is what a gateway's own servers do before they call back: HMAC
// the exact bytes with the shared secret. Computed here from first principles
// rather than borrowed from the gateway package, so a change to how the
// server verifies a callback fails this test instead of following it.
func signWebhook(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// TestTheWholeProductEndToEnd is P20's regression walk: one test that goes
// from an empty database to a delivered, paid, reviewed order with a support
// ticket closed on it, through the public HTTP surface only, with every step
// taken by the party who really owns it.
//
// Every module has its own E2E suite, and each proves its own slice deeply.
// What none of them proves is that the slices still line up: that the shop a
// merchant registered is the one discovery returns, that the total the cart
// quoted is the amount the gateway is asked for, that the capture a gateway
// confirms is what lets the shop start cooking, that the rider dispatch
// offered is the one tracking shows and reviews can rate. The seams are
// where this product breaks, so this is the test that walks all of them in
// one process, in order, exactly once.
//
// Three things it deliberately does differently from the per-module suites:
//
//   - **The order is prepaid, not cash.** Every other E2E chain places
//     `cash`, because cash is the shorter path. That left the online branch —
//     checkout, a signed callback, `pending_payment` → `placed` — proven only
//     against fakes. It is the branch that handles money.
//   - **The shop is built, approved and stocked here.** Nothing is inherited
//     from the seed, so the walk starts from an empty database and the
//     merchant lifecycle is part of what is regressed.
//   - **The rider goes on shift before the food is ready.** ALG-04's first
//     offer round runs once, at `ready`, and only asks partners already on
//     shift. A rider who signs on afterwards waits for a sweep, and a test
//     that ordered those two steps the other way round would be testing the
//     sweep by accident.
func TestTheWholeProductEndToEnd(t *testing.T) {
	base, tail, stop := startWholeProductAPI(t)
	defer stop()

	// ---- the shop opens -------------------------------------------------
	//
	// An owner registers, files their papers and submits; an admin approves.
	// Until that approval the shop is invisible, which merchant's own suite
	// proves; here it is simply the door the rest of the walk comes through.
	shopID, owner, admin := approvedShop(t, base, tail, "restaurant")

	allDay := `{"days":{"0":["00:00-24:00"],"1":["00:00-24:00"],"2":["00:00-24:00"],` +
		`"3":["00:00-24:00"],"4":["00:00-24:00"],"5":["00:00-24:00"],"6":["00:00-24:00"]}}`
	if status := requestAs(t, http.MethodPut, base+"/v1/merchants/me/hours",
		allDay, owner.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("set hours: status = %d", status)
	}

	categoryID := addCategory(t, base, shopID, owner.AccessToken, "কাচ্চি")
	var item idPayload
	if status := requestAs(t, http.MethodPost, catalogueBase(base, shopID)+"/items", `{
		"category_id":"`+categoryID+`","name":"কাচ্চি বিরিয়ানি","price_minor":35000,"sort_order":1
	}`, owner.AccessToken, &item); status != http.StatusCreated {
		t.Fatalf("create item: status = %d", status)
	}

	// ---- a customer finds it --------------------------------------------
	customer := signInAs(t, base, tail, uniquePhone(t), "customer phone", "customer")
	if status := requestAs(t, http.MethodPost, base+"/v1/me/device",
		`{"platform":"android","token":"regression-device-token"}`,
		customer.AccessToken, nil); status != http.StatusNoContent {
		t.Fatalf("register device: status = %d", status)
	}

	var address addressPayload
	if status := requestAs(t, http.MethodPost, base+"/v1/me/addresses", dhanmondiAddress,
		customer.AccessToken, &address); status != http.StatusCreated {
		t.Fatalf("create address: status = %d", status)
	}

	// Discovery, from the customer's own coordinates, has to return the shop
	// the merchant half of this test just built — with its distance and its
	// delivery fee already composed (2.9). This is the seam between merchant,
	// geo, discovery and pricing, and nothing else in the suite crosses all
	// four in one call.
	found := discover(t, base, dhanmondiLat, dhanmondiLng, "lang=en")
	var mine *discoveryMerchant
	for i := range found.Merchants {
		if found.Merchants[i].ID == shopID {
			mine = &found.Merchants[i]
			break
		}
	}
	if mine == nil {
		t.Fatalf("the shop just approved is not in discovery: %+v", found.Merchants)
	}
	if mine.Distance == "" || mine.Delivery.Display == "" {
		t.Errorf("the client would have to compose these itself: %+v", *mine)
	}

	// The menu the customer reads is the public projection of what the owner
	// typed — one call, no shelf counts, no visibility switches.
	var menu struct {
		MerchantID string `json:"merchant_id"`
		Items      []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Orderable bool   `json:"orderable"`
			Price     struct {
				Minor   int64  `json:"minor"`
				Display string `json:"display"`
			} `json:"price"`
		} `json:"items"`
	}
	if status := getJSON(t, base+"/v1/catalogue/"+shopID+"/menu", &menu); status != http.StatusOK {
		t.Fatalf("read menu: status = %d", status)
	}
	if len(menu.Items) != 1 || menu.Items[0].ID != item.ID || !menu.Items[0].Orderable {
		t.Fatalf("menu = %+v", menu)
	}
	if menu.Items[0].Price.Display == "" {
		t.Error("an item's price arrives without a display string")
	}

	// ---- the cart ---------------------------------------------------------
	if status := requestAs(t, http.MethodPost, base+"/v1/cart/items",
		fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":2}`, shopID, item.ID),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("fill cart: status = %d", status)
	}

	// Binding the address is what makes the bill real: no destination, no
	// distance; no distance, no delivery fee.
	var cart cartResponse
	if status := requestAs(t, http.MethodPut, base+"/v1/cart/address",
		fmt.Sprintf(`{"address_id":%q,"lat":%f,"lng":%f}`, address.ID, dhanmondiLat, dhanmondiLng),
		customer.AccessToken, &cart); status != http.StatusOK {
		t.Fatalf("bind address: status = %d", status)
	}
	if !cart.Orderable {
		t.Fatalf("cart is not orderable: %+v", cart)
	}
	if cart.Subtotal.Minor != 2*menu.Items[0].Price.Minor {
		t.Errorf("subtotal = %d, want twice the menu price %d",
			cart.Subtotal.Minor, menu.Items[0].Price.Minor)
	}
	if cart.Pricing == nil {
		t.Fatal("an address-bound cart arrives without a bill")
	}
	if cart.Subtotal.Display == "" || len(cart.Pricing.Rows) == 0 {
		t.Error("the receipt arrives unformatted")
	}

	// ---- the rider signs on, before the food is ready ---------------------
	rider, partner := onShift(t, base, tail, "Karim", dhanmondiLat, dhanmondiLng)

	// ---- the order, paid online -------------------------------------------
	var order orderResponse
	if status := requestAs(t, http.MethodPost, base+"/v1/orders",
		fmt.Sprintf(`{"address_id":%q,"payment_method":"online"}`, address.ID),
		customer.AccessToken, &order); status != http.StatusCreated {
		t.Fatalf("place order: status = %d", status)
	}
	if order.Status != "pending_payment" {
		t.Fatalf("a prepaid order starts at %q, not pending_payment", order.Status)
	}
	if order.Total.Minor != cart.Pricing.Total.Minor {
		t.Errorf("order total %d does not match the cart's %d",
			order.Total.Minor, cart.Pricing.Total.Minor)
	}

	// A shop must not be able to start cooking an order nobody has paid for.
	if status := requestAs(t, http.MethodPost,
		base+"/v1/merchants/"+shopID+"/orders/"+order.ID+"/accept", "",
		owner.AccessToken, nil); status != http.StatusConflict {
		t.Fatalf("accepting an unpaid order: status = %d, want 409", status)
	}

	var checkout struct {
		PaymentID string `json:"payment_id"`
		OrderID   string `json:"order_id"`
		Status    string `json:"status"`
		Amount    struct {
			Minor int64 `json:"minor"`
		} `json:"amount"`
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/payments/checkout",
		fmt.Sprintf(`{"order_id":%q}`, order.ID), customer.AccessToken, &checkout); status != http.StatusOK {
		t.Fatalf("checkout: status = %d", status)
	}
	if checkout.Status != "pending" || checkout.Amount.Minor != order.Total.Minor {
		t.Fatalf("checkout = %+v, want pending for %d", checkout, order.Total.Minor)
	}

	// An admin can read any order's payment, which is the surface a support
	// agent answering "did this go through?" actually uses.
	var seenByAdmin struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/admin/payments/"+order.ID, "",
		admin.AccessToken, &seenByAdmin); status != http.StatusOK {
		t.Fatalf("read payment as admin: status = %d", status)
	}
	if seenByAdmin.ID != checkout.PaymentID || seenByAdmin.Status != "pending" {
		t.Fatalf("admin sees %+v, want the pending payment %s", seenByAdmin, checkout.PaymentID)
	}

	// Now the gateway calls back. This test is standing in for it, so it calls
	// back about the reference it was handed at checkout and signs the bytes
	// the way a provider's own servers would.
	callback := fmt.Sprintf(
		`{"reference":%q,"gateway_ref":"GW-1","succeeded":true,"amount_minor":%d}`,
		checkout.PaymentID, order.Total.Minor)
	if status := postWithHeader(t, base+"/v1/payments/manual/webhook", callback, "",
		"X-Webhook-Signature", signWebhook(regressionWebhookSecret, callback),
		nil); status != http.StatusNoContent {
		t.Fatalf("webhook: status = %d", status)
	}

	// A gateway guarantees at-least-once delivery, so the same callback
	// arrives again. Nothing may be written twice.
	if status := postWithHeader(t, base+"/v1/payments/manual/webhook", callback, "",
		"X-Webhook-Signature", signWebhook(regressionWebhookSecret, callback),
		nil); status != http.StatusNoContent {
		t.Fatalf("replayed webhook: status = %d", status)
	}

	var paid struct {
		Status      string `json:"status"`
		StatusLabel string `json:"status_label"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/payments/"+order.ID, "",
		customer.AccessToken, &paid); status != http.StatusOK {
		t.Fatalf("read own payment: status = %d", status)
	}
	if paid.Status != "captured" || paid.StatusLabel == "" {
		t.Fatalf("payment = %+v, want captured with a label", paid)
	}

	// Capture is what releases the order to the shop.
	if status := requestAs(t, http.MethodGet, base+"/v1/orders/"+order.ID, "",
		customer.AccessToken, &order); status != http.StatusOK {
		t.Fatalf("re-read order: status = %d", status)
	}
	if order.Status != "placed" {
		t.Fatalf("a captured payment left the order at %q, not placed", order.Status)
	}

	// ---- the shop cooks it ------------------------------------------------
	walkShopToReady(t, base, owner, shopID, order.ID)

	// ---- the rider carries it ---------------------------------------------
	//
	// The offer is already on the rider's own list, because they were on
	// shift when the shop said ready.
	var offered jobListPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/jobs?live=true", "",
		rider.AccessToken, &offered); status != http.StatusOK || offered.Total != 1 {
		t.Fatalf("rider's jobs: status = %d, jobs = %+v", status, offered)
	}
	job := offered.Jobs[0]
	if job.OrderID != order.ID || job.Status != "offered" {
		t.Fatalf("job = %+v, want an offer for %s", job, order.ID)
	}
	if job.Destination.SingleLine == "" || job.Pickup.SingleLine == "" {
		t.Error("a rider's job arrives without composed addresses")
	}

	for _, step := range []string{"/accept", "/collect"} {
		if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+job.ID+step, "",
			rider.AccessToken, nil); status != http.StatusOK {
			t.Fatalf("%s: status = %d", step, status)
		}
	}

	// While it is on the road the customer's stream shows the rider who is
	// actually carrying it — the one dispatch chose, not a name the client
	// guessed.
	frames := streamFrames(t, base, order.ID, customer.AccessToken, 1)
	if len(frames) != 1 || frames[0].Partner == nil {
		t.Fatalf("frames = %+v, want one with a rider on it", frames)
	}
	if frames[0].Partner.ID != partner.ID || !frames[0].Live {
		t.Errorf("frame = %+v, want live and carried by %s", frames[0], partner.ID)
	}

	if status := requestAs(t, http.MethodPost, base+"/v1/partner/jobs/"+job.ID+"/deliver", "",
		rider.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("deliver: status = %d", status)
	}

	// A delivered order's stream sends one last frame and closes itself.
	final := streamFrames(t, base, order.ID, customer.AccessToken, 4)
	if len(final) == 0 {
		t.Fatal("a delivered order's stream sent nothing")
	}
	last := final[len(final)-1]
	if last.Status != "delivered" || last.Live {
		t.Errorf("last frame = %+v, want a terminal delivered frame", last)
	}

	// Nothing was collected in cash, so the rider is carrying none. An online
	// order that landed in the COD ledger would be the platform asking a
	// rider to remit money they never took.
	var ledger struct {
		PartnerID   string `json:"partner_id"`
		Outstanding struct {
			Minor int64 `json:"minor"`
		} `json:"outstanding"`
		Held []struct {
			OrderID string `json:"order_id"`
		} `json:"held"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/cod", "",
		rider.AccessToken, &ledger); status != http.StatusOK {
		t.Fatalf("read cod ledger: status = %d", status)
	}
	if ledger.PartnerID != partner.ID || ledger.Outstanding.Minor != 0 || len(ledger.Held) != 0 {
		t.Errorf("a prepaid delivery put cash on the rider: %+v", ledger)
	}

	// ---- what the customer is left with -----------------------------------
	if status := requestAs(t, http.MethodGet, base+"/v1/orders/"+order.ID, "",
		customer.AccessToken, &order); status != http.StatusOK {
		t.Fatalf("read finished order: status = %d", status)
	}
	if order.Status != "delivered" || order.Live {
		t.Fatalf("order = %q live=%v, want a finished one", order.Status, order.Live)
	}
	if order.StatusLabel == "" || len(order.Events) == 0 {
		t.Error("the order's history arrives without composed labels")
	}

	// Every status change along the way told the customer about it.
	var notifications []struct {
		Title  string `json:"title"`
		Body   string `json:"body"`
		Status string `json:"status"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/me/notifications", "",
		customer.AccessToken, &notifications); status != http.StatusOK {
		t.Fatalf("read notifications: status = %d", status)
	}
	if len(notifications) == 0 {
		t.Fatal("a whole delivery produced no notifications")
	}
	for _, n := range notifications {
		if n.Title == "" || n.Body == "" {
			t.Errorf("notification arrives empty: %+v", n)
		}
	}

	// ---- reviews, and a complaint -----------------------------------------
	for _, subject := range []struct{ kind, id string }{
		{"merchant", shopID},
		{"item", item.ID},
		{"partner", partner.ID},
	} {
		var review reviewResponse
		if status := requestAs(t, http.MethodPost, base+"/v1/reviews", fmt.Sprintf(`{
			"order_id":%q,"subject":%q,"subject_id":%q,"rating":5,"comment":"ঠিক সময়ে এসেছে"
		}`, order.ID, subject.kind, subject.id), customer.AccessToken, &review); status != http.StatusOK {
			t.Fatalf("review the %s: status = %d", subject.kind, status)
		}
		var rating ratingResponse
		if status := requestAs(t, http.MethodGet,
			base+"/v1/ratings?subject="+subject.kind+"&subject_id="+subject.id, "",
			customer.AccessToken, &rating); status != http.StatusOK {
			t.Fatalf("read the %s's rating: status = %d", subject.kind, status)
		}
		if rating.Average != 5 || rating.Count != 1 {
			t.Errorf("%s rating = %+v", subject.kind, rating)
		}
	}

	var ticket struct {
		ID      string `json:"id"`
		OrderID string `json:"order_id"`
		Status  string `json:"status"`
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/support/tickets",
		fmt.Sprintf(`{"order_id":%q,"subject":"একটি আইটেম কম ছিল"}`, order.ID),
		customer.AccessToken, &ticket); status != http.StatusOK {
		t.Fatalf("raise a ticket: status = %d", status)
	}
	if ticket.OrderID != order.ID || ticket.Status != "open" {
		t.Fatalf("ticket = %+v", ticket)
	}

	var queue struct {
		Tickets []struct {
			ID string `json:"id"`
		} `json:"tickets"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/admin/support/tickets", "",
		admin.AccessToken, &queue); status != http.StatusOK {
		t.Fatalf("read the support queue: status = %d", status)
	}
	if !ticketInQueue(queue.Tickets, ticket.ID) {
		t.Fatalf("the ticket just raised is not in the admin queue: %+v", queue.Tickets)
	}

	if status := requestAs(t, http.MethodPost, base+"/v1/admin/support/tickets/resolve",
		fmt.Sprintf(`{"ticket_id":%q,"resolution":"refunded","note":"একটি আইটেমের দাম ফেরত"}`, ticket.ID),
		admin.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("resolve the ticket: status = %d", status)
	}

	var closed struct {
		Tickets []struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			Resolution string `json:"resolution"`
		} `json:"tickets"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/me/support/tickets", "",
		customer.AccessToken, &closed); status != http.StatusOK {
		t.Fatalf("read own tickets: status = %d", status)
	}
	if len(closed.Tickets) != 1 || closed.Tickets[0].Status != "resolved" ||
		closed.Tickets[0].Resolution != "refunded" {
		t.Fatalf("the customer's own ticket reads %+v", closed.Tickets)
	}
}

// ticketInQueue reports whether the admin queue carries a ticket id.
func ticketInQueue(tickets []struct {
	ID string `json:"id"`
}, want string) bool {
	for _, ticket := range tickets {
		if ticket.ID == want {
			return true
		}
	}
	return false
}
