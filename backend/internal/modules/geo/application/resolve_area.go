// Package application holds the geo use cases.
package application

import (
	"context"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// ResolveAreaUseCase turns a coordinate into the administrative area, district
// and division that contain it.
//
// Every config lookup depends on this: Appendix B resolves values
// area -> district -> division -> global, so a delivery address that cannot be
// placed in an area cannot be priced or served.
type ResolveAreaUseCase struct {
	repo ports.GeoRepository
}

// NewResolveAreaUseCase wires the use case.
func NewResolveAreaUseCase(repo ports.GeoRepository) *ResolveAreaUseCase {
	return &ResolveAreaUseCase{repo: repo}
}

// ResolvedArea is what a caller gets back.
type ResolvedArea struct {
	Area     domain.Area
	Division domain.Division
}

// Execute resolves a coordinate.
//
// A point outside every division is KindNotFound, not KindInvalid: the caller
// sent a perfectly well-formed coordinate, and what is missing is a service
// area covering it. Keeping the two apart matters operationally — a rise in
// invalid_coordinate means a client is sending nonsense and is our bug, while
// a rise in outside_service_area is ordinary traffic from people in places we
// have not reached yet. Collapsing both into 400 would hide the first inside
// the second. The message is written for the user, because this is a case they
// will really hit: someone travelling, or a GPS fix that landed in the bay.
func (uc *ResolveAreaUseCase) Execute(ctx context.Context, c domain.Coordinate) (ResolvedArea, error) {
	division, err := uc.repo.DivisionContaining(ctx, c)
	if err != nil {
		if errors.Is(err, domain.ErrUnknownDivision) {
			return ResolvedArea{}, errs.Wrap(err, errs.KindNotFound, "outside_service_area",
				"We do not deliver to this location yet.")
		}
		return ResolvedArea{}, errs.Wrap(err, errs.KindUnavailable, "geo_lookup_failed",
			"We could not check this location. Please try again.")
	}

	area, err := uc.repo.AreaContaining(ctx, c)
	if err != nil {
		if errors.Is(err, domain.ErrUnknownDivision) {
			return ResolvedArea{}, errs.Wrap(err, errs.KindNotFound, "outside_service_area",
				"We do not deliver to this location yet.")
		}
		return ResolvedArea{}, errs.Wrap(err, errs.KindUnavailable, "geo_lookup_failed",
			"We could not check this location. Please try again.")
	}

	return ResolvedArea{Area: area, Division: division}, nil
}
