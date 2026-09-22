// Package pricing is the discovery module's view of the pricing module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
//
// Replaces the provisional fee calculation discovery carried from P08 to P10.
// ALG-05 now has exactly one implementation, in pricing's domain, which is the
// point: two implementations of a price are two prices, and the customer sees
// both — the fee on the shop card and the fee on the receipt.
package pricing

import (
	"context"

	pricingcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/contract"
)

// Placement is where a price is being worked out.
type Placement = pricingcontract.Placement

// QuoteRequest is one price question.
type QuoteRequest = pricingcontract.QuoteRequest

// Fee is a delivery charge on its own, which is what a shop card shows.
type Fee = pricingcontract.Fee

// Tariff is the pricing configuration resolved for one place.
type Tariff = pricingcontract.Tariff

// Service is the part of pricing discovery depends on.
//
// One method, returning a snapshot. A search prices twenty shop cards; if
// pricing were asked per card, a search would make twenty configuration reads
// and could in principle quote two of them against different settings.
type Service interface {
	Tariff(ctx context.Context, p Placement) (Tariff, error)
}
