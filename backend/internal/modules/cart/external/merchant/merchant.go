// Package merchant is the cart module's view of the merchant module.
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

// Service is the part of merchant the cart depends on.
//
// The cart needs the shop's own state — listed, open, what it is called — to
// say why a cart cannot be checked out. "Closed until 9am" and "suspended" are
// different sentences and the customer should get the right one.
type Service interface {
	Merchant(ctx context.Context, merchantID string) (Merchant, error)
}
