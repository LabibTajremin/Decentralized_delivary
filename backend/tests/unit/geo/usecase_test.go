package geo

import (
	"context"
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

func newFake(t *testing.T) *fakeRepo {
	t.Helper()
	area, err := domain.NewArea("DHN", "Dhanmondi", "DHK", domain.DivisionDhaka, domain.MustCoordinate(23.7461, 90.3742))
	if err != nil {
		t.Fatalf("build area: %v", err)
	}
	return &fakeRepo{divisions: twoDivisions(t), area: area}
}

func TestResolveAreaSucceeds(t *testing.T) {
	repo := newFake(t)
	uc := application.NewResolveAreaUseCase(repo)

	got, err := uc.Execute(context.Background(), domain.MustCoordinate(23.81, 90.41))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if got.Area.Code != "DHN" {
		t.Errorf("Area.Code = %q, want DHN", got.Area.Code)
	}
	if got.Division.Code != domain.DivisionDhaka {
		t.Errorf("Division = %v, want DHA", got.Division.Code)
	}
}

func TestResolveAreaOutsideBangladeshIsAUserError(t *testing.T) {
	repo := newFake(t)
	uc := application.NewResolveAreaUseCase(repo)

	_, err := uc.Execute(context.Background(), domain.MustCoordinate(15, 88))
	if errs.KindOf(err) != errs.KindInvalid {
		t.Errorf("Kind = %v, want KindInvalid", errs.KindOf(err))
	}
	if errs.CodeOf(err) != "outside_service_area" {
		t.Errorf("Code = %q, want outside_service_area", errs.CodeOf(err))
	}
	if errs.MessageOf(err) != "We do not deliver to this location yet." {
		t.Errorf("Message = %q, want a user-safe explanation", errs.MessageOf(err))
	}
}

func TestResolveAreaSurfacesInfrastructureFailureAsUnavailable(t *testing.T) {
	repo := newFake(t)
	repo.divisionErr = errBoom
	uc := application.NewResolveAreaUseCase(repo)

	_, err := uc.Execute(context.Background(), domain.MustCoordinate(23.81, 90.41))
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("Kind = %v, want KindUnavailable", errs.KindOf(err))
	}
	if !errors.Is(err, errBoom) {
		t.Error("the cause must stay wrapped for the logs")
	}
	if errs.MessageOf(err) == errBoom.Error() {
		t.Error("the raw infrastructure message must not become the user message")
	}
}

func TestResolveAreaHandlesAreaLookupFailures(t *testing.T) {
	repo := newFake(t)
	repo.areaErr = domain.ErrUnknownDivision
	uc := application.NewResolveAreaUseCase(repo)
	if _, err := uc.Execute(context.Background(), domain.MustCoordinate(23.81, 90.41)); errs.KindOf(err) != errs.KindInvalid {
		t.Errorf("unknown area Kind = %v, want KindInvalid", errs.KindOf(err))
	}

	repo2 := newFake(t)
	repo2.areaErr = errBoom
	uc2 := application.NewResolveAreaUseCase(repo2)
	if _, err := uc2.Execute(context.Background(), domain.MustCoordinate(23.81, 90.41)); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("area failure Kind = %v, want KindUnavailable", errs.KindOf(err))
	}
}

// TestSearchPassesDivisionIntoTheQuery is the D3 guard: the division ceiling
// must reach the query predicate, not be applied to the results afterwards.
func TestSearchPassesDivisionIntoTheQuery(t *testing.T) {
	repo := newFake(t)
	uc := application.NewMerchantsWithinRadiusUseCase(repo)

	_, err := uc.Execute(context.Background(), application.RadiusQuery{
		Centre: domain.MustCoordinate(23.81, 90.41),
		Radius: domain.KilometresFrom(5),
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if repo.lastDivision != domain.DivisionDhaka {
		t.Errorf("division passed to the query = %q, want DHA", repo.lastDivision)
	}
	if repo.lastRadius != domain.KilometresFrom(5) {
		t.Errorf("radius passed to the query = %v, want 5 km", repo.lastRadius)
	}
	if repo.lastLimit != 10 {
		t.Errorf("limit passed to the query = %d, want 10", repo.lastLimit)
	}
}

func TestSearchReturnsResults(t *testing.T) {
	repo := newFake(t)
	repo.merchants = []ports.MerchantLocation{
		{MerchantID: "MER-1", Location: domain.MustCoordinate(23.80, 90.40), Distance: domain.KilometresFrom(1.2)},
		{MerchantID: "MER-2", Location: domain.MustCoordinate(23.79, 90.42), Distance: domain.KilometresFrom(2.4)},
	}
	uc := application.NewMerchantsWithinRadiusUseCase(repo)

	got, err := uc.Execute(context.Background(), application.RadiusQuery{
		Centre: domain.MustCoordinate(23.81, 90.41),
		Radius: domain.KilometresFrom(5),
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(got) != 2 || got[0].MerchantID != "MER-1" {
		t.Errorf("results = %+v, want both merchants nearest first", got)
	}
}

func TestSearchClampsLimit(t *testing.T) {
	for _, limit := range []int{0, -1, 1000} {
		repo := newFake(t)
		uc := application.NewMerchantsWithinRadiusUseCase(repo)
		if _, err := uc.Execute(context.Background(), application.RadiusQuery{
			Centre: domain.MustCoordinate(23.81, 90.41),
			Radius: domain.KilometresFrom(5),
			Limit:  limit,
		}); err != nil {
			t.Fatalf("Execute error: %v", err)
		}
		if repo.lastLimit != 200 {
			t.Errorf("limit %d was passed through as %d, want it clamped to 200", limit, repo.lastLimit)
		}
	}
}

func TestSearchRejectsNonPositiveRadius(t *testing.T) {
	uc := application.NewMerchantsWithinRadiusUseCase(newFake(t))
	for _, r := range []domain.Distance{0, -1} {
		_, err := uc.Execute(context.Background(), application.RadiusQuery{
			Centre: domain.MustCoordinate(23.81, 90.41),
			Radius: r,
		})
		if errs.CodeOf(err) != "invalid_radius" {
			t.Errorf("radius %v: code = %q, want invalid_radius", r, errs.CodeOf(err))
		}
	}
}

func TestSearchOutsideServiceArea(t *testing.T) {
	uc := application.NewMerchantsWithinRadiusUseCase(newFake(t))
	_, err := uc.Execute(context.Background(), application.RadiusQuery{
		Centre: domain.MustCoordinate(15, 88),
		Radius: domain.KilometresFrom(5),
	})
	if errs.CodeOf(err) != "outside_service_area" {
		t.Errorf("code = %q, want outside_service_area", errs.CodeOf(err))
	}
}

func TestSearchSurfacesRepositoryFailure(t *testing.T) {
	repo := newFake(t)
	repo.searchErr = errBoom
	uc := application.NewMerchantsWithinRadiusUseCase(repo)

	_, err := uc.Execute(context.Background(), application.RadiusQuery{
		Centre: domain.MustCoordinate(23.81, 90.41),
		Radius: domain.KilometresFrom(5),
	})
	if errs.KindOf(err) != errs.KindUnavailable || errs.CodeOf(err) != "merchant_search_failed" {
		t.Errorf("error = %v (%v), want unavailable/merchant_search_failed", errs.CodeOf(err), errs.KindOf(err))
	}
}

func TestCountWithin(t *testing.T) {
	repo := newFake(t)
	repo.count = 7
	uc := application.NewMerchantsWithinRadiusUseCase(repo)

	got, err := uc.CountWithin(context.Background(), domain.MustCoordinate(23.81, 90.41), domain.KilometresFrom(5))
	if err != nil {
		t.Fatalf("CountWithin error: %v", err)
	}
	if got != 7 {
		t.Errorf("count = %d, want 7", got)
	}
	if repo.lastDivision != domain.DivisionDhaka {
		t.Errorf("count query division = %q, want DHA", repo.lastDivision)
	}
}

func TestCountWithinFailureModes(t *testing.T) {
	uc := application.NewMerchantsWithinRadiusUseCase(newFake(t))
	if _, err := uc.CountWithin(context.Background(), domain.MustCoordinate(23.81, 90.41), 0); errs.CodeOf(err) != "invalid_radius" {
		t.Errorf("zero radius code = %q, want invalid_radius", errs.CodeOf(err))
	}
	if _, err := uc.CountWithin(context.Background(), domain.MustCoordinate(15, 88), domain.KilometresFrom(5)); errs.CodeOf(err) != "outside_service_area" {
		t.Errorf("outside area code = %q, want outside_service_area", errs.CodeOf(err))
	}

	repo := newFake(t)
	repo.countErr = errBoom
	uc2 := application.NewMerchantsWithinRadiusUseCase(repo)
	if _, err := uc2.CountWithin(context.Background(), domain.MustCoordinate(23.81, 90.41), domain.KilometresFrom(5)); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("repo failure Kind = %v, want KindUnavailable", errs.KindOf(err))
	}
}
