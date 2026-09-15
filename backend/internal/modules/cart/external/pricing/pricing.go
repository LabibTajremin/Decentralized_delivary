// Package pricing is the cart module's view of the pricing module.
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

// Row is one line of the receipt.
type Row = pricingcontract.Row

// Money is an amount in both forms.
type Money = pricingcontract.Money

// Tariff is the pricing configuration resolved for one place.
type Tariff = pricingcontract.Tariff

// Service is the part of pricing the cart depends on.
//
// The cart owns the goods and stops there. It hands pricing the subtotal, the
// distance discovery measured and the expansion level discovery decided, and
// takes back a receipt — because a cart that worked out the delivery fee
// itself would be a second implementation of ALG-05, and the second one is
// always the one that is wrong.
type Service interface {
	Tariff(ctx context.Context, p Placement) (Tariff, error)
}
