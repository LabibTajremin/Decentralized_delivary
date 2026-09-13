package geo

import (
	"context"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
)

// errBoom stands in for an infrastructure failure.
var errBoom = errors.New("database unreachable")

// fakeRepo is an in-memory GeoRepository. Use cases are tested against it so
// their rules are exercised without a database (05-architecture.md 2.3).
type fakeRepo struct {
	divisions []domain.Division
	area      domain.Area
	merchants []ports.MerchantLocation
	count     int

	divisionErr error
	areaErr     error
	searchErr   error
	countErr    error

	// lastDivision records what the use case passed down, so a test can prove
	// the D3 ceiling reached the query rather than being applied afterwards.
	lastDivision domain.DivisionCode
	lastRadius   domain.Distance
	lastLimit    int
}

func (f *fakeRepo) Divisions(context.Context) ([]domain.Division, error) {
	return f.divisions, nil
}

func (f *fakeRepo) DivisionContaining(_ context.Context, c domain.Coordinate) (domain.Division, error) {
	if f.divisionErr != nil {
		return domain.Division{}, f.divisionErr
	}
	return domain.DivisionOf(f.divisions, c)
}

func (f *fakeRepo) AreaContaining(context.Context, domain.Coordinate) (domain.Area, error) {
	if f.areaErr != nil {
		return domain.Area{}, f.areaErr
	}
	return f.area, nil
}

func (f *fakeRepo) MerchantsWithinRadius(_ context.Context, _ domain.Coordinate, radius domain.Distance, division domain.DivisionCode, limit int) ([]ports.MerchantLocation, error) {
	f.lastDivision, f.lastRadius, f.lastLimit = division, radius, limit
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.merchants, nil
}

func (f *fakeRepo) CountMerchantsWithinRadius(_ context.Context, _ domain.Coordinate, radius domain.Distance, division domain.DivisionCode) (int, error) {
	f.lastDivision, f.lastRadius = division, radius
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.count, nil
}
