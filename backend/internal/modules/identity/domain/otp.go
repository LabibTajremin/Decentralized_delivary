package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Errors returned when working with one-time codes.
var (
	// ErrInvalidOTPFormat means the submitted code is not six digits.
	ErrInvalidOTPFormat = errors.New("the code must be six digits")
	// ErrOTPMismatch means the code does not match the one issued.
	ErrOTPMismatch = errors.New("that code is not correct")
)

// OTPLength is the number of digits in a one-time code.
//
// Six digits is a million possibilities. On its own that is weak; combined with
// the five-attempt cap and the lockout window it is not, which is why the cap
// is a hard requirement of this design rather than a nicety.
const OTPLength = 6

// OTP is a one-time code.
//
// The plaintext exists only long enough to be sent. What is stored is the hash,
// so a dump of the session store does not let an attacker sign in as anyone:
// the same reasoning as never storing a password.
type OTP struct {
	code string
}

// GenerateOTP produces a uniformly random six-digit code.
//
// Rejection sampling rather than modulo: taking a random byte mod 10 makes the
// digits 0-5 slightly more likely than 6-9, and a code generator with a
// measurable bias is a code generator worth attacking.
func GenerateOTP(entropy io.Reader) (OTP, error) {
	if entropy == nil {
		entropy = rand.Reader
	}
	digits := make([]byte, 0, OTPLength)
	buf := make([]byte, 1)
	for len(digits) < OTPLength {
		if _, err := io.ReadFull(entropy, buf); err != nil {
			return OTP{}, fmt.Errorf("generate otp: %w", err)
		}
		// 250 is the largest multiple of 10 below 256; anything above is
		// discarded so every digit is equally likely.
		if buf[0] >= 250 {
			continue
		}
		digits = append(digits, '0'+buf[0]%10)
	}
	return OTP{code: string(digits)}, nil
}

// ParseOTP validates a code submitted by a user.
func ParseOTP(raw string) (OTP, error) {
	code := strings.TrimSpace(raw)
	if len(code) != OTPLength {
		return OTP{}, ErrInvalidOTPFormat
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return OTP{}, ErrInvalidOTPFormat
		}
	}
	return OTP{code: code}, nil
}

// String returns the plaintext code. Only the SMS sender should call it.
func (o OTP) String() string { return o.code }

// Hash returns the stored form: SHA-256 of the code, salted with the phone
// number.
//
// The phone salt matters. Without it, six-digit codes have only a million
// possible hashes, so one precomputed table would reverse every OTP in the
// store at once. With it, an attacker must build a table per number, which
// makes a store dump worth far less than the effort.
func (o OTP) Hash(phone Phone) string {
	sum := sha256.Sum256([]byte(phone.String() + ":" + o.code))
	return hex.EncodeToString(sum[:])
}

// Matches reports whether a submitted code matches a stored hash.
//
// The comparison is constant-time. A byte-by-byte comparison leaks how many
// leading digits were right through its timing, which turns a million-guess
// search into about sixty.
func (o OTP) Matches(phone Phone, storedHash string) bool {
	computed := o.Hash(phone)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(storedHash)) == 1
}
