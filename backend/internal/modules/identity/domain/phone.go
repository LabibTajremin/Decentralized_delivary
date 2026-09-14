// Package domain holds the identity model: who someone is, how they prove it,
// and what they are allowed to do.
package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Errors returned when building a phone number.
var (
	// ErrEmptyPhone means no number was given.
	ErrEmptyPhone = errors.New("phone number is required")
	// ErrInvalidPhone means the number is not a Bangladeshi mobile number.
	ErrInvalidPhone = errors.New("not a valid Bangladeshi mobile number")
)

// Phone is a validated Bangladeshi mobile number in E.164 form.
//
// It is a distinct type rather than a string because a phone number is a
// credential here: it is the rate-limit key, the OTP key, and the account
// identity. Two spellings of one number — 01712345678 and +8801712345678 —
// would otherwise be two separate rate-limit buckets, and an attacker could
// double their allowance by alternating format.
type Phone struct {
	e164 string
}

// bdMobilePrefixes are the operator codes in use: Grameenphone (17, 13),
// Robi (18), Airtel (16), Banglalink (19, 14) and Teletalk (15).
var bdMobilePrefixes = []string{"13", "14", "15", "16", "17", "18", "19"}

// NewPhone parses and normalises a Bangladeshi mobile number.
//
// It accepts the three spellings people actually type — 01712345678,
// 8801712345678 and +8801712345678 — and stores exactly one of them.
func NewPhone(raw string) (Phone, error) {
	digits := keepDigits(raw)
	if digits == "" {
		return Phone{}, ErrEmptyPhone
	}

	// Reduce every accepted spelling to the 10 digits after the country code.
	var national string
	switch {
	case strings.HasPrefix(digits, "880") && len(digits) == 13:
		national = digits[3:]
	case strings.HasPrefix(digits, "0") && len(digits) == 11:
		national = digits[1:]
	case len(digits) == 10:
		national = digits
	default:
		return Phone{}, fmt.Errorf("%w: %q", ErrInvalidPhone, raw)
	}

	if national[0] != '1' {
		return Phone{}, fmt.Errorf("%w: %q", ErrInvalidPhone, raw)
	}
	prefix := national[:2]
	known := false
	for _, p := range bdMobilePrefixes {
		if prefix == p {
			known = true
			break
		}
	}
	if !known {
		return Phone{}, fmt.Errorf("%w: %q is not a known operator prefix", ErrInvalidPhone, raw)
	}

	return Phone{e164: "+880" + national}, nil
}

// keepDigits strips spaces, dashes, brackets and a leading plus.
func keepDigits(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// String returns the E.164 form, which is what goes in Redis keys and logs.
func (p Phone) String() string { return p.e164 }

// IsZero reports whether this is the zero value.
func (p Phone) IsZero() bool { return p.e164 == "" }

// Masked returns the number with the middle hidden, for anything a user or an
// operator sees: "+88017*****678".
//
// Logs and error messages use this. A full phone number in a log line is
// personal data sitting in a system many people can read, and an OTP screen
// showing the whole number tells an attacker who has guessed a number that they
// guessed right.
func (p Phone) Masked() string {
	if len(p.e164) < 8 {
		return p.e164
	}
	return p.e164[:6] + strings.Repeat("*", len(p.e164)-9) + p.e164[len(p.e164)-3:]
}
