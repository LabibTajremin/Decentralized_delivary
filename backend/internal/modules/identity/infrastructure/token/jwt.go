// Package token signs and verifies access tokens.
//
// The JWT is assembled by hand rather than with a library. The format is small
// and fully specified here, and hand-rolling it removes a dependency from the
// most security-sensitive path in the system — along with that library's own
// history of algorithm-confusion bugs. The one algorithm this accepts is
// HS256; see Verify for why that matters.
package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
)

// Errors returned when verifying a token.
var (
	// ErrMalformedToken means the token is not three base64 segments.
	ErrMalformedToken = errors.New("malformed access token")
	// ErrBadSignature means the signature does not match.
	ErrBadSignature = errors.New("access token signature is not valid")
	// ErrExpiredToken means the token is past its expiry. It is the domain's
	// error, re-exported here so callers of this package can use either name.
	ErrExpiredToken = domain.ErrAccessTokenExpired
	// ErrWrongIssuer means the token was issued by another environment.
	ErrWrongIssuer = errors.New("access token was issued by a different environment")
	// ErrUnsupportedAlgorithm means the header asks for anything but HS256.
	ErrUnsupportedAlgorithm = errors.New("unsupported token algorithm")
	// ErrShortKey means the signing key is too short to be safe.
	ErrShortKey = errors.New("signing key must be at least 32 bytes")
)

// minKeyBytes is the shortest signing key accepted.
//
// HS256 is HMAC-SHA256, so a key shorter than the 256-bit output is the weak
// link. Thirty-two bytes is the floor, enforced at construction rather than
// hoped for in configuration.
const minKeyBytes = 32

// header is the fixed JWT header. It is a constant rather than something parsed
// from the token, so there is nothing for an attacker to influence.
type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// encodedHeader is {"alg":"HS256","typ":"JWT"} pre-encoded.
//
// Precomputed because it never varies: marshalling a constant on every sign
// would add an error branch no test could reach, and this is also the hot path
// for every token issued.
const encodedHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

// payload is the wire form of the claims.
type payload struct {
	Sub       string `json:"sub"`
	Role      string `json:"role"`
	SessionID string `json:"sid"`
	JTI       string `json:"jti"`
	Iat       int64  `json:"iat"`
	Exp       int64  `json:"exp"`
	Iss       string `json:"iss"`
}

// Signer issues and verifies HS256 access tokens.
type Signer struct {
	key      []byte
	issuer   string
	leeway   time.Duration
	nowFunc  func() time.Time
	encoding *base64.Encoding
}

// Option configures a Signer.
type Option func(*Signer)

// WithClock replaces the time source, for tests.
func WithClock(now func() time.Time) Option {
	return func(s *Signer) { s.nowFunc = now }
}

// WithLeeway allows for clock skew between servers.
//
// Small on purpose: a generous window extends the life of every revoked token
// by that much, and the servers that matter here share a clock source.
func WithLeeway(d time.Duration) Option {
	return func(s *Signer) { s.leeway = d }
}

// NewSigner builds a signer.
func NewSigner(key []byte, issuer string, opts ...Option) (*Signer, error) {
	if len(key) < minKeyBytes {
		return nil, fmt.Errorf("%w: got %d", ErrShortKey, len(key))
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, errors.New("token issuer is required")
	}
	s := &Signer{
		key:      append([]byte(nil), key...),
		issuer:   issuer,
		leeway:   30 * time.Second,
		nowFunc:  time.Now,
		encoding: base64.RawURLEncoding,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Sign issues an access token.
func (s *Signer) Sign(claims domain.Claims) (string, error) {
	if claims.UserID == "" || claims.SessionID == "" {
		return "", errors.New("access token needs a subject and a session")
	}
	if !claims.Role.IsValid() {
		return "", fmt.Errorf("%w: %q", domain.ErrUnknownRole, claims.Role)
	}

	// A struct of strings and int64s cannot fail to marshal, so the error is
	// discarded rather than becoming a branch nothing can exercise.
	body, _ := json.Marshal(payload{
		Sub:       claims.UserID,
		Role:      claims.Role.String(),
		SessionID: claims.SessionID,
		JTI:       claims.TokenID,
		Iat:       claims.IssuedAt.Unix(),
		Exp:       claims.ExpiresAt.Unix(),
		Iss:       s.issuer,
	})

	signing := encodedHeader + "." + s.encoding.EncodeToString(body)
	return signing + "." + s.encoding.EncodeToString(s.mac(signing)), nil
}

// Verify checks a token and returns its claims.
//
// The order of checks is deliberate: the signature is verified before anything
// in the payload is believed. Reading claims first and checking the signature
// afterwards is how "none algorithm" bugs happen — the code acts on attacker-
// supplied data before establishing it came from us.
func (s *Signer) Verify(token string) (domain.Claims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return domain.Claims{}, ErrMalformedToken
	}

	headBytes, err := s.encoding.DecodeString(parts[0])
	if err != nil {
		return domain.Claims{}, ErrMalformedToken
	}
	var head header
	if err := json.Unmarshal(headBytes, &head); err != nil {
		return domain.Claims{}, ErrMalformedToken
	}
	// Only HS256. Accepting whatever the header asks for is the classic
	// algorithm-confusion hole: "none" would make every forged token valid, and
	// "RS256" against an HMAC key turns a public key into a signing key.
	if head.Alg != "HS256" {
		return domain.Claims{}, fmt.Errorf("%w: %q", ErrUnsupportedAlgorithm, head.Alg)
	}

	signature, err := s.encoding.DecodeString(parts[2])
	if err != nil {
		return domain.Claims{}, ErrMalformedToken
	}
	expected := s.mac(parts[0] + "." + parts[1])
	if !hmac.Equal(signature, expected) {
		return domain.Claims{}, ErrBadSignature
	}

	// Only now is the payload trusted.
	bodyBytes, err := s.encoding.DecodeString(parts[1])
	if err != nil {
		return domain.Claims{}, ErrMalformedToken
	}
	var body payload
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return domain.Claims{}, ErrMalformedToken
	}

	if body.Iss != s.issuer {
		return domain.Claims{}, fmt.Errorf("%w: %q", ErrWrongIssuer, body.Iss)
	}
	now := s.nowFunc()
	expiry := time.Unix(body.Exp, 0)
	if now.After(expiry.Add(s.leeway)) {
		return domain.Claims{}, ErrExpiredToken
	}

	role, err := domain.ParseRole(body.Role)
	if err != nil {
		return domain.Claims{}, err
	}

	return domain.Claims{
		UserID:    body.Sub,
		Role:      role,
		SessionID: body.SessionID,
		TokenID:   body.JTI,
		IssuedAt:  time.Unix(body.Iat, 0),
		ExpiresAt: expiry,
		Issuer:    body.Iss,
	}, nil
}

// mac computes the HMAC over the signing input.
func (s *Signer) mac(signingInput string) []byte {
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte(signingInput))
	return h.Sum(nil)
}
