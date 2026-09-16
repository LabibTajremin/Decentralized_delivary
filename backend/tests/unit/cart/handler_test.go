package cart

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/contract"
	carthttp "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/transport/http"
)

// serve mounts the handler over a rig, with a guard that passes everything and
// a principal reader that answers with the given user id.
func serve(r *rig, userID string) *http.ServeMux {
	mux := http.NewServeMux()
	carthttp.NewHandler(
		r.carts,
		func(next http.Handler) http.Handler { return next },
		func(*http.Request) (string, bool) { return userID, userID != "" },
	).Register(mux)
	return mux
}

func call(t *testing.T, mux *http.ServeMux, method, target, body string, into any) int {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if into != nil && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
			t.Fatalf("decode %s %s: %v (%s)", method, target, err, rec.Body)
		}
	}
	return rec.Code
}

// A customer with no cart gets an empty body rather than a 404: an empty cart
// and no cart are the same screen.
func TestGetCartWithNothingInIt(t *testing.T) {
	mux := serve(withBurger(), "USR-1")
	var body struct {
		Lines   []any  `json:"lines"`
		Blocker string `json:"blocker"`
	}
	if status := call(t, mux, http.MethodGet, "/v1/cart", "", &body); status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if len(body.Lines) != 0 || body.Blocker != "empty" {
		t.Fatalf("body = %+v", body)
	}
}

// And a customer who has one gets it back, revalidated on the read: the GET is
// the screen the app opens on, so it is the one that has to be right.
func TestGetCartWithSomethingInIt(t *testing.T) {
	r := withBurger()
	mux := serve(r, "USR-1")
	if status := call(t, mux, http.MethodPost, "/v1/cart/items",
		`{"merchant_id":"MER-1","kind":"item","target_id":"ITM-burger","quantity":2}`, nil); status != http.StatusOK {
		t.Fatalf("add: status = %d", status)
	}

	var body struct {
		MerchantName string `json:"merchant_name"`
		Count        int    `json:"count"`
		Lines        []struct {
			Quantity  int `json:"quantity"`
			LineTotal struct {
				Display string `json:"display"`
			} `json:"line_total"`
		} `json:"lines"`
	}
	if status := call(t, mux, http.MethodGet, "/v1/cart?lang=en", "", &body); status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if body.MerchantName != "Star Kabab" || body.Count != 2 || len(body.Lines) != 1 {
		t.Fatalf("body = %+v", body)
	}
	if body.Lines[0].LineTotal.Display == "" {
		t.Error("the line total crossed the wire with no rendered string")
	}
}

func TestTheWholeCartOverHTTP(t *testing.T) {
	r := withBurger()
	mux := serve(r, "USR-1")

	type lineBody struct {
		ID        string `json:"id"`
		Quantity  int    `json:"quantity"`
		Orderable bool   `json:"orderable"`
		LineTotal struct {
			Minor   int64  `json:"minor"`
			Display string `json:"display"`
		} `json:"line_total"`
	}
	var body struct {
		MerchantName string     `json:"merchant_name"`
		Lines        []lineBody `json:"lines"`
		Count        int        `json:"count"`
		Subtotal     struct {
			Minor int64 `json:"minor"`
		} `json:"subtotal"`
		Orderable   bool   `json:"orderable"`
		Blocker     string `json:"blocker"`
		BlockerText string `json:"blocker_text"`
	}

	status := call(t, mux, http.MethodPost, "/v1/cart/items?lang=en",
		`{"merchant_id":"MER-1","kind":"item","target_id":"ITM-burger","quantity":2}`, &body)
	if status != http.StatusOK {
		t.Fatalf("add: status = %d", status)
	}
	if body.MerchantName != "Star Kabab" || body.Count != 2 || body.Subtotal.Minor != 50000 {
		t.Fatalf("body = %+v", body)
	}
	if body.Lines[0].LineTotal.Display == "" {
		t.Error("the line total has no rendered string")
	}
	lineID := body.Lines[0].ID

	// A fresh target for each response: decoding into the same struct twice
	// leaves the previous value in any field the new body omits, which is
	// exactly the blocker we are asserting is gone.
	var placed struct {
		Orderable bool   `json:"orderable"`
		Blocker   string `json:"blocker"`
	}
	if status := call(t, mux, http.MethodPut, "/v1/cart/address",
		`{"address_id":"ADR-1","lat":23.7465,"lng":90.3760}`, &placed); status != http.StatusOK {
		t.Fatalf("address: status = %d", status)
	}
	if !placed.Orderable || placed.Blocker != "" {
		t.Fatalf("after an address: %+v", placed)
	}

	if status := call(t, mux, http.MethodPut, "/v1/cart/lines/"+lineID,
		`{"quantity":5}`, &body); status != http.StatusOK {
		t.Fatalf("quantity: status = %d", status)
	}
	if body.Count != 5 {
		t.Errorf("count = %d, want 5", body.Count)
	}

	if status := call(t, mux, http.MethodDelete, "/v1/cart/lines/"+lineID, "", &body); status != http.StatusOK {
		t.Fatalf("remove: status = %d", status)
	}
	if len(body.Lines) != 0 {
		t.Errorf("the line survived removal: %+v", body.Lines)
	}

	if status := call(t, mux, http.MethodDelete, "/v1/cart", "", nil); status != http.StatusNoContent {
		t.Fatalf("clear: status = %d", status)
	}
}

func TestReplaceOverHTTP(t *testing.T) {
	r := withBurger()
	mux := serve(r, "USR-1")
	if status := call(t, mux, http.MethodPost, "/v1/cart/items",
		`{"merchant_id":"MER-1","kind":"item","target_id":"ITM-burger","quantity":1}`, nil); status != http.StatusOK {
		t.Fatalf("add: status = %d", status)
	}

	// A second shop is a conflict, with a sentence.
	var failure struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if status := call(t, mux, http.MethodPost, "/v1/cart/items",
		`{"merchant_id":"MER-2","kind":"item","target_id":"ITM-burger","quantity":1}`, &failure); status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if failure.Error.Code != "different_merchant" || failure.Error.Message == "" {
		t.Errorf("error = %+v", failure.Error)
	}

	r.catalogue.items["ITM-rice"] = orderableItem("ITM-rice", "Miniket Rice", 7000)
	r.merchant.shop.ID = "MER-2"
	var replaced struct {
		MerchantID string `json:"merchant_id"`
		Lines      []struct {
			TargetID string `json:"target_id"`
		} `json:"lines"`
	}
	if status := call(t, mux, http.MethodPost, "/v1/cart/replace",
		`{"merchant_id":"MER-2","kind":"item","target_id":"ITM-rice","quantity":1}`, &replaced); status != http.StatusOK {
		t.Fatalf("replace: status = %d", status)
	}
	if replaced.MerchantID != "MER-2" || replaced.Lines[0].TargetID != "ITM-rice" {
		t.Fatalf("replaced = %+v", replaced)
	}
}

func TestOptionsAreSentAsIdsOnly(t *testing.T) {
	r := newRig()
	r.catalogue.items["ITM-pizza"] = pizza()
	mux := serve(r, "USR-1")

	var body struct {
		Lines []struct {
			UnitPrice struct {
				Minor int64 `json:"minor"`
			} `json:"unit_price"`
			Options []struct {
				OptionID string `json:"option_id"`
				Price    struct {
					Minor int64 `json:"minor"`
				} `json:"price"`
			} `json:"options"`
		} `json:"lines"`
	}
	status := call(t, mux, http.MethodPost, "/v1/cart/items", `{
		"merchant_id":"MER-1","kind":"item","target_id":"ITM-pizza","quantity":1,
		"choices":[{"group_id":"GRP-size","option_id":"OPT-large"}]
	}`, &body)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	// The price came from the catalogue, not the request, which sent none.
	if body.Lines[0].UnitPrice.Minor != 65000 || body.Lines[0].Options[0].Price.Minor != 15000 {
		t.Fatalf("line = %+v", body.Lines[0])
	}
}

func TestEveryRouteRefusesAnAnonymousCaller(t *testing.T) {
	mux := serve(withBurger(), "")
	for _, pattern := range carthttp.Patterns() {
		method, path, _ := strings.Cut(pattern, " ")
		target := strings.ReplaceAll(path, "{lineId}", "CLN-1")
		if status := call(t, mux, method, target, "{}", nil); status != http.StatusUnauthorized {
			t.Errorf("%s returned %d, want 401", pattern, status)
		}
	}
}

func TestMalformedBodiesAreRefused(t *testing.T) {
	r := withBurger()
	mux := serve(r, "USR-1")
	if status := call(t, mux, http.MethodPost, "/v1/cart/items",
		`{"merchant_id":"MER-1","kind":"item","target_id":"ITM-burger","quantity":1}`, nil); status != http.StatusOK {
		t.Fatalf("setup: status = %d", status)
	}

	cases := []struct {
		name, method, target, body string
	}{
		{"add", http.MethodPost, "/v1/cart/items", `{"quantity":`},
		{"replace", http.MethodPost, "/v1/cart/replace", `not json`},
		{"quantity", http.MethodPut, "/v1/cart/lines/CLN-1", `{`},
		{"address", http.MethodPut, "/v1/cart/address", `[]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status := call(t, mux, tc.method, tc.target, tc.body, nil); status != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", status)
			}
		})
	}
}

func TestFailuresReachTheClient(t *testing.T) {
	r := withBurger()
	r.repo.readErr = errBoom
	mux := serve(r, "USR-1")

	cases := []struct {
		name, method, target, body string
	}{
		{"get", http.MethodGet, "/v1/cart", ""},
		{"add", http.MethodPost, "/v1/cart/items", `{"merchant_id":"MER-1","kind":"item","target_id":"ITM-burger","quantity":1}`},
		{"replace", http.MethodPost, "/v1/cart/replace", `{"merchant_id":"MER-1","kind":"item","target_id":"ITM-burger","quantity":1}`},
		{"quantity", http.MethodPut, "/v1/cart/lines/CLN-1", `{"quantity":2}`},
		{"remove", http.MethodDelete, "/v1/cart/lines/CLN-1", ""},
		{"address", http.MethodPut, "/v1/cart/address", `{"address_id":"ADR-1","lat":23.7,"lng":90.4}`},
		{"clear", http.MethodDelete, "/v1/cart", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status := call(t, mux, tc.method, tc.target, tc.body, nil); status != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", status)
			}
		})
	}
}

func TestNewHandlerRefusesToRunWithoutAGuard(t *testing.T) {
	// A nil guard would let any caller read any cart. Refused at wiring time
	// rather than becoming a hole nobody notices.
	defer func() {
		if recover() == nil {
			t.Fatal("a handler with no guard was built")
		}
	}()
	carthttp.NewHandler(nil, nil, nil)
}

func TestPatternsMatchWhatIsMounted(t *testing.T) {
	patterns := carthttp.Patterns()
	if len(patterns) != 7 {
		t.Fatalf("Patterns() = %v", patterns)
	}
	mux := serve(withBurger(), "USR-1")
	for _, pattern := range patterns {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Fatalf("pattern %q is not %q", pattern, "METHOD /path")
		}
		target := strings.ReplaceAll(path, "{lineId}", "CLN-1")
		// Asked of the mux rather than read off a status code: a cart route
		// answers 404 for a line that is not there, which is a business answer
		// and not a routing one.
		_, matched := mux.Handler(httptest.NewRequest(method, target, bytes.NewBufferString("{}")))
		if matched != pattern {
			t.Errorf("%s is in Patterns() but the mux resolved %q", pattern, matched)
		}
	}
}

// The contract order will call. It gets the revalidated cart at today's prices,
// because an order frozen from a stale snapshot is an order the shop disputes.
func TestTheContractOrderWillCall(t *testing.T) {
	r := withBurger()
	ctx := context.Background()

	var api contract.CartContract = r.service
	if _, found, err := api.Current(ctx, "USR-1"); err != nil || found {
		t.Fatalf("a customer with no cart: found = %v, err = %v", found, err)
	}

	view := add(t, r, "ITM-burger", 2)
	if _, err := r.carts.SetAddress(ctx, "USR-1", "ADR-1", 23.7, 90.4, ""); err != nil {
		t.Fatalf("SetAddress: %v", err)
	}

	r.catalogue.items["ITM-burger"] = orderableItem("ITM-burger", "Beef Burger", 28000)
	got, found, err := api.Current(ctx, "USR-1")
	if err != nil || !found {
		t.Fatalf("Current: found = %v, err = %v", found, err)
	}
	if got.UserID != "USR-1" || got.MerchantID != "MER-1" || got.AddressID != "ADR-1" {
		t.Fatalf("cart = %+v", got)
	}
	if got.Lat != 23.7 || got.Lng != 90.4 {
		t.Errorf("the delivery point did not cross the contract: %+v", got)
	}
	// Today's price, not the one in the snapshot.
	if got.Lines[0].UnitPrice.Minor != 28000 || got.Subtotal.Minor != 56000 {
		t.Fatalf("the contract carried stale prices: %+v", got)
	}
	if got.Lines[0].ID != view.Lines[0].ID || got.Lines[0].LineTotal.Minor != 56000 {
		t.Errorf("line = %+v", got.Lines[0])
	}
	if !got.Orderable || got.Blocker != "" {
		t.Errorf("orderable = %v, blocker = %q", got.Orderable, got.Blocker)
	}

	if err := api.Clear(ctx, "USR-1"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, found, _ := api.Current(ctx, "USR-1"); found {
		t.Error("Clear left the cart behind")
	}
}

func TestTheContractReportsAFailure(t *testing.T) {
	r := withBurger()
	r.repo.readErr = errBoom
	if _, _, err := r.service.Current(context.Background(), "USR-1"); err == nil {
		t.Fatal("a broken store was reported as no cart")
	}
}

// The contract carries a line's options too, so order can freeze what was
// actually chosen rather than just the item.
func TestTheContractCarriesOptions(t *testing.T) {
	r := newRig()
	r.catalogue.items["ITM-pizza"] = pizza()
	if _, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-1", Kind: "item", TargetID: "ITM-pizza", Quantity: 1,
		Choices: []application.Choice{{GroupID: "GRP-size", OptionID: "OPT-large"}},
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, _, err := r.service.Current(context.Background(), "USR-1")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if len(got.Lines[0].Options) != 1 || got.Lines[0].Options[0].OptionID != "OPT-large" {
		t.Fatalf("options = %+v", got.Lines[0].Options)
	}
	if got.Lines[0].Options[0].Price.Display == "" {
		t.Error("an option price crossed the contract with no rendered string")
	}
}
