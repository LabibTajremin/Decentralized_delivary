package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/token"
)

// The access token is the credential every request carries. These tests are
// written against the ways JWT implementations are actually broken in the wild,
// not against the happy path alone.

var signingKey = []byte("a-test-signing-key-of-at-least-32-bytes")

var issuedAt = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func newSigner(t *testing.T, now time.Time) *token.Signer {
	t.Helper()
	s, err := token.NewSigner(signingKey, "goklay-test", token.WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return s
}

func sampleClaims() domain.Claims {
	return domain.Claims{
		UserID:    "usr_1",
		Role:      domain.RoleCustomer,
		SessionID: "ses_1",
		TokenID:   "jti_1",
		IssuedAt:  issuedAt,
		ExpiresAt: issuedAt.Add(15 * time.Minute),
	}
}

func TestATokenRoundTrips(t *testing.T) {
	signer := newSigner(t, issuedAt)
	signed, err := signer.Sign(sampleClaims())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	got, err := signer.Verify(signed)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.UserID != "usr_1" || got.Role != domain.RoleCustomer || got.SessionID != "ses_1" {
		t.Errorf("claims = %+v", got)
	}
	if got.TokenID != "jti_1" || got.Issuer != "goklay-test" {
		t.Errorf("claims = %+v", got)
	}
	if !got.ExpiresAt.Equal(issuedAt.Add(15 * time.Minute)) {
		t.Errorf("expiry = %v", got.ExpiresAt)
	}
}

// TestTheNoneAlgorithmIsRefused is the classic JWT hole: a library that honours
// the header's `alg` will accept a token with no signature at all.
func TestTheNoneAlgorithmIsRefused(t *testing.T) {
	signer := newSigner(t, issuedAt)
	enc := base64.RawURLEncoding

	header, _ := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{
		"sub": "usr_attacker", "role": "admin", "sid": "ses_x",
		"iat": issuedAt.Unix(), "exp": issuedAt.Add(time.Hour).Unix(),
		"iss": "goklay-test",
	})
	forged := enc.EncodeToString(header) + "." + enc.EncodeToString(payload) + "."

	if _, err := signer.Verify(forged); err == nil {
		t.Fatal("a token with alg=none was accepted")
	}
}

// An attacker who knows the server uses HMAC may try RS256, hoping the code
// verifies an RSA signature against what it thinks is a public key — which here
// is the HMAC secret.
func TestAnotherAlgorithmIsRefused(t *testing.T) {
	signer := newSigner(t, issuedAt)
	enc := base64.RawURLEncoding

	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{
		"sub": "usr_1", "role": "admin", "sid": "ses_1",
		"iat": issuedAt.Unix(), "exp": issuedAt.Add(time.Hour).Unix(), "iss": "goklay-test",
	})
	signing := enc.EncodeToString(header) + "." + enc.EncodeToString(payload)
	// Even with a correct HMAC over the content, the algorithm is wrong.
	valid, _ := signer.Sign(sampleClaims())
	forged := signing + "." + strings.Split(valid, ".")[2]

	if _, err := signer.Verify(forged); !errors.Is(err, token.ErrUnsupportedAlgorithm) {
		t.Errorf("error = %v, want ErrUnsupportedAlgorithm", err)
	}
}

// TestATamperedPayloadIsRefused: the signature is checked before anything in
// the payload is believed, so escalating a role in the body changes nothing.
func TestATamperedPayloadIsRefused(t *testing.T) {
	signer := newSigner(t, issuedAt)
	signed, err := signer.Sign(sampleClaims())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	parts := strings.Split(signed, ".")
	enc := base64.RawURLEncoding
	body, _ := enc.DecodeString(parts[1])
	escalated := strings.Replace(string(body), `"role":"customer"`, `"role":"admin"`, 1)
	if escalated == string(body) {
		t.Fatal("the test did not manage to alter the role; the payload shape has changed")
	}
	tampered := parts[0] + "." + enc.EncodeToString([]byte(escalated)) + "." + parts[2]

	if _, err := signer.Verify(tampered); !errors.Is(err, token.ErrBadSignature) {
		t.Errorf("error = %v, want ErrBadSignature", err)
	}
}

func TestATokenSignedWithAnotherKeyIsRefused(t *testing.T) {
	mine := newSigner(t, issuedAt)
	theirs, err := token.NewSigner([]byte("a-completely-different-key-32-bytes-x"), "goklay-test",
		token.WithClock(func() time.Time { return issuedAt }))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	foreign, err := theirs.Sign(sampleClaims())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := mine.Verify(foreign); !errors.Is(err, token.ErrBadSignature) {
		t.Errorf("error = %v, want ErrBadSignature", err)
	}
}

// TestAStagingTokenCannotBeReplayedAtProduction is why `iss` is checked.
func TestATokenFromAnotherEnvironmentIsRefused(t *testing.T) {
	staging, err := token.NewSigner(signingKey, "goklay-staging",
		token.WithClock(func() time.Time { return issuedAt }))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	stagingToken, err := staging.Sign(sampleClaims())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	production := newSigner(t, issuedAt)
	if _, err := production.Verify(stagingToken); !errors.Is(err, token.ErrWrongIssuer) {
		t.Errorf("error = %v, want ErrWrongIssuer", err)
	}
}

// TestAnExpiredTokenIsRefusedWithItsOwnError: the middleware branches on this
// to tell the client "refresh and retry" rather than "sign in again", which is
// what makes auto-login invisible.
func TestAnExpiredTokenIsRefusedWithItsOwnError(t *testing.T) {
	signer := newSigner(t, issuedAt)
	signed, err := signer.Sign(sampleClaims())
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	later := newSigner(t, issuedAt.Add(20*time.Minute))
	_, err = later.Verify(signed)
	if !errors.Is(err, domain.ErrAccessTokenExpired) {
		t.Errorf("error = %v, want domain.ErrAccessTokenExpired", err)
	}
	if !errors.Is(err, token.ErrExpiredToken) {
		t.Errorf("error = %v, want the signer's alias to match too", err)
	}
}

// A token a few seconds past expiry is still accepted, because servers do not
// share a clock to the millisecond. The leeway is small on purpose: a generous
// window extends the life of every revoked token by that much.
func TestSmallClockSkewIsTolerated(t *testing.T) {
	signer := newSigner(t, issuedAt)
	signed, _ := signer.Sign(sampleClaims())

	justPast := newSigner(t, issuedAt.Add(15*time.Minute+10*time.Second))
	if _, err := justPast.Verify(signed); err != nil {
		t.Errorf("a token 10s past expiry was refused: %v", err)
	}
}

func TestMalformedTokensAreRefused(t *testing.T) {
	signer := newSigner(t, issuedAt)
	for _, bad := range []string{
		"", "   ", "not.a.token", "onlyonepart",
		"a.b", "a.b.c.d",
		"!!!.!!!.!!!",
		"eyJhbGciOiJIUzI1NiJ9.!!!notbase64!!!.sig",
	} {
		if _, err := signer.Verify(bad); err == nil {
			t.Errorf("Verify(%q) was accepted", bad)
		}
	}
}

// TestAShortKeyIsRefusedAtConstruction: HS256 is HMAC-SHA256, so a key shorter
// than its 256-bit output is the weak link.
func TestAShortKeyIsRefusedAtConstruction(t *testing.T) {
	if _, err := token.NewSigner([]byte("too short"), "goklay"); !errors.Is(err, token.ErrShortKey) {
		t.Errorf("error = %v, want ErrShortKey", err)
	}
}

func TestAnIssuerIsRequired(t *testing.T) {
	if _, err := token.NewSigner(signingKey, "  "); err == nil {
		t.Error("a signer with no issuer was built; tokens would then be replayable across environments")
	}
}

// The signer copies its key, so a caller that later scribbles on the slice it
// passed cannot change how tokens verify.
func TestTheSigningKeyIsCopied(t *testing.T) {
	key := append([]byte(nil), signingKey...)
	signer, err := token.NewSigner(key, "goklay-test", token.WithClock(func() time.Time { return issuedAt }))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	signed, _ := signer.Sign(sampleClaims())

	for i := range key {
		key[i] = 'x'
	}
	if _, err := signer.Verify(signed); err != nil {
		t.Errorf("verification broke after the caller's key slice was overwritten: %v", err)
	}
}

func TestSigningRejectsIncompleteClaims(t *testing.T) {
	signer := newSigner(t, issuedAt)

	noUser := sampleClaims()
	noUser.UserID = ""
	if _, err := signer.Sign(noUser); err == nil {
		t.Error("a token with no subject was signed")
	}

	noSession := sampleClaims()
	noSession.SessionID = ""
	if _, err := signer.Sign(noSession); err == nil {
		t.Error("a token with no session was signed")
	}

	badRole := sampleClaims()
	badRole.Role = "superuser"
	if _, err := signer.Sign(badRole); !errors.Is(err, domain.ErrUnknownRole) {
		t.Errorf("error = %v, want ErrUnknownRole", err)
	}
}

// A correctly-signed token whose role claim is not one of the four is refused
// rather than defaulted. Defaulting an unknown role to customer would grant
// access to anyone who could get a typo into a claim.
func TestACorrectlySignedTokenWithAnUnknownRoleIsRefused(t *testing.T) {
	signer := newSigner(t, issuedAt)
	enc := base64.RawURLEncoding

	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{
		"sub": "usr_1", "role": "superuser", "sid": "ses_1", "jti": "jti_1",
		"iat": issuedAt.Unix(), "exp": issuedAt.Add(time.Hour).Unix(),
		"iss": "goklay-test",
	})
	signing := enc.EncodeToString(header) + "." + enc.EncodeToString(payload)

	// Signed with the real key, so the signature check passes and the role
	// check is the only thing standing between this token and admin access.
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(signing))
	forged := signing + "." + enc.EncodeToString(mac.Sum(nil))

	if _, err := signer.Verify(forged); !errors.Is(err, domain.ErrUnknownRole) {
		t.Errorf("error = %v, want ErrUnknownRole", err)
	}
}

func TestWithLeewayIsApplied(t *testing.T) {
	signer, err := token.NewSigner(signingKey, "goklay-test",
		token.WithClock(func() time.Time { return issuedAt.Add(16 * time.Minute) }),
		token.WithLeeway(5*time.Minute))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	base := newSigner(t, issuedAt)
	signed, _ := base.Sign(sampleClaims())

	if _, err := signer.Verify(signed); err != nil {
		t.Errorf("a token inside a 5 minute leeway was refused: %v", err)
	}
}

// TestAValidSignatureOverAMalformedPayloadIsRejected.
//
// The signature covers the raw base64 text, not the decoded bytes, so someone
// holding the key could sign a payload segment that is not valid base64 or not
// valid JSON. The signature check passing does not mean the payload is
// readable, and treating it as readable is how a verifier ends up acting on
// nothing.
func TestAValidSignatureOverAMalformedPayloadIsRejected(t *testing.T) {
	signer := newSigner(t, issuedAt)
	enc := base64.RawURLEncoding

	for name, segment := range map[string]string{
		"not base64": "!!!not-base64!!!",
		"not json":   enc.EncodeToString([]byte("this is not json")),
	} {
		signing := encodedTestHeader + "." + segment
		mac := hmac.New(sha256.New, signingKey)
		mac.Write([]byte(signing))
		forged := signing + "." + enc.EncodeToString(mac.Sum(nil))

		if _, err := signer.Verify(forged); !errors.Is(err, token.ErrMalformedToken) {
			t.Errorf("%s: error = %v, want ErrMalformedToken", name, err)
		}
	}
}

// encodedTestHeader is {"alg":"HS256","typ":"JWT"}, the header the signer emits.
const encodedTestHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
