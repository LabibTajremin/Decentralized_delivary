// Package ports declares what the discovery use cases need from the outside
// world. No use case ever touches a database handle (05-architecture.md 2.3).
//
// Discovery owns no storage at all: every fact it uses belongs to geo, merchant,
// catalogue or config, and it reaches those through external/. The one port
// here is the delivery quote, which is a calculation rather than a lookup.
package ports

import "context"

// DeliveryQuoteRequest is one fee question: how much to carry an order this far,
// at this expansion level.
type DeliveryQuoteRequest struct {
	// DistanceM is the straight-line distance from customer to shop.
	DistanceM float64
	// ExpansionLevel is 0 for a base-radius search and higher when the customer
	// has widened it. D2 makes a wider search cost more, and the level is what
	// says by how much.
	ExpansionLevel int
}

// DeliveryQuote is a fee as it reaches a client: the integer to compute with
// and the string to render (2.9).
type DeliveryQuote struct {
	Minor    int64
	Currency string
	Display  string
	// Expanded is whether an expansion surcharge was applied, so the app can
	// say why the fee is higher instead of leaving the customer to guess.
	Expanded bool
}

// DeliveryQuoter prices a delivery.
//
// A port rather than a call into pricing because discovery is built before
// pricing is (P08 before P10). The provisional implementation under
// infrastructure/fees applies ALG-05 from configuration; P10 replaces it with
// discovery's external/pricing service, and no use case changes.
type DeliveryQuoter interface {
	QuoteDelivery(ctx context.Context, placement Placement, req DeliveryQuoteRequest) (DeliveryQuote, error)
}

// Placement is where the question is being asked, restated here so the port
// does not depend on config's contract.
type Placement struct {
	AreaCode     string
	DistrictCode string
	DivisionCode string
}
