package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// These drive the profile and address book through the real binary against real
// PostGIS: HTTP in, geo resolution, transactional write, JSON out.

func requestAs(t *testing.T, method, url, body, bearer string, into any) int {
	t.Helper()
	var reader *bytes.Buffer
	if body == "" {
		reader = bytes.NewBufferString("")
	} else {
		reader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, url, reader) //nolint:noctx // fixed test-local URL
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if into != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp.StatusCode
}

// dhanmondiAddress is a real place inside the seeded Dhanmondi area.
const dhanmondiAddress = `{
	"label":"Home","recipient_name":"Ayesha Rahman","recipient_phone":"01712345678",
	"line1":"House 12, Road 7","line2":"Dhanmondi","instructions":"Blue gate",
	"lat":23.7461,"lng":90.3742
}`

type addressPayload struct {
	ID           string  `json:"id"`
	Label        string  `json:"label"`
	SingleLine   string  `json:"single_line"`
	Lat          float64 `json:"lat"`
	Lng          float64 `json:"lng"`
	AreaCode     string  `json:"area_code"`
	AreaName     string  `json:"area_name"`
	DistrictCode string  `json:"district_code"`
	DivisionCode string  `json:"division_code"`
	IsDefault    bool    `json:"is_default"`
}

// TestAnAddressIsPlacedAgainstRealGeometry is the phase's acceptance criterion,
// end to end: the address resolves to an area, and that area is what drives
// config resolution and pricing downstream.
func TestAnAddressIsPlacedAgainstRealGeometry(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()
	customer := signIn(t, base, tail, uniquePhone(t), "Pixel 8")

	var created addressPayload
	status := requestAs(t, http.MethodPost, base+"/v1/me/addresses",
		dhanmondiAddress, customer.AccessToken, &created)
	if status != http.StatusCreated {
		t.Fatalf("status = %d", status)
	}

	if created.AreaCode != "DHK-DHM" || created.AreaName != "Dhanmondi" {
		t.Errorf("area = %q/%q, want the seeded Dhanmondi", created.AreaCode, created.AreaName)
	}
	if created.DistrictCode != "DHK" || created.DivisionCode != "DHA" {
		t.Errorf("placement = %+v", created)
	}
	if !created.IsDefault {
		t.Error("the first address is not the default")
	}
	if !strings.Contains(created.SingleLine, "Dhanmondi") {
		t.Errorf("single line = %q", created.SingleLine)
	}
}

// An address outside the service area is refused, with geo's own wording.
func TestAnAddressOutsideBangladeshIsRefused(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()
	customer := signIn(t, base, tail, uniquePhone(t), "Pixel 8")

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	// The Eiffel Tower.
	status := requestAs(t, http.MethodPost, base+"/v1/me/addresses", `{
		"recipient_name":"A","recipient_phone":"01712345678",
		"line1":"Champ de Mars","lat":48.8584,"lng":2.2945
	}`, customer.AccessToken, &body)

	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}
	if body.Error.Code != "outside_service_area" {
		t.Errorf("code = %q", body.Error.Code)
	}
}

// TestOneCustomerCannotReachAnothersAddressBook is the property that matters
// most here: an address is a home address and a phone number.
func TestOneCustomerCannotReachAnothersAddressBook(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	owner := signIn(t, base, tail, uniquePhone(t), "owner phone")
	attacker := signIn(t, base, tail, uniquePhone(t), "attacker phone")

	var created addressPayload
	if status := requestAs(t, http.MethodPost, base+"/v1/me/addresses",
		dhanmondiAddress, owner.AccessToken, &created); status != http.StatusCreated {
		t.Fatalf("create: status %d", status)
	}

	var listed struct {
		Addresses []addressPayload `json:"addresses"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/me/addresses", "",
		attacker.AccessToken, &listed); status != http.StatusOK {
		t.Fatalf("list: status %d", status)
	}
	if len(listed.Addresses) != 0 {
		t.Errorf("the attacker sees %d addresses", len(listed.Addresses))
	}

	for _, attempt := range []struct {
		method, path, body string
	}{
		{http.MethodPut, "/v1/me/addresses/" + created.ID, dhanmondiAddress},
		{http.MethodDelete, "/v1/me/addresses/" + created.ID, ""},
		{http.MethodPost, "/v1/me/addresses/" + created.ID + "/default", ""},
	} {
		status := requestAs(t, attempt.method, base+attempt.path, attempt.body, attacker.AccessToken, nil)
		if status != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want 404", attempt.method, attempt.path, status)
		}
	}

	// And the owner's address is untouched.
	if status := requestAs(t, http.MethodGet, base+"/v1/me/addresses", "",
		owner.AccessToken, &listed); status != http.StatusOK {
		t.Fatalf("owner list: status %d", status)
	}
	if len(listed.Addresses) != 1 {
		t.Errorf("the owner has %d addresses", len(listed.Addresses))
	}
}

func TestTheAddressBookRequiresASignedInCaller(t *testing.T) {
	base, _, stop := startAuthAPI(t)
	defer stop()

	for _, path := range []string{"/v1/me", "/v1/me/addresses"} {
		if status := requestAs(t, http.MethodGet, base+path, "", "", nil); status != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", path, status)
		}
	}
}

func TestTheDefaultAddressMovesAndOnlyOneIsEverDefault(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()
	customer := signIn(t, base, tail, uniquePhone(t), "Pixel 8")

	var first, second addressPayload
	if status := requestAs(t, http.MethodPost, base+"/v1/me/addresses",
		dhanmondiAddress, customer.AccessToken, &first); status != http.StatusCreated {
		t.Fatalf("first: status %d", status)
	}
	// A second address in Gulshan.
	if status := requestAs(t, http.MethodPost, base+"/v1/me/addresses", `{
		"label":"Office","recipient_name":"Ayesha","recipient_phone":"01712345678",
		"line1":"Road 11","lat":23.7925,"lng":90.4152
	}`, customer.AccessToken, &second); status != http.StatusCreated {
		t.Fatalf("second: status %d", status)
	}
	if second.AreaCode != "DHK-GUL" {
		t.Errorf("second address area = %q, want Gulshan", second.AreaCode)
	}

	if status := requestAs(t, http.MethodPost,
		base+"/v1/me/addresses/"+second.ID+"/default", "", customer.AccessToken, nil); status != http.StatusOK {
		t.Fatalf("set default: status %d", status)
	}

	var listed struct {
		Addresses []addressPayload `json:"addresses"`
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/me/addresses", "",
		customer.AccessToken, &listed); status != http.StatusOK {
		t.Fatalf("list: status %d", status)
	}

	defaults := 0
	for _, a := range listed.Addresses {
		if a.IsDefault {
			defaults++
			if a.ID != second.ID {
				t.Errorf("the default is %s, want %s", a.ID, second.ID)
			}
		}
	}
	if defaults != 1 {
		t.Errorf("%d defaults, want exactly 1", defaults)
	}

	// Deleting the default promotes the other, so the customer always has one.
	if status := requestAs(t, http.MethodDelete,
		base+"/v1/me/addresses/"+second.ID, "", customer.AccessToken, nil); status != http.StatusNoContent {
		t.Fatalf("delete: status %d", status)
	}
	if status := requestAs(t, http.MethodGet, base+"/v1/me/addresses", "",
		customer.AccessToken, &listed); status != http.StatusOK {
		t.Fatalf("list: status %d", status)
	}
	if len(listed.Addresses) != 1 || !listed.Addresses[0].IsDefault {
		t.Errorf("after deleting the default: %+v", listed.Addresses)
	}
}

func TestTheProfileRoundTripsOverHTTP(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()
	customer := signIn(t, base, tail, uniquePhone(t), "Pixel 8")

	var profile struct {
		UserID      string `json:"user_id"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Language    string `json:"language"`
	}

	// A brand-new customer has a usable profile without having saved one.
	if status := requestAs(t, http.MethodGet, base+"/v1/me", "", customer.AccessToken, &profile); status != http.StatusOK {
		t.Fatalf("get: status %d", status)
	}
	if profile.UserID != customer.UserID || profile.DisplayName == "" {
		t.Errorf("profile = %+v", profile)
	}

	if status := requestAs(t, http.MethodPatch, base+"/v1/me",
		`{"name":"Ayesha Rahman","email":"ayesha@example.com","language":"en"}`,
		customer.AccessToken, &profile); status != http.StatusOK {
		t.Fatalf("patch: status %d", status)
	}
	if profile.Name != "Ayesha Rahman" || profile.Language != "en" {
		t.Errorf("profile = %+v", profile)
	}

	// A partial update leaves the rest alone.
	if status := requestAs(t, http.MethodPatch, base+"/v1/me",
		`{"name":"Ayesha R"}`, customer.AccessToken, &profile); status != http.StatusOK {
		t.Fatalf("second patch: status %d", status)
	}
	if profile.Email != "ayesha@example.com" {
		t.Errorf("email = %q; updating the name erased it", profile.Email)
	}
	if profile.Language != "en" {
		t.Errorf("language = %q; updating the name reset it", profile.Language)
	}
}
