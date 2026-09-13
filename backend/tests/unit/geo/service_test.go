package geo

import (
	"context"
	"math"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// newService builds the public service over a fake repository, and returns
// both so a test can assert what reached the query.
func newService(t *testing.T) (*application.Service, *fakeRepo) {
	t.Helper()
	repo := newFake(t)
	return application.NewService(
		application.NewResolveAreaUseCase(repo),
		application.NewMerchantsWithinRadiusUseCase(repo),
	), repo
}

// TestServiceSatisfiesTheContract fails to compile if the public service ever
// stops implementing what other modules depend on.
func TestServiceSatisfiesTheContract(t *testing.T) {
	var _ contract.GeoContract = (*application.Service)(nil)
}

func TestServiceResolveArea(t *testing.T) {
	svc, _ := newService(t)

	got, err := svc.ResolveArea(context.Background(), contract.Point{Lat: 23.81, Lng: 90.41})
	if err != nil {
		t.Fatalf("ResolveArea error: %v", err)
	}
	want := contract.Area{
		AreaCode: "DHN", AreaName: "Dhanmondi",
		DistrictCode: "DHK", DivisionCode: "DHA", DivisionName: "Dhaka",
	}
	if got != want {
		t.Errorf("ResolveArea = %+v, want %+v", got, want)
	}
}

func TestServiceRejectsInvalidCoordinates(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	bad := contract.Point{Lat: 999, Lng: 0}

	if _, err := svc.ResolveArea(ctx, bad); errs.CodeOf(err) != "invalid_coordinate" {
		t.Errorf("ResolveArea code = %q, want invalid_coordinate", errs.CodeOf(err))
	}
	if _, err := svc.MerchantsWithinRadius(ctx, bad, 5000, 10); errs.CodeOf(err) != "invalid_coordinate" {
		t.Errorf("MerchantsWithinRadius code = %q, want invalid_coordinate", errs.CodeOf(err))
	}
	if _, err := svc.CountMerchantsWithinRadius(ctx, bad, 5000); errs.CodeOf(err) != "invalid_coordinate" {
		t.Errorf("CountMerchantsWithinRadius code = %q, want invalid_coordinate", errs.CodeOf(err))
	}
	good := contract.Point{Lat: 23.81, Lng: 90.41}
	if _, err := svc.DistanceBetween(ctx, bad, good); errs.CodeOf(err) != "invalid_coordinate" {
		t.Errorf("DistanceBetween(bad, good) code = %q, want invalid_coordinate", errs.CodeOf(err))
	}
	if _, err := svc.DistanceBetween(ctx, good, bad); errs.CodeOf(err) != "invalid_coordinate" {
		t.Errorf("DistanceBetween(good, bad) code = %q, want invalid_coordinate", errs.CodeOf(err))
	}
}

func TestServiceMerchantsWithinRadiusMapsToPrimitives(t *testing.T) {
	svc, repo := newService(t)
	repo.merchants = []ports.MerchantLocation{
		{MerchantID: "MER-1", Location: domain.MustCoordinate(23.80, 90.40), Distance: domain.KilometresFrom(1.2)},
	}

	got, err := svc.MerchantsWithinRadius(context.Background(), contract.Point{Lat: 23.81, Lng: 90.41}, 5000, 10)
	if err != nil {
		t.Fatalf("MerchantsWithinRadius error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("results = %d, want 1", len(got))
	}
	want := contract.NearbyMerchant{MerchantID: "MER-1", Lat: 23.80, Lng: 90.40, DistanceM: 1200}
	if got[0] != want {
		t.Errorf("result = %+v, want %+v", got[0], want)
	}
}

func TestServiceMerchantsWithinRadiusEmptyResult(t *testing.T) {
	svc, _ := newService(t)
	got, err := svc.MerchantsWithinRadius(context.Background(), contract.Point{Lat: 23.81, Lng: 90.41}, 5000, 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("results = %v, want empty", got)
	}
}

func TestServiceSurfacesUseCaseErrors(t *testing.T) {
	svc, repo := newService(t)
	repo.searchErr = errBoom
	if _, err := svc.MerchantsWithinRadius(context.Background(), contract.Point{Lat: 23.81, Lng: 90.41}, 5000, 10); err == nil {
		t.Error("a repository failure must reach the caller")
	}

	svc2, repo2 := newService(t)
	repo2.divisionErr = errBoom
	if _, err := svc2.ResolveArea(context.Background(), contract.Point{Lat: 23.81, Lng: 90.41}); err == nil {
		t.Error("a division lookup failure must reach the caller")
	}
}

func TestServiceCountMerchantsWithinRadius(t *testing.T) {
	svc, repo := newService(t)
	repo.count = 4

	got, err := svc.CountMerchantsWithinRadius(context.Background(), contract.Point{Lat: 23.81, Lng: 90.41}, 5000)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != 4 {
		t.Errorf("count = %d, want 4", got)
	}
}

// TestServiceDistanceBetweenIsTheOneSourceOfTruth guards 2.9: pricing and
// dispatch ask geo rather than computing distance themselves, so every part of
// the system quotes the same number.
func TestServiceDistanceBetweenIsTheOneSourceOfTruth(t *testing.T) {
	svc, _ := newService(t)
	dhaka := contract.Point{Lat: 23.8103, Lng: 90.4125}
	chattogram := contract.Point{Lat: 22.3569, Lng: 91.7832}

	got, err := svc.DistanceBetween(context.Background(), dhaka, chattogram)
	if err != nil {
		t.Fatalf("DistanceBetween error: %v", err)
	}
	if math.Abs(got/1000-214) > 4 {
		t.Errorf("distance = %.1f km, want ~214 km", got/1000)
	}
}
