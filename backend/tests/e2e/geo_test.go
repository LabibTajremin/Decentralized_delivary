package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

// These exercise the geo endpoints through the real binary against a real
// PostGIS, so the wiring in cmd/api — router, middleware, pool, repository — is
// proven end to end rather than assumed. The unit tests cover the handler's
// parameter handling; this covers everything underneath it.

// startAPIWithDB runs the binary pointed at the real database and seeds it, so
// the assertions below describe data that is actually there.
func startAPIWithDB(t *testing.T) (string, func()) {
	t.Helper()
	// Its own schema, for the same reason the integration suite has one: these
	// binaries run concurrently and both reshape the database.
	url := dbtestSchemaURL(t)
	seedDemoData(t, url)
	return startAPI(t, buildAPI(t), "DATABASE_URL="+url)
}

func getJSON(t *testing.T, url string, into any) int {
	t.Helper()
	return getJSONAs(t, url, "", into)
}

// getJSONAs is getJSON with a bearer token, for endpoints behind a role.
func getJSONAs(t *testing.T, url, bearer string, into any) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil) //nolint:noctx // fixed test-local URL
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if into != nil {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp.StatusCode
}

// TestResolveAreaPlacesADhakaAddress proves the whole path: HTTP in, PostGIS
// ST_Contains, domain validation, JSON out.
func TestResolveAreaPlacesADhakaAddress(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	var got struct {
		AreaCode     string `json:"area_code"`
		AreaName     string `json:"area_name"`
		DivisionCode string `json:"division_code"`
	}
	status := getJSON(t, base+"/v1/geo/resolve?lat=23.7461&lng=90.3742", &got)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if got.DivisionCode != "DHA" {
		t.Errorf("division = %q, want DHA", got.DivisionCode)
	}
	if got.AreaCode != "DHK-DHM" || got.AreaName != "Dhanmondi" {
		t.Errorf("area = %q/%q, want DHK-DHM/Dhanmondi", got.AreaCode, got.AreaName)
	}
}

// TestResolveAreaOutsideBangladeshIsNotFound: the service area has an edge, and
// a point beyond it must be refused rather than guessed at.
func TestResolveAreaOutsideBangladeshIsNotFound(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	if status := getJSON(t, base+"/v1/geo/resolve?lat=48.8566&lng=2.3522", nil); status != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a point in Paris", status)
	}
}

func TestMerchantsWithinRadiusReturnsNearestFirst(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	var got struct {
		Merchants []struct {
			MerchantID string  `json:"merchant_id"`
			DistanceM  float64 `json:"distance_m"`
		} `json:"merchants"`
		Count int `json:"count"`
	}
	status := getJSON(t, base+"/v1/geo/merchants?lat=23.7461&lng=90.3742&radius_m=5000", &got)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if got.Count == 0 {
		t.Fatal("no merchants found near Dhanmondi; the seed should place several")
	}
	for i := 1; i < len(got.Merchants); i++ {
		if got.Merchants[i].DistanceM < got.Merchants[i-1].DistanceM {
			t.Errorf("results are not nearest-first: %v", got.Merchants)
			break
		}
	}
}

// TestRadiusSearchNeverCrossesADivision is D3, the one invariant nothing may
// disable. A Dhaka customer must not see a Chattogram merchant however large
// the radius.
func TestRadiusSearchNeverCrossesADivision(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	var got struct {
		Merchants []struct {
			MerchantID string `json:"merchant_id"`
		} `json:"merchants"`
	}
	// 25 km is the transport maximum and far wider than any real search.
	status := getJSON(t, base+"/v1/geo/merchants?lat=23.7461&lng=90.3742&radius_m=25000&limit=100", &got)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	// The seed puts MER-DEMO-0011 and 0012 in Chattogram.
	for _, m := range got.Merchants {
		if m.MerchantID == "MER-DEMO-0011" || m.MerchantID == "MER-DEMO-0012" {
			t.Errorf("%s is in another division and must never appear", m.MerchantID)
		}
	}
}

// TestDistanceMatchesTheDomain: Dhaka to Sylhet is about 191 km great-circle.
// Pricing bands off this number, so it has to be the server's and it has to be
// right.
func TestDistanceBetweenTwoCities(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	var got struct {
		DistanceM float64 `json:"distance_m"`
	}
	status := getJSON(t,
		base+"/v1/geo/distance?from_lat=23.8103&from_lng=90.4125&to_lat=24.8949&to_lng=91.8687", &got)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	km := got.DistanceM / 1000
	if km < 187 || km > 195 {
		t.Errorf("Dhaka to Sylhet = %.1f km, want about 191", km)
	}
}

func TestGeoRejectsAMissingParameterOverHTTP(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	status := getJSON(t, base+"/v1/geo/resolve?lat=23.7", &body)
	if status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", status)
	}
	if body.Error.Code != "missing_parameter" {
		t.Errorf("code = %q", body.Error.Code)
	}
}

// TestEveryResponseCarriesARequestID proves the middleware chain is actually
// mounted, not merely written.
func TestEveryResponseCarriesARequestIDAndSecurityHeaders(t *testing.T) {
	base, stop := startAPIWithDB(t)
	defer stop()

	resp, err := http.Get(base + "/healthz") //nolint:gosec,noctx // fixed test-local URL
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("no X-Request-Id: the request-id middleware is not mounted")
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("no nosniff: the security-headers middleware is not mounted")
	}
}
