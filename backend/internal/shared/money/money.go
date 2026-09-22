// Package money is the one representation of an amount in this system.
//
// Minor units in an int64, never a float. A float64 cannot hold 0.1, and money
// that cannot be added twice without drifting is money a customer will one day
// see two of. Every fee band, item price and total in the product is built from
// this type.
//
// It also owns the *display* string. The thin-client rule (2.9) says the app
// performs no arithmetic on money and renders what the server formatted, so the
// formatting has to live somewhere the server owns — and having two places
// decide whether a price reads "৳1240" or "৳ 1,240" is how one screen ends up
// disagreeing with a receipt.
package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Errors returned when building an amount.
var (
	// ErrUnknownCurrency means the currency is not one we handle.
	ErrUnknownCurrency = errors.New("unknown currency")
	// ErrCurrencyMismatch means two amounts in different currencies met.
	ErrCurrencyMismatch = errors.New("those amounts are in different currencies")
	// ErrNegative means an amount was negative where it cannot be.
	ErrNegative = errors.New("that amount cannot be negative")
)

// Currency is an ISO 4217 code.
type Currency string

// BDT is the Bangladeshi taka, and for now the only currency.
//
// Named rather than assumed so the type signatures already carry it: adding a
// second currency later is then a matter of allowing more values, not of
// finding every place that silently meant taka.
const BDT Currency = "BDT"

// minorPerUnit is how many minor units make one major unit. 100 poisha to the
// taka.
const minorPerUnit = 100

// Money is an amount in a currency.
//
// The fields are unexported and every operation returns a new value, so an
// amount handed to another package cannot be edited behind its owner's back.
type Money struct {
	minor    int64
	currency Currency
}

// New builds an amount from minor units.
//
// Negative amounts are allowed here because a price *delta* — a variant that
// costs less than the default — is a real thing. Where an amount must not be
// negative, the caller says so; see MustBeNonNegative.
func New(minor int64, c Currency) (Money, error) {
	if c != BDT {
		return Money{}, fmt.Errorf("%w: %q", ErrUnknownCurrency, c)
	}
	return Money{minor: minor, currency: c}, nil
}

// Taka builds an amount in the product's only currency.
func Taka(minor int64) Money { return Money{minor: minor, currency: BDT} }

// Zero is nothing, in the given currency.
func Zero(c Currency) Money { return Money{minor: 0, currency: c} }

// Minor returns the amount in minor units, which is what the wire carries.
func (m Money) Minor() int64 { return m.minor }

// Currency returns the currency, defaulting to BDT for the zero value so an
// uninitialised amount is still a usable zero rather than a mismatch waiting to
// happen.
func (m Money) Currency() Currency {
	if m.currency == "" {
		return BDT
	}
	return m.currency
}

// IsZero reports whether the amount is nothing.
func (m Money) IsZero() bool { return m.minor == 0 }

// IsNegative reports whether the amount is below zero.
func (m Money) IsNegative() bool { return m.minor < 0 }

// MustBeNonNegative returns the amount, or an error if it is below zero.
func (m Money) MustBeNonNegative() (Money, error) {
	if m.IsNegative() {
		return Money{}, fmt.Errorf("%w: %s", ErrNegative, m.Display())
	}
	return m, nil
}

// Add returns the sum, refusing to mix currencies.
func (m Money) Add(other Money) (Money, error) {
	if err := m.sameCurrency(other); err != nil {
		return Money{}, err
	}
	return Money{minor: m.minor + other.minor, currency: m.Currency()}, nil
}

// Sub returns the difference, refusing to mix currencies.
func (m Money) Sub(other Money) (Money, error) {
	if err := m.sameCurrency(other); err != nil {
		return Money{}, err
	}
	return Money{minor: m.minor - other.minor, currency: m.Currency()}, nil
}

// Times returns the amount repeated n times, for a line of n identical items.
func (m Money) Times(n int) Money {
	return Money{minor: m.minor * int64(n), currency: m.Currency()}
}

// Compare returns -1, 0 or 1. Currencies are not checked: with one currency in
// the system there is nothing to check, and returning an error from a
// comparison would put an error branch in every sort.
func (m Money) Compare(other Money) int {
	switch {
	case m.minor < other.minor:
		return -1
	case m.minor > other.minor:
		return 1
	default:
		return 0
	}
}

// sameCurrency guards an arithmetic operation.
func (m Money) sameCurrency(other Money) error {
	if m.Currency() != other.Currency() {
		return fmt.Errorf("%w: %s and %s", ErrCurrencyMismatch, m.Currency(), other.Currency())
	}
	return nil
}

// Display renders the amount for a person to read.
//
// "৳ 1,240" for a whole number of taka, "৳ 1,240.50" when there is poisha.
// Trailing ".00" is dropped because prices in this market are whole taka almost
// always, and a column of "৳ 120.00" reads as a form rather than a menu.
//
// The taka sign is used rather than the currency code because every price in
// the product is in taka and "BDT 120" is not how anyone here writes it.
func (m Money) Display() string {
	sign := ""
	minor := m.minor
	if minor < 0 {
		sign = "-"
		minor = -minor
	}

	units := minor / minorPerUnit
	fraction := minor % minorPerUnit

	out := sign + "৳ " + group(units)
	if fraction != 0 {
		out += "." + strconv.FormatInt(fraction, 10)
		if fraction < 10 {
			// 5 poisha is .05, not .5.
			out = out[:len(out)-1] + "0" + out[len(out)-1:]
		}
	}
	return out
}

// String makes Money readable in a log line or a test failure. It is the same
// text as Display; there is no second spelling of an amount.
func (m Money) String() string { return m.Display() }

// group inserts thousands separators.
//
// Western grouping (1,240,000) rather than the South Asian lakh/crore grouping
// (12,40,000). Both are read here; the app's own number formatting and every
// price list already use this one, and one consistent grouping is worth more
// than each surface choosing.
func group(n int64) string {
	digits := strconv.FormatInt(n, 10)
	if len(digits) <= 3 {
		return digits
	}

	var b strings.Builder
	lead := len(digits) % 3
	if lead > 0 {
		b.WriteString(digits[:lead])
	}
	for i := lead; i < len(digits); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}
