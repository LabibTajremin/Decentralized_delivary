// Package domain is ALG-05: what a delivery costs, and what an order comes to.
// It imports nothing outside itself except the shared money value object
// (ADR 0007).
//
// This is the only implementation of the delivery fee in the system. Discovery
// quotes a fee on every shop card and the cart quotes one at checkout; both go
// through here, because two implementations of a price are two prices, and the
// customer sees both.
package domain

import (
	"errors"
	"math"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// BandM is the width of one distance band, in metres.
//
// Banding rather than a continuous rate: a fee that changes by a few poisha as
// a phone's GPS drifts across a street looks broken, and a customer who saw
// ৳50 on the shop card must be charged ৳50. One kilometre is coarse enough to
// be stable and fine enough that a 9 km delivery does not cost the same as a
// 2 km one.
const BandM = 1000

// The rules this package refuses to bend.
var (
	ErrNegativeDistance = errors.New("a distance cannot be negative")
	ErrNegativeBase     = errors.New("the base delivery fee cannot be negative")
	ErrNegativeRate     = errors.New("the per-kilometre rate cannot be negative")
	ErrMultiplierBelow1 = errors.New("the expansion multiplier cannot be below one")
	ErrNegativeSubtotal = errors.New("an order subtotal cannot be negative")
)

// Tariff is the resolved pricing configuration for one place.
//
// Every field comes from Appendix B, resolved area → district → division →
// global, which is why the same journey costs differently in Dhaka and in a
// haor upazila.
type Tariff struct {
	base       money.Money
	perBand    money.Money
	multiplier float64
	freeAbove  money.Money
}

// NewTariff builds a tariff from resolved configuration.
//
// The multiplier is required to be at least 1. Appendix B already bounds it
// there, and the reason is D2: a multiplier below one would make a longer,
// dearer journey cost the customer *less*, which is the opposite of the rule
// the expansion ladder exists to enforce.
func NewTariff(base, perBand money.Money, multiplier float64, freeAbove money.Money) (Tariff, error) {
	switch {
	case base.IsNegative():
		return Tariff{}, ErrNegativeBase
	case perBand.IsNegative():
		return Tariff{}, ErrNegativeRate
	case multiplier < 1:
		return Tariff{}, ErrMultiplierBelow1
	}
	return Tariff{base: base, perBand: perBand, multiplier: multiplier, freeAbove: freeAbove}, nil
}

// Bands is how many distance bands a journey falls into.
//
// Rounded up, so any part of a kilometre is a kilometre — and exact at the
// edges: 1000 m is one band and 1001 m is two. O(1).
func Bands(distanceM float64) (int64, error) {
	if distanceM < 0 {
		return 0, ErrNegativeDistance
	}
	return int64(math.Ceil(distanceM / BandM)), nil
}

// DeliveryFee is ALG-05.
//
//	fee = (base + rate × bands) × multiplier¹ᶠ ᵉˣᵖᵃⁿᵈᵉᵈ
//
// Piecewise-linear in the distance, with one step at each band edge and one
// multiplication when the customer widened the radius (D2). O(1).
//
// The multiplier is applied to the whole fee rather than to the distance part
// alone. Expansion costs the platform more than extra kilometres: it is a rider
// out of their usual area, further from their next job, which the base fee
// covers as much as the rate does.
//
// The surcharge comes back with the fee rather than from a second call. It is
// the difference between this fee and what the same journey would have cost
// unexpanded, and deriving it here is what makes it impossible for the two to
// disagree — or for a caller to validate the distance twice and handle an
// error that the first validation already ruled out.
func (t Tariff) DeliveryFee(distanceM float64, expanded bool) (fee, surcharge money.Money, err error) {
	bands, err := Bands(distanceM)
	if err != nil {
		return money.Money{}, money.Money{}, err
	}

	plain := t.feeFor(bands, false)
	if !expanded {
		return plain, money.Taka(0), nil
	}
	full := t.feeFor(bands, true)
	return full, money.Taka(full.Minor() - plain.Minor()), nil
}

// feeFor is the fee for a band count, total because the bands are already
// known to be valid.
func (t Tariff) feeFor(bands int64, expanded bool) money.Money {
	minor := t.base.Minor() + t.perBand.Minor()*bands
	if expanded {
		// Rounded to nearest rather than truncated, so a 1.5× multiplier on an
		// odd amount does not quietly favour the platform.
		minor = int64(math.Round(float64(minor) * t.multiplier))
	}
	return money.Taka(minor)
}

// FreeDeliveryApplies reports whether this order earns free delivery.
//
// Only at the base radius. That is a real decision and worth stating: free
// delivery is a promotion on a *local* order, and letting it survive an
// expansion would give a customer a free ride across the division and break D2
// outright — "when the radius is extended, the delivery charge increases" would
// stop being true above the threshold. A shop that wants a wider free radius
// raises `discovery.base_radius` for its area, which is the knob that means
// what it says.
//
// A threshold of zero switches the promotion off rather than making every
// delivery free: nobody configures "free delivery on orders over nothing", and
// reading it that way would give away every delivery in an area the moment
// somebody cleared the field.
func (t Tariff) FreeDeliveryApplies(subtotal money.Money, expanded bool) bool {
	if expanded || t.freeAbove.IsZero() {
		return false
	}
	return subtotal.Compare(t.freeAbove) >= 0
}
