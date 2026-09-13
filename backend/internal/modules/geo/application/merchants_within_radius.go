package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// maxSearchLimit caps how many merchants one radius query may return. A
// low-end device is never handed more than a page (05-architecture.md 2.9),
// and an unbounded spatial query is the easiest way to melt the database.
const maxSearchLimit = 200

// MerchantsWithinRadiusUseCase implements ALG-01: merchants inside a radius,
// nearest first, never crossing the division boundary.
type MerchantsWithinRadiusUseCase struct {
	repo ports.GeoRepository
}

// NewMerchantsWithinRadiusUseCase wires the use case.
func NewMerchantsWithinRadiusUseCase(repo ports.GeoRepository) *MerchantsWithinRadiusUseCase {
	return &MerchantsWithinRadiusUseCase{repo: repo}
}

// RadiusQuery asks for merchants around a point.
type RadiusQuery struct {
	Centre domain.Coordinate
	Radius domain.Distance
	Limit  int
}

// Execute resolves the division for the centre and searches within it.
//
// The division is resolved here and passed into the query rather than
// filtering results afterwards: D3 is the one invariant that can never be
// disabled, so it belongs in the query predicate where nothing downstream can
// forget to apply it.
func (uc *MerchantsWithinRadiusUseCase) Execute(ctx context.Context, q RadiusQuery) ([]ports.MerchantLocation, error) {
	if q.Radius <= 0 {
		return nil, errs.New(errs.KindInvalid, "invalid_radius", "Search radius must be greater than zero.")
	}
	limit := q.Limit
	if limit <= 0 || limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	division, err := uc.repo.DivisionContaining(ctx, q.Centre)
	if err != nil {
		return nil, errs.Wrap(err, errs.KindInvalid, "outside_service_area",
			"We do not deliver to this location yet.")
	}

	found, err := uc.repo.MerchantsWithinRadius(ctx, q.Centre, q.Radius, division.Code, limit)
	if err != nil {
		return nil, errs.Wrap(err, errs.KindUnavailable, "merchant_search_failed",
			"We could not search nearby shops. Please try again.")
	}
	return found, nil
}

// CountWithin returns how many merchants sit inside a radius. Radius expansion
// (ALG-02, D2) only needs the count to decide whether to offer a wider search,
// so this avoids transferring rows nobody will read.
func (uc *MerchantsWithinRadiusUseCase) CountWithin(ctx context.Context, centre domain.Coordinate, radius domain.Distance) (int, error) {
	if radius <= 0 {
		return 0, errs.New(errs.KindInvalid, "invalid_radius", "Search radius must be greater than zero.")
	}
	division, err := uc.repo.DivisionContaining(ctx, centre)
	if err != nil {
		return 0, errs.Wrap(err, errs.KindInvalid, "outside_service_area",
			"We do not deliver to this location yet.")
	}
	n, err := uc.repo.CountMerchantsWithinRadius(ctx, centre, radius, division.Code)
	if err != nil {
		return 0, errs.Wrap(err, errs.KindUnavailable, "merchant_search_failed",
			"We could not search nearby shops. Please try again.")
	}
	return n, nil
}
