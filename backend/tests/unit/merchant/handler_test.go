package merchant

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	merchanthttp "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/transport/http"
)

// The owner routes act on the caller's own shop and take the owner id from the
// verified token, never from the request. The admin routes take an id and sit
// behind the admin role. These tests hold both lines.

const ownerID = "usr_owner"

type rig struct {
	*harness
	mux *http.ServeMux
	// principal is what the fake guard reports; a test changes it to act as
	// somebody else, or clears it to act as nobody.
	principal string
	// adminCalls counts how often the admin guard was consulted, so a route
	// that quietly escaped it fails the test.
	adminCalls int
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{harness: newHarness(t), principal: ownerID}

	mux := http.NewServeMux()
	merchanthttp.NewHandler(
		r.registration, r.operations, r.moderation, r.clock,
		func(next http.Handler) http.Handler { return next },
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				r.adminCalls++
				next.ServeHTTP(w, req)
			})
		},
		func(*http.Request) (string, bool) { return r.principal, r.principal != "" },
	).Register(mux)
	r.mux = mux
	return r
}

func (r *rig) do(method, target, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.mux.ServeHTTP(rec, req)
	return rec
}

// registerOverHTTP creates a shop through the API and returns its id.
func (r *rig) registerOverHTTP(t *testing.T) string {
	t.Helper()
	rec := r.do(http.MethodPost, "/v1/merchants", `{
		"name": "Nurjahan Hotel", "type": "restaurant", "phone": "01712345678",
		"line1": "12/A, Mirpur Road", "lat": 23.7509, "lng": 90.3925
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: status = %d, body = %s", rec.Code, rec.Body)
	}
	var body struct {
		ID string `json:"id"`
	}
	decodeBody(t, rec, &body)
	return body.ID
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body)
	}
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, rec, &body)
	return body.Error.Code
}

// ------------------------------------------------------------------ wiring

// TestTheHandlerRefusesToStartWithoutItsGuards: a nil guard would publish the
// approval queue to anyone who found the URL, so it fails at wiring time rather
// than becoming a hole nobody notices.
func TestTheHandlerRefusesToStartWithoutItsGuards(t *testing.T) {
	pass := func(next http.Handler) http.Handler { return next }
	principal := func(*http.Request) (string, bool) { return "u", true }

	cases := map[string]func(){
		"no authenticated guard": func() {
			merchanthttp.NewHandler(nil, nil, nil, nil, nil, merchanthttp.Guard(pass), principal)
		},
		"no admin guard": func() {
			merchanthttp.NewHandler(nil, nil, nil, nil, merchanthttp.Guard(pass), nil, principal)
		},
		"no principal reader": func() {
			merchanthttp.NewHandler(nil, nil, nil, nil, merchanthttp.Guard(pass), merchanthttp.Guard(pass), nil)
		},
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("the handler was built without its guard")
				}
			}()
			build()
		})
	}
}

// TestEveryAdminRouteIsBehindTheAdminGuard walks the whole admin surface and
// checks each route was actually wrapped. A route added to the map and mounted
// on the wrong guard is the bug this catches.
func TestEveryAdminRouteIsBehindTheAdminGuard(t *testing.T) {
	r := newRig(t)
	id := r.registerOverHTTP(t)

	requests := [][2]string{
		{http.MethodGet, "/v1/admin/merchants"},
		{http.MethodGet, "/v1/admin/merchants/" + id},
		{http.MethodGet, "/v1/admin/merchants/" + id + "/history"},
		{http.MethodPost, "/v1/admin/merchants/" + id + "/approve"},
		{http.MethodPost, "/v1/admin/merchants/" + id + "/reject"},
		{http.MethodPost, "/v1/admin/merchants/" + id + "/suspend"},
		{http.MethodPost, "/v1/admin/merchants/" + id + "/reinstate"},
	}

	for _, req := range requests {
		before := r.adminCalls
		r.do(req[0], req[1], `{"note": "x"}`)
		if r.adminCalls == before {
			t.Errorf("%s %s did not pass through the admin guard", req[0], req[1])
		}
	}
	if len(requests) != len(adminPatterns()) {
		t.Errorf("this test exercises %d admin routes but %d are mounted",
			len(requests), len(adminPatterns()))
	}
}

// adminPatterns is the mounted admin surface, read back from the handler.
func adminPatterns() []string {
	var out []string
	for _, pattern := range merchanthttp.Patterns() {
		if strings.Contains(pattern, "/v1/admin/") {
			out = append(out, pattern)
		}
	}
	return out
}

// TestEveryRouteRefusesAnUnauthenticatedCaller: the guard is faked open here,
// so this checks the handler's own check rather than the middleware's.
func TestEveryRouteRefusesACallerWithNoIdentity(t *testing.T) {
	r := newRig(t)
	r.principal = ""

	requests := [][2]string{
		{http.MethodPost, "/v1/merchants"},
		{http.MethodGet, "/v1/merchants/me"},
		{http.MethodPatch, "/v1/merchants/me"},
		{http.MethodDelete, "/v1/merchants/me"},
		{http.MethodPut, "/v1/merchants/me/documents"},
		{http.MethodPost, "/v1/merchants/me/submit"},
		{http.MethodPut, "/v1/merchants/me/hours"},
		{http.MethodPut, "/v1/merchants/me/holiday"},
		{http.MethodDelete, "/v1/merchants/me/holiday"},
		{http.MethodPost, "/v1/admin/merchants/mch_1/approve"},
	}

	for _, req := range requests {
		rec := r.do(req[0], req[1], `{}`)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", req[0], req[1], rec.Code)
		}
		if got := errorCode(t, rec); got != "not_authenticated" {
			t.Errorf("%s %s: code = %q", req[0], req[1], got)
		}
	}
}

func TestPatternsMatchWhatIsMounted(t *testing.T) {
	patterns := merchanthttp.Patterns()
	if len(patterns) == 0 {
		t.Fatal("the module reports no routes")
	}
	for i := 1; i < len(patterns); i++ {
		if patterns[i-1] >= patterns[i] {
			t.Errorf("Patterns() is not sorted: %q before %q", patterns[i-1], patterns[i])
		}
	}

	// Every reported pattern must actually route somewhere.
	r := newRig(t)
	for _, pattern := range patterns {
		method, path, found := strings.Cut(pattern, " ")
		if !found {
			t.Fatalf("pattern %q is not \"METHOD /path\"", pattern)
		}
		path = strings.ReplaceAll(path, "{id}", "mch_1")
		if rec := r.do(method, path, `{}`); rec.Code == http.StatusNotFound && rec.Body.Len() == 0 {
			t.Errorf("%s is reported but nothing is mounted at it", pattern)
		}
	}
}

// ----------------------------------------------------------- registration

func TestRegisteringOverHTTPReturnsTheWholeShop(t *testing.T) {
	r := newRig(t)
	rec := r.do(http.MethodPost, "/v1/merchants", `{
		"name": "Nurjahan Hotel", "type": "restaurant", "phone": "01712345678",
		"email": "shop@example.com", "logo_url": "/static/demo/l.png",
		"line1": "12/A, Mirpur Road", "line2": "Level 2",
		"lat": 23.7509, "lng": 90.3925
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	var body struct {
		ID                string   `json:"id"`
		OwnerUserID       string   `json:"owner_user_id"`
		Status            string   `json:"status"`
		Phone             string   `json:"phone"`
		SingleLine        string   `json:"single_line"`
		OpenStatus        string   `json:"open_status"`
		IsListed          bool     `json:"is_listed"`
		RequiredDocuments []string `json:"required_documents"`
		MissingDocuments  []string `json:"missing_documents"`
		CanSubmit         bool     `json:"can_submit"`
		NextStatuses      []string `json:"next_statuses"`
		Hours             map[string][]string
		CreatedAt         string `json:"created_at"`
	}
	decodeBody(t, rec, &body)

	if body.OwnerUserID != ownerID {
		t.Errorf("owner = %q, want the token's subject", body.OwnerUserID)
	}
	if body.Status != "draft" || body.IsListed {
		t.Errorf("status = %q, listed = %v; want an invisible draft", body.Status, body.IsListed)
	}
	if body.Phone != "+8801712345678" {
		t.Errorf("phone = %q, want the normalised form", body.Phone)
	}
	if body.SingleLine != "12/A, Mirpur Road, Level 2, Dhanmondi" {
		t.Errorf("single_line = %q", body.SingleLine)
	}
	if body.OpenStatus == "" {
		t.Error("open_status is empty; the client has nothing to render")
	}
	if len(body.RequiredDocuments) != 3 || len(body.MissingDocuments) != 3 {
		t.Errorf("required = %v, missing = %v; want all three outstanding",
			body.RequiredDocuments, body.MissingDocuments)
	}
	if body.CanSubmit {
		t.Error("can_submit is true with no documents attached")
	}
	if len(body.NextStatuses) != 1 || body.NextStatuses[0] != "pending_review" {
		t.Errorf("next_statuses = %v", body.NextStatuses)
	}
	if len(body.Hours) != 7 {
		t.Errorf("hours = %v, want the default week", body.Hours)
	}
	if _, err := time.Parse(time.RFC3339, body.CreatedAt); err != nil {
		t.Errorf("created_at = %q: %v", body.CreatedAt, err)
	}
}

// TestTheOwnerIsTakenFromTheTokenNotTheBody: an endpoint that accepted an owner
// id from the client is an endpoint that will be asked to register a shop for
// somebody else.
func TestTheOwnerIsTakenFromTheTokenNotTheBody(t *testing.T) {
	r := newRig(t)
	rec := r.do(http.MethodPost, "/v1/merchants", `{
		"owner_user_id": "usr_someone_else",
		"name": "Nurjahan Hotel", "type": "restaurant", "phone": "01712345678",
		"line1": "12/A", "lat": 23.7509, "lng": 90.3925
	}`)
	// Unknown fields are refused outright, which is the strongest form of the
	// same guarantee: the body cannot name an owner at all.
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unknown field", rec.Code)
	}
}

func TestABodyThatIsNotJSONIsRefused(t *testing.T) {
	r := newRig(t)
	for _, target := range []string{
		"/v1/merchants", "/v1/merchants/me/documents",
		"/v1/merchants/me/hours", "/v1/merchants/me/holiday",
	} {
		method := http.MethodPut
		if target == "/v1/merchants" {
			method = http.MethodPost
		}
		if rec := r.do(method, target, `{`); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, rec.Code)
		}
	}
	if rec := r.do(http.MethodPatch, "/v1/merchants/me", `{`); rec.Code != http.StatusBadRequest {
		t.Errorf("PATCH /v1/merchants/me: status = %d, want 400", rec.Code)
	}
	if rec := r.do(http.MethodPost, "/v1/admin/merchants/mch_1/approve", `{`); rec.Code != http.StatusBadRequest {
		t.Errorf("approve: status = %d, want 400", rec.Code)
	}
}

func TestAnOwnerWithNoShopGetsA404(t *testing.T) {
	r := newRig(t)
	rec := r.do(http.MethodGet, "/v1/merchants/me", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if got := errorCode(t, rec); got != "no_shop" {
		t.Errorf("code = %q, want no_shop", got)
	}
}

// -------------------------------------------------------- the whole journey

// TestAShopGoesFromRegistrationToVisibleOverHTTP walks the phase's acceptance
// criteria end to end through the API: register from anywhere, attach papers,
// submit, and become visible only once an admin approves.
func TestAShopGoesFromRegistrationToVisibleOverHTTP(t *testing.T) {
	r := newRig(t)
	id := r.registerOverHTTP(t)

	// Submitting with nothing attached is refused, and says what is missing.
	rec := r.do(http.MethodPost, "/v1/merchants/me/submit", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("premature submit: status = %d, want 409", rec.Code)
	}
	if got := errorCode(t, rec); got != "documents_incomplete" {
		t.Errorf("code = %q, want documents_incomplete", got)
	}

	for _, kind := range []string{"trade_licence", "national_id", "food_licence"} {
		rec := r.do(http.MethodPut, "/v1/merchants/me/documents",
			`{"kind": "`+kind+`", "number": "N-1", "file_url": "/f.png"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("upload %s: status = %d, body = %s", kind, rec.Code, rec.Body)
		}
	}

	rec = r.do(http.MethodGet, "/v1/merchants/me", "")
	var ready struct {
		CanSubmit        bool     `json:"can_submit"`
		MissingDocuments []string `json:"missing_documents"`
		Documents        []struct {
			Kind       string `json:"kind"`
			UploadedAt string `json:"uploaded_at"`
		} `json:"documents"`
	}
	decodeBody(t, rec, &ready)
	if !ready.CanSubmit || len(ready.MissingDocuments) != 0 {
		t.Errorf("can_submit = %v, missing = %v", ready.CanSubmit, ready.MissingDocuments)
	}
	if len(ready.Documents) != 3 {
		t.Fatalf("documents = %d, want 3", len(ready.Documents))
	}
	if _, err := time.Parse(time.RFC3339, ready.Documents[0].UploadedAt); err != nil {
		t.Errorf("uploaded_at = %q: %v", ready.Documents[0].UploadedAt, err)
	}

	if rec := r.do(http.MethodPost, "/v1/merchants/me/submit", ""); rec.Code != http.StatusOK {
		t.Fatalf("submit: status = %d, body = %s", rec.Code, rec.Body)
	}

	// Still invisible while it waits.
	rec = r.do(http.MethodGet, "/v1/merchants/me", "")
	var pending struct {
		Status   string `json:"status"`
		IsListed bool   `json:"is_listed"`
	}
	decodeBody(t, rec, &pending)
	if pending.Status != "pending_review" || pending.IsListed {
		t.Errorf("status = %q, listed = %v", pending.Status, pending.IsListed)
	}

	// An admin approves it.
	r.principal = "usr_admin"
	rec = r.do(http.MethodPost, "/v1/admin/merchants/"+id+"/approve", `{"note": ""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve: status = %d, body = %s", rec.Code, rec.Body)
	}
	var approved struct {
		Status     string `json:"status"`
		IsListed   bool   `json:"is_listed"`
		IsOpenNow  bool   `json:"is_open_now"`
		OpenStatus string `json:"open_status"`
	}
	decodeBody(t, rec, &approved)
	if approved.Status != "approved" || !approved.IsListed || !approved.IsOpenNow {
		t.Errorf("approved shop = %+v", approved)
	}
	if !r.geo.isActive(id) {
		t.Error("an approved shop is not searchable")
	}
}

func TestRejectionAndSuspensionOverHTTP(t *testing.T) {
	r := newRig(t)
	id := r.registerOverHTTP(t)
	for _, kind := range []string{"trade_licence", "national_id", "food_licence"} {
		r.do(http.MethodPut, "/v1/merchants/me/documents",
			`{"kind": "`+kind+`", "number": "N-1", "file_url": "/f.png"}`)
	}
	r.do(http.MethodPost, "/v1/merchants/me/submit", "")

	r.principal = "usr_admin"

	// A rejection with no reason is refused.
	rec := r.do(http.MethodPost, "/v1/admin/merchants/"+id+"/reject", `{"note": "  "}`)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "reason_required" {
		t.Errorf("reject with no reason: status = %d, code = %q", rec.Code, errorCode(t, rec))
	}

	rec = r.do(http.MethodPost, "/v1/admin/merchants/"+id+"/reject",
		`{"note": "The trade licence photo is unreadable."}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("reject: status = %d, body = %s", rec.Code, rec.Body)
	}
	var rejected struct {
		Status     string `json:"status"`
		ReviewNote string `json:"review_note"`
	}
	decodeBody(t, rec, &rejected)
	if rejected.Status != "rejected" || rejected.ReviewNote == "" {
		t.Errorf("rejected = %+v", rejected)
	}

	// The owner fixes it and resubmits; the admin approves, then suspends.
	r.principal = ownerID
	r.do(http.MethodPut, "/v1/merchants/me/documents",
		`{"kind": "trade_licence", "number": "N-2", "file_url": "/clear.png"}`)
	r.do(http.MethodPost, "/v1/merchants/me/submit", "")

	r.principal = "usr_admin"
	r.do(http.MethodPost, "/v1/admin/merchants/"+id+"/approve", `{}`)

	rec = r.do(http.MethodPost, "/v1/admin/merchants/"+id+"/suspend", `{"note": "Complaints."}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("suspend: status = %d, body = %s", rec.Code, rec.Body)
	}
	if r.geo.isActive(id) {
		t.Error("a suspended shop is still searchable")
	}

	rec = r.do(http.MethodPost, "/v1/admin/merchants/"+id+"/reinstate", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("reinstate: status = %d, body = %s", rec.Code, rec.Body)
	}
	if !r.geo.isActive(id) {
		t.Error("a reinstated shop is not searchable")
	}
}

func TestTheApprovalQueueIsServedWithItsFilters(t *testing.T) {
	r := newRig(t)
	r.registerOverHTTP(t)
	r.principal = "usr_admin"

	rec := r.do(http.MethodGet, "/v1/admin/merchants?status=draft&type=restaurant&division=BD-C&limit=10&offset=0", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var body struct {
		Merchants []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"merchants"`
	}
	decodeBody(t, rec, &body)
	if len(body.Merchants) != 1 || body.Merchants[0].Status != "draft" {
		t.Errorf("merchants = %+v", body.Merchants)
	}

	// A filter we do not understand is a 400, not an empty page.
	rec = r.do(http.MethodGet, "/v1/admin/merchants?status=banished", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// TestAnUnreadablePageSizeFallsBackToTheDefault: a 400 for "limit=abc" is a
// support call about a page that will not load, and the use case clamps it
// anyway.
func TestAnUnreadablePageSizeFallsBackToTheDefault(t *testing.T) {
	r := newRig(t)
	r.registerOverHTTP(t)
	r.principal = "usr_admin"

	if rec := r.do(http.MethodGet, "/v1/admin/merchants?limit=abc&offset=xyz", ""); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestOneShopIsServedToAnAdminById(t *testing.T) {
	r := newRig(t)
	id := r.registerOverHTTP(t)
	r.principal = "usr_admin"

	rec := r.do(http.MethodGet, "/v1/admin/merchants/"+id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	rec = r.do(http.MethodGet, "/v1/admin/merchants/mch_nope", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestTheDecisionHistoryIsServed(t *testing.T) {
	r := newRig(t)
	id := r.registerOverHTTP(t)
	for _, kind := range []string{"trade_licence", "national_id", "food_licence"} {
		r.do(http.MethodPut, "/v1/merchants/me/documents",
			`{"kind": "`+kind+`", "number": "N-1", "file_url": "/f.png"}`)
	}
	r.do(http.MethodPost, "/v1/merchants/me/submit", "")
	r.principal = "usr_admin"
	r.do(http.MethodPost, "/v1/admin/merchants/"+id+"/approve", `{}`)

	rec := r.do(http.MethodGet, "/v1/admin/merchants/"+id+"/history?limit=5", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var body struct {
		Events []struct {
			From        string `json:"from"`
			To          string `json:"to"`
			ActorUserID string `json:"actor_user_id"`
			At          string `json:"at"`
		} `json:"events"`
	}
	decodeBody(t, rec, &body)
	if len(body.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(body.Events))
	}
	if body.Events[0].To != "approved" || body.Events[0].ActorUserID != "usr_admin" {
		t.Errorf("newest event = %+v", body.Events[0])
	}
	if _, err := time.Parse(time.RFC3339, body.Events[0].At); err != nil {
		t.Errorf("at = %q: %v", body.Events[0].At, err)
	}
}

func TestTheHistoryReportsAStoreThatIsDown(t *testing.T) {
	r := newRig(t)
	id := r.registerOverHTTP(t)
	r.principal = "usr_admin"
	r.repo.historyErr = errStore

	if rec := r.do(http.MethodGet, "/v1/admin/merchants/"+id+"/history", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

// -------------------------------------------------------------- operations

func TestOpeningHoursAndHolidayModeOverHTTP(t *testing.T) {
	r := newRig(t)
	r.registerOverHTTP(t)

	rec := r.do(http.MethodPut, "/v1/merchants/me/hours",
		`{"days": {"1": ["07:00-11:00", "18:00-23:00"], "5": ["15:00-24:00"]}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("set hours: status = %d, body = %s", rec.Code, rec.Body)
	}
	var withHours struct {
		Hours map[string][]string `json:"hours"`
	}
	decodeBody(t, rec, &withHours)
	if len(withHours.Hours) != 2 || len(withHours.Hours["1"]) != 2 {
		t.Errorf("hours = %v", withHours.Hours)
	}

	// A schedule that never opens is refused.
	if rec := r.do(http.MethodPut, "/v1/merchants/me/hours", `{"days": {}}`); rec.Code != http.StatusBadRequest {
		t.Errorf("empty schedule: status = %d, want 400", rec.Code)
	}

	rec = r.do(http.MethodPut, "/v1/merchants/me/holiday",
		`{"until": "2026-03-10T00:00:00Z", "reason": "Eid"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("holiday: status = %d, body = %s", rec.Code, rec.Body)
	}
	var onHoliday struct {
		IsListed bool `json:"is_listed"`
		Holiday  *struct {
			Until  string `json:"until"`
			Reason string `json:"reason"`
		} `json:"holiday"`
	}
	decodeBody(t, rec, &onHoliday)
	if onHoliday.IsListed {
		t.Error("a shop on holiday is listed")
	}
	if onHoliday.Holiday == nil || onHoliday.Holiday.Reason != "Eid" || onHoliday.Holiday.Until == "" {
		t.Errorf("holiday = %+v", onHoliday.Holiday)
	}

	rec = r.do(http.MethodDelete, "/v1/merchants/me/holiday", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("end holiday: status = %d, body = %s", rec.Code, rec.Body)
	}
	var back struct {
		Holiday *json.RawMessage `json:"holiday"`
	}
	decodeBody(t, rec, &back)
	if back.Holiday != nil {
		t.Errorf("holiday is still reported after it ended: %s", *back.Holiday)
	}
}

// TestAnIndefiniteHolidayOmitsItsEndDate: the field is absent rather than a
// zero timestamp, so a client cannot render "reopens 1 January 0001".
func TestAnIndefiniteHolidayOmitsItsEndDate(t *testing.T) {
	r := newRig(t)
	r.registerOverHTTP(t)

	rec := r.do(http.MethodPut, "/v1/merchants/me/holiday", `{"reason": "Family emergency"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var body struct {
		Holiday struct {
			Until  string `json:"until"`
			Reason string `json:"reason"`
		} `json:"holiday"`
	}
	decodeBody(t, rec, &body)
	if body.Holiday.Until != "" {
		t.Errorf("until = %q, want it omitted", body.Holiday.Until)
	}
	if body.Holiday.Reason != "Family emergency" {
		t.Errorf("reason = %q", body.Holiday.Reason)
	}
}

func TestAHolidayDateMustBeADate(t *testing.T) {
	r := newRig(t)
	r.registerOverHTTP(t)

	rec := r.do(http.MethodPut, "/v1/merchants/me/holiday", `{"until": "next Tuesday"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := errorCode(t, rec); got != "invalid_date" {
		t.Errorf("code = %q, want invalid_date", got)
	}
}

func TestEditingAndWithdrawingOverHTTP(t *testing.T) {
	r := newRig(t)
	r.registerOverHTTP(t)

	rec := r.do(http.MethodPatch, "/v1/merchants/me", `{
		"name": "Nurjahan Hotel & Restaurant", "type": "restaurant",
		"phone": "01812345678", "line1": "14/B, Mirpur Road",
		"lat": 23.7600, "lng": 90.3600
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: status = %d, body = %s", rec.Code, rec.Body)
	}
	var updated struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	decodeBody(t, rec, &updated)
	if updated.Name != "Nurjahan Hotel & Restaurant" || updated.Phone != "+8801812345678" {
		t.Errorf("updated = %+v", updated)
	}

	if rec := r.do(http.MethodDelete, "/v1/merchants/me", ""); rec.Code != http.StatusNoContent {
		t.Errorf("withdraw: status = %d, want 204", rec.Code)
	}
	if rec := r.do(http.MethodGet, "/v1/merchants/me", ""); rec.Code != http.StatusNotFound {
		t.Errorf("after withdrawal: status = %d, want 404", rec.Code)
	}
}

// ------------------------------------------------------------ requirements

// TestTheRequirementsAreServedRatherThanHardCodedInTheApp: a Flutter build that
// decided for itself which licences a pharmacy needs would be a second copy of
// a legal requirement, updated on a different schedule (2.9).
func TestTheRequirementsAreServedRatherThanHardCodedInTheApp(t *testing.T) {
	r := newRig(t)
	rec := r.do(http.MethodGet, "/v1/merchants/registration-requirements", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	var body struct {
		Types []struct {
			Type              string   `json:"type"`
			RequiredDocuments []string `json:"required_documents"`
		} `json:"types"`
	}
	decodeBody(t, rec, &body)
	if len(body.Types) != 3 {
		t.Fatalf("types = %d, want 3", len(body.Types))
	}

	required := map[string][]string{}
	for _, entry := range body.Types {
		required[entry.Type] = entry.RequiredDocuments
	}
	if len(required["pharmacy"]) != 3 || required["pharmacy"][2] != "drug_licence" {
		t.Errorf("pharmacy = %v, want a drug licence", required["pharmacy"])
	}
	if len(required["grocery"]) != 2 {
		t.Errorf("grocery = %v, want two documents", required["grocery"])
	}
}

// --------------------------------------------------------------- language

// TestTheOpenStatusFollowsTheRequestedLanguage: Bengali unless the client asks
// otherwise, because the audience is (1.4).
func TestTheOpenStatusFollowsTheRequestedLanguage(t *testing.T) {
	r := newRig(t)
	r.registerOverHTTP(t)

	read := func(query string) string {
		t.Helper()
		rec := r.do(http.MethodGet, "/v1/merchants/me"+query, "")
		var body struct {
			OpenStatus string `json:"open_status"`
		}
		decodeBody(t, rec, &body)
		return body.OpenStatus
	}

	bengali := read("")
	if read("?lang=bn") != bengali {
		t.Error("an explicit bn differs from the default")
	}
	if read("?lang=fr") != bengali {
		t.Error("an unknown language did not fall back to Bengali")
	}
	if read("?lang=en") == bengali {
		t.Error("lang=en returned the Bengali line")
	}
}
