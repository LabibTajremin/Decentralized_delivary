// Package domain holds the configuration model: what a variable is, what it may
// hold, and who may change it.
//
// Every business rule that an operator can tune lives here rather than in a
// deploy-time environment variable (ADR 0004). A rule that needs a redeploy to
// change is a rule in the wrong place.
package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Errors returned when building or reading a value.
var (
	// ErrKindMismatch means a value was read as the wrong type, or set with one.
	ErrKindMismatch = errors.New("configuration value has a different type")
	// ErrUnparseable means stored text could not be read back as its type.
	ErrUnparseable = errors.New("configuration value could not be parsed")
	// ErrNegative means a quantity that cannot be negative was given one.
	ErrNegative = errors.New("configuration value must not be negative")
)

// Kind is the type of a configuration value.
//
// The set is closed and small. Money, distance and duration are separate kinds
// rather than all being "a number", because the unit is the part people get
// wrong: a delivery fee of 40 and a radius of 40 are not interchangeable, and a
// type system that cannot tell them apart will eventually let one be used as
// the other.
type Kind uint8

const (
	// KindCount is a plain whole number, such as a maximum number of steps.
	KindCount Kind = iota
	// KindBool is a switch.
	KindBool
	// KindRatio is a multiplier such as 1.5. The only non-integer kind.
	KindRatio
	// KindMoney is an amount in minor units — poisha, not taka. Money is never
	// a float anywhere in this system (2.9).
	KindMoney
	// KindDistance is a length in metres. Config is authored in kilometres and
	// stored in metres, so no consumer has to remember a scale factor.
	KindDistance
	// KindDuration is a length of time in seconds.
	KindDuration
)

// String renders the kind for error messages and the admin API.
func (k Kind) String() string {
	switch k {
	case KindBool:
		return "bool"
	case KindRatio:
		return "ratio"
	case KindMoney:
		return "money_minor"
	case KindDistance:
		return "distance_m"
	case KindDuration:
		return "duration_s"
	case KindCount:
		return "count"
	default:
		return "unknown"
	}
}

// Unit is how the value is displayed to an admin.
func (k Kind) Unit() string {
	switch k {
	case KindMoney:
		return "BDT"
	case KindDistance:
		return "m"
	case KindDuration:
		return "s"
	case KindRatio:
		return "×"
	case KindBool, KindCount:
		return ""
	default:
		return ""
	}
}

// Value is one configuration value.
//
// It is a closed struct with unexported fields so a value can only be built
// through a constructor that fixes its kind. A bare Value{} is a valid count of
// zero rather than an undefined state.
type Value struct {
	kind  Kind
	whole int64
	ratio float64
	flag  bool
}

// Count builds a plain whole number.
func Count(n int64) (Value, error) {
	if n < 0 {
		return Value{}, fmt.Errorf("%w: count %d", ErrNegative, n)
	}
	return Value{kind: KindCount, whole: n}, nil
}

// Bool builds a switch.
func Bool(b bool) Value { return Value{kind: KindBool, flag: b} }

// Ratio builds a multiplier. A negative multiplier has no meaning for any
// variable in Appendix B and would silently invert a fee.
func Ratio(f float64) (Value, error) {
	if f < 0 {
		return Value{}, fmt.Errorf("%w: ratio %v", ErrNegative, f)
	}
	return Value{kind: KindRatio, ratio: f}, nil
}

// Money builds an amount in minor units.
func Money(minor int64) (Value, error) {
	if minor < 0 {
		return Value{}, fmt.Errorf("%w: money %d", ErrNegative, minor)
	}
	return Value{kind: KindMoney, whole: minor}, nil
}

// Distance builds a length in metres.
func Distance(metres int64) (Value, error) {
	if metres < 0 {
		return Value{}, fmt.Errorf("%w: distance %d", ErrNegative, metres)
	}
	return Value{kind: KindDistance, whole: metres}, nil
}

// Duration builds a length of time in seconds.
func Duration(seconds int64) (Value, error) {
	if seconds < 0 {
		return Value{}, fmt.Errorf("%w: duration %d", ErrNegative, seconds)
	}
	return Value{kind: KindDuration, whole: seconds}, nil
}

// Kind reports the value's type.
func (v Value) Kind() Kind { return v.kind }

// Int returns a count, money, distance or duration as a whole number.
//
// It returns an error rather than a zero for the wrong kind: a silent zero for
// a delivery fee read as a radius is a bug that ships.
func (v Value) Int() (int64, error) {
	switch v.kind {
	case KindCount, KindMoney, KindDistance, KindDuration:
		return v.whole, nil
	}
	return 0, fmt.Errorf("%w: %s is not a whole number", ErrKindMismatch, v.kind)
}

// Ratio returns a multiplier.
func (v Value) Ratio() (float64, error) {
	if v.kind != KindRatio {
		return 0, fmt.Errorf("%w: %s is not a ratio", ErrKindMismatch, v.kind)
	}
	return v.ratio, nil
}

// Bool returns a switch.
func (v Value) Bool() (bool, error) {
	if v.kind != KindBool {
		return false, fmt.Errorf("%w: %s is not a bool", ErrKindMismatch, v.kind)
	}
	return v.flag, nil
}

// String renders the canonical stored form.
//
// This is what goes in the database and the audit log, so it has to round-trip
// exactly through Parse. A ratio uses 'g' with full precision for that reason:
// a rounded multiplier that reads back as a different number would silently
// change every fee in an area.
func (v Value) String() string {
	switch v.kind {
	case KindBool:
		return strconv.FormatBool(v.flag)
	case KindRatio:
		return strconv.FormatFloat(v.ratio, 'g', -1, 64)
	}
	return strconv.FormatInt(v.whole, 10)
}

// Parse reads a stored value back as the given kind.
func Parse(kind Kind, raw string) (Value, error) {
	text := strings.TrimSpace(raw)
	switch kind {
	case KindBool:
		b, err := strconv.ParseBool(text)
		if err != nil {
			return Value{}, fmt.Errorf("%w: %q as bool", ErrUnparseable, raw)
		}
		return Bool(b), nil
	case KindRatio:
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return Value{}, fmt.Errorf("%w: %q as ratio", ErrUnparseable, raw)
		}
		return Ratio(f)
	case KindCount, KindMoney, KindDistance, KindDuration:
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return Value{}, fmt.Errorf("%w: %q as %s", ErrUnparseable, raw, kind)
		}
		// The constructors re-apply the non-negative rule, so a row edited by
		// hand cannot install a negative fee on the way back in.
		switch kind {
		case KindMoney:
			return Money(n)
		case KindDistance:
			return Distance(n)
		case KindDuration:
			return Duration(n)
		}
		return Count(n)
	default:
		return Value{}, fmt.Errorf("%w: unknown kind %d", ErrUnparseable, kind)
	}
}

// Equal reports whether two values are the same kind and the same value.
//
// Used to decide whether a "change" is really a change: writing an audit entry
// for a no-op set would fill the log with noise and bury the real ones.
func (v Value) Equal(other Value) bool {
	if v.kind != other.kind {
		return false
	}
	switch v.kind {
	case KindBool:
		return v.flag == other.flag
	case KindRatio:
		return v.ratio == other.ratio
	}
	return v.whole == other.whole
}

// Compare orders two values of the same kind: -1, 0 or 1.
//
// Bounds checking needs it. Bools are unordered and report an error rather than
// a made-up answer.
func (v Value) Compare(other Value) (int, error) {
	if v.kind != other.kind {
		return 0, fmt.Errorf("%w: cannot compare %s with %s", ErrKindMismatch, v.kind, other.kind)
	}
	switch v.kind {
	case KindRatio:
		switch {
		case v.ratio < other.ratio:
			return -1, nil
		case v.ratio > other.ratio:
			return 1, nil
		default:
			return 0, nil
		}
	case KindCount, KindMoney, KindDistance, KindDuration:
		switch {
		case v.whole < other.whole:
			return -1, nil
		case v.whole > other.whole:
			return 1, nil
		default:
			return 0, nil
		}
	}
	// Bools are the only unordered kind. Reporting a made-up answer would let a
	// bounds check silently pass on a value it never really compared.
	return 0, fmt.Errorf("%w: %s values have no order", ErrKindMismatch, v.kind)
}
