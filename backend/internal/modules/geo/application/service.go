package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Service implements contract.GeoContract on top of the use cases. It is the
// single adapter between geo's internal types and the primitives other modules
// see, so geo's domain never leaks across a module boundary.
type Service struct {
	resolveArea *ResolveAreaUseCase
	merchants   *MerchantsWithinRadiusUseCase
	place       *PlaceMerchantUseCase
}

// NewService wires the public service.
func NewService(resolveArea *ResolveAreaUseCase, merchants *MerchantsWithinRadiusUseCase, place *PlaceMerchantUseCase) *Service {
	return &Service{resolveArea: resolveArea, merchants: merchants, place: place}
}

// toCoordinate validates an inbound point once, at the boundary.
func toCoordinate(p contract.Point) (domain.Coordinate, error) {
	c, err := domain.NewCoordinate(p.Lat, p.Lng)
	if err != nil {
		return domain.Coordinate{}, errs.Wrap(err, errs.KindInvalid, "invalid_coordinate",
			"That location does not look valid.")
	}
	return c, nil
}

// ResolveArea places a point in the administrative hierarchy.
func (s *Service) ResolveArea(ctx context.Context, p contract.Point) (contract.Area, error) {
	c, err := toCoordinate(p)
	if err != nil {
		return contract.Area{}, err
	}
	resolved, err := s.resolveArea.Execute(ctx, c)
	if err != nil {
		return contract.Area{}, err
	}
	return contract.Area{
		AreaCode:     resolved.Area.Code,
		AreaName:     resolved.Area.Name,
		DistrictCode: resolved.Area.District,
		DivisionCode: resolved.Division.Code.String(),
		DivisionName: resolved.Division.Name,
	}, nil
}

// ResolveDivision places a point when only the division is required.
func (s *Service) ResolveDivision(ctx context.Context, p contract.Point) (contract.Area, error) {
	c, err := toCoordinate(p)
	if err != nil {
		return contract.Area{}, err
	}
	resolved, err := s.resolveArea.Division(ctx, c)
	if err != nil {
		return contract.Area{}, err
	}
	return contract.Area{
		AreaCode:     resolved.Area.Code,
		AreaName:     resolved.Area.Name,
		DistrictCode: resolved.Area.District,
		DivisionCode: resolved.Division.Code.String(),
		DivisionName: resolved.Division.Name,
	}, nil
}

// MerchantsWithinRadius returns merchants in range, nearest first.
func (s *Service) MerchantsWithinRadius(ctx context.Context, p contract.Point, radiusM float64, limit int) ([]contract.NearbyMerchant, error) {
	c, err := toCoordinate(p)
	if err != nil {
		return nil, err
	}
	found, err := s.merchants.Execute(ctx, RadiusQuery{
		Centre: c,
		Radius: domain.Distance(radiusM),
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]contract.NearbyMerchant, 0, len(found))
	for _, m := range found {
		out = append(out, contract.NearbyMerchant{
			MerchantID: m.MerchantID,
			Lat:        m.Location.Lat(),
			Lng:        m.Location.Lng(),
			DistanceM:  m.Distance.Metres(),
		})
	}
	return out, nil
}

// CountMerchantsWithinRadius returns how many merchants are in range.
func (s *Service) CountMerchantsWithinRadius(ctx context.Context, p contract.Point, radiusM float64) (int, error) {
	c, err := toCoordinate(p)
	if err != nil {
		return 0, err
	}
	return s.merchants.CountWithin(ctx, c, domain.Distance(radiusM))
}

// DistanceBetween returns the great-circle distance in metres.
func (s *Service) DistanceBetween(_ context.Context, a, b contract.Point) (float64, error) {
	ca, err := toCoordinate(a)
	if err != nil {
		return 0, err
	}
	cb, err := toCoordinate(b)
	if err != nil {
		return 0, err
	}
	return ca.DistanceTo(cb).Metres(), nil
}

// PlaceMerchant records a merchant's location and whether it is searchable.
func (s *Service) PlaceMerchant(ctx context.Context, m contract.MerchantPlacement) error {
	c, err := toCoordinate(contract.Point{Lat: m.Lat, Lng: m.Lng})
	if err != nil {
		return err
	}
	return s.place.Execute(ctx, m.MerchantID, c, m.Active)
}

// RemoveMerchant drops a merchant from the spatial index.
func (s *Service) RemoveMerchant(ctx context.Context, merchantID string) error {
	return s.place.Remove(ctx, merchantID)
}
