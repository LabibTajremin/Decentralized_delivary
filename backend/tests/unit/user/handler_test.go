package user

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/application"
	userhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/user/transport/http"
)

// Every route acts on the caller's own data, and the user id comes from the
// verified token rather than the request. These tests hold that line.

const callerID = "usr_caller"

type httpRig struct {
	repo *memoryRepo
	geo  *stubGeo
	mux  *http.ServeMux
	// principal is what the fake guard reports. A test changes it to act as
	// somebody else.
	principal string
}

func newHTTPRig(t *testing.T) *httpRig {
	t.Helper()
	rig := &httpRig{repo: newRepo(), geo: newGeo(), principal: callerID}

	mux := http.NewServeMux()
	userhttp.NewHandler(
		application.NewProfileUseCase(rig.repo),
		newAddresses(rig.repo, rig.geo),
		func(next http.Handler) http.Handler { return next },
		func(*http.Request) (string, bool) {
			return rig.principal, rig.principal != ""
		},
	).Register(mux)
	rig.mux = mux
	return rig
}

func (h *httpRig) do(method, target, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body.String())
	}
}

func responseCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decode(t, rec, &body)
	return body.Error.Code
}

const validAddressBody = `{
	"label":"Home","recipient_name":"Ayesha","recipient_phone":"01712345678",
	"line1":"House 12, Road 7","line2":"Dhanmondi","lat":23.7461,"lng":90.3742
}`

func TestTheProfileIsReadableAndUpdatable(t *testing.T) {
	h := newHTTPRig(t)

	rec := h.do(http.MethodGet, "/v1/me", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var profile struct {
		UserID      string `json:"user_id"`
		DisplayName string `json:"display_name"`
		Language    string `json:"language"`
	}
	decode(t, rec, &profile)
	if profile.UserID != callerID || profile.DisplayName == "" || profile.Language != "bn" {
		t.Errorf("profile = %+v", profile)
	}

	rec = h.do(http.MethodPatch, "/v1/me", `{"name":"Ayesha Rahman","language":"en"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d: %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &profile)
	if profile.DisplayName != "Ayesha Rahman" || profile.Language != "en" {
		t.Errorf("profile = %+v", profile)
	}
}

// TestTheUserIdComesFromTheTokenNotTheBody. An endpoint that takes a user id
// from the client is one that will be asked for somebody else's home address.
func TestTheUserIdComesFromTheTokenNotTheBody(t *testing.T) {
	h := newHTTPRig(t)

	// A body naming another user is rejected as an unknown field rather than
	// honoured.
	rec := h.do(http.MethodPatch, "/v1/me", `{"user_id":"usr_victim","name":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unexpected field", rec.Code)
	}

	// And the profile that gets written belongs to the caller.
	if rec := h.do(http.MethodPatch, "/v1/me", `{"name":"Ayesha"}`); rec.Code != http.StatusOK {
		t.Fatalf("patch: status %d", rec.Code)
	}
	if _, ok := h.repo.profiles[callerID]; !ok {
		t.Error("the caller's profile was not the one written")
	}
	if _, ok := h.repo.profiles["usr_victim"]; ok {
		t.Error("a profile was written for the id in the body")
	}
}

// TestOneUserCannotReachAnothersAddresses.
func TestOneUserCannotReachAnothersAddresses(t *testing.T) {
	h := newHTTPRig(t)

	rec := h.do(http.MethodPost, "/v1/me/addresses", validAddressBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	decode(t, rec, &created)

	// Another signed-in user asks for it by id.
	h.principal = "usr_attacker"

	if rec := h.do(http.MethodGet, "/v1/me/addresses", ""); rec.Code == http.StatusOK {
		var body struct {
			Addresses []struct {
				ID string `json:"id"`
			} `json:"addresses"`
		}
		decode(t, rec, &body)
		for _, a := range body.Addresses {
			if a.ID == created.ID {
				t.Error("another user's address appeared in the attacker's list")
			}
		}
	}
	for _, r := range []*httptest.ResponseRecorder{
		h.do(http.MethodPut, "/v1/me/addresses/"+created.ID, validAddressBody),
		h.do(http.MethodDelete, "/v1/me/addresses/"+created.ID, ""),
		h.do(http.MethodPost, "/v1/me/addresses/"+created.ID+"/default", ""),
	} {
		if r.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404 for another user's address", r.Code)
		}
	}
}

func TestAnUnauthenticatedCallerIsRefused(t *testing.T) {
	h := newHTTPRig(t)
	h.principal = ""

	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/v1/me"},
		{http.MethodPatch, "/v1/me"},
		{http.MethodGet, "/v1/me/addresses"},
		{http.MethodPost, "/v1/me/addresses"},
		{http.MethodPut, "/v1/me/addresses/adr_a"},
		{http.MethodDelete, "/v1/me/addresses/adr_a"},
		{http.MethodPost, "/v1/me/addresses/adr_a/default"},
	} {
		body := ""
		if route.method != http.MethodGet && route.method != http.MethodDelete {
			body = "{}"
		}
		rec := h.do(route.method, route.path, body)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", route.method, route.path, rec.Code)
		}
	}
}

func TestTheAddressBookRoundTripsOverHTTP(t *testing.T) {
	h := newHTTPRig(t)

	rec := h.do(http.MethodPost, "/v1/me/addresses", validAddressBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID           string `json:"id"`
		SingleLine   string `json:"single_line"`
		AreaCode     string `json:"area_code"`
		DivisionCode string `json:"division_code"`
		IsDefault    bool   `json:"is_default"`
	}
	decode(t, rec, &created)

	if created.AreaCode != "DHK-DHM" || created.DivisionCode != "DHA" {
		t.Errorf("the address was not placed: %+v", created)
	}
	if created.SingleLine == "" {
		t.Error("no single_line; every surface needs the same composed string")
	}
	if !created.IsDefault {
		t.Error("the first address is not the default")
	}

	rec = h.do(http.MethodGet, "/v1/me/addresses", "")
	var listed struct {
		Addresses []struct {
			ID string `json:"id"`
		} `json:"addresses"`
	}
	decode(t, rec, &listed)
	if len(listed.Addresses) != 1 || listed.Addresses[0].ID != created.ID {
		t.Errorf("list = %+v", listed.Addresses)
	}

	if rec := h.do(http.MethodDelete, "/v1/me/addresses/"+created.ID, ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete: status %d", rec.Code)
	}
}

// An empty address book is an empty list, never null: a client that must
// special-case null before iterating is one that will forget to.
func TestAnEmptyAddressBookIsAnEmptyList(t *testing.T) {
	h := newHTTPRig(t)
	rec := h.do(http.MethodGet, "/v1/me/addresses", "")

	var raw map[string]json.RawMessage
	decode(t, rec, &raw)
	if string(raw["addresses"]) != "[]" {
		t.Errorf("addresses = %s, want []", raw["addresses"])
	}
}

func TestSettingADefaultOverHTTP(t *testing.T) {
	h := newHTTPRig(t)

	var first, second struct {
		ID string `json:"id"`
	}
	decode(t, h.do(http.MethodPost, "/v1/me/addresses", validAddressBody), &first)
	decode(t, h.do(http.MethodPost, "/v1/me/addresses", validAddressBody), &second)

	rec := h.do(http.MethodPost, "/v1/me/addresses/"+second.ID+"/default", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var promoted struct {
		ID        string `json:"id"`
		IsDefault bool   `json:"is_default"`
	}
	decode(t, rec, &promoted)
	if promoted.ID != second.ID || !promoted.IsDefault {
		t.Errorf("promoted = %+v", promoted)
	}
}

func TestInvalidAddressesAreRefusedOverHTTP(t *testing.T) {
	h := newHTTPRig(t)

	cases := map[string]string{
		"invalid_pin":           `{"recipient_name":"A","recipient_phone":"017","line1":"x","lat":0,"lng":0}`,
		"address_line_required": `{"recipient_name":"A","recipient_phone":"017","line1":"","lat":23.7,"lng":90.3}`,
		"recipient_required":    `{"recipient_name":"","recipient_phone":"","line1":"x","lat":23.7,"lng":90.3}`,
	}
	for wantCode, body := range cases {
		rec := h.do(http.MethodPost, "/v1/me/addresses", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", wantCode, rec.Code)
		}
		if got := responseCode(t, rec); got != wantCode {
			t.Errorf("code = %q, want %q", got, wantCode)
		}
	}
}

func TestMalformedBodiesAreRefused(t *testing.T) {
	h := newHTTPRig(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodPatch, "/v1/me"},
		{http.MethodPost, "/v1/me/addresses"},
		{http.MethodPut, "/v1/me/addresses/adr_a"},
	} {
		for _, body := range []string{`{"name":`, `{"nope":1}`} {
			rec := h.do(route.method, route.path, body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s %s with %s: status = %d, want 400", route.method, route.path, body, rec.Code)
			}
		}
	}
}

func TestStoreFailuresSurfaceOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	h.repo.listErr = errStore
	if rec := h.do(http.MethodGet, "/v1/me/addresses", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("list: status = %d, want 503", rec.Code)
	}

	h2 := newHTTPRig(t)
	h2.repo.profileReadErr = errStore
	if rec := h2.do(http.MethodGet, "/v1/me", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("profile: status = %d, want 503", rec.Code)
	}
}

// A nil guard or principal reader would publish every customer's address book,
// so both are refused at wiring time.
func TestAMissingGuardIsRefusedAtWiringTime(t *testing.T) {
	for name, build := range map[string]func(){
		"no guard":     func() { userhttp.NewHandler(nil, nil, nil, func(*http.Request) (string, bool) { return "", false }) },
		"no principal": func() { userhttp.NewHandler(nil, nil, func(h http.Handler) http.Handler { return h }, nil) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: a handler was built anyway", name)
				}
			}()
			build()
		}()
	}
}

func TestEveryRouteIsBehindTheGuard(t *testing.T) {
	var guarded []string
	rig := &httpRig{repo: newRepo(), geo: newGeo(), principal: callerID}
	mux := http.NewServeMux()
	userhttp.NewHandler(
		application.NewProfileUseCase(rig.repo),
		newAddresses(rig.repo, rig.geo),
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				guarded = append(guarded, r.Method+" "+r.URL.Path)
				next.ServeHTTP(w, r)
			})
		},
		func(*http.Request) (string, bool) { return rig.principal, true },
	).Register(mux)
	rig.mux = mux

	for _, pattern := range userhttp.Patterns() {
		method, path, found := strings.Cut(pattern, " ")
		if !found {
			t.Fatalf("pattern %q is not %q", pattern, "METHOD /path")
		}
		path = strings.ReplaceAll(path, "{id}", "adr_a")
		guarded = nil
		rig.do(method, path, "{}")
		if len(guarded) == 0 {
			t.Errorf("%s reached its handler without passing the guard", pattern)
		}
	}
}

func TestAProfileUpdateFailureSurfacesOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	h.repo.profileWriteErr = errStore
	if rec := h.do(http.MethodPatch, "/v1/me", `{"name":"Ayesha"}`); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestAnAddressUpdateFailureSurfacesOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	var created struct {
		ID string `json:"id"`
	}
	decode(t, h.do(http.MethodPost, "/v1/me/addresses", validAddressBody), &created)

	h.repo.saveErr = errStore
	rec := h.do(http.MethodPut, "/v1/me/addresses/"+created.ID, validAddressBody)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

// An unsupported language is refused with a code the app can map to a message,
// rather than a generic 400.
func TestAnUnsupportedLanguageIsRefusedOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	rec := h.do(http.MethodPatch, "/v1/me", `{"language":"fr"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := responseCode(t, rec); got != "unsupported_language" {
		t.Errorf("code = %q", got)
	}
}

func TestEditingAnAddressOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	var created struct {
		ID string `json:"id"`
	}
	decode(t, h.do(http.MethodPost, "/v1/me/addresses", validAddressBody), &created)

	// The customer corrects the street and nudges the pin.
	h.geo.area.AreaCode, h.geo.area.AreaName = "DHK-GUL", "Gulshan"
	rec := h.do(http.MethodPut, "/v1/me/addresses/"+created.ID, `{
		"label":"Office","recipient_name":"Ayesha","recipient_phone":"01712345678",
		"line1":"House 99, Road 1","line2":"Gulshan","lat":23.7925,"lng":90.4152
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var updated struct {
		ID         string `json:"id"`
		Label      string `json:"label"`
		Line1      string `json:"line1"`
		AreaCode   string `json:"area_code"`
		SingleLine string `json:"single_line"`
		IsDefault  bool   `json:"is_default"`
	}
	decode(t, rec, &updated)

	if updated.ID != created.ID {
		t.Errorf("an edit changed the address id: %s -> %s", created.ID, updated.ID)
	}
	if updated.Label != "Office" || updated.Line1 != "House 99, Road 1" {
		t.Errorf("address = %+v", updated)
	}
	// The pin moved, so the stored area must have moved with it — otherwise
	// the address prices the wrong zone, silently.
	if updated.AreaCode != "DHK-GUL" {
		t.Errorf("area = %q, want it re-resolved after the pin moved", updated.AreaCode)
	}
	if !updated.IsDefault {
		t.Error("editing an address took away its default")
	}
	if updated.SingleLine == "" {
		t.Error("no recomposed single line after the edit")
	}
}
