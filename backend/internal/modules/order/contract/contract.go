// Package contract is the order module's only public surface.
//
// Consumed by: dispatch, payment, tracking, review (Appendix A).
//
// Four consumers wanting four different things. Dispatch needs somewhere to
// collect from and somewhere to take it to. Payment needs an amount and a way
// to say the money arrived. Tracking needs the status and the history. Review
// needs to know an order was actually delivered before it accepts a rating.
// None of them may read an order table.
package contract

import (
	"context"
	"time"
)

// Money is an amount as it crosses a module boundary: the integer to compute
// with and the string to render (2.9).
type Money struct {
	Minor    int64
	Currency string
	Display  string
}

// Place is one end of a delivery.
type Place struct {
	Name       string
	Phone      string
	SingleLine string
	Lat        float64
	Lng        float64
}

// Line is one thing that was bought, as another module sees it.
//
// Note what is absent: the option prices and the customer's note. A rider needs
// to know they are carrying three biryanis, not what was paid for the extra
// raita.
type Line struct {
	Name     string
	Quantity int
}

// Event is one thing that happened, for tracking's timeline.
type Event struct {
	Status string
	Actor  string
	Reason string
	At     time.Time
}

// Order is an order as the rest of the system sees it.
type Order struct {
	ID   string
	Code string
	// CustomerID and MerchantID, so a consumer can address the people involved
	// through their own contracts rather than being handed their details here.
	CustomerID string
	MerchantID string

	// Status is the lifecycle state, as a primitive.
	Status string
	// Live is whether it is still going, so a consumer branches on one boolean
	// rather than keeping a copy of which statuses are terminal.
	Live bool
	// PaymentMethod is "cash" or "online".
	PaymentMethod string

	Lines []Line
	Count int

	Subtotal Money
	Delivery Money
	Total    Money
	// Expanded is whether this delivery crossed the base radius (D2), which
	// dispatch uses to understand a job that is further out than usual.
	Expanded  bool
	DistanceM float64

	Pickup      Place
	Destination Place

	Events    []Event
	PlacedAt  time.Time
	UpdatedAt time.Time
}

// OrderContract is the order module's public interface.
type OrderContract interface {
	// Order returns one order, whatever its status.
	Order(ctx context.Context, orderID string) (Order, error)

	// MarkPaid moves a prepaid order from pending_payment to placed.
	//
	// Payment calls it and nothing else does. It is a named method rather than
	// a general "set status", because a contract that let any consumer write
	// any status would put the lifecycle back in the hands of every module that
	// imports it.
	MarkPaid(ctx context.Context, orderID string) error

	// MarkPaymentFailed cancels an order whose payment never arrived.
	MarkPaymentFailed(ctx context.Context, orderID, reason string) error

	// Advance moves an order along its lifecycle on behalf of a party that
	// owns a later stage of it — dispatch marking a pickup or a delivery.
	//
	// The transition is still checked against the state machine: this is a way
	// in, not a way round. The actor may be "partner", "admin" or "system"
	// only. A customer's cancellation is bound by a window checked on their own
	// path, and a shop's authority over an order is "this shop is yours", which
	// only the HTTP layer can establish — accepting either here would mean any
	// consumer could act as anybody.
	Advance(ctx context.Context, orderID, to, actor, actorID, reason string) error
}
