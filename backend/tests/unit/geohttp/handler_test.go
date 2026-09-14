package geohttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
	geohttp "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/transport/http"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// stubGeo is a scripted GeoContract. The handler depends on the contract, not
// on the use cases, so these tests exercise transport alone: parameter
// handling, status mapping and response shape.
type stubGeo struct {
	area      contract.Area
	merchants []contract.NearbyMerchant
	count     int
	distance  float64
	err       error

	gotPoint  contract.Point
	gotRadius float64
	gotLimit  int
	gotFrom   contract.Point
	gotTo     contract.Point
}

func (s *stubGeo) ResolveArea(_ context.Context, p contract.Point) (contract.Area, error) {
	s.gotPoint = p
	return s.area, s.err
}

func (s *stubGeo) MerchantsWithinRadius(_ context.Context, p contract.Point, radiusM float64, limit int) ([]contract.NearbyMerchant, error) {
	s.gotPoint, s.gotRadius, s.gotLimit = p, radiusM, limit
	return s.merchants, s.err
}

func (s *stubGeo) CountMerchantsWithinRadius(_ context.Context, p contract.Point, radiusM float64) (int, error) {
	s.gotPoint, s.gotRadius = p, radiusM
	return s.count, s.err
}

// The geo HTTP surface is read-only: nothing it serves places or withdraws a
// merchant. These satisfy the contract so the stub stays a whole geo module
// rather than a partial one a handler could accidentally depend on.
func (s *stubGeo) ResolveDivision(ctx context.Context, p contract.Point) (contract.Area, error) {
	return s.ResolveArea(ctx, p)
}

func (s *stubGeo) PlaceMerchant(context.Context, contract.MerchantPlacement) error {
	return s.err
}

func (s *stubGeo) RemoveMerchant(context.Context, string) error { return s.err }

func (s *stubGeo) DistanceBetween(_ context.Context, a, b contract.Point) (float64, error) {
	s.gotFrom, s.gotTo = a, b
	return s.distance, s.err
}

// call routes a request through the registered handler.
func call(geo contract.GeoContract, target string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	geohttp.NewHandler(geo).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body.String())
	}
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decode(t, rec, &body)
	return body.Error.Code
}

func TestResolveAreaReturnsTheAdministrativePlacement(t *testing.T) {
	geo := &stubGeo{area: contract.Area{
		AreaCode: "DHK-DHM", AreaName: "Dhanmondi",
		DistrictCode: "DHK", DivisionCode: "DHA", DivisionName: "Dhaka",
	}}
	rec := call(geo, "/v1/geo/resolve?lat=23.7461&lng=90.3742")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var got map[string]string
	decode(t, rec, &got)
	if got["area_code"] != "DHK-DHM" || got["division_code"] != "DHA" {
		t.Errorf("body = %v", got)
	}
	if geo.gotPoint.Lat != 23.7461 || geo.gotPoint.Lng != 90.3742 {
		t.Errorf("point passed through = %+v", geo.gotPoint)
	}
}

func TestResolveAreaRequiresBothCoordinates(t *testing.T) {
	for _, target := range []string{
		"/v1/geo/resolve",
		"/v1/geo/resolve?lat=23.7",
		"/v1/geo/resolve?lng=90.3",
	} {
		rec := call(&stubGeo{}, target)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, rec.Code)
		}
		if got := errorCode(t, rec); got != "missing_parameter" {
			t.Errorf("%s: code = %q", target, got)
		}
	}
}

// TestResolveAreaMapsADomainRefusalToItsStatus: the handler must not decide
// statuses itself; it carries the error's Kind through.
func TestResolveAreaMapsADomainRefusalToItsStatus(t *testing.T) {
	geo := &stubGeo{err: errs.New(errs.KindNotFound, "outside_service_area",
		"We do not deliver to that location yet.")}
	rec := call(geo, "/v1/geo/resolve?lat=10&lng=10")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if got := errorCode(t, rec); got != "outside_service_area" {
		t.Errorf("code = %q", got)
	}
}

func TestMerchantsWithinRadiusReturnsResultsAndTheRadiusUsed(t *testing.T) {
	geo := &stubGeo{merchants: []contract.NearbyMerchant{
		{MerchantID: "MER-1", Lat: 23.74, Lng: 90.37, DistanceM: 120.5},
		{MerchantID: "MER-2", Lat: 23.75, Lng: 90.38, DistanceM: 940.0},
	}}
	rec := call(geo, "/v1/geo/merchants?lat=23.7461&lng=90.3742&radius_m=3000&limit=10")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Merchants []struct {
			MerchantID string  `json:"merchant_id"`
			DistanceM  float64 `json:"distance_m"`
		} `json:"merchants"`
		Count   int     `json:"count"`
		RadiusM float64 `json:"radius_m"`
	}
	decode(t, rec, &got)

	if got.Count != 2 || got.RadiusM != 3000 {
		t.Errorf("count = %d radius = %v", got.Count, got.RadiusM)
	}
	if got.Merchants[0].MerchantID != "MER-1" || got.Merchants[0].DistanceM != 120.5 {
		t.Errorf("first result = %+v", got.Merchants[0])
	}
	if geo.gotRadius != 3000 || geo.gotLimit != 10 {
		t.Errorf("radius = %v limit = %d", geo.gotRadius, geo.gotLimit)
	}
}

// TestMerchantsWithinRadiusReturnsAnEmptyListNotNull: a client that must
// special-case null before iterating is a client that will forget to.
func TestMerchantsWithinRadiusReturnsAnEmptyListNotNull(t *testing.T) {
	rec := call(&stubGeo{merchants: nil}, "/v1/geo/merchants?lat=23.7&lng=90.3&radius_m=1000")

	var raw map[string]json.RawMessage
	decode(t, rec, &raw)
	if string(raw["merchants"]) != "[]" {
		t.Errorf("merchants = %s, want []", raw["merchants"])
	}
}

func TestMerchantsWithinRadiusDefaultsTheLimit(t *testing.T) {
	geo := &stubGeo{}
	call(geo, "/v1/geo/merchants?lat=23.7&lng=90.3&radius_m=1000")
	if geo.gotLimit != 20 {
		t.Errorf("limit = %d, want the default 20", geo.gotLimit)
	}
}

func TestMerchantsWithinRadiusClampsAnExcessiveLimit(t *testing.T) {
	geo := &stubGeo{}
	call(geo, "/v1/geo/merchants?lat=23.7&lng=90.3&radius_m=1000&limit=100000")
	if geo.gotLimit != 100 {
		t.Errorf("limit = %d, want it clamped to 100", geo.gotLimit)
	}
}

// TestMerchantsWithinRadiusRejectsAnUnreasonableRadius stops one request
// scanning the country. The real per-area limit comes from config; this is only
// the outer bound.
func TestMerchantsWithinRadiusRejectsAnUnreasonableRadius(t *testing.T) {
	for _, radius := range []string{"0", "-500", "100000"} {
		rec := call(&stubGeo{}, "/v1/geo/merchants?lat=23.7&lng=90.3&radius_m="+radius)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("radius %s: status = %d, want 400", radius, rec.Code)
		}
		if got := errorCode(t, rec); got != "invalid_radius" {
			t.Errorf("radius %s: code = %q", radius, got)
		}
	}
}

func TestMerchantsWithinRadiusRequiresARadius(t *testing.T) {
	rec := call(&stubGeo{}, "/v1/geo/merchants?lat=23.7&lng=90.3")
	if got := errorCode(t, rec); got != "missing_parameter" {
		t.Errorf("code = %q", got)
	}
}

func TestMerchantsWithinRadiusRequiresCoordinates(t *testing.T) {
	rec := call(&stubGeo{}, "/v1/geo/merchants?radius_m=1000")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestMerchantsWithinRadiusRejectsABadLimit(t *testing.T) {
	rec := call(&stubGeo{}, "/v1/geo/merchants?lat=23.7&lng=90.3&radius_m=1000&limit=0")
	if got := errorCode(t, rec); got != "invalid_parameter" {
		t.Errorf("code = %q", got)
	}
}

func TestMerchantsWithinRadiusSurfacesARepositoryFailure(t *testing.T) {
	geo := &stubGeo{err: errs.New(errs.KindUnavailable, "geo_unavailable", "Try again shortly.")}
	rec := call(geo, "/v1/geo/merchants?lat=23.7&lng=90.3&radius_m=1000")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestDistanceReturnsTheServersNumber(t *testing.T) {
	geo := &stubGeo{distance: 190_700}
	rec := call(geo, "/v1/geo/distance?from_lat=23.81&from_lng=90.41&to_lat=24.89&to_lng=91.87")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		DistanceM float64 `json:"distance_m"`
	}
	decode(t, rec, &got)
	if got.DistanceM != 190_700 {
		t.Errorf("distance = %v", got.DistanceM)
	}
	if geo.gotFrom.Lat != 23.81 || geo.gotTo.Lng != 91.87 {
		t.Errorf("points = %+v -> %+v", geo.gotFrom, geo.gotTo)
	}
}

func TestDistanceRequiresBothPoints(t *testing.T) {
	for _, target := range []string{
		"/v1/geo/distance?to_lat=24.89&to_lng=91.87",
		"/v1/geo/distance?from_lat=23.81&from_lng=90.41",
		"/v1/geo/distance?from_lat=23.81&from_lng=90.41&to_lat=24.89",
	} {
		rec := call(&stubGeo{}, target)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, rec.Code)
		}
	}
}

func TestDistanceRejectsAnInvalidCoordinate(t *testing.T) {
	geo := &stubGeo{err: errs.New(errs.KindInvalid, "invalid_coordinate", "That location does not look valid.")}
	rec := call(geo, "/v1/geo/distance?from_lat=999&from_lng=90.41&to_lat=24.89&to_lng=91.87")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// A route the module does not serve must 404 rather than fall through to
// another module's handler.
func TestUnknownGeoRouteIs404(t *testing.T) {
	rec := call(&stubGeo{}, "/v1/geo/nope")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
