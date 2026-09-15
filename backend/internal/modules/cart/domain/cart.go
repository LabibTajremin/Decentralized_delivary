// Package domain holds what a customer has chosen and the rules about holding
// it. It imports nothing outside itself except the shared money value object
// (ADR 0007).
//
// The rule that shapes everything here is single-merchant: a cart belongs to
// one shop. Not because it is easier, but because an order is collected by one
// rider from one counter — a cart spanning two shops is two orders wearing one
// checkout button, and the customer finds that out when half of it arrives.
package domain

import (
	"errors"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// The limits a cart holds itself to.
const (
	// MaxLines is how many distinct lines one cart may carry. Generous for a
	// grocery run, bounded so a single cart cannot make checkout a report.
	MaxLines = 60
	// MaxQuantity is the most of one line. Above this is a wholesale order,
	// which is a different product and a different conversation with the shop.
	MaxQuantity = 20
	// MaxNote is the length of a customer's note on a line — "no chilli".
	MaxNote = 200
)

// The rules this package refuses to bend.
var (
	ErrDifferentMerchant = errors.New("a cart belongs to one shop")
	ErrNoMerchant        = errors.New("a cart must name the shop it belongs to")
	ErrNoUser            = errors.New("a cart must belong to someone")
	ErrQuantityRange     = errors.New("quantity must be between one and the per-line maximum")
	ErrTooManyLines      = errors.New("this cart already holds as many different things as it can")
	ErrNoSuchLine        = errors.New("that line is not in this cart")
	ErrNoTarget          = errors.New("a line must name what it is for")
	ErrNoteTooLong       = errors.New("that note is too long")
	ErrEmptyCart         = errors.New("the cart is empty")
)

// Kind is what a line is for.
type Kind string

// The two things a cart can hold.
const (
	// KindItem is a single product, possibly with variants and add-ons.
	KindItem Kind = "item"
	// KindCombo is a bundle sold at a set price. Its own kind rather than an
	// item with a flag, because a combo's price is not the sum of its parts and
	// treating it as one would silently overcharge.
	KindCombo Kind = "combo"
)

// Option is one choice the customer made on a line.
//
// The price is carried with it because a variant's price is a delta on the
// item's and an add-on's is its own: recomputing either at checkout from the
// group it came from would need the group, which may have changed since.
type Option struct {
	GroupID  string
	OptionID string
	Name     string
	Price    money.Money
}

// Line is one thing in the cart, with the price it carried when it went in.
//
// A snapshot, deliberately. The customer agreed to a number, and a cart that
// silently tracked the shop's edits would show one total on the list and
// charge another at checkout. Revalidation compares the snapshot against what
// the shop charges now and *says so* — see validate.go.
type Line struct {
	ID string
	// Kind and TargetID say what this line is for: an item or a combo.
	Kind     Kind
	TargetID string
	Name     string
	// UnitPrice is the base price of one, before options.
	UnitPrice money.Money
	Options   []Option
	Quantity  int
	Note      string
}

// UnitTotal is the price of one of this line, options included.
//
// Total, with no error return. A cart is single-currency by construction:
// every amount in it was built with money.Taka, from a catalogue that prices
// in taka only. Summing minor units directly says that out loud instead of
// threading an error through every caller for a mismatch that cannot occur —
// and the day a second currency exists, this is one of the places that has to
// change, which is better than a branch nobody ever exercised.
func (l Line) UnitTotal() money.Money {
	minor := l.UnitPrice.Minor()
	for _, o := range l.Options {
		minor += o.Price.Minor()
	}
	return money.Taka(minor)
}

// Total is the price of this line as it stands.
func (l Line) Total() money.Money {
	return l.UnitTotal().Times(l.Quantity)
}

// SameSelection reports whether two lines are the same choice.
//
// Used so that adding a second "large, extra cheese" bumps the quantity rather
// than making a second line. The options are compared as a set: the order the
// customer tapped them in is not part of what they ordered.
func (l Line) SameSelection(other Line) bool {
	if l.Kind != other.Kind || l.TargetID != other.TargetID || l.Note != other.Note {
		return false
	}
	if len(l.Options) != len(other.Options) {
		return false
	}
	chosen := make(map[string]int, len(l.Options))
	for _, o := range l.Options {
		chosen[o.GroupID+"/"+o.OptionID]++
	}
	for _, o := range other.Options {
		key := o.GroupID + "/" + o.OptionID
		chosen[key]--
		if chosen[key] < 0 {
			return false
		}
	}
	return true
}

// Cart is one customer's choice from one shop, held against one address.
type Cart struct {
	ID         string
	UserID     string
	MerchantID string
	// AddressID, Lat and Lng are where this cart is to be delivered. They are
	// on the cart rather than looked up at checkout because reachability is
	// decided against the delivery point (D3), and a cart whose address is
	// only resolved at the end is a cart that can be refused after the customer
	// has finished filling it.
	AddressID string
	Lat       float64
	Lng       float64
	Lines     []Line
	UpdatedAt time.Time
}

// NewCart opens a cart for one customer at one shop.
func NewCart(id, userID, merchantID string, now time.Time) (Cart, error) {
	switch {
	case userID == "":
		return Cart{}, ErrNoUser
	case merchantID == "":
		return Cart{}, ErrNoMerchant
	}
	return Cart{ID: id, UserID: userID, MerchantID: merchantID, UpdatedAt: now}, nil
}

// IsEmpty reports whether there is nothing to order.
func (c Cart) IsEmpty() bool { return len(c.Lines) == 0 }

// Count is how many individual things are in the cart, which is the number the
// badge on the cart icon shows.
func (c Cart) Count() int {
	n := 0
	for _, l := range c.Lines {
		n += l.Quantity
	}
	return n
}

// Subtotal is what the goods cost, before delivery and before any discount.
//
// Cart stops here. Delivery, surcharges and the grand total are pricing's
// (P10): a cart that worked out the fee itself would be a second implementation
// of ALG-05, and the second one is always the one that is wrong.
func (c Cart) Subtotal() money.Money {
	var minor int64
	for _, l := range c.Lines {
		minor += l.Total().Minor()
	}
	return money.Taka(minor)
}

// Add puts a line in, merging it into an identical one if there is one.
//
// merchantID is passed so the single-merchant rule is enforced here, in the
// domain, rather than in whichever use case happened to remember it.
func (c *Cart) Add(merchantID string, line Line) error {
	if merchantID != c.MerchantID {
		return ErrDifferentMerchant
	}
	if err := validLine(line); err != nil {
		return err
	}

	for i := range c.Lines {
		if !c.Lines[i].SameSelection(line) {
			continue
		}
		merged := c.Lines[i].Quantity + line.Quantity
		if merged > MaxQuantity {
			return ErrQuantityRange
		}
		c.Lines[i].Quantity = merged
		return nil
	}

	if len(c.Lines) >= MaxLines {
		return ErrTooManyLines
	}
	c.Lines = append(c.Lines, line)
	return nil
}

// SetQuantity changes how many of a line. Zero removes it, because that is what
// tapping the minus button down to nothing means to the customer.
func (c *Cart) SetQuantity(lineID string, quantity int) error {
	if quantity == 0 {
		return c.Remove(lineID)
	}
	if quantity < 0 || quantity > MaxQuantity {
		return ErrQuantityRange
	}
	for i := range c.Lines {
		if c.Lines[i].ID == lineID {
			c.Lines[i].Quantity = quantity
			return nil
		}
	}
	return ErrNoSuchLine
}

// Remove takes a line out.
func (c *Cart) Remove(lineID string) error {
	for i := range c.Lines {
		if c.Lines[i].ID != lineID {
			continue
		}
		c.Lines = append(c.Lines[:i], c.Lines[i+1:]...)
		return nil
	}
	return ErrNoSuchLine
}

// Clear empties the cart, keeping it open at the same shop.
func (c *Cart) Clear() { c.Lines = nil }

// PlaceAt binds the cart to a delivery point.
func (c *Cart) PlaceAt(addressID string, lat, lng float64) {
	c.AddressID, c.Lat, c.Lng = addressID, lat, lng
}

// HasAddress reports whether a delivery point has been chosen. Until one has,
// nothing can be said about reachability or delivery cost.
func (c Cart) HasAddress() bool { return c.AddressID != "" }

// validLine checks what a line must be true of regardless of the shop.
func validLine(l Line) error {
	switch {
	case l.TargetID == "" || (l.Kind != KindItem && l.Kind != KindCombo):
		return ErrNoTarget
	case l.Quantity < 1 || l.Quantity > MaxQuantity:
		return ErrQuantityRange
	case len(l.Note) > MaxNote:
		return ErrNoteTooLong
	}
	return nil
}
