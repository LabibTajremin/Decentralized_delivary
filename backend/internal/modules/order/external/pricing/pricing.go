// Package pricing is the order module's view of the pricing module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package pricing

import (
	"context"

	pricingcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/contract"
)

// Placement is where a price is being worked out.
type Placement = pricingcontract.Placement

// QuoteRequest is one price question.
type QuoteRequest = pricingcontract.QuoteRequest

// Quote is the whole price of an order.
type Quote = pricingcontract.Quote

// Tariff is the pricing configuration resolved for one place.
type Tariff = pricingcontract.Tariff

// Service is the part of pricing the order depends on.
//
// Order re-quotes on the way in rather than trusting the figure the cart
// showed. A price a customer was shown is not a price the system promised: the
// cart's quote was composed for a screen, possibly minutes ago, by a request
// the customer controls the timing of. What gets written down is what the
// server works out at the moment it writes it.
type Service interface {
	Tariff(ctx context.Context, p Placement) (Tariff, error)
}
