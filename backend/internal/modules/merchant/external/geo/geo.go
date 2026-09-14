// Package geo is the merchant module's view of the geo module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5). Extracting geo into
// its own service is then a change to this file alone.
package geo

import (
	"context"

	geocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
)

// Point is a coordinate pair.
type Point = geocontract.Point

// Area is the administrative placement of a point.
type Area = geocontract.Area

// Placement states where a merchant is and whether it should be searchable.
type Placement = geocontract.MerchantPlacement

// Service is the part of the geo module the merchant module depends on.
//
// Three methods: place a shop administratively, publish it to the radius index,
// and withdraw it. Merchant never asks geo *who is nearby* — that is discovery's
// question (P08), and an interface that offered it here would let registration
// grow a dependency on search.
type Service interface {
	// ResolveDivision, not ResolveArea: D1 says a shop may register from
	// anywhere in Bangladesh, and an upazila we have not drawn an area for is
	// still in Bangladesh. The division is required because it is the D3
	// ceiling; the area is whatever geo can tell us.
	ResolveDivision(ctx context.Context, p Point) (Area, error)
	PlaceMerchant(ctx context.Context, m Placement) error
	RemoveMerchant(ctx context.Context, merchantID string) error
}
