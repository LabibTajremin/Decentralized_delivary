// Package ports declares what the identity use cases need from the outside
// world. Every implementation lives in infrastructure/.
package ports

import (
	"context"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
)

// OTPStore holds pending one-time codes and their attempt counters.
type OTPStore interface {
	// PutOTP stores a hashed code, replacing any pending one for this number.
	// Replacing rather than adding matters: two live codes for one number
	// doubles an attacker's chances for no user benefit.
	PutOTP(ctx context.Context, phone domain.Phone, hash string, ttl time.Duration) error

	// TakeOTP atomically reads and deletes the pending hash.
	//
	// Atomic because a read-then-delete lets two concurrent submissions of the
	// same correct code both succeed, creating two sessions from one code.
	// Returns ok=false when there is nothing pending.
	TakeOTP(ctx context.Context, phone domain.Phone) (hash string, ok bool, err error)

	// PeekOTP reads the pending hash without consuming it, for a wrong-code
	// attempt: a mistyped digit must not burn the user's real code.
	PeekOTP(ctx context.Context, phone domain.Phone) (hash string, ok bool, err error)

	// RecordFailedAttempt increments the failure counter and returns the new
	// total. The counter's TTL is the lockout window.
	RecordFailedAttempt(ctx context.Context, phone domain.Phone, window time.Duration) (int, error)

	// FailedAttempts returns the current failure count.
	FailedAttempts(ctx context.Context, phone domain.Phone) (int, error)

	// ClearAttempts resets the counter after a successful verification.
	ClearAttempts(ctx context.Context, phone domain.Phone) error

	// DeleteOTP removes a pending code, used when a number locks out so a
	// lockout cannot be waited out with the code still live.
	DeleteOTP(ctx context.Context, phone domain.Phone) error
}

// RotationOutcome says what happened when a refresh token was presented.
type RotationOutcome int

const (
	// RotationUnknownToken means the hash was never issued, or belongs to a
	// session that has expired.
	RotationUnknownToken RotationOutcome = iota
	// RotationReuse means the hash was issued but has already been rotated
	// away. Someone is presenting a spent token, which means a copy exists.
	RotationReuse
	// RotationOK means the swap succeeded.
	RotationOK
)

// SessionStore holds sessions and the refresh tokens that belong to them.
//
// The rotation operation is a single method rather than a read followed by a
// write, because it must be atomic: two refreshes racing from one device must
// produce one rotation, not two tokens or a false theft alarm.
type SessionStore interface {
	// CreateSession stores a new session and its first refresh token hash.
	CreateSession(ctx context.Context, session domain.Session, refreshHash string, ttl time.Duration) error

	// Rotate atomically exchanges one refresh token hash for another.
	//
	// Returns the session the token belonged to when the outcome is
	// RotationOK, and the session id to revoke when the outcome is
	// RotationReuse.
	Rotate(ctx context.Context, oldHash, newHash string, ttl time.Duration, now time.Time) (RotationOutcome, domain.Session, error)

	// Session returns one session by id.
	Session(ctx context.Context, sessionID string) (domain.Session, error)

	// UserSessions lists every active session for a user, oldest first, so the
	// eviction rule has a defined victim.
	UserSessions(ctx context.Context, userID string) ([]domain.Session, error)

	// RevokeSession deletes one session and every refresh token in its family.
	RevokeSession(ctx context.Context, sessionID string) error

	// RevokeUserSessions deletes every session for a user.
	RevokeUserSessions(ctx context.Context, userID string) error
}

// TokenSigner issues and verifies access tokens.
type TokenSigner interface {
	// Sign issues an access token for the given claims.
	Sign(claims domain.Claims) (string, error)

	// Verify checks a token's signature, issuer and expiry, and returns its
	// claims. It never consults a store: access tokens are stateless by design
	// so the common request path costs no round trip.
	Verify(token string) (domain.Claims, error)
}

// RateLimiter counts events in a fixed window.
type RateLimiter interface {
	// Allow records one event against a key and reports whether it is within
	// the limit, along with when the window resets.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)
}

// SMSSender delivers a one-time code.
type SMSSender interface {
	// SendOTP sends a code to a phone number. A delivery failure is reported:
	// telling a user "we have sent a code" when nothing was sent leaves them
	// waiting for a message that will never arrive.
	SendOTP(ctx context.Context, phone domain.Phone, code string) error
}

// UserDirectory maps a phone number to an account, creating one on first
// sign-in.
//
// Identity owns authentication, not the profile. The name, addresses and
// preferences live in the user module (P05); this interface is the seam between
// them, so identity never grows a user table of its own.
type UserDirectory interface {
	// EnsureUser returns the user id for a phone number and role, creating the
	// account if this is the first sign-in. The bool reports whether it was
	// created, which the client uses to decide whether to collect a name.
	EnsureUser(ctx context.Context, phone domain.Phone, role domain.Role) (userID string, created bool, err error)

	// PhoneFor is the reverse lookup: the number behind an account id.
	//
	// Added for notification's SMS fallback (P14) — a push that cannot be
	// delivered needs the one other channel this platform has for reaching a
	// person, and the number identity already collected at sign-in is that
	// channel. False for an id that does not exist or belongs to a suspended
	// account, so a stale reference cannot be used to text someone who closed
	// their account.
	PhoneFor(ctx context.Context, userID string) (phone domain.Phone, found bool, err error)
}
