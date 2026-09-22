package identity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/token"
	identityhttp "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/transport/http"
)

// The handler is tested over the real use cases with fake stores, so a mistake
// in wiring — a body field read into the wrong place, a role parsed after it is
// used — shows up here rather than in production.

var handlerKey = []byte("a-handler-test-signing-key-32-byte")

// httpRig serves the auth routes over the shared fakes.
type httpRig struct {
	*rig
	mux    *http.ServeMux
	signer *token.Signer
}

func newHTTPRig(t *testing.T) *httpRig {
	t.Helper()
	r := newRig()

	signer, err := token.NewSigner(handlerKey, "goklay-test",
		token.WithClock(func() time.Time { return signInAt }))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	mux := http.NewServeMux()
	identityhttp.NewHandler(
		r.request, r.verify, r.refresh, r.logout, r.sessList,
		identityhttp.NewAuthenticator(signer),
	).Register(mux)

	return &httpRig{rig: r, mux: mux, signer: signer}
}

// bearerFor mints a real access token for an identity the fakes know about.
func (h *httpRig) bearerFor(t *testing.T, userID, sessionID string, role domain.Role) string {
	t.Helper()
	signed, err := h.signer.Sign(domain.Claims{
		UserID: userID, Role: role, SessionID: sessionID, TokenID: "jti_1",
		IssuedAt: signInAt, ExpiresAt: signInAt.Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return signed
}

func (h *httpRig) do(method, target, body, bearer string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body.String())
	}
}

func responseCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, rec, &body)
	return body.Error.Code
}

func TestRequestingACodeOverHTTP(t *testing.T) {
	h := newHTTPRig(t)

	rec := h.do(http.MethodPost, "/v1/auth/otp/request", `{"phone":"01712345678"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Phone       string `json:"phone"`
		ExpiresIn   int64  `json:"expires_in"`
		ResendAfter int64  `json:"resend_after"`
	}
	decodeBody(t, rec, &body)

	if body.ExpiresIn != 300 {
		t.Errorf("expires_in = %d", body.ExpiresIn)
	}
	if body.ResendAfter <= 0 {
		t.Errorf("resend_after = %d; the client needs it to disable the resend button", body.ResendAfter)
	}
	// The response must not reveal the code or the full number.
	if strings.Contains(rec.Body.String(), h.sms.codes[0]) {
		t.Errorf("the response contains the code: %s", rec.Body.String())
	}
	if strings.Contains(body.Phone, "2345678") {
		t.Errorf("the response contains the full number: %q", body.Phone)
	}
}

// An area hint is optional: someone signing in has not chosen an address yet.
func TestTheAreaHintIsOptional(t *testing.T) {
	h := newHTTPRig(t)

	withHint := h.do(http.MethodPost, "/v1/auth/otp/request?area=DHK-DHM&district=DHK&division=DHA",
		`{"phone":"01712345678"}`, "")
	if withHint.Code != http.StatusOK {
		t.Errorf("with a placement: status = %d", withHint.Code)
	}
	without := h.do(http.MethodPost, "/v1/auth/otp/request", `{"phone":"01812345678"}`, "")
	if without.Code != http.StatusOK {
		t.Errorf("without a placement: status = %d", without.Code)
	}
}

func TestVerifyingACodeOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	if rec := h.do(http.MethodPost, "/v1/auth/otp/request", `{"phone":"01712345678"}`, ""); rec.Code != http.StatusOK {
		t.Fatalf("request: status %d", rec.Code)
	}

	rec := h.do(http.MethodPost, "/v1/auth/otp/verify", `{
		"phone":"01712345678","code":"`+h.sms.codes[0]+`","role":"customer","device":"Pixel 8"
	}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Role         string `json:"role"`
		NewUser      bool   `json:"new_user"`
	}
	decodeBody(t, rec, &body)

	if body.AccessToken == "" || body.RefreshToken == "" {
		t.Fatalf("body = %+v", body)
	}
	if body.TokenType != "Bearer" || body.ExpiresIn != 900 || body.Role != "customer" {
		t.Errorf("body = %+v", body)
	}
	if !body.NewUser {
		t.Error("a first sign-in was not flagged, so the app would not ask for a name")
	}
}

func TestVerifyRejectsAnUnknownRoleBeforeTouchingTheStore(t *testing.T) {
	h := newHTTPRig(t)
	rec := h.do(http.MethodPost, "/v1/auth/otp/verify", `{
		"phone":"01712345678","code":"123456","role":"superuser","device":"x"
	}`, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if got := responseCode(t, rec); got != "invalid_role" {
		t.Errorf("code = %q", got)
	}
}

func TestMalformedBodiesAreRejected(t *testing.T) {
	h := newHTTPRig(t)
	for _, target := range []string{"/v1/auth/otp/request", "/v1/auth/otp/verify", "/v1/auth/refresh"} {
		for _, body := range []string{`{"phone":`, `{"unknown_field":1}`} {
			rec := h.do(http.MethodPost, target, body, "")
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s with %s: status = %d, want 400", target, body, rec.Code)
			}
		}
	}
}

func TestRefreshingOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	pair := h.signIn(t)

	rec := h.do(http.MethodPost, "/v1/auth/refresh",
		`{"refresh_token":"`+pair.RefreshToken.String()+`"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		RefreshToken string `json:"refresh_token"`
		NewUser      bool   `json:"new_user"`
	}
	decodeBody(t, rec, &body)
	if body.RefreshToken == pair.RefreshToken.String() {
		t.Error("the token was not rotated")
	}
	// A refresh is not a sign-up, so new_user must not be set — the app would
	// otherwise ask an existing user for their name on every restart.
	if body.NewUser {
		t.Error("a refresh reported the user as new")
	}
}

func TestRefreshRejectsAStolenToken(t *testing.T) {
	h := newHTTPRig(t)
	pair := h.signIn(t)

	if rec := h.do(http.MethodPost, "/v1/auth/refresh",
		`{"refresh_token":"`+pair.RefreshToken.String()+`"}`, ""); rec.Code != http.StatusOK {
		t.Fatalf("first refresh: status %d", rec.Code)
	}
	rec := h.do(http.MethodPost, "/v1/auth/refresh",
		`{"refresh_token":"`+pair.RefreshToken.String()+`"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := responseCode(t, rec); got != "token_reuse_detected" {
		t.Errorf("code = %q", got)
	}
}

func TestLoggingOutOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	pair := h.signIn(t)
	bearer := h.bearerFor(t, pair.UserID, pair.SessionID, pair.Role)

	rec := h.do(http.MethodPost, "/v1/auth/logout", "", bearer)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("a 204 carried a body: %s", rec.Body.String())
	}

	// The refresh token is dead.
	after := h.do(http.MethodPost, "/v1/auth/refresh",
		`{"refresh_token":"`+pair.RefreshToken.String()+`"}`, "")
	if after.Code != http.StatusUnauthorized {
		t.Errorf("the token still worked after logout: status %d", after.Code)
	}
}

func TestLoggingOutOfEveryDeviceOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	pair := h.signIn(t)
	bearer := h.bearerFor(t, pair.UserID, pair.SessionID, pair.Role)

	if rec := h.do(http.MethodPost, "/v1/auth/logout-all", "", bearer); rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	after := h.do(http.MethodPost, "/v1/auth/refresh",
		`{"refresh_token":"`+pair.RefreshToken.String()+`"}`, "")
	if after.Code != http.StatusUnauthorized {
		t.Errorf("a token survived logout-all: status %d", after.Code)
	}
}

func TestListingSessionsOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	pair := h.signIn(t)
	bearer := h.bearerFor(t, pair.UserID, pair.SessionID, pair.Role)

	rec := h.do(http.MethodGet, "/v1/auth/sessions", "", bearer)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Sessions []struct {
			SessionID  string `json:"session_id"`
			Device     string `json:"device"`
			CreatedAt  string `json:"created_at"`
			LastSeenAt string `json:"last_seen_at"`
			Current    bool   `json:"current"`
		} `json:"sessions"`
	}
	decodeBody(t, rec, &body)

	if len(body.Sessions) != 1 {
		t.Fatalf("sessions = %d", len(body.Sessions))
	}
	if !body.Sessions[0].Current || body.Sessions[0].Device != "Pixel 8" {
		t.Errorf("session = %+v", body.Sessions[0])
	}
	if body.Sessions[0].CreatedAt == "" || body.Sessions[0].LastSeenAt == "" {
		t.Error("the device list has no timestamps for a user to recognise a device by")
	}
	if strings.Contains(rec.Body.String(), pair.RefreshToken.String()) {
		t.Error("the device list leaks a refresh token")
	}
}

// TestTheAuthenticatedRoutesReallyRequireAToken. The route table declares the
// roles; this checks the declaration is applied rather than merely written.
func TestTheAuthenticatedRoutesReallyRequireAToken(t *testing.T) {
	h := newHTTPRig(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/v1/auth/logout"},
		{http.MethodPost, "/v1/auth/logout-all"},
		{http.MethodGet, "/v1/auth/sessions"},
	} {
		rec := h.do(route.method, route.path, "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a token: status = %d, want 401", route.method, route.path, rec.Code)
		}
	}
}

// TestOnlyThreeRoutesArePublic: sign-in cannot require a token by definition,
// and rule 2.7 says every other endpoint declares what it needs. A fourth
// public route appearing here is a decision that deserves noticing.
func TestOnlyThreeRoutesArePublic(t *testing.T) {
	h := newHTTPRig(t)
	public := map[string]bool{
		"POST /v1/auth/otp/request": true,
		"POST /v1/auth/otp/verify":  true,
		"POST /v1/auth/refresh":     true,
	}

	for _, pattern := range identityhttp.Patterns() {
		method, path, found := strings.Cut(pattern, " ")
		if !found {
			t.Fatalf("pattern %q is not %q", pattern, "METHOD /path")
		}
		rec := h.do(method, path, "{}", "")

		// A 401 alone is not enough to tell the two apart: the refresh
		// endpoint is public but still answers 401 to an empty body, because
		// the token it was given is not valid. The middleware's refusal has
		// its own code, so that is what distinguishes "needs a token" from
		// "was given a bad one".
		guarded := responseCode(t, rec) == "missing_token"

		if public[pattern] && guarded {
			t.Errorf("%s is behind the authentication guard, but sign-in cannot require a token", pattern)
		}
		if !public[pattern] && !guarded {
			t.Errorf("%s answered without the guard refusing it; it is not in the public list", pattern)
		}
	}
}

func TestPatternsMatchWhatIsMounted(t *testing.T) {
	h := newHTTPRig(t)
	for _, pattern := range identityhttp.Patterns() {
		method, path, _ := strings.Cut(pattern, " ")
		rec := h.do(method, path, "{}", "")
		if rec.Code == http.StatusNotFound {
			t.Errorf("%s is in Patterns but nothing is mounted for it", pattern)
		}
	}
}

func TestAnUnknownAuthRouteIs404(t *testing.T) {
	h := newHTTPRig(t)
	if rec := h.do(http.MethodPost, "/v1/auth/nope", "{}", ""); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAWrongCodeIsRefusedOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	if rec := h.do(http.MethodPost, "/v1/auth/otp/request", `{"phone":"01712345678"}`, ""); rec.Code != http.StatusOK {
		t.Fatalf("request: status %d", rec.Code)
	}
	wrong := "000000"
	if wrong == h.sms.codes[0] {
		wrong = "111111"
	}

	rec := h.do(http.MethodPost, "/v1/auth/otp/verify", `{
		"phone":"01712345678","code":"`+wrong+`","role":"customer","device":"x"
	}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if got := responseCode(t, rec); got != "invalid_code" {
		t.Errorf("code = %q", got)
	}
}

func TestAnInvalidPhoneIsRefusedOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	rec := h.do(http.MethodPost, "/v1/auth/otp/request", `{"phone":"12345"}`, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if got := responseCode(t, rec); got != "invalid_phone" {
		t.Errorf("code = %q", got)
	}
}

// A store failure during logout must surface, not be swallowed into a 204 that
// tells the user they are signed out when they are not.
func TestALogoutFailureSurfacesOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	pair := h.signIn(t)
	bearer := h.bearerFor(t, pair.UserID, pair.SessionID, pair.Role)
	h.sessions.revokeErr = errStore

	if rec := h.do(http.MethodPost, "/v1/auth/logout", "", bearer); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("logout: status = %d, want 503", rec.Code)
	}
	if rec := h.do(http.MethodPost, "/v1/auth/logout-all", "", bearer); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("logout-all: status = %d, want 503", rec.Code)
	}
}

func TestAListingFailureSurfacesOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	pair := h.signIn(t)
	bearer := h.bearerFor(t, pair.UserID, pair.SessionID, pair.Role)
	h.sessions.listErr = errStore

	if rec := h.do(http.MethodGet, "/v1/auth/sessions", "", bearer); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

// A refresh failure must not be reported as a successful rotation with an empty
// token, which a client would store and then be unable to use.
func TestARefreshFailureSurfacesOverHTTP(t *testing.T) {
	h := newHTTPRig(t)
	rec := h.do(http.MethodPost, "/v1/auth/refresh", `{"refresh_token":"not-a-token"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
