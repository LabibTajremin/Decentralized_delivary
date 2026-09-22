// Package contract is the pricing module's only public surface.
//
// Consumed by: cart, order, discovery (Appendix A).
//
// It carries no HTTP surface of its own. A price is never asked for on its own
// — it is asked for *about* something, a cart or a shop card — so the module
// that owns that thing quotes it and returns it with the rest of the answer.
package contract

import "context"

// Placement is where a price is being worked out. It is the output of geo's
// resolution, restated in primitives so pricing and geo stay independent of
// each other's types.
type Placement struct {
	AreaCode     string
	DistrictCode string
	DivisionCode string
}

// Money is an amount as it crosses a module boundary and reaches a client: the
// integer to compute with and the string to render (2.9).
type Money struct {
	Minor    int64
	Currency string
	Display  string
}

// Fee is a delivery charge on its own, which is what a shop card shows.
type Fee struct {
	Amount Money
	// Expanded is whether the D2 surcharge was applied, so the app can say why
	// the fee is higher instead of leaving the customer to guess.
	Expanded bool
	// Surcharge is how much of Amount is that surcharge. Zero at the base
	// radius.
	Surcharge Money
}

// Row is one line of the receipt, composed by the server.
type Row struct {
	// Key is what the row is — "subtotal", "delivery", "expansion_surcharge",
	// "free_delivery", "total" — so a client can style it without parsing the
	// label.
	Key string
	// Label is the words to show, in the requested language.
	Label  string
	Amount Money
}

// Quote is the whole price of an order.
type Quote struct {
	Subtotal Money
	// Delivery is what the customer pays after any waiver;
	// DeliveryBeforeWaiver is what it would have cost.
	Delivery             Money
	DeliveryBeforeWaiver Money
	ExpansionSurcharge   Money
	FreeDelivery         bool
	// FreeDeliveryAt and AwayFromFreeDelivery let a client say "৳120 more for
	// free delivery" without holding the threshold itself, which 2.9 forbids.
	FreeDeliveryAt       Money
	AwayFromFreeDelivery Money
	Total                Money

	DistanceM float64
	Expanded  bool

	// Rows is the receipt, already ordered, with only the lines that say
	// something.
	Rows []Row
	// Notice is the one line to show under the total — what free delivery would
	// take, or why the fee is higher — or empty when there is nothing to say.
	Notice string
}

// QuoteRequest is one price question.
type QuoteRequest struct {
	// SubtotalMinor is what the goods cost, in poisha. The caller owns the
	// goods; pricing owns everything after them.
	SubtotalMinor int64
	// DistanceM is the straight-line distance from the customer to the shop,
	// measured by geo so every part of the system agrees on one number.
	DistanceM float64
	// ExpansionLevel is 0 at the base radius and higher when the customer
	// widened it (D2).
	ExpansionLevel int
	// Lang selects the language of the composed strings. Anything but "en" is
	// Bengali, because the audience is (1.4).
	Lang string
}

// Tariff is the pricing configuration resolved for one place, held so a caller
// pricing twenty shop cards resolves it once.
//
// The same shape as config's Settings, and for the same reason: a fee computed
// against one read and a threshold against another is the inconsistency that
// only appears under load.
type Tariff interface {
	// DeliveryFee is the fee alone, for a shop card.
	DeliveryFee(req QuoteRequest) (Fee, error)

	// Quote is the whole receipt, for a cart or an order.
	Quote(req QuoteRequest) (Quote, error)
}

// PricingContract is the pricing module's public interface.
type PricingContract interface {
	// Tariff resolves the pricing configuration in force at a placement.
	Tariff(ctx context.Context, p Placement) (Tariff, error)
}
