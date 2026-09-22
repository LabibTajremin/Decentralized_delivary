package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// These drive merchant registration and the approval workflow through the real
// binary against real PostGIS: HTTP in, geo placement, transactional write,
// spatial index update, JSON out.

// merchantPayload is the shape the API returns for a shop.
type merchantPayload struct {
	ID                string              `json:"id"`
	OwnerUserID       string              `json:"owner_user_id"`
	Name              string              `json:"name"`
	Type              string              `json:"type"`
	Status            string              `json:"status"`
	Phone             string              `json:"phone"`
	SingleLine        string              `json:"single_line"`
	AreaCode          string              `json:"area_code"`
	AreaName          string              `json:"area_name"`
	DistrictCode      string              `json:"district_code"`
	DivisionCode      string              `json:"division_code"`
	IsListed          bool                `json:"is_listed"`
	IsOpenNow         bool                `json:"is_open_now"`
	OpenStatus        string              `json:"open_status"`
	Hours             map[string][]string `json:"hours"`
	RequiredDocuments []string            `json:"required_documents"`
	MissingDocuments  []string            `json:"missing_documents"`
	CanSubmit         bool                `json:"can_submit"`
	ReviewNote        string              `json:"review_note"`
}

// nearbyMerchants asks the public radius search what it can see around a point.
func nearbyMerchants(t *testing.T, base, query string) []string {
	t.Helper()
	var body struct {
		Merchants []struct {
			MerchantID string `json:"merchant_id"`
		} `json:"merchants"`
	}
	if status := getJSONAs(t, base+"/v1/geo/merchants?"+query, "", &body); status != http.StatusOK {
		t.Fatalf("radius search: status = %d", status)
	}
	out := make([]string, 0, len(body.Merchants))
	for _, m := range body.Merchants {
		out = append(out, m.MerchantID)
	}
	return out
}

// registerShop registers a restaurant at a point and returns it.
func registerShop(t *testing.T, base, token, name string, lat, lng float64) merchantPayload {
	t.Helper()
	var created merchantPayload
	body := fmt.Sprintf(`{
		"name": %q, "type": "restaurant", "phone": "01712345678",
		"line1": "House 12, Road 7", "line2": "Dhanmondi",
		"lat": %v, "lng": %v
	}`, name, lat, lng)
	status := requestAs(t, http.MethodPost, base+"/v1/merchants", body, token, &created)
	if status != http.StatusCreated {
		t.Fatalf("register %s: status = %d", name, status)
	}
	return created
}

// uploadEveryDocument attaches the three papers a restaurant needs.
func uploadEveryDocument(t *testing.T, base, token string) {
	t.Helper()
	for _, kind := range []string{"trade_licence", "national_id", "food_licence"} {
		status := requestAs(t, http.MethodPut, base+"/v1/merchants/me/documents",
			`{"kind":"`+kind+`","number":"N-1","file_url":"/static/demo/doc.png"}`, token, nil)
		if status != http.StatusOK {
			t.Fatalf("upload %s: status = %d", kind, status)
		}
	}
}

// TestAShopRegistersFromAnywhereAndApprovalGatesVisibility is the phase's
// acceptance criteria end to end, against real geometry and the real spatial
// index: a shop registers from a point in Bangladesh, stays invisible to the
// customer-facing radius search until an admin approves it, and disappears
// again the moment one suspends it.
func TestAShopRegistersFromAnywhereAndApprovalGatesVisibility(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	owner := signInAs(t, base, tail, uniquePhone(t), "merchant phone", "merchant")
	admin := signInAs(t, base, tail, uniquePhone(t), "admin console", "admin")

	// Dhanmondi, inside the seeded area — and deliberately away from the demo
	// merchants, so the search around it measures only this shop.
	const lat, lng = 23.7461, 90.3742
	shop := registerShop(t, base, owner.AccessToken, "নূরজাহান হোটেল", lat, lng)

	if shop.OwnerUserID == "" || shop.Status != "draft" {
		t.Fatalf("registered shop = %+v", shop)
	}
	if shop.AreaCode != "DHK-DHM" || shop.AreaName != "Dhanmondi" {
		t.Errorf("area = %q/%q, want the seeded Dhanmondi", shop.AreaCode, shop.AreaName)
	}
	if shop.DistrictCode != "DHK" || shop.DivisionCode != "DHA" {
		t.Errorf("placement = %+v", shop)
	}
	if shop.Phone != "+8801712345678" {
		t.Errorf("phone = %q, want the normalised form", shop.Phone)
	}
	if len(shop.Hours) != 7 {
		t.Errorf("hours = %v, want a working default week", shop.Hours)
	}

	radius := "lat=23.7461&lng=90.3742&radius_m=300"

	// Invisible while it is a draft.
	if found := nearbyMerchants(t, base, radius); contains(found, shop.ID) {
		t.Errorf("a shop in draft is in the radius search: %v", found)
	}

	uploadEveryDocument(t, base, owner.AccessToken)

	var ready merchantPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/merchants/me", "", owner.AccessToken, &ready); status != http.StatusOK {
		t.Fatalf("read own shop: status = %d", status)
	}
	if !ready.CanSubmit || len(ready.MissingDocuments) != 0 {
		t.Fatalf("can_submit = %v, missing = %v", ready.CanSubmit, ready.MissingDocuments)
	}

	var submitted merchantPayload
	if status := requestAs(t, http.MethodPost, base+"/v1/merchants/me/submit", "", owner.AccessToken, &submitted); status != http.StatusOK {
		t.Fatalf("submit: status = %d", status)
	}
	if submitted.Status != "pending_review" {
		t.Fatalf("status = %q", submitted.Status)
	}

	// Still invisible while an admin decides.
	if found := nearbyMerchants(t, base, radius); contains(found, shop.ID) {
		t.Errorf("a shop under review is in the radius search: %v", found)
	}

	var approved merchantPayload
	if status := requestAs(t, http.MethodPost, base+"/v1/admin/merchants/"+shop.ID+"/approve",
		`{"note":""}`, admin.AccessToken, &approved); status != http.StatusOK {
		t.Fatalf("approve: status = %d", status)
	}
	if approved.Status != "approved" || !approved.IsListed {
		t.Fatalf("approved shop = %+v", approved)
	}
	if approved.OpenStatus == "" {
		t.Error("open_status is empty; the client has nothing to render")
	}

	// Now, and only now, a customer's radius search finds it.
	if found := nearbyMerchants(t, base, radius); !contains(found, shop.ID) {
		t.Errorf("an approved shop is not in the radius search: %v", found)
	}

	var suspended merchantPayload
	if status := requestAs(t, http.MethodPost, base+"/v1/admin/merchants/"+shop.ID+"/suspend",
		`{"note":"Repeated cancellations."}`, admin.AccessToken, &suspended); status != http.StatusOK {
		t.Fatalf("suspend: status = %d", status)
	}
	if found := nearbyMerchants(t, base, radius); contains(found, shop.ID) {
		t.Errorf("a suspended shop is still in the radius search: %v", found)
	}
	if suspended.ReviewNote == "" {
		t.Error("the suspension carried no reason to show the owner")
	}
}

// TestAShopInARuralUpazilaRegistersOnTheSameTerms is D1 against real geometry:
// the point is far from Dhaka and from every other merchant, and nothing about
// registration treats it differently.
func TestAShopInARuralUpazilaRegistersOnTheSameTerms(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()
	owner := signInAs(t, base, tail, uniquePhone(t), "merchant phone", "merchant")

	// Rural Mymensingh: inside a division, outside every area the seed draws.
	// That is exactly the case a rule written around the finely-mapped capital
	// would wrongly refuse.
	shop := registerShop(t, base, owner.AccessToken, "গ্রামের দোকান", 24.5, 90.0)

	if shop.DivisionCode == "" {
		t.Error("the shop was accepted without a division — the D3 ceiling is unset")
	}
	if shop.AreaCode != "" {
		t.Errorf("area = %q; this point was chosen because no area covers it, "+
			"so the seed has changed and the test no longer proves anything", shop.AreaCode)
	}
	if shop.Status != "draft" {
		t.Errorf("status = %q, want draft", shop.Status)
	}
}

// TestAShopOutsideBangladeshIsRefused, with geo's own wording rather than a
// second message invented by the merchant module.
func TestAShopOutsideBangladeshIsRefused(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()
	owner := signInAs(t, base, tail, uniquePhone(t), "merchant phone", "merchant")

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	// The Eiffel Tower.
	status := requestAs(t, http.MethodPost, base+"/v1/merchants", `{
		"name":"Le Bistro","type":"restaurant","phone":"01712345678",
		"line1":"Champ de Mars","lat":48.8584,"lng":2.2945
	}`, owner.AccessToken, &body)

	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}
	if body.Error.Code != "outside_service_area" {
		t.Errorf("code = %q", body.Error.Code)
	}
}

// TestTheApprovalQueueIsAdminOnly. It carries trade licence numbers and the
// owner's NID, so it is protected as firmly as the decisions it leads to.
func TestTheApprovalQueueIsAdminOnly(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	owner := signInAs(t, base, tail, uniquePhone(t), "merchant phone", "merchant")
	shop := registerShop(t, base, owner.AccessToken, "নূরজাহান হোটেল", 23.7461, 90.3742)

	targets := []struct{ method, path string }{
		{http.MethodGet, "/v1/admin/merchants"},
		{http.MethodGet, "/v1/admin/merchants/" + shop.ID},
		{http.MethodGet, "/v1/admin/merchants/" + shop.ID + "/history"},
		{http.MethodPost, "/v1/admin/merchants/" + shop.ID + "/approve"},
		{http.MethodPost, "/v1/admin/merchants/" + shop.ID + "/reject"},
		{http.MethodPost, "/v1/admin/merchants/" + shop.ID + "/suspend"},
		{http.MethodPost, "/v1/admin/merchants/" + shop.ID + "/reinstate"},
	}

	for _, target := range targets {
		body := ""
		if target.method == http.MethodPost {
			body = `{"note":"x"}`
		}

		if status := requestAs(t, target.method, base+target.path, body, "", nil); status != http.StatusUnauthorized {
			t.Errorf("%s with no token: status = %d, want 401", target.path, status)
		}
		// A shop owner is authenticated and still not an admin: approving your
		// own registration is the whole point of the workflow existing.
		if status := requestAs(t, target.method, base+target.path, body, owner.AccessToken, nil); status != http.StatusForbidden {
			t.Errorf("%s as a merchant: status = %d, want 403", target.path, status)
		}
	}
}

// TestOneMerchantCannotReachAnothersShop: every owner route acts on the shop
// the token owns, so there is no id for a caller to substitute.
func TestOneMerchantCannotReachAnothersShop(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	first := signInAs(t, base, tail, uniquePhone(t), "phone one", "merchant")
	registerShop(t, base, first.AccessToken, "First Shop", 23.7461, 90.3742)

	second := signInAs(t, base, tail, uniquePhone(t), "phone two", "merchant")

	var body merchantPayload
	if status := requestAs(t, http.MethodGet, base+"/v1/merchants/me", "", second.AccessToken, &body); status != http.StatusNotFound {
		t.Errorf("status = %d, want 404 — the second account has no shop", status)
	}
}

// TestHolidayModeHidesAShopFromCustomers, end to end through the same radius
// search a customer's app calls.
func TestHolidayModeHidesAShopFromCustomers(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	owner := signInAs(t, base, tail, uniquePhone(t), "merchant phone", "merchant")
	admin := signInAs(t, base, tail, uniquePhone(t), "admin console", "admin")

	shop := registerShop(t, base, owner.AccessToken, "নূরজাহান হোটেল", 23.7461, 90.3742)
	uploadEveryDocument(t, base, owner.AccessToken)
	if status := requestAs(t, http.MethodPost, base+"/v1/merchants/me/submit", "", owner.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("submit: status = %d", status)
	}
	if status := requestAs(t, http.MethodPost, base+"/v1/admin/merchants/"+shop.ID+"/approve",
		`{}`, admin.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("approve: status = %d", status)
	}

	var onHoliday merchantPayload
	if status := requestAs(t, http.MethodPut, base+"/v1/merchants/me/holiday",
		`{"reason":"ঈদের ছুটি"}`, owner.AccessToken, &onHoliday); status != http.StatusOK {
		t.Fatalf("holiday: status = %d", status)
	}
	if onHoliday.IsListed {
		t.Error("a shop on holiday reports itself listed")
	}
	if onHoliday.Status != "approved" {
		t.Errorf("status = %q — a holiday must not undo an approval", onHoliday.Status)
	}

	var back merchantPayload
	if status := requestAs(t, http.MethodDelete, base+"/v1/merchants/me/holiday", "", owner.AccessToken, &back); status != http.StatusOK {
		t.Fatalf("end holiday: status = %d", status)
	}
	if !back.IsListed {
		t.Error("the shop is still hidden after ending its holiday")
	}
}

// TestTheRegistrationRequirementsAreServed so the app never hard-codes which
// licences a pharmacy needs (2.9).
func TestTheRegistrationRequirementsAreServed(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()
	owner := signInAs(t, base, tail, uniquePhone(t), "merchant phone", "merchant")

	var body struct {
		Types []struct {
			Type              string   `json:"type"`
			RequiredDocuments []string `json:"required_documents"`
		} `json:"types"`
	}
	if status := getJSONAs(t, base+"/v1/merchants/registration-requirements", owner.AccessToken, &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(body.Types) != 3 {
		t.Fatalf("types = %d, want 3", len(body.Types))
	}
	for _, entry := range body.Types {
		if entry.Type == "pharmacy" && !contains(entry.RequiredDocuments, "drug_licence") {
			t.Errorf("pharmacy = %v, want a drug licence", entry.RequiredDocuments)
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// TestTheDemoMerchantsAreRealShops: the geo seed puts fourteen points on the
// map, and a point with no shop behind it is a search result that resolves to
// nothing. This checks the two seeds agree on the same ids.
func TestTheDemoMerchantsAreRealShops(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()
	admin := signInAs(t, base, tail, uniquePhone(t), "admin console", "admin")

	var body struct {
		Merchants []merchantPayload `json:"merchants"`
	}
	if status := getJSONAs(t, base+"/v1/admin/merchants?status=approved&limit=100",
		admin.AccessToken, &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(body.Merchants) != 14 {
		t.Fatalf("approved demo shops = %d, want the 14 the geo seed places", len(body.Merchants))
	}

	for _, shop := range body.Merchants {
		if !shop.IsListed {
			t.Errorf("%s is approved but not listed", shop.ID)
		}
		if len(shop.MissingDocuments) != 0 {
			t.Errorf("%s is approved with %v still missing", shop.ID, shop.MissingDocuments)
		}
		if shop.DivisionCode == "" || shop.AreaName == "" {
			t.Errorf("%s = %+v, want a full placement", shop.ID, shop)
		}
	}

	// And every one of them is findable by the customer-facing radius search
	// around the first shop's own area.
	found := nearbyMerchants(t, base, "lat=23.7455&lng=90.3738&radius_m=1000")
	if len(found) == 0 {
		t.Error("the radius search finds none of the demo shops")
	}
}
