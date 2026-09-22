package discovery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/geo"
	discohttp "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/transport/http"
	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
	pricingapp "github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/application"
)

// server assembles the handler over fakes and returns it with the fakes.
func server(t *testing.T, settings cfgcontract.Settings) (*http.ServeMux, *fakeGeo, *fakeMerchant) {
	t.Helper()
	g := &fakeGeo{area: dhaka()}
	m := &fakeMerchant{one: shop("m1", "Star Kabab", merchantcontract.TypeRestaurant, true)}
	c := &fakeConfig{settings: settings}

	mux := http.NewServeMux()
	discohttp.NewHandler(
		application.NewSearchUseCase(g, m, c, pricingapp.NewService(c)),
		application.NewReachUseCase(g, m, c),
	).Register(mux)
	return mux, g, m
}

func get(t *testing.T, mux *http.ServeMux, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestSearchEndpoint(t *testing.T) {
	mux, g, m := server(t, defaultSettings())
	g.nearby = []geo.NearbyMerchant{{MerchantID: "m1", DistanceM: 400}}
	m.listed = []merchantcontract.Merchant{shop("m1", "Star Kabab", merchantcontract.TypeRestaurant, true)}

	rec := get(t, mux, "/v1/discovery/merchants?lat=23.7465&lng=90.3760&lang=en&limit=5")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}

	var body struct {
		Division  string `json:"division"`
		Notice    string `json:"notice"`
		Total     int    `json:"total"`
		Expansion struct {
			Level      int     `json:"level"`
			RadiusM    float64 `json:"radius_m"`
			Radius     string  `json:"radius"`
			Stage      string  `json:"stage"`
			CanExpand  bool    `json:"can_expand"`
			Offered    bool    `json:"offered"`
			AtCeiling  bool    `json:"at_ceiling"`
			NextRadius string  `json:"next_radius"`
		} `json:"expansion"`
		Merchants []struct {
			ID       string `json:"id"`
			Distance string `json:"distance"`
			Delivery struct {
				Minor    int64  `json:"minor"`
				Display  string `json:"display"`
				Expanded bool   `json:"expanded"`
			} `json:"delivery"`
		} `json:"merchants"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Division != "Dhaka" || body.Total != 1 || len(body.Merchants) != 1 {
		t.Fatalf("body = %+v", body)
	}
	if body.Merchants[0].Distance != "400 m" || body.Merchants[0].Delivery.Display == "" {
		t.Errorf("the card is not fully composed: %+v", body.Merchants[0])
	}
	if body.Expansion.Stage != "base" || !body.Expansion.CanExpand || body.Expansion.NextRadius != "10 km" {
		t.Errorf("expansion = %+v", body.Expansion)
	}
	if body.Notice == "" {
		t.Error("no notice")
	}
}

// The terminal state on the wire. A client branches its layout on `stage` and
// `atCeiling`, so both have to be present and both have to be right.
func TestSearchEndpointAtTheCeiling(t *testing.T) {
	mux, _, _ := server(t, defaultSettings())
	rec := get(t, mux, "/v1/discovery/merchants?lat=23.7465&lng=90.3760&level=9&lang=en")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Expansion struct {
			Level       int     `json:"level"`
			Stage       string  `json:"stage"`
			CanExpand   bool    `json:"can_expand"`
			AtCeiling   bool    `json:"at_ceiling"`
			NextRadiusM float64 `json:"next_radius_m"`
		} `json:"expansion"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Expansion.Level != 4 || body.Expansion.Stage != "ceiling" ||
		body.Expansion.CanExpand || body.Expansion.AtCeiling != true || body.Expansion.NextRadiusM != 0 {
		t.Fatalf("expansion = %+v", body.Expansion)
	}
}

func TestReachEndpoint(t *testing.T) {
	mux, g, _ := server(t, defaultSettings())
	g.distance = 8200

	rec := get(t, mux, "/v1/discovery/merchants/m1?lat=23.7465&lng=90.3760&lang=en")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var body struct {
		MerchantID    string `json:"merchant_id"`
		Distance      string `json:"distance"`
		Reachable     bool   `json:"reachable"`
		RequiredLevel int    `json:"required_level"`
		Expanded      bool   `json:"expanded"`
		Reason        string `json:"reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.MerchantID != "m1" || !body.Reachable || body.RequiredLevel != 1 ||
		!body.Expanded || body.Distance != "8.2 km" || body.Reason != "" {
		t.Fatalf("body = %+v", body)
	}
}

func TestEndpointsRejectBadInput(t *testing.T) {
	mux, _, _ := server(t, defaultSettings())
	cases := []struct {
		name, target string
		want         int
	}{
		{"no lat", "/v1/discovery/merchants?lng=90.3", http.StatusBadRequest},
		{"no lng", "/v1/discovery/merchants?lat=23.7", http.StatusBadRequest},
		{"lat is not a number", "/v1/discovery/merchants?lat=north&lng=90.3", http.StatusBadRequest},
		{"level is not a number", "/v1/discovery/merchants?lat=23.7&lng=90.3&level=wide", http.StatusBadRequest},
		// A limit above the maximum is clamped rather than refused — a client
		// asking for 500 means "as many as you will give me" — but a limit
		// below the minimum is a caller bug and is refused.
		{"limit below the minimum", "/v1/discovery/merchants?lat=23.7&lng=90.3&limit=0", http.StatusBadRequest},
		{"offset is not a number", "/v1/discovery/merchants?lat=23.7&lng=90.3&offset=x", http.StatusBadRequest},
		{"an unknown shop type", "/v1/discovery/merchants?lat=23.7&lng=90.3&type=hardware", http.StatusBadRequest},
		{"reach with no lat", "/v1/discovery/merchants/m1?lng=90.3", http.StatusBadRequest},
		{"reach with no lng", "/v1/discovery/merchants/m1?lat=23.7", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := get(t, mux, tc.target).Code; got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEndpointsReportDownstreamFailures(t *testing.T) {
	mux, g, _ := server(t, defaultSettings())
	g.nearErr = errBoom
	if got := get(t, mux, "/v1/discovery/merchants?lat=23.7&lng=90.3").Code; got != http.StatusInternalServerError {
		t.Errorf("a failed search returned %d", got)
	}

	mux, g, _ = server(t, defaultSettings())
	g.distErr = errBoom
	if got := get(t, mux, "/v1/discovery/merchants/m1?lat=23.7&lng=90.3").Code; got != http.StatusInternalServerError {
		t.Errorf("a failed reach check returned %d", got)
	}
}

// Patterns is what the OpenAPI drift check reads. It has to be the same list
// Register mounts, not a second copy kept by hand.
func TestPatternsMatchWhatIsMounted(t *testing.T) {
	patterns := discohttp.Patterns()
	if len(patterns) != 2 {
		t.Fatalf("Patterns() = %v", patterns)
	}
	mux, _, _ := server(t, defaultSettings())
	for _, pattern := range patterns {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Fatalf("pattern %q is not %q", pattern, "METHOD /path")
		}
		// A mounted route answers something other than 404; an unmounted one
		// cannot. The path variable is filled in so the mux matches.
		target := strings.ReplaceAll(path, "{merchantId}", "m1") + "?lat=23.7&lng=90.3"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
		if rec.Code == http.StatusNotFound {
			t.Errorf("%s is in Patterns() but nothing serves it", pattern)
		}
	}
}
