package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Errors returned when working with sessions and refresh tokens.
var (
	// ErrInvalidRefreshToken means the presented token is not one we issued, or
	// has already been rotated away.
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	// ErrTokenReuseDetected means a token that was already rotated has been
	// presented again. It is treated as theft, not as a mistake.
	ErrTokenReuseDetected = errors.New("refresh token reuse detected")
	// ErrSessionNotFound means the session has expired or been revoked.
	ErrSessionNotFound = errors.New("session not found")
	// ErrMalformedToken means the presented string is not shaped like a token
	// we ever issue.
	ErrMalformedToken = errors.New("malformed refresh token")

	// ErrAccessTokenExpired means a well-formed, correctly signed access token
	// is past its expiry.
	//
	// It lives in the domain because both sides need it and neither may import
	// the other: the signer returns it, and the middleware branches on it to
	// tell the client "refresh and retry" rather than "sign in again". That
	// distinction is what makes auto-login invisible, so it is a domain
	// concept rather than an implementation detail of one signer.
	ErrAccessTokenExpired = errors.New("access token has expired")
)

// refreshTokenBytes is the entropy in a refresh token.
//
// 256 bits. A refresh token is a bearer credential valid for sixty days, so it
// has to be unguessable for sixty days against an attacker who can try
// continuously.
const refreshTokenBytes = 32

// RefreshToken is an opaque bearer credential.
//
// Opaque, not a JWT. There is nothing a client needs to read inside it, and a
// self-describing long-lived token invites someone to trust its claims without
// checking the server. Its only meaning is "the server recognises this".
type RefreshToken struct {
	plaintext string
}

// GenerateRefreshToken produces a new random token.
func GenerateRefreshToken(entropy io.Reader) (RefreshToken, error) {
	if entropy == nil {
		entropy = rand.Reader
	}
	buf := make([]byte, refreshTokenBytes)
	if _, err := io.ReadFull(entropy, buf); err != nil {
		return RefreshToken{}, fmt.Errorf("generate refresh token: %w", err)
	}
	// URL-safe and unpadded so the token survives being put in a header, a
	// query string or a JSON body without re-encoding.
	return RefreshToken{plaintext: base64.RawURLEncoding.EncodeToString(buf)}, nil
}

// ParseRefreshToken validates the shape of a presented token.
//
// Checking the shape before touching Redis means a flood of garbage costs a
// string length check rather than a network round trip each.
func ParseRefreshToken(raw string) (RefreshToken, error) {
	token := strings.TrimSpace(raw)
	if token == "" {
		return RefreshToken{}, ErrMalformedToken
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != refreshTokenBytes {
		return RefreshToken{}, ErrMalformedToken
	}
	return RefreshToken{plaintext: token}, nil
}

// String returns the plaintext. Only the response writer should call it: this
// value goes to the client once and is never stored server-side.
func (t RefreshToken) String() string { return t.plaintext }

// Hash returns the stored form.
//
// Only the hash is stored, exactly as for a password. Someone who reads the
// session store learns which sessions exist but cannot use any of them, because
// a hash cannot be presented as a token.
//
// Unlike the OTP hash this needs no salt: the token already carries 256 bits of
// entropy, so there is no dictionary to precompute.
func (t RefreshToken) Hash() string {
	sum := sha256.Sum256([]byte(t.plaintext))
	return hex.EncodeToString(sum[:])
}

// Session is one signed-in device.
type Session struct {
	ID     string
	UserID string
	Role   Role
	// Device is a free-text label the client sends, shown in the "active
	// devices" list so a user can recognise which one to revoke.
	Device     string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

// IsExpired reports whether the session has passed its expiry.
//
// Redis TTLs do the real work — an expired session simply stops existing — but
// this is checked as well, because a store that has not yet reclaimed a key
// must not hand back a session that should be gone.
func (s Session) IsExpired(now time.Time) bool {
	return !s.ExpiresAt.IsZero() && !now.Before(s.ExpiresAt)
}

// TokenPair is what a client receives on sign-in and on every refresh.
type TokenPair struct {
	AccessToken  string
	RefreshToken RefreshToken
	// ExpiresIn is the access token's remaining life in seconds.
	//
	// The server states it rather than letting the client decode the JWT and
	// work it out. The client holds no business rule about when to refresh
	// (2.9): it reacts to this number and to a 401.
	ExpiresIn int64
	SessionID string
	Role      Role
	UserID    string
}

// Claims are what an access token carries.
//
// Deliberately minimal: an identity, a role, the session it came from, and the
// times. Anything else — a name, a phone number, a permission list — would be a
// copy of state that can change, embedded in a credential that cannot be
// recalled for fifteen minutes.
type Claims struct {
	UserID    string
	Role      Role
	SessionID string
	TokenID   string
	IssuedAt  time.Time
	ExpiresAt time.Time
	Issuer    string
}

// Principal turns verified claims into an authenticated caller.
func (c Claims) Principal() Principal {
	return Principal{UserID: c.UserID, Role: c.Role, SessionID: c.SessionID}
}
