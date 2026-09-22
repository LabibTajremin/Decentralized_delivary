package application

import (
	"context"
	"math"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/domain"
	cfg "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/geo"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/merchant"
)

// ReachUseCase answers the single-merchant version of a search: may this
// address order from this shop, and at what expansion level.
//
// Cart asks it. A cart is built against one delivery address and then the
// customer changes the address at checkout — at which point the shop they
// chose may be in another division entirely, and the order must be refused
// before it is placed rather than after a rider has been dispatched.
type ReachUseCase struct {
	geo      geo.Service
	merchant merchant.Service
	config   cfg.Service
}

// NewReachUseCase wires the use case.
func NewReachUseCase(g geo.Service, m merchant.Service, c cfg.Service) *ReachUseCase {
	return &ReachUseCase{geo: g, merchant: m, config: c}
}

// Execute reports whether a merchant is visible from a point.
func (uc *ReachUseCase) Execute(ctx context.Context, from geo.Point, merchantID, lang string) (contract.Reach, error) {
	shop, err := uc.merchant.Merchant(ctx, merchantID)
	if err != nil {
		return contract.Reach{}, err
	}

	placement, err := uc.geo.ResolveDivision(ctx, from)
	if err != nil {
		return contract.Reach{}, err
	}

	distance, err := uc.geo.DistanceBetween(ctx, from, geo.Point{Lat: shop.Lat, Lng: shop.Lng})
	if err != nil {
		return contract.Reach{}, err
	}

	reach := contract.Reach{
		MerchantID:   merchantID,
		AreaCode:     placement.AreaCode,
		DistrictCode: placement.DistrictCode,
		DivisionCode: placement.DivisionCode,
		DistanceM:    distance,
		DistanceText: domain.FormatDistance(distance, lang),
	}

	// D3 first, and before any radius arithmetic. A shop across a division
	// boundary is unreachable at every level, so asking how far it is and
	// which step would reach it would produce a number that is true and an
	// offer that is a lie.
	if shop.DivisionCode != placement.DivisionCode {
		reach.Reason = contract.ReasonOutsideDivision
		return reach, nil
	}

	settings, err := uc.config.Settings(ctx, configPlacement(placement))
	if err != nil {
		return contract.Reach{}, configError(err)
	}
	policy, err := policyFrom(settings)
	if err != nil {
		return contract.Reach{}, err
	}

	if distance > policy.Radius(policy.MaxLevel()) {
		reach.Reason = contract.ReasonBeyondMaxRadius
		return reach, nil
	}

	reach.Reachable = true
	reach.RequiredLevel = levelFor(policy, distance)
	reach.Expanded = reach.RequiredLevel > 0
	return reach, nil
}

// levelFor is the lowest expansion level whose radius covers a distance.
//
// Solved rather than walked: the ladder is linear, so the step count is
// arithmetic and a loop would only be a slower way of computing a ceiling.
func levelFor(policy domain.Policy, distanceM float64) int {
	over := distanceM - policy.BaseRadiusM()
	if over <= 0 {
		return 0
	}
	return policy.ClampLevel(int(math.Ceil(over / policy.StepM())))
}
