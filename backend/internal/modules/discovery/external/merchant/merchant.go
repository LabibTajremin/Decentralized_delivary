// Package merchant is the discovery module's view of the merchant module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package merchant

import (
	"context"

	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
)

// Merchant is a shop as the rest of the system sees it.
type Merchant = merchantcontract.Merchant

// Type is what kind of shop it is.
type Type = merchantcontract.Type

// The shop types, restated so discovery's filter validation does not link
// merchant's domain.
const (
	TypeRestaurant = merchantcontract.TypeRestaurant
	TypeGrocery    = merchantcontract.TypeGrocery
	TypePharmacy   = merchantcontract.TypePharmacy
)

// Service is the part of merchant discovery depends on.
//
// Listed is the important one: the spatial index says where shops are, and the
// merchant record says which of them a customer may see. Keeping those two
// answers in two modules is what stops a suspended shop reappearing because
// somebody forgot a WHERE clause in a geospatial query.
type Service interface {
	// Listed returns the subset of these ids a customer may see, in the order
	// given — which is the nearest-first order the radius search produced.
	Listed(ctx context.Context, merchantIDs []string) ([]Merchant, error)

	// Merchant returns one shop whatever its status, for a reachability check
	// on a shop the customer has already chosen.
	Merchant(ctx context.Context, merchantID string) (Merchant, error)
}
