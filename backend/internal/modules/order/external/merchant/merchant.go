// Package merchant is the order module's view of the merchant module.
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

// Service is the part of merchant the order depends on.
//
// Merchant rather than Listed: order has to resolve a shop it already holds an
// order against even after a suspension, because the order still has to be
// shown, tracked and argued about afterwards. IsAcceptingOrders is the check
// that runs on the way in.
type Service interface {
	Merchant(ctx context.Context, merchantID string) (Merchant, error)
	IsAcceptingOrders(ctx context.Context, merchantID string) (bool, error)

	// OwnedBy answers "is this shop yours" before a queue is shown or an order
	// is accepted on its behalf.
	OwnedBy(ctx context.Context, ownerUserID string) ([]string, error)
}
