// Package contract is the cart module's only public surface.
//
// Consumed by: order (Appendix A).
//
// Order needs to turn a cart into something it can freeze: the lines, the
// prices they carry now, the shop, and the delivery point they were held
// against. It also needs the cart to disappear once the order exists, which is
// why the contract carries Clear rather than leaving order to guess.
package contract

import "context"

// Money is an amount as it crosses a module boundary: the integer to compute
// with and the string to render (2.9).
type Money struct {
	Minor    int64
	Currency string
	Display  string
}

// Option is one choice on a line.
type Option struct {
	GroupID  string
	OptionID string
	Name     string
	Price    Money
}

// Line is one thing in the cart, at the price it costs now.
type Line struct {
	ID string
	// Kind is "item" or "combo".
	Kind     string
	TargetID string
	Name     string
	// UnitPrice is the price of one, options included, as the shop charges it
	// now rather than when the customer added it.
	UnitPrice Money
	Options   []Option
	Quantity  int
	Note      string
	// LineTotal is UnitPrice × Quantity, computed by the server.
	LineTotal Money
}

// Delivery is what the cart was quoted, restated in this module's own
// primitives.
//
// Restated rather than pricing's own type: a contract package may not import
// another module's contract (2.5), and the restatement is the price of that
// rule. Order re-quotes anyway before it writes an order — a price a customer
// was shown is not a price the system promised until the server says so on the
// way in.
type Delivery struct {
	Fee                Money
	ExpansionSurcharge Money
	Total              Money
	FreeDelivery       bool
	Expanded           bool
	DistanceM          float64
}

// Cart is a whole cart as order receives it.
type Cart struct {
	ID         string
	UserID     string
	MerchantID string
	AddressID  string
	Lat        float64
	Lng        float64
	Lines      []Line
	// Subtotal is what the goods cost. Delivery and the grand total are
	// pricing's, not the cart's.
	Subtotal Money
	// Delivery is the quote the customer was shown, or nil when there was
	// nothing true to quote — no address, or an address this shop cannot
	// deliver to.
	Delivery *Delivery
	// Orderable is whether checkout may proceed, and Blocker says why not.
	// Order re-checks rather than trusting a client that claims it may: an
	// endpoint that took "this cart is fine" from the caller is an endpoint
	// that will be told so.
	Orderable bool
	Blocker   string
}

// CartContract is the cart module's public interface.
type CartContract interface {
	// Current returns the customer's cart, revalidated against the shop as it
	// stands. The second return is false when they have no cart.
	Current(ctx context.Context, userID string) (Cart, bool, error)

	// Clear empties the customer's cart once its contents have become an
	// order. Cart owns its own storage, so order asks rather than deleting.
	Clear(ctx context.Context, userID string) error
}
