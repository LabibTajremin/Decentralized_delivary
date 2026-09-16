package dispatch

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application/ports"
	dispatchhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/transport/http"
)

// server mounts the handler over a rig. The guards pass everything through —
// what is under test is what the handler does with a caller, not the middleware
// that identifies one.
func server(r *rig, userID string) *http.ServeMux {
	mux := http.NewServeMux()
	open := func(next http.Handler) http.Handler { return next }
	dispatchhttp.NewHandler(
		r.partners, r.offers, open, open,
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

type partnerResponse struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Availability      string  `json:"availability"`
	AvailabilityLabel string  `json:"availability_label"`
	Preference        string  `json:"preference"`
	PreferenceLabel   string  `json:"preference_label"`
	Lat               float64 `json:"lat"`
	Lng               float64 `json:"lng"`
	Carrying          int     `json:"carrying"`
	AcceptancePercent int     `json:"acceptance_percent"`
}

type jobResponse struct {
	ID          string  `json:"id"`
	OrderID     string  `json:"order_id"`
	Code        string  `json:"code"`
	Status      string  `json:"status"`
	StatusLabel string  `json:"status_label"`
	Live        bool    `json:"live"`
	Band        string  `json:"band"`
	BandLabel   string  `json:"band_label"`
	Distance    string  `json:"distance"`
	ToPickup    string  `json:"to_pickup"`
	SecondsLeft int     `json:"seconds_left"`
	Reason      string  `json:"reason"`
	DistanceM   float64 `json:"distance_m"`
}

// The whole rider journey over HTTP.
func TestARiderWorksOverHTTP(t *testing.T) {
	r := newRig()
	mux := server(r, "USR-1")

	var partner partnerResponse
	if status := call(t, mux, http.MethodPost, "/v1/partner",
		`{"name":"Rafi","phone":"+8801711111111","vehicle":"motorcycle"}`, &partner); status != http.StatusCreated {
		t.Fatalf("register: status = %d", status)
	}
	if partner.Availability != "offline" || partner.AcceptancePercent != 100 {
		t.Fatalf("partner = %+v", partner)
	}

	var me partnerResponse
	if status := call(t, mux, http.MethodGet, "/v1/partner", "", &me); status != http.StatusOK {
		t.Fatalf("me: status = %d", status)
	}
	if me.ID != partner.ID {
		t.Fatalf("me = %+v", me)
	}

	var online partnerResponse
	if status := call(t, mux, http.MethodPut, "/v1/partner/availability",
		`{"availability":"available"}`, &online); status != http.StatusOK {
		t.Fatalf("availability: status = %d", status)
	}
	if online.Availability != "available" || online.AvailabilityLabel == "" {
		t.Fatalf("partner = %+v", online)
	}

	var preference partnerResponse
	if status := call(t, mux, http.MethodPut, "/v1/partner/preference",
		`{"preference":"short"}`, &preference); status != http.StatusOK {
		t.Fatalf("preference: status = %d", status)
	}
	if preference.Preference != "short" {
		t.Fatalf("partner = %+v", preference)
	}

	var located partnerResponse
	if status := call(t, mux, http.MethodPut, "/v1/partner/location",
		`{"lat":23.746,"lng":90.375}`, &located); status != http.StatusOK {
		t.Fatalf("location: status = %d", status)
	}
	if located.Lat != 23.746 {
		t.Fatalf("partner = %+v", located)
	}

	// A job on the board, offered to this rider.
	r.repo.nearby = []ports.PartnerDistance{{Partner: r.repo.partners[partner.ID], DistanceM: 200}}
	job, err := r.service.Offer(t.Context(), offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}

	var accepted jobResponse
	if status := call(t, mux, http.MethodPost, "/v1/partner/jobs/"+job.ID+"/accept?lang=en", "", &accepted); status != http.StatusOK {
		t.Fatalf("accept: status = %d", status)
	}
	if accepted.Status != "assigned" || accepted.StatusLabel != "Collect from the shop" {
		t.Fatalf("job = %+v", accepted)
	}
	if accepted.Code != "ABC234" || accepted.BandLabel == "" || accepted.Distance == "" {
		t.Errorf("job = %+v", accepted)
	}

	var list struct {
		Jobs  []jobResponse `json:"jobs"`
		Total int           `json:"total"`
	}
	if status := call(t, mux, http.MethodGet, "/v1/partner/jobs?live=true", "", &list); status != http.StatusOK {
		t.Fatalf("jobs: status = %d", status)
	}
	if list.Total != 1 {
		t.Fatalf("jobs = %+v", list)
	}

	for _, step := range []struct{ path, want string }{
		{"/collect", "collected"},
		{"/deliver", "delivered"},
	} {
		var got jobResponse
		if status := call(t, mux, http.MethodPost, "/v1/partner/jobs/"+job.ID+step.path, "", &got); status != http.StatusOK {
			t.Fatalf("%s: status = %d", step.path, status)
		}
		if got.Status != step.want {
			t.Fatalf("%s = %q", step.path, got.Status)
		}
	}
}

func TestTheFeedOverHTTP(t *testing.T) {
	r := newRig()
	mux := server(r, "USR-1")
	r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")

	job, err := r.service.Offer(t.Context(), offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	r.repo.waiting = []ports.JobDistance{{Job: r.repo.jobs[job.ID], DistanceM: 400}}

	var feed struct {
		Partner partnerResponse `json:"partner"`
		Jobs    []jobResponse   `json:"jobs"`
		RadiusM float64         `json:"radius_m"`
		Reason  string          `json:"reason"`
		Notice  string          `json:"notice"`
	}
	if status := call(t, mux, http.MethodGet, "/v1/partner/feed?lang=en", "", &feed); status != http.StatusOK {
		t.Fatalf("feed: status = %d", status)
	}
	if len(feed.Jobs) != 1 || feed.RadiusM != 7000 {
		t.Fatalf("feed = %+v", feed)
	}
	if feed.Jobs[0].ToPickup == "" {
		t.Errorf("the entry does not say how far the pickup is: %+v", feed.Jobs[0])
	}

	// Going offline changes the answer, with a sentence.
	if status := call(t, mux, http.MethodPut, "/v1/partner/availability",
		`{"availability":"offline"}`, nil); status != http.StatusOK {
		t.Fatalf("offline: status = %d", status)
	}
	var empty struct {
		Jobs   []jobResponse `json:"jobs"`
		Reason string        `json:"reason"`
		Notice string        `json:"notice"`
	}
	if status := call(t, mux, http.MethodGet, "/v1/partner/feed?lang=en", "", &empty); status != http.StatusOK {
		t.Fatalf("feed: status = %d", status)
	}
	if empty.Reason != "offline" || empty.Notice == "" || len(empty.Jobs) != 0 {
		t.Fatalf("feed = %+v", empty)
	}
}

func TestDeclineAndFailOverHTTP(t *testing.T) {
	r := newRig()
	rafi := server(r, "USR-1")
	nadia := server(r, "USR-2")
	first := r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")
	second := r.onShift(t, "USR-2", "Nadia", 23.747, 90.376, "")
	r.repo.nearby = []ports.PartnerDistance{
		{Partner: r.repo.partners[first.ID], DistanceM: 200},
		{Partner: r.repo.partners[second.ID], DistanceM: 400},
	}

	job, err := r.service.Offer(t.Context(), offerRequest("ORD-1", 3000))
	if err != nil {
		t.Fatalf("Offer: %v", err)
	}
	// Rafi is nearer, so the first round is his.
	if job.Partner.ID != first.ID {
		t.Fatalf("job = %+v, want it offered to %s", job, first.ID)
	}
	var declined jobResponse
	if status := call(t, rafi, http.MethodPost, "/v1/partner/jobs/"+job.ID+"/decline", "", &declined); status != http.StatusOK {
		t.Fatalf("decline: status = %d", status)
	}
	if declined.Status != "waiting" {
		t.Fatalf("job = %+v", declined)
	}

	// The sweep is what puts it back to somebody — and not back to Rafi, who
	// has already said no.
	var sweep struct {
		Expired int `json:"expired"`
		Offered int `json:"offered"`
	}
	if status := call(t, server(r, "USR-ADMIN"), http.MethodPost, "/v1/admin/dispatch/sweep", "", &sweep); status != http.StatusOK {
		t.Fatalf("sweep: status = %d", status)
	}
	if sweep.Offered != 1 {
		t.Fatalf("sweep = %+v, want one job re-offered", sweep)
	}
	if held := r.repo.jobs[job.ID]; held.PartnerID != second.ID {
		t.Fatalf("the declined job went back to %q, want %q", held.PartnerID, second.ID)
	}

	// Nadia takes it, collects the food, and then cannot complete it — which
	// needs a reason.
	for _, step := range []string{"/accept", "/collect"} {
		if status := call(t, nadia, http.MethodPost, "/v1/partner/jobs/"+job.ID+step, "", nil); status != http.StatusOK {
			t.Fatalf("%s: status = %d", step, status)
		}
	}
	if status := call(t, nadia, http.MethodPost, "/v1/partner/jobs/"+job.ID+"/fail", `{}`, nil); status != http.StatusBadRequest {
		t.Fatalf("a reasonless failure returned %d, want 400", status)
	}
	var failed jobResponse
	if status := call(t, nadia, http.MethodPost, "/v1/partner/jobs/"+job.ID+"/fail",
		`{"reason":"nobody at the address"}`, &failed); status != http.StatusOK {
		t.Fatalf("fail: status = %d", status)
	}
	if failed.Status != "failed" || failed.Reason != "nobody at the address" {
		t.Fatalf("job = %+v", failed)
	}
}

func TestTheSweepEndpoint(t *testing.T) {
	r := newRig()
	mux := server(r, "USR-ADMIN")

	var body struct {
		Expired int `json:"expired"`
		Offered int `json:"offered"`
	}
	if status := call(t, mux, http.MethodPost, "/v1/admin/dispatch/sweep?limit=10", "", &body); status != http.StatusOK {
		t.Fatalf("sweep: status = %d", status)
	}
	if body.Expired != 0 || body.Offered != 0 {
		t.Errorf("an empty board swept %+v", body)
	}
	if status := call(t, mux, http.MethodPost, "/v1/admin/dispatch/sweep?limit=0", "", nil); status != http.StatusBadRequest {
		t.Errorf("a bad limit returned %d", status)
	}
}

func TestEveryRouteRefusesAnAnonymousCaller(t *testing.T) {
	mux := server(newRig(), "")
	for _, pattern := range dispatchhttp.Patterns() {
		method, path, _ := strings.Cut(pattern, " ")
		if strings.Contains(path, "/admin/") {
			continue // the admin guard is identity's, not this handler's
		}
		target := strings.ReplaceAll(path, "{jobId}", "JOB-1")
		if status := call(t, mux, method, target, "{}", nil); status != http.StatusUnauthorized {
			t.Errorf("%s returned %d, want 401", pattern, status)
		}
	}
}

// A registration the domain refuses reaches the client as a 400 with something
// to fix, not a 500.
func TestRegisteringWithoutANameOverHTTP(t *testing.T) {
	r := newRig()
	mux := server(r, "USR-1")
	if status := call(t, mux, http.MethodPost, "/v1/partner", `{"phone":"01711111111"}`, nil); status != http.StatusBadRequest {
		t.Fatalf("a nameless registration returned %d, want 400", status)
	}
}

func TestMalformedRequestsAreRefused(t *testing.T) {
	r := newRig()
	mux := server(r, "USR-1")
	r.onShift(t, "USR-1", "Rafi", 23.746, 90.375, "")

	cases := []struct {
		name, method, target, body string
	}{
		{"register", http.MethodPost, "/v1/partner", `{"name":`},
		{"availability", http.MethodPut, "/v1/partner/availability", `not json`},
		{"preference", http.MethodPut, "/v1/partner/preference", `[]`},
		{"location", http.MethodPut, "/v1/partner/location", `{`},
		{"fail", http.MethodPost, "/v1/partner/jobs/JOB-1/fail", `{`},
		{"a bad limit", http.MethodGet, "/v1/partner/jobs?limit=0", ""},
		{"a bad offset", http.MethodGet, "/v1/partner/jobs?offset=x", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status := call(t, mux, tc.method, tc.target, tc.body, nil); status != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", status)
			}
		})
	}
}

func TestDownstreamFailuresReachTheClient(t *testing.T) {
	r := newRig()
	r.repo.partnerErr = errBoom
	mux := server(r, "USR-1")

	cases := []struct{ name, method, target string }{
		{"me", http.MethodGet, "/v1/partner"},
		{"feed", http.MethodGet, "/v1/partner/feed"},
		{"jobs", http.MethodGet, "/v1/partner/jobs"},
		{"accept", http.MethodPost, "/v1/partner/jobs/JOB-1/accept"},
		{"decline", http.MethodPost, "/v1/partner/jobs/JOB-1/decline"},
		{"collect", http.MethodPost, "/v1/partner/jobs/JOB-1/collect"},
		{"deliver", http.MethodPost, "/v1/partner/jobs/JOB-1/deliver"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status := call(t, mux, tc.method, tc.target, "", nil); status != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", status)
			}
		})
	}

	sweeping := newRig()
	sweeping.repo.lapsedErr = errBoom
	if status := call(t, server(sweeping, "USR-ADMIN"), http.MethodPost, "/v1/admin/dispatch/sweep", "", nil); status != http.StatusServiceUnavailable {
		t.Errorf("sweep returned %d, want 503", status)
	}
}

func TestNewHandlerRefusesToRunWithoutItsGuards(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a handler with no guards was built")
		}
	}()
	dispatchhttp.NewHandler(nil, nil, nil, nil, nil)
}

func TestPatternsMatchWhatIsMounted(t *testing.T) {
	patterns := dispatchhttp.Patterns()
	if len(patterns) != 13 {
		t.Fatalf("Patterns() = %v", patterns)
	}
	mux := server(newRig(), "USR-1")
	for _, pattern := range patterns {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Fatalf("pattern %q is not %q", pattern, "METHOD /path")
		}
		target := strings.ReplaceAll(path, "{jobId}", "JOB-1")
		_, matched := mux.Handler(httptest.NewRequest(method, target, bytes.NewBufferString("{}")))
		if matched != pattern {
			t.Errorf("%s is in Patterns() but the mux resolved %q", pattern, matched)
		}
	}
}
