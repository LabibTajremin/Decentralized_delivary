// Package merchant is the catalogue module's view of the merchant module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5). Extracting merchant
// into its own service is then a change to this file alone.
package merchant

import (
	"context"

	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
)

// Merchant is a shop as the catalogue sees it.
type Merchant = merchantcontract.Merchant

// Service is the part of the merchant module the catalogue depends on.
//
// One method. The catalogue needs to know a shop exists, who owns it and what
// kind it is — the kind decides the whole shape of its items (Capabilities) —
// and nothing else. An interface that offered the approval queue here would let
// a menu screen grow a dependency on moderation.
type Service interface {
	Merchant(ctx context.Context, merchantID string) (Merchant, error)
}
