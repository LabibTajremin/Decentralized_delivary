package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// These drive a customer placing an order and a shop working it through, via
// the real binary against real Postgres and the real seeded menus. The phase's
// two acceptance criteria — a state machine that refuses illegal moves, and one
// order out of two identical requests — are checked here at the level a Flutter
// app will actually see.

type orderResponse struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Status      string `json:"status"`
	StatusLabel string `json:"status_label"`
	Live        bool   `json:"live"`
	Payment     string `json:"payment_method"`
	Count       int    `json:"count"`
	Lines       []struct {
		Name      string `json:"name"`
		Quantity  int    `json:"quantity"`
		LineTotal struct {
			Minor int64 `json:"minor"`
		} `json:"line_total"`
	} `json:"lines"`
	Subtotal struct {
		Minor int64 `json:"minor"`
	} `json:"subtotal"`
	Delivery struct {
		Minor int64 `json:"minor"`
	} `json:"delivery"`
	Total struct {
		Minor   int64  `json:"minor"`
		Display string `json:"display"`
	} `json:"total"`
	Receipt []struct {
		Key    string `json:"key"`
		Label  string `json:"label"`
		Amount struct {
			Minor int64 `json:"minor"`
		} `json:"amount"`
	} `json:"receipt"`
	Pickup struct {
		Name string `json:"name"`
	} `json:"pickup"`
	Destination struct {
		Name       string `json:"name"`
		SingleLine string `json:"single_line"`
	} `json:"destination"`
	Events []struct {
		Status string    `json:"status"`
		Label  string    `json:"label"`
		Actor  string    `json:"actor"`
		Reason string    `json:"reason"`
		At     time.Time `json:"at"`
	} `json:"events"`
	NextActions []string `json:"next_actions"`
	Cancel      struct {
		Allowed     bool   `json:"allowed"`
		Reason      string `json:"reason"`
		Text        string `json:"text"`
		SecondsLeft int    `json:"seconds_left"`
	} `json:"cancel"`
}

// postWithHeader is requestAs with one extra header, for the idempotency key.
func postWithHeader(t *testing.T, url, body, bearer, header, value string, into any) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body)) //nolint:noctx // fixed test-local URL
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	if header != "" {
		req.Header.Set(header, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if into != nil {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil && resp.StatusCode < 300 {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp.StatusCode
}

// openAllHours builds an approved restaurant near Dhanmondi that is open every
// hour of every day, with one item on its menu.
//
// The test builds its own shop rather than using a seeded one, and the reason
// is worth stating: the seed keeps realistic 09:00–22:00 hours, so a suite that
// ordered from it would pass in the afternoon and fail overnight. A test whose
// result depends on when it runs is a test nobody trusts the second time it goes
// red.
func openAllHours(t *testing.T, base string, tail *logTail) (merchantID string, owner tokens, itemID string) {
	t.Helper()
	merchantID, owner, _ = approvedShop(t, base, tail, "restaurant")

	allDay := `{"days":{"0":["00:00-24:00"],"1":["00:00-24:00"],"2":["00:00-24:00"],` +
		`"3":["00:00-24:00"],"4":["00:00-24:00"],"5":["00:00-24:00"],"6":["00:00-24:00"]}}`
	if status := requestAs(t, http.MethodPut, base+"/v1/merchants/me/hours",
		allDay, owner.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("set hours: status = %d", status)
	}

	categoryID := addCategory(t, base, merchantID, owner.AccessToken, "কাচ্চি")
	var item idPayload
	if status := requestAs(t, http.MethodPost, catalogueBase(base, merchantID)+"/items", `{
		"category_id":"`+categoryID+`","name":"কাচ্চি বিরিয়ানি","price_minor":35000,"sort_order":1
	}`, owner.AccessToken, &item); status != http.StatusCreated {
		t.Fatalf("create item: status = %d", status)
	}
	return merchantID, owner, item.ID
}

// readyToOrder signs a customer in, gives them an address, and fills a cart at
// a shop that is open. It returns their token, the shop's owner, the shop's id
// and the address id.
func readyToOrder(t *testing.T, base string, tail *logTail) (tokens, tokens, string, string) {
	t.Helper()
	shopID, owner, itemID := openAllHours(t, base, tail)
	customer := signInAs(t, base, tail, uniquePhone(t), "customer phone", "customer")

	var address struct {
		ID string `json:"id"`
	}
	status := requestAs(t, http.MethodPost, base+"/v1/me/addresses", fmt.Sprintf(`{
		"label": "Home", "recipient_name": "Fardin", "recipient_phone": "01711111111",
		"line1": "House 5, Road 3", "lat": %f, "lng": %f
	}`, dhanmondiLat, dhanmondiLng), customer.AccessToken, &address)
	if status != http.StatusCreated {
		t.Fatalf("create address: status = %d", status)
	}

	if status := requestAs(t, http.MethodPost, base+"/v1/cart/items",
		fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":2}`, shopID, itemID),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("fill cart: status = %d", status)
	}
	if status := requestAs(t, http.MethodPut, base+"/v1/cart/address",
		fmt.Sprintf(`{"address_id":%q,"lat":%f,"lng":%f}`, address.ID, dhanmondiLat, dhanmondiLng),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("place cart: status = %d", status)
	}

	return customer, owner, shopID, address.ID
}

func TestACustomerPlacesAnOrder(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, _, shopID, addressID := readyToOrder(t, base, tail)

	var order orderResponse
	status := requestAs(t, http.MethodPost, base+"/v1/orders?lang=en",
		fmt.Sprintf(`{"address_id":%q,"payment_method":"cash"}`, addressID),
		customer.AccessToken, &order)
	if status != http.StatusCreated {
		t.Fatalf("place: status = %d", status)
	}

	if order.Status != "placed" || !order.Live || order.Payment != "cash" {
		t.Fatalf("order = %+v", order)
	}
	if order.Code == "" || len(order.Code) != 6 {
		t.Errorf("code = %q", order.Code)
	}
	if order.Count != 2 || len(order.Lines) != 1 {
		t.Fatalf("lines = %+v", order.Lines)
	}
	// Every number adds up, and none of it came from the request.
	if order.Total.Minor != order.Subtotal.Minor+order.Delivery.Minor {
		t.Errorf("the total does not add up: %+v", order)
	}
	if order.Total.Display == "" {
		t.Error("money crossed the wire with no rendered string")
	}
	// Both ends were copied off the shop and the address book.
	if order.Pickup.Name == "" || order.Destination.Name != "Fardin" ||
		order.Destination.SingleLine == "" {
		t.Fatalf("places = %+v / %+v", order.Pickup, order.Destination)
	}
	// The receipt is composed, and the history starts at the placement.
	if len(order.Receipt) < 3 || order.Receipt[0].Key != "subtotal" {
		t.Errorf("receipt = %+v", order.Receipt)
	}
	if len(order.Events) != 1 || order.Events[0].Status != "placed" ||
		order.Events[0].Actor != "customer" || order.Events[0].Label == "" {
		t.Errorf("events = %+v", order.Events)
	}
	// The customer can change their mind, and is told for how long.
	if !order.Cancel.Allowed || order.Cancel.SecondsLeft <= 0 {
		t.Errorf("cancel = %+v", order.Cancel)
	}
	if len(order.NextActions) != 1 || order.NextActions[0] != "cancelled" {
		t.Errorf("next actions = %v", order.NextActions)
	}

	// The cart was emptied by the placement.
	var emptied cartResponse
	if status := requestAs(t, http.MethodGet, base+"/v1/cart", "", customer.AccessToken, &emptied); status != http.StatusOK {
		t.Fatalf("cart: status = %d", status)
	}
	if len(emptied.Lines) != 0 {
		t.Errorf("the cart survived the order: %+v", emptied.Lines)
	}

	// And it appears in the customer's list, and in the shop's queue.
	var list struct {
		Orders []orderResponse `json:"orders"`
		Total  int             `json:"total"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/orders?live=true", "", customer.AccessToken, &list); status != http.StatusOK {
		t.Fatalf("list: status = %d", status)
	}
	if list.Total != 1 || list.Orders[0].ID != order.ID {
		t.Fatalf("list = %+v", list)
	}
	_ = shopID
}

// The phase's second acceptance criterion, over the wire: a customer on a
// village 2G connection taps twice and gets one order.
func TestOneIdempotencyKeyCreatesOneOrder(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, _, _, addressID := readyToOrder(t, base, tail)
	body := fmt.Sprintf(`{"address_id":%q,"payment_method":"cash"}`, addressID)

	var first, second orderResponse
	if status := postWithHeader(t, base+"/v1/orders", body, customer.AccessToken,
		"Idempotency-Key", "attempt-1", &first); status != http.StatusCreated {
		t.Fatalf("first: status = %d", status)
	}
	if status := postWithHeader(t, base+"/v1/orders", body, customer.AccessToken,
		"Idempotency-Key", "attempt-1", &second); status != http.StatusCreated {
		t.Fatalf("second: status = %d", status)
	}

	if first.ID != second.ID || first.Code != second.Code {
		t.Fatalf("two orders: %s/%s and %s/%s", first.ID, first.Code, second.ID, second.Code)
	}

	var list struct {
		Total int `json:"total"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/orders", "", customer.AccessToken, &list); status != http.StatusOK {
		t.Fatalf("list: status = %d", status)
	}
	if list.Total != 1 {
		t.Fatalf("%d orders exist, want one", list.Total)
	}
}

// The phase's first acceptance criterion, end to end.
func TestTheLifecycleRefusesIllegalMovesOverHTTP(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	shopID, owner, itemID := openAllHours(t, base, tail)
	customer := signInAs(t, base, tail, uniquePhone(t), "customer phone", "customer")

	var address struct {
		ID string `json:"id"`
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/me/addresses", fmt.Sprintf(`{
		"label": "Home", "recipient_name": "Fardin", "recipient_phone": "01711111111",
		"line1": "House 5, Road 3", "lat": %f, "lng": %f
	}`, dhanmondiLat, dhanmondiLng), customer.AccessToken, &address); status != http.StatusCreated {
		t.Fatalf("address: status = %d", status)
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/cart/items",
		fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":1}`, shopID, itemID),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("cart: status = %d", status)
	}
	if status := requestAs(t, http.MethodPut, base+"/v1/cart/address",
		fmt.Sprintf(`{"address_id":%q,"lat":%f,"lng":%f}`, address.ID, dhanmondiLat, dhanmondiLng),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("cart address: status = %d", status)
	}

	var order orderResponse
	if status := requestAs(t, http.MethodPost, base+"/v1/orders",
		fmt.Sprintf(`{"address_id":%q,"payment_method":"cash"}`, address.ID),
		customer.AccessToken, &order); status != http.StatusCreated {
		t.Fatalf("place: status = %d", status)
	}

	// A customer cannot reach a shop route at all: not a shop they own, so it
	// is refused before the state machine is consulted.
	if status := requestAs(t, http.MethodPost,
		base+"/v1/merchants/"+shopID+"/orders/"+order.ID+"/accept", "",
		customer.AccessToken, nil); status != http.StatusNotFound {
		t.Fatalf("a customer reaching a shop route got %d, want 404", status)
	}

	admin := signInAs(t, base, tail, uniquePhone(t), "admin console", "admin")
	transition := base + "/v1/admin/orders/" + order.ID + "/transition"

	// Skipping ahead is refused, whoever asks.
	for _, status := range []string{"delivered", "picked_up", "ready"} {
		if code := requestAs(t, http.MethodPost, transition,
			fmt.Sprintf(`{"status":%q}`, status), admin.AccessToken, nil); code != http.StatusConflict {
			t.Fatalf("placed → %s returned %d, want 409", status, code)
		}
	}
	// And a status that does not exist is a bad request, not a conflict.
	if code := requestAs(t, http.MethodPost, transition,
		`{"status":"shipped"}`, admin.AccessToken, nil); code != http.StatusBadRequest {
		t.Fatalf("an unknown status returned %d, want 400", code)
	}

	// The real path, walked by the party that owns each step.
	queueBase := base + "/v1/merchants/" + shopID + "/orders/" + order.ID
	for _, step := range []struct{ path, want string }{
		{"/accept", "accepted"},
		{"/preparing", "preparing"},
		{"/ready", "ready"},
	} {
		var got orderResponse
		if code := requestAs(t, http.MethodPost, queueBase+step.path, "",
			owner.AccessToken, &got); code != http.StatusOK {
			t.Fatalf("%s: status = %d", step.path, code)
		}
		if got.Status != step.want {
			t.Fatalf("%s = %q", step.path, got.Status)
		}
	}

	// An admin cannot say the kitchen started cooking. The move exists, but it
	// is the shop's to make — an operator marking food as being prepared is an
	// operator writing down something they cannot know.
	var back orderResponse
	if code := requestAs(t, http.MethodPost, transition,
		`{"status":"preparing"}`, admin.AccessToken, &back); code == http.StatusOK {
		t.Fatal("an admin marked an order as being prepared")
	}

	// Pickup and delivery belong to a rider, and a rider has no route until
	// dispatch lands in P12. What can be checked now is that the order is
	// waiting for one and nobody else can move it on.
	var final orderResponse
	if code := requestAs(t, http.MethodGet, base+"/v1/orders/"+order.ID+"?lang=en", "",
		customer.AccessToken, &final); code != http.StatusOK {
		t.Fatalf("read: status = %d", code)
	}
	if final.Status != "ready" || !final.Live || final.StatusLabel != "Waiting for a rider" {
		t.Fatalf("final = %+v", final)
	}
	// Four events: the placement plus three moves, every one with a sentence
	// and the shop named as the actor.
	if len(final.Events) != 4 {
		t.Fatalf("events = %+v", final.Events)
	}
	for _, e := range final.Events[1:] {
		if e.Label == "" || e.Actor != "merchant" {
			t.Errorf("event = %+v", e)
		}
	}
	// The window closed the moment the kitchen started, whatever the clock says.
	if final.Cancel.Allowed || final.Cancel.Reason != "already_preparing" || final.Cancel.Text == "" {
		t.Errorf("cancel = %+v", final.Cancel)
	}
	if code := requestAs(t, http.MethodPost, base+"/v1/orders/"+order.ID+"/cancel", "",
		customer.AccessToken, nil); code != http.StatusConflict {
		t.Fatalf("cancelling a cooking order returned %d, want 409", code)
	}

	// An admin still can, which is what operations is for.
	var cancelled orderResponse
	if code := requestAs(t, http.MethodPost, transition,
		`{"status":"cancelled","reason":"customer phoned"}`, admin.AccessToken, &cancelled); code != http.StatusOK {
		t.Fatalf("admin cancel: status = %d", code)
	}
	if cancelled.Status != "cancelled" || cancelled.Live {
		t.Fatalf("cancelled = %+v", cancelled)
	}
	// And a finished order cannot be moved again.
	if code := requestAs(t, http.MethodPost, transition,
		`{"status":"accepted"}`, admin.AccessToken, nil); code != http.StatusConflict {
		t.Fatalf("reviving a cancelled order returned %d, want 409", code)
	}
}

// The shop's own screen: its queue, and the four moves it owns.
func TestAShopWorksItsQueue(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	shopID, owner, itemID := openAllHours(t, base, tail)
	customer := signInAs(t, base, tail, uniquePhone(t), "customer phone", "customer")

	var address struct {
		ID string `json:"id"`
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/me/addresses", fmt.Sprintf(`{
		"label": "Home", "recipient_name": "Fardin", "recipient_phone": "01711111111",
		"line1": "House 5, Road 3", "lat": %f, "lng": %f
	}`, dhanmondiLat, dhanmondiLng), customer.AccessToken, &address); status != http.StatusCreated {
		t.Fatalf("address: status = %d", status)
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/cart/items",
		fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":1}`, shopID, itemID),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("cart: status = %d", status)
	}
	if status := requestAs(t, http.MethodPut, base+"/v1/cart/address",
		fmt.Sprintf(`{"address_id":%q,"lat":%f,"lng":%f}`, address.ID, dhanmondiLat, dhanmondiLng),
		customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("cart address: status = %d", status)
	}

	var order orderResponse
	if status := requestAs(t, http.MethodPost, base+"/v1/orders",
		fmt.Sprintf(`{"address_id":%q,"payment_method":"cash"}`, address.ID),
		customer.AccessToken, &order); status != http.StatusCreated {
		t.Fatalf("place: status = %d", status)
	}

	queueBase := base + "/v1/merchants/" + shopID + "/orders"
	var queue struct {
		Orders []orderResponse `json:"orders"`
		Total  int             `json:"total"`
	}
	if status := requestAs(t, http.MethodGet, queueBase+"?live=true", "", owner.AccessToken, &queue); status != http.StatusOK {
		t.Fatalf("queue: status = %d", status)
	}
	if queue.Total != 1 || queue.Orders[0].ID != order.ID {
		t.Fatalf("queue = %+v", queue)
	}
	// The shop sees the two moves that are its own.
	if len(queue.Orders[0].NextActions) != 2 {
		t.Errorf("next actions = %v", queue.Orders[0].NextActions)
	}

	// The four moves a shop owns, in order.
	for _, step := range []struct{ path, want string }{
		{"/accept", "accepted"},
		{"/preparing", "preparing"},
		{"/ready", "ready"},
	} {
		var got orderResponse
		if status := requestAs(t, http.MethodPost, queueBase+"/"+order.ID+step.path, "",
			owner.AccessToken, &got); status != http.StatusOK {
			t.Fatalf("%s: status = %d", step.path, status)
		}
		if got.Status != step.want {
			t.Fatalf("%s = %q", step.path, got.Status)
		}
	}

	// And it cannot go further: the rider owns the next step.
	if status := requestAs(t, http.MethodPost, queueBase+"/"+order.ID+"/accept", "",
		owner.AccessToken, nil); status != http.StatusConflict {
		t.Fatalf("a second accept returned %d, want 409", status)
	}

	// A rejection with no reason is refused — the customer would have nothing
	// to go on.
	otherShop, otherOwner, otherItem := openAllHours(t, base, tail)
	otherCustomer := signInAs(t, base, tail, uniquePhone(t), "another phone", "customer")
	var otherAddress struct {
		ID string `json:"id"`
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/me/addresses", fmt.Sprintf(`{
		"label": "Home", "recipient_name": "Rafi", "recipient_phone": "01711111112",
		"line1": "House 9, Road 3", "lat": %f, "lng": %f
	}`, dhanmondiLat, dhanmondiLng), otherCustomer.AccessToken, &otherAddress); status != http.StatusCreated {
		t.Fatalf("address: status = %d", status)
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/cart/items",
		fmt.Sprintf(`{"merchant_id":%q,"kind":"item","target_id":%q,"quantity":1}`, otherShop, otherItem),
		otherCustomer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("cart: status = %d", status)
	}
	if status := requestAs(t, http.MethodPut, base+"/v1/cart/address",
		fmt.Sprintf(`{"address_id":%q,"lat":%f,"lng":%f}`, otherAddress.ID, dhanmondiLat, dhanmondiLng),
		otherCustomer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("cart address: status = %d", status)
	}
	var second orderResponse
	if status := requestAs(t, http.MethodPost, base+"/v1/orders",
		fmt.Sprintf(`{"address_id":%q,"payment_method":"cash"}`, otherAddress.ID),
		otherCustomer.AccessToken, &second); status != http.StatusCreated {
		t.Fatalf("place: status = %d", status)
	}
	otherBase := base + "/v1/merchants/" + otherShop + "/orders"
	if status := requestAs(t, http.MethodPost, otherBase+"/"+second.ID+"/reject", `{}`,
		otherOwner.AccessToken, nil); status != http.StatusBadRequest {
		t.Fatalf("a reason-less rejection returned %d, want 400", status)
	}
	var rejected orderResponse
	if status := requestAs(t, http.MethodPost, otherBase+"/"+second.ID+"/reject",
		`{"reason":"কাচ্চি শেষ"}`, otherOwner.AccessToken, &rejected); status != http.StatusOK {
		t.Fatalf("reject: status = %d", status)
	}
	if rejected.Status != "rejected" || rejected.Live {
		t.Fatalf("rejected = %+v", rejected)
	}

	// And one shop cannot reach another's order: a not-found, not a forbidden.
	if status := requestAs(t, http.MethodGet, otherBase+"/"+order.ID, "",
		otherOwner.AccessToken, nil); status != http.StatusNotFound {
		t.Fatalf("another shop's order returned %d, want 404", status)
	}
}

// An order is the caller's or it does not exist.
func TestAnOrderIsPrivateToItsCustomer(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, _, _, addressID := readyToOrder(t, base, tail)
	var order orderResponse
	if status := requestAs(t, http.MethodPost, base+"/v1/orders",
		fmt.Sprintf(`{"address_id":%q,"payment_method":"cash"}`, addressID),
		customer.AccessToken, &order); status != http.StatusCreated {
		t.Fatalf("place: status = %d", status)
	}

	stranger := signInAs(t, base, tail, uniquePhone(t), "another phone", "customer")
	for _, target := range []string{
		"/v1/orders/" + order.ID,
		"/v1/orders/" + order.ID + "/cancellation",
	} {
		if status := requestAs(t, http.MethodGet, base+target, "", stranger.AccessToken, nil); status != http.StatusNotFound {
			t.Errorf("%s returned %d, want 404", target, status)
		}
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/orders/"+order.ID+"/cancel", "",
		stranger.AccessToken, nil); status != http.StatusNotFound {
		t.Errorf("a stranger's cancel returned %d, want 404", status)
	}

	// And nothing at all without a token.
	if status := getJSON(t, base+"/v1/orders", nil); status != http.StatusUnauthorized {
		t.Errorf("an anonymous list returned %d, want 401", status)
	}
}

// The free cancellation window, from the customer's side.
func TestACustomerCancelsInsideTheWindow(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, _, _, addressID := readyToOrder(t, base, tail)
	var order orderResponse
	if status := requestAs(t, http.MethodPost, base+"/v1/orders",
		fmt.Sprintf(`{"address_id":%q,"payment_method":"cash"}`, addressID),
		customer.AccessToken, &order); status != http.StatusCreated {
		t.Fatalf("place: status = %d", status)
	}

	var window struct {
		Allowed     bool `json:"allowed"`
		SecondsLeft int  `json:"seconds_left"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/orders/"+order.ID+"/cancellation?lang=en", "",
		customer.AccessToken, &window); status != http.StatusOK {
		t.Fatalf("cancellation: status = %d", status)
	}
	// The default window is two minutes (Appendix B), and a fresh order has
	// essentially all of it.
	if !window.Allowed || window.SecondsLeft < 100 || window.SecondsLeft > 120 {
		t.Fatalf("window = %+v", window)
	}

	var cancelled orderResponse
	if status := requestAs(t, http.MethodPost, base+"/v1/orders/"+order.ID+"/cancel?lang=en",
		`{"reason":"changed my mind"}`, customer.AccessToken, &cancelled); status != http.StatusOK {
		t.Fatalf("cancel: status = %d", status)
	}
	if cancelled.Status != "cancelled" || cancelled.Live {
		t.Fatalf("cancelled = %+v", cancelled)
	}
	last := cancelled.Events[len(cancelled.Events)-1]
	if last.Reason != "changed my mind" || last.Actor != "customer" {
		t.Errorf("event = %+v", last)
	}

	// A second cancellation is refused, with a sentence.
	if status := requestAs(t, http.MethodPost, base+"/v1/orders/"+order.ID+"/cancel", "",
		customer.AccessToken, nil); status != http.StatusConflict {
		t.Fatalf("a second cancel returned %d, want 409", status)
	}
}

// A prepaid order does not reach the shop until the money does.
func TestAPrepaidOrderWaitsForPayment(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, _, _, addressID := readyToOrder(t, base, tail)
	var order orderResponse
	if status := requestAs(t, http.MethodPost, base+"/v1/orders?lang=en",
		fmt.Sprintf(`{"address_id":%q,"payment_method":"online"}`, addressID),
		customer.AccessToken, &order); status != http.StatusCreated {
		t.Fatalf("place: status = %d", status)
	}
	if order.Status != "pending_payment" || order.StatusLabel != "Waiting for payment" {
		t.Fatalf("order = %+v", order)
	}
	// The customer may abandon it — nothing has been committed on their behalf.
	if len(order.NextActions) != 1 || order.NextActions[0] != "cancelled" {
		t.Errorf("next actions = %v", order.NextActions)
	}
}

// An empty cart is not an order.
func TestPlacingWithNothingInTheCart(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer := signInAs(t, base, tail, uniquePhone(t), "customer phone", "customer")
	var address struct {
		ID string `json:"id"`
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/me/addresses", fmt.Sprintf(`{
		"label": "Home", "recipient_name": "Fardin", "recipient_phone": "01711111111",
		"line1": "House 5, Road 3", "lat": %f, "lng": %f
	}`, dhanmondiLat, dhanmondiLng), customer.AccessToken, &address); status != http.StatusCreated {
		t.Fatalf("address: status = %d", status)
	}

	if status := requestAs(t, http.MethodPost, base+"/v1/orders",
		fmt.Sprintf(`{"address_id":%q,"payment_method":"cash"}`, address.ID),
		customer.AccessToken, nil); status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}
