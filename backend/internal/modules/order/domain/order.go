package domain

import (
	"errors"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// The rules an order holds itself to.
var (
	ErrNoCustomer     = errors.New("an order must belong to somebody")
	ErrNoMerchant     = errors.New("an order must name the shop it is from")
	ErrNoLines        = errors.New("an order must have something in it")
	ErrNoAddress      = errors.New("an order must have somewhere to go")
	ErrUnknownPayment = errors.New("that is not a way to pay")
	ErrCODLimit       = errors.New("that order is too large to pay for in cash")
	ErrNegativeAmount = errors.New("an order cannot cost less than nothing")
	ErrWindowClosed   = errors.New("the free cancellation window has closed")
	ErrNoCancelReason = errors.New("a cancellation needs a reason")
	ErrNoEventID      = errors.New("the placement event needs an id")
)

// PaymentMethod is how the customer will pay.
type PaymentMethod string

// The two paths.
const (
	// PaymentCash is cash on delivery. The order is placed outright: there is
	// nothing to wait for, and the money arrives with the rider.
	PaymentCash PaymentMethod = "cash"
	// PaymentOnline is prepaid. The order starts at pending_payment and only
	// reaches the shop once payment confirms — a shop that started cooking on
	// an unpaid order would be carrying the platform's risk.
	PaymentOnline PaymentMethod = "online"
)

// Valid reports whether a string names a payment method.
func (p PaymentMethod) Valid() bool {
	return p == PaymentCash || p == PaymentOnline
}

// InitialStatus is where an order on this path starts.
func (p PaymentMethod) InitialStatus() Status {
	if p == PaymentOnline {
		return StatusPendingPayment
	}
	return StatusPlaced
}

// Option is one choice the customer made on a line, frozen.
type Option struct {
	GroupID  string
	OptionID string
	Name     string
	Price    money.Money
}

// Line is one thing that was bought, at the price it was bought for.
//
// A copy, not a reference. The shop will edit its menu, rename the item and
// change the price; none of that may reach an order that has been placed. An
// order is a record of an agreement, and an agreement that changed afterwards
// was not an agreement.
type Line struct {
	ID string
	// Kind is "item" or "combo"; TargetID is what it was, so a reorder button
	// has something to point at.
	Kind      string
	TargetID  string
	Name      string
	UnitPrice money.Money
	Options   []Option
	Quantity  int
	Note      string
}

// Total is what this line came to.
func (l Line) Total() money.Money {
	minor := l.UnitPrice.Minor()
	for _, o := range l.Options {
		minor += o.Price.Minor()
	}
	return money.Taka(minor * int64(l.Quantity))
}

// Charges is what the order cost, frozen at placement.
//
// Frozen rather than recomputed on read, for the same reason the lines are: a
// receipt that changed when an admin retuned the area's delivery rate would be
// a receipt nobody could reconcile.
type Charges struct {
	Subtotal           money.Money
	Delivery           money.Money
	ExpansionSurcharge money.Money
	Total              money.Money
	FreeDelivery       bool
	Expanded           bool
	DistanceM          float64
}

// Destination is where the order is going, copied from the address book.
//
// Copied, again deliberately. A customer who edits an address after ordering
// must not silently redirect a rider who is already on the road.
type Destination struct {
	AddressID  string
	Label      string
	Recipient  string
	Phone      string
	Line1      string
	Line2      string
	SingleLine string
	Lat        float64
	Lng        float64
	AreaCode   string
	AreaName   string
	Directions string
}

// Pickup is where the order is collected from, likewise copied.
type Pickup struct {
	MerchantID string
	Name       string
	Phone      string
	SingleLine string
	Lat        float64
	Lng        float64
}

// Event is one thing that happened to an order.
//
// The history is kept rather than derived, because "when did the shop accept
// it" is a question the customer asks, tracking answers (P14), and support
// settles arguments with (P16). A status column alone cannot answer it.
type Event struct {
	ID     string
	Status Status
	Actor  Actor
	// ActorID is who specifically, when there is a who. Empty for the system.
	ActorID string
	// Reason is why, for the transitions that need one.
	Reason string
	At     time.Time
}

// Order is one agreement between a customer and a shop.
type Order struct {
	ID string
	// Code is the short human reference a customer reads out on the phone.
	Code       string
	CustomerID string
	MerchantID string

	Status  Status
	Payment PaymentMethod

	Lines       []Line
	Charges     Charges
	Destination Destination
	Pickup      Pickup

	// ExpansionLevel is the radius rung this order was placed at (D2), kept so
	// an admin looking at a division's numbers can see how much of its traffic
	// needed expansion.
	ExpansionLevel int

	Events    []Event
	PlacedAt  time.Time
	UpdatedAt time.Time
}

// Draft is everything needed to open an order, gathered by the use case.
type Draft struct {
	ID string
	// EventID identifies the placement event. Taken here rather than left for
	// the repository to fill in, because an order is never without its first
	// event and an event without an id is a row that collides with the next
	// one — which is a failure that only appears on the *second* order a
	// process writes.
	EventID     string
	Code        string
	CustomerID  string
	MerchantID  string
	Payment     PaymentMethod
	Lines       []Line
	Charges     Charges
	Destination Destination
	Pickup      Pickup

	ExpansionLevel int
	CODLimit       money.Money
	Now            time.Time
}

// NewOrder freezes a draft into an order.
//
// The COD limit is checked here rather than at the transport edge, because it
// is a rule about the order and not about the request. A cash order above the
// limit is a rider carrying more of somebody else's money than the platform is
// willing to risk on one doorstep.
func NewOrder(d Draft) (Order, error) {
	switch {
	case d.CustomerID == "":
		return Order{}, ErrNoCustomer
	case d.MerchantID == "":
		return Order{}, ErrNoMerchant
	case len(d.Lines) == 0:
		return Order{}, ErrNoLines
	case d.Destination.AddressID == "":
		return Order{}, ErrNoAddress
	case !d.Payment.Valid():
		return Order{}, ErrUnknownPayment
	case d.Charges.Total.IsNegative() || d.Charges.Subtotal.IsNegative():
		return Order{}, ErrNegativeAmount
	case d.EventID == "":
		return Order{}, ErrNoEventID
	}

	if d.Payment == PaymentCash && !d.CODLimit.IsZero() &&
		d.Charges.Total.Compare(d.CODLimit) > 0 {
		return Order{}, ErrCODLimit
	}

	status := d.Payment.InitialStatus()
	return Order{
		ID: d.ID, Code: d.Code,
		CustomerID: d.CustomerID, MerchantID: d.MerchantID,
		Status: status, Payment: d.Payment,
		Lines: d.Lines, Charges: d.Charges,
		Destination: d.Destination, Pickup: d.Pickup,
		ExpansionLevel: d.ExpansionLevel,
		Events: []Event{{
			ID: d.EventID, Status: status,
			Actor: ActorCustomer, ActorID: d.CustomerID, At: d.Now,
		}},
		PlacedAt: d.Now, UpdatedAt: d.Now,
	}, nil
}

// Move applies a transition, recording it.
//
// The order carries its own history: every caller that changed a status and
// forgot to write an event would leave a customer asking "when was this
// accepted" with no answer.
func (o *Order) Move(to Status, actor Actor, actorID, reason string, now time.Time) error {
	if err := CanTransition(o.Status, to, actor); err != nil {
		return err
	}
	o.Status = to
	o.UpdatedAt = now
	o.Events = append(o.Events, Event{
		Status: to, Actor: actor, ActorID: actorID, Reason: reason, At: now,
	})
	return nil
}

// Count is how many individual things are in the order.
func (o Order) Count() int {
	n := 0
	for _, l := range o.Lines {
		n += l.Quantity
	}
	return n
}

// BelongsTo reports whether this is a given customer's order.
func (o Order) BelongsTo(customerID string) bool { return o.CustomerID == customerID }

// AcceptedAt is when the shop said yes, and whether it ever did.
func (o Order) AcceptedAt() (time.Time, bool) {
	for _, e := range o.Events {
		if e.Status == StatusAccepted {
			return e.At, true
		}
	}
	return time.Time{}, false
}
