package identityhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/token"
	identityhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/transport/http"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
)

// The middleware is the only thing standing between a request and every
// endpoint in the system, so these tests are about what it refuses.

var signingKey = []byte("a-test-signing-key-of-at-least-32-bytes")

var now = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func newSigner(t *testing.T, at time.Time) *token.Signer {
	t.Helper()
	s, err := token.NewSigner(signingKey, "goklay-test", token.WithClock(func() time.Time { return at }))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return s
}

func tokenFor(t *testing.T, signer *token.Signer, role domain.Role, userID string) string {
	t.Helper()
	signed, err := signer.Sign(domain.Claims{
		UserID:    userID,
		Role:      role,
		SessionID: "ses_1",
		TokenID:   "jti_1",
		IssuedAt:  now,
		ExpiresAt: now.Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return signed
}

// guarded builds a handler behind a role requirement and reports what reached it.
func guarded(t *testing.T, signer *token.Signer, roles ...domain.Role) (http.Handler, *domain.Principal) {
	t.Helper()
	var seen domain.Principal
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := identityhttp.PrincipalFrom(r.Context()); ok {
			seen = p
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "reached"})
	})
	auth := identityhttp.NewAuthenticator(signer)
	return auth.Require(roles...)(inner), &seen
}

func call(h http.Handler, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body.String())
	}
	return body.Error.Code
}

func TestAValidTokenReachesTheHandlerWithItsIdentity(t *testing.T) {
	signer := newSigner(t, now)
	h, seen := guarded(t, signer, domain.RoleCustomer)

	rec := call(h, "Bearer "+tokenFor(t, signer, domain.RoleCustomer, "usr_1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if seen.UserID != "usr_1" || seen.Role != domain.RoleCustomer || seen.SessionID != "ses_1" {
		t.Errorf("principal = %+v", *seen)
	}
}

func TestNoTokenIsUnauthorized(t *testing.T) {
	signer := newSigner(t, now)
	h, _ := guarded(t, signer, domain.RoleCustomer)

	rec := call(h, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if got := errorCode(t, rec); got != "missing_token" {
		t.Errorf("code = %q", got)
	}
}

func TestAMalformedAuthorizationHeaderIsRejected(t *testing.T) {
	signer := newSigner(t, now)
	h, _ := guarded(t, signer, domain.RoleCustomer)
	valid := tokenFor(t, signer, domain.RoleCustomer, "usr_1")

	for _, header := range []string{
		valid,            // no scheme
		"Basic " + valid, // wrong scheme
		"Bearer",         // no token
		"Bearer   ",      // blank token
		"Bearer  " + valid + " extra",
	} {
		rec := call(h, header)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("header %q: status = %d, want 401", header, rec.Code)
		}
	}
}

// The scheme is case-insensitive per RFC 7235, and clients really do send
// "bearer".
func TestTheBearerSchemeIsCaseInsensitive(t *testing.T) {
	signer := newSigner(t, now)
	h, _ := guarded(t, signer, domain.RoleCustomer)

	if rec := call(h, "bearer "+tokenFor(t, signer, domain.RoleCustomer, "usr_1")); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for a lower-case scheme", rec.Code)
	}
}

// TestAnExpiredTokenGetsItsOwnCode: the client handles this one silently — one
// refresh and a retry — which is what makes auto-login invisible. Everything
// else means "sign in again".
func TestAnExpiredTokenGetsItsOwnCode(t *testing.T) {
	issuer := newSigner(t, now)
	expired := tokenFor(t, issuer, domain.RoleCustomer, "usr_1")

	later := newSigner(t, now.Add(time.Hour))
	h, _ := guarded(t, later, domain.RoleCustomer)

	rec := call(h, "Bearer "+expired)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := errorCode(t, rec); got != "token_expired" {
		t.Errorf("code = %q, want token_expired so the client refreshes rather than signing out", got)
	}
}

func TestAForgedTokenIsRejected(t *testing.T) {
	signer := newSigner(t, now)
	h, _ := guarded(t, signer, domain.RoleCustomer)

	other, err := token.NewSigner([]byte("a-different-key-also-32-bytes-long--"), "goklay-test",
		token.WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	forged := tokenFor(t, other, domain.RoleAdmin, "usr_attacker")

	rec := call(h, "Bearer "+forged)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if got := errorCode(t, rec); got != "invalid_token" {
		t.Errorf("code = %q", got)
	}
}

// TestTheWrongRoleIsForbidden: a partner token must not reach a merchant
// endpoint.
func TestTheWrongRoleIsForbidden(t *testing.T) {
	signer := newSigner(t, now)
	h, _ := guarded(t, signer, domain.RoleMerchant)

	rec := call(h, "Bearer "+tokenFor(t, signer, domain.RolePartner, "usr_partner"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if got := errorCode(t, rec); got != "insufficient_role" {
		t.Errorf("code = %q", got)
	}
}

// TestAdminIsNotImplicitlyAllowedEverywhere: an admin token reaching a partner
// endpoint is nearly always a mis-wired route or a confused client, and
// silently permitting it hides the bug.
func TestAdminIsNotImplicitlyAllowedEverywhere(t *testing.T) {
	signer := newSigner(t, now)
	h, _ := guarded(t, signer, domain.RolePartner)

	rec := call(h, "Bearer "+tokenFor(t, signer, domain.RoleAdmin, "usr_admin"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403; admin must not be a silent superset", rec.Code)
	}
}

// Where an admin genuinely acts for another role, the route lists both.
func TestARouteMayAllowSeveralRoles(t *testing.T) {
	signer := newSigner(t, now)
	h, _ := guarded(t, signer, domain.RoleMerchant, domain.RoleAdmin)

	for _, role := range []domain.Role{domain.RoleMerchant, domain.RoleAdmin} {
		if rec := call(h, "Bearer "+tokenFor(t, signer, role, "usr_1")); rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", role, rec.Code)
		}
	}
	if rec := call(h, "Bearer "+tokenFor(t, signer, domain.RoleCustomer, "usr_1")); rec.Code != http.StatusForbidden {
		t.Errorf("customer: status = %d, want 403", rec.Code)
	}
}

// The response does not name the role it wanted: telling an attacker that an
// endpoint requires "admin" maps the system for them.
func TestTheForbiddenResponseDoesNotNameTheRequiredRole(t *testing.T) {
	signer := newSigner(t, now)
	h, _ := guarded(t, signer, domain.RoleAdmin)

	rec := call(h, "Bearer "+tokenFor(t, signer, domain.RoleCustomer, "usr_1"))
	if body := rec.Body.String(); contains(body, "admin") {
		t.Errorf("the 403 body names the required role: %s", body)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

// Authenticated() is written as "every role" rather than "skip the check", so
// adding a fifth role forces a decision rather than silently widening access.
func TestAuthenticatedAdmitsEveryRoleAndNoneWithoutAToken(t *testing.T) {
	signer := newSigner(t, now)
	auth := identityhttp.NewAuthenticator(signer)
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "reached"})
	})
	h := auth.Authenticated()(inner)

	for _, role := range domain.AllRoles() {
		if rec := call(h, "Bearer "+tokenFor(t, signer, role, "usr_1")); rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", role, rec.Code)
		}
	}
	if rec := call(h, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: status = %d, want 401", rec.Code)
	}
}

// A Require() with no roles would admit nobody and look like a broken endpoint
// rather than a broken guard, so it fails loudly at wiring time instead.
func TestRequireWithNoRolesPanicsAtWiringTime(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Require() with no roles did not panic")
		}
	}()
	identityhttp.NewAuthenticator(newSigner(t, now)).Require()
}

func TestPrincipalFromIsAbsentWithoutAuthentication(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, ok := identityhttp.PrincipalFrom(req.Context()); ok {
		t.Error("an unauthenticated request reported a principal")
	}
}

func TestWithPrincipalRoundTrips(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	want := domain.Principal{UserID: "usr_1", Role: domain.RoleAdmin, SessionID: "ses_1"}
	ctx := identityhttp.WithPrincipal(req.Context(), want)

	got, ok := identityhttp.PrincipalFrom(ctx)
	if !ok || got != want {
		t.Errorf("principal = %+v, %v", got, ok)
	}
}
