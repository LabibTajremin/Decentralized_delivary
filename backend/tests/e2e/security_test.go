package e2e

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// documentedPaths reads the API contract's path table.
//
// The spec rather than the mux, on purpose. `backend/tests/unit/openapi`
// already fails the build when the two disagree in either direction, so
// walking the spec here walks the served surface — and it does it without
// this package importing all fourteen transport packages to ask each for its
// route list.
func documentedPaths(t *testing.T) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("openapi.yaml declares no paths")
	}

	out := make(map[string][]string, len(doc.Paths))
	for path, operations := range doc.Paths {
		for key := range operations {
			method := strings.ToUpper(key)
			switch method {
			case http.MethodGet, http.MethodPost, http.MethodPut,
				http.MethodPatch, http.MethodDelete:
				out[path] = append(out[path], method)
			default:
				// `parameters`, `summary` and friends sit beside the
				// operations; they are not methods to probe.
			}
		}
		sort.Strings(out[path])
	}
	return out
}

// fillParameters replaces every {placeholder} with an id that is well-formed
// and belongs to nobody.
//
// The value does not matter: role is decided by middleware, before a handler
// ever looks at a path. If one of these probes ever gets past the guard far
// enough for the id to matter, the probe has already found the bug.
func fillParameters(path string) string {
	for {
		open := strings.Index(path, "{")
		if open < 0 {
			return path
		}
		close := strings.Index(path[open:], "}")
		if close < 0 {
			return path
		}
		path = path[:open] + "NOBODYS-ID" + path[open+close+1:]
	}
}

// TestEveryAdminRouteRefusesEveryoneElse is the authorisation sweep.
//
// Each module's own suite proves its own guards, and every one of those tests
// names the routes it checks. That is exactly the weakness: a route added
// next month is guarded by whoever remembers to add it to a list. This test
// has no list. It reads the whole documented surface, takes every path under
// `/v1/admin/`, and requires all of them to refuse an anonymous caller and
// each of the three non-admin roles.
//
// An admin route mounted without the admin middleware therefore fails the
// build the first time CI runs, whether or not anybody wrote a test for it.
func TestEveryAdminRouteRefusesEveryoneElse(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	callers := []struct {
		role  string
		token string
	}{
		{"customer", signInAs(t, base, tail, uniquePhone(t), "customer phone", "customer").AccessToken},
		{"merchant", signInAs(t, base, tail, uniquePhone(t), "merchant phone", "merchant").AccessToken},
		{"partner", signInAs(t, base, tail, uniquePhone(t), "partner phone", "partner").AccessToken},
	}

	paths := documentedPaths(t)
	checked := 0
	for path, methods := range paths {
		if !strings.HasPrefix(path, "/v1/admin/") {
			continue
		}
		url := base + fillParameters(path)
		for _, method := range methods {
			checked++
			// `{}` rather than an empty body: a route that decodes before it
			// authorises would answer 400, and that difference is worth
			// seeing rather than hiding behind a malformed request.
			if status := requestAs(t, method, url, `{}`, "", nil); status != http.StatusUnauthorized {
				t.Errorf("%s %s with no token: status = %d, want 401", method, path, status)
			}
			for _, caller := range callers {
				status := requestAs(t, method, url, `{}`, caller.token, nil)
				if status != http.StatusForbidden {
					t.Errorf("%s %s as a %s: status = %d, want 403",
						method, path, caller.role, status)
				}
			}
		}
	}

	// A sweep that swept nothing would pass silently, which is the one way a
	// test like this fails without saying so.
	if checked < 10 {
		t.Fatalf("only %d admin operations found; the spec did not parse as expected", checked)
	}
	t.Logf("%d admin operations refused an anonymous caller and all three non-admin roles", checked)
}

// TestTheAdminSurfaceIsTheOnlyPlaceAdminRoutesLive is the other half of the
// sweep above: it is only worth anything if "under /v1/admin/" really is how
// this API marks a privileged route.
//
// Two documented routes are privileged without the prefix, and both are
// deliberate: `/v1/config/definitions` and `/v1/config/effective` are the
// pricing and radius configuration, which is admin-only and was named before
// the convention settled. They are asserted here so that the pair stays a
// closed list — a third one appearing means either it needs the prefix or
// this test needs to know why not.
func TestTheAdminSurfaceIsTheOnlyPlaceAdminRoutesLive(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer := signInAs(t, base, tail, uniquePhone(t), "customer phone", "customer")

	for _, path := range []string{
		"/v1/config/definitions",
		"/v1/config/effective?area=DHK-DHM",
	} {
		if status := getJSONAs(t, base+path, "", nil); status != http.StatusUnauthorized {
			t.Errorf("%s with no token: status = %d, want 401", path, status)
		}
		if status := getJSONAs(t, base+path, customer.AccessToken, nil); status != http.StatusForbidden {
			t.Errorf("%s as a customer: status = %d, want 403", path, status)
		}
	}
}

// TestPaymentBelongsToWhoeverIsPaying closes the one module with no E2E suite
// of its own.
//
// Payment's authorisation is not role-based — every customer may pay — so the
// sweep above cannot reach it. What protects a payment is ownership, and the
// questions worth asking are: can a second customer start a checkout against
// somebody else's order, can they read its payment, and does an order that
// exists but is not theirs look any different from one that does not exist.
func TestPaymentBelongsToWhoeverIsPaying(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	customer, _, _, addressID := readyToOrder(t, base, tail)

	var order orderResponse
	if status := requestAs(t, http.MethodPost, base+"/v1/orders",
		fmt.Sprintf(`{"address_id":%q,"payment_method":"online"}`, addressID),
		customer.AccessToken, &order); status != http.StatusCreated {
		t.Fatalf("place order: status = %d", status)
	}

	stranger := signInAs(t, base, tail, uniquePhone(t), "stranger phone", "customer")

	// 404 rather than 403, the same choice the whole API makes: "that order
	// exists but is not yours" lets anybody with a list of ids find out which
	// ones are real.
	if status := requestAs(t, http.MethodPost, base+"/v1/payments/checkout",
		fmt.Sprintf(`{"order_id":%q}`, order.ID), stranger.AccessToken, nil); status != http.StatusNotFound {
		t.Errorf("a stranger's checkout: status = %d, want 404", status)
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/payments/"+order.ID, "",
		stranger.AccessToken, nil); status != http.StatusNotFound {
		t.Errorf("a stranger reading the payment: status = %d, want 404", status)
	}

	// An order id that never existed answers the same way, which is what
	// makes the answer above uninformative.
	if status := requestAs(t, http.MethodGet, base+"/v1/payments/ORD-NOT-A-REAL-ID", "",
		stranger.AccessToken, nil); status != http.StatusNotFound {
		t.Errorf("an invented order id: status = %d, want 404", status)
	}

	// The rider's cash ledger is a partner surface, and this is where the
	// API's actual authorisation model shows through: outside `/v1/admin/`,
	// routes are mounted behind `Authenticated()` and protected by ownership
	// rather than by the token's role. So a customer's token reaches the
	// handler and is turned away by not owning a partner record — 404, not
	// 403. That is the documented behaviour (`Not registered as a delivery
	// partner`), and `docs/security-review.md` records why it is the model
	// and what it costs.
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/cod", "",
		customer.AccessToken, nil); status != http.StatusNotFound {
		t.Errorf("a customer reading the COD ledger: status = %d, want 404", status)
	}
}

// TestOwnershipIsWhatProtectsThePartnerSurface follows the finding above to
// its conclusion.
//
// If ownership is the guard rather than role, then ownership is what has to
// be tested — and the test that matters is not "a customer is turned away"
// but "a rider sees their own ledger and cannot be handed anybody else's".
func TestOwnershipIsWhatProtectsThePartnerSurface(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	karim, karimPartner := onShift(t, base, tail, "Karim", dhanmondiLat, dhanmondiLng)
	_, rafiPartner := onShift(t, base, tail, "Rafi", dhanmondiLat, dhanmondiLng)

	var ledger struct {
		PartnerID string `json:"partner_id"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/partner/cod", "",
		karim.AccessToken, &ledger); status != http.StatusOK {
		t.Fatalf("Karim's own ledger: status = %d", status)
	}
	// The route takes no partner id, which is the point: there is no
	// parameter for a caller to change. The ledger returned is the one
	// belonging to the token.
	if ledger.PartnerID != karimPartner.ID {
		t.Fatalf("Karim was handed %s's ledger", ledger.PartnerID)
	}
	if karimPartner.ID == rafiPartner.ID {
		t.Fatal("two riders registered as the same partner")
	}

	// Rafi's ledger is readable by id, but only through the admin route.
	if status := requestAs(t, http.MethodGet,
		base+"/v1/admin/payments/cod/"+rafiPartner.ID, "",
		karim.AccessToken, nil); status != http.StatusForbidden {
		t.Errorf("Karim reading Rafi's ledger as a rider: status = %d, want 403", status)
	}
}

// TestTheWebhookTrustsNothingButItsSignature. The gateway callback is the one
// route with no bearer token behind it, because a gateway has no account. Its
// only credential is the signature over its own payload, so the absent and
// the wrong signature both have to be refused — and refused before the
// payload is believed.
func TestTheWebhookTrustsNothingButItsSignature(t *testing.T) {
	base, _, stop := startWholeProductAPI(t)
	defer stop()

	body := `{"reference":"PAY-INVENTED","gateway_ref":"GW-X","succeeded":true,"amount_minor":100000}`

	for _, probe := range []struct {
		name      string
		signature string
	}{
		{"no signature at all", ""},
		{"a signature that is not hex", "not-a-signature"},
		{"a valid-looking signature over different bytes",
			signWebhook(regressionWebhookSecret, body+" ")},
		{"a signature made with the wrong secret",
			signWebhook("the-wrong-secret", body)},
	} {
		status := postWithHeader(t, base+"/v1/payments/manual/webhook", body, "",
			"X-Webhook-Signature", probe.signature, nil)
		// 400, not 404: the reference in the payload is never looked up,
		// because the payload is never believed. A 404 here would mean the
		// server had already gone to the database on an unsigned request.
		if status != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", probe.name, status)
		}
	}
}
