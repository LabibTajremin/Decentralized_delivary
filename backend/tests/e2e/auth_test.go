package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// These drive authentication through the real binary against real Postgres and
// real Redis. The unit tests prove the rules and the integration tests prove
// the stores; this proves the whole thing works when wired together.

// uniquePhone returns a valid Bangladeshi mobile number unique to this run.
//
// Rate-limit counters and lockouts live in Redis with hour-long TTLs and are
// not reset between runs, so a fixed number would be throttled the second time
// the suite ran — which looks like a flake and is not one. The Postgres schema
// is rebuilt per test; Redis is shared, so the isolation has to come from the
// key instead.
func uniquePhone(t *testing.T) string {
	t.Helper()
	// 017 plus eight digits from the clock: a Grameenphone number the domain
	// accepts, distinct per test and per run.
	return fmt.Sprintf("017%08d", time.Now().UnixNano()%100_000_000)
}

// startAuthAPI runs the server with both dependencies attached and returns a
// client that can read the one-time codes out of its log.
func startAuthAPI(t *testing.T) (string, *logTail, func()) {
	t.Helper()
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Fatal("REDIS_URL is not set; the auth tests need a real Redis")
	}
	dbURL := dbtestSchemaURL(t)
	seedDemoData(t, dbURL)

	tail := newLogTail(t)
	base, stop := startAPIWithOutput(t, buildAPI(t), tail, "DATABASE_URL="+dbURL, "REDIS_URL="+redisURL)
	return base, tail, stop
}

// logTail captures the server's stdout so a test can read the OTP the
// development SMS sender writes there. That is exactly how a developer signs in
// locally, so the test uses the same route rather than a back door.
type logTail struct {
	buf *bytes.Buffer
	mu  chan struct{}
	// consumed counts the codes already handed to a caller, so the next
	// sign-in waits for a genuinely new one rather than re-reading the last.
	consumed int
}

func newLogTail(t *testing.T) *logTail {
	t.Helper()
	return &logTail{buf: &bytes.Buffer{}, mu: make(chan struct{}, 1)}
}

func (l *logTail) Write(p []byte) (int, error) {
	l.mu <- struct{}{}
	defer func() { <-l.mu }()
	return l.buf.Write(p)
}

var codePattern = regexp.MustCompile(`"code":"(\d{6})"`)

// lastCode returns the most recent one-time code the server logged.
//
// It polls rather than reading once. The server's stdout reaches this buffer
// through an OS pipe, so the HTTP response can arrive a moment before the log
// bytes are drained — reading immediately passes on an idle machine and fails
// under load, which is a flake manufactured by the test rather than a fault in
// the server.
//
// It waits for a code it has not already handed out, not merely for the buffer
// to be non-empty. The buffer accumulates across every sign-in a test makes
// against one server, so "return the last code present" hands back the previous
// account's code whenever the new one has not been flushed yet — and the verify
// then fails with a 401 that looks like a rate limit. A test doing two sign-ins
// usually got away with it; one doing six does not.
func (l *logTail) lastCode(t *testing.T) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if code, ok := l.nextCode(); ok {
			return code
		}
		if time.Now().After(deadline) {
			t.Fatalf("no new one-time code was logged within 5s; server output was:\n%s", l.snapshot())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// nextCode returns the newest code the caller has not seen yet.
//
// The count of codes handed out is the cursor rather than the code itself,
// because two accounts can legitimately be issued the same six digits and
// comparing values would then wait forever for a code that had already arrived.
func (l *logTail) nextCode() (string, bool) {
	l.mu <- struct{}{}
	defer func() { <-l.mu }()

	var codes []string
	scanner := bufio.NewScanner(bytes.NewReader(l.buf.Bytes()))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if m := codePattern.FindStringSubmatch(scanner.Text()); m != nil {
			codes = append(codes, m[1])
		}
	}

	if len(codes) <= l.consumed {
		return "", false
	}
	l.consumed = len(codes)
	return codes[len(codes)-1], true
}

// snapshot copies the captured output for an error message.
func (l *logTail) snapshot() string {
	l.mu <- struct{}{}
	defer func() { <-l.mu }()
	return l.buf.String()
}

// postJSON sends a JSON body and decodes the response.
func postJSON(t *testing.T, url, body string, bearer string, into any) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body)) //nolint:noctx // fixed test-local URL
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if into != nil {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil && resp.StatusCode != http.StatusNoContent {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp.StatusCode
}

type tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Role         string `json:"role"`
	UserID       string `json:"user_id"`
	SessionID    string `json:"session_id"`
	NewUser      bool   `json:"new_user"`
}

// signIn runs a full request-then-verify as a customer.
func signIn(t *testing.T, base string, tail *logTail, phone, device string) tokens {
	t.Helper()
	return signInAs(t, base, tail, phone, device, "customer")
}

// signInAs runs a full request-then-verify in a chosen role.
func signInAs(t *testing.T, base string, tail *logTail, phone, device, role string) tokens {
	t.Helper()
	if status := postJSON(t, base+"/v1/auth/otp/request",
		`{"phone":"`+phone+`"}`, "", nil); status != http.StatusOK {
		t.Fatalf("request otp: status %d", status)
	}
	code := tail.lastCode(t)

	var pair tokens
	status := postJSON(t, base+"/v1/auth/otp/verify",
		`{"phone":"`+phone+`","code":"`+code+`","role":"`+role+`","device":"`+device+`"}`, "", &pair)
	if status != http.StatusOK {
		t.Fatalf("verify otp: status %d", status)
	}
	return pair
}

func errorBody(t *testing.T, url, body, bearer string) (int, string) {
	t.Helper()
	var out struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	status := postJSON(t, url, body, bearer, &out)
	return status, out.Error.Code
}

// TestANewUserCanSignIn is the acceptance case: someone who has never signed in
// requests a code, verifies it, and receives both tokens.
func TestANewUserCanSignIn(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	pair := signIn(t, base, tail, uniquePhone(t), "Pixel 8")

	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("tokens = %+v", pair)
	}
	if pair.TokenType != "Bearer" {
		t.Errorf("token_type = %q", pair.TokenType)
	}
	if pair.ExpiresIn != 900 {
		t.Errorf("expires_in = %d, want the configured 900", pair.ExpiresIn)
	}
	if !pair.NewUser {
		t.Error("a first sign-in was not flagged as new, so the app would not ask for a name")
	}
	if pair.Role != "customer" {
		t.Errorf("role = %q", pair.Role)
	}
}

// TestTheAccessTokenOpensProtectedEndpoints.
func TestTheAccessTokenOpensProtectedEndpoints(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	pair := signIn(t, base, tail, uniquePhone(t), "Pixel 8")

	req, err := http.NewRequest(http.MethodGet, base+"/v1/auth/sessions", nil) //nolint:noctx // fixed test-local URL
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET sessions: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		Sessions []struct {
			Device  string `json:"device"`
			Current bool   `json:"current"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Sessions) != 1 || body.Sessions[0].Device != "Pixel 8" || !body.Sessions[0].Current {
		t.Errorf("sessions = %+v", body.Sessions)
	}
}

func TestProtectedEndpointsRefuseAnAbsentOrForgedToken(t *testing.T) {
	base, _, stop := startAuthAPI(t)
	defer stop()

	for _, bearer := range []string{"", "not-a-token", "a.b.c"} {
		status, _ := errorBody(t, base+"/v1/auth/logout", "", bearer)
		if status != http.StatusUnauthorized {
			t.Errorf("bearer %q: status = %d, want 401", bearer, status)
		}
	}
}

// TestAutoLoginRotatesSilently is the requirement: an app that holds a valid
// refresh token gets a new pair with no login screen.
func TestAutoLoginRotatesSilently(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	first := signIn(t, base, tail, uniquePhone(t), "Pixel 8")

	var second tokens
	status := postJSON(t, base+"/v1/auth/refresh",
		`{"refresh_token":"`+first.RefreshToken+`"}`, "", &second)
	if status != http.StatusOK {
		t.Fatalf("refresh: status %d", status)
	}

	if second.RefreshToken == first.RefreshToken {
		t.Error("the refresh token was not rotated")
	}
	if second.AccessToken == "" {
		t.Error("no access token was issued")
	}
	if second.SessionID != first.SessionID {
		t.Errorf("session changed from %s to %s; a refresh keeps the session", first.SessionID, second.SessionID)
	}
	if second.UserID != first.UserID {
		t.Errorf("identity changed across a refresh")
	}
}

// TestARefreshTokenWorksExactlyOnceAndReuseRevokesEverything is the theft case.
func TestARefreshTokenWorksExactlyOnceAndReuseRevokesEverything(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	first := signIn(t, base, tail, uniquePhone(t), "Pixel 8")

	var second tokens
	if status := postJSON(t, base+"/v1/auth/refresh",
		`{"refresh_token":"`+first.RefreshToken+`"}`, "", &second); status != http.StatusOK {
		t.Fatalf("refresh: status %d", status)
	}

	// Replay the spent token, as a thief with a copy would.
	status, code := errorBody(t, base+"/v1/auth/refresh",
		`{"refresh_token":"`+first.RefreshToken+`"}`, "")
	if status != http.StatusUnauthorized || code != "token_reuse_detected" {
		t.Fatalf("replay: status %d code %q, want 401 token_reuse_detected", status, code)
	}

	// The honest client's current token is now dead too. Which of the two is
	// the thief cannot be known, so both lose access: signing the real user out
	// is recoverable, leaving an attacker signed in is not.
	status, _ = errorBody(t, base+"/v1/auth/refresh",
		`{"refresh_token":"`+second.RefreshToken+`"}`, "")
	if status != http.StatusUnauthorized {
		t.Errorf("the current token still worked after a reuse detection: status %d", status)
	}
}

func TestAnUnknownRefreshTokenIsRejectedWithoutRevealingAnything(t *testing.T) {
	base, _, stop := startAuthAPI(t)
	defer stop()

	status, code := errorBody(t, base+"/v1/auth/refresh",
		`{"refresh_token":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`, "")
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", status)
	}
	if code != "invalid_refresh_token" {
		t.Errorf("code = %q", code)
	}
}

// TestLogoutInvalidatesTheRefreshTokenImmediately.
func TestLogoutInvalidatesTheRefreshTokenImmediately(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	pair := signIn(t, base, tail, uniquePhone(t), "Pixel 8")

	if status := postJSON(t, base+"/v1/auth/logout", "", pair.AccessToken, nil); status != http.StatusNoContent {
		t.Fatalf("logout: status %d", status)
	}
	status, _ := errorBody(t, base+"/v1/auth/refresh",
		`{"refresh_token":"`+pair.RefreshToken+`"}`, "")
	if status != http.StatusUnauthorized {
		t.Errorf("the refresh token still worked after logout: status %d", status)
	}
}

// A discarded token after logout must read as stale, not as a theft: an alarm
// that fires on ordinary logouts is one nobody will investigate.
func TestALogoutIsNotReportedAsATheft(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	pair := signIn(t, base, tail, uniquePhone(t), "Pixel 8")
	if status := postJSON(t, base+"/v1/auth/logout", "", pair.AccessToken, nil); status != http.StatusNoContent {
		t.Fatalf("logout: status %d", status)
	}

	_, code := errorBody(t, base+"/v1/auth/refresh",
		`{"refresh_token":"`+pair.RefreshToken+`"}`, "")
	if code == "token_reuse_detected" {
		t.Error("a token discarded at logout was reported as a theft")
	}
}

func TestLogoutAllClearsEveryDevice(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	phone := uniquePhone(t)
	phoneOne := signIn(t, base, tail, phone, "Pixel 8")
	phoneTwo := signIn(t, base, tail, phone, "iPad")

	if status := postJSON(t, base+"/v1/auth/logout-all", "", phoneOne.AccessToken, nil); status != http.StatusNoContent {
		t.Fatalf("logout-all: status %d", status)
	}
	for name, pair := range map[string]tokens{"first": phoneOne, "second": phoneTwo} {
		status, _ := errorBody(t, base+"/v1/auth/refresh",
			`{"refresh_token":"`+pair.RefreshToken+`"}`, "")
		if status != http.StatusUnauthorized {
			t.Errorf("%s device still has a working token: status %d", name, status)
		}
	}
}

// TestOTPBruteForceIsStopped: six digits is a million possibilities, and the
// attempt cap is what makes that enough.
func TestOTPBruteForceIsStopped(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	phone := uniquePhone(t)
	if status := postJSON(t, base+"/v1/auth/otp/request", `{"phone":"`+phone+`"}`, "", nil); status != http.StatusOK {
		t.Fatalf("request otp: status %d", status)
	}
	real := tail.lastCode(t)

	guesses := []string{"000001", "000002", "000003", "000004", "000005"}
	for i, guess := range guesses {
		if guess == real {
			guess = "999999"
		}
		status, _ := errorBody(t, base+"/v1/auth/otp/verify",
			`{"phone":"`+phone+`","code":"`+guess+`","role":"customer","device":"attacker"}`, "")
		if status == http.StatusOK {
			t.Fatalf("guess %d was accepted", i)
		}
	}

	// Locked out, and the real code no longer works either.
	status, code := errorBody(t, base+"/v1/auth/otp/verify",
		`{"phone":"`+phone+`","code":"`+real+`","role":"customer","device":"victim"}`, "")
	if status != http.StatusTooManyRequests || code != "phone_locked_out" {
		t.Errorf("after the cap: status %d code %q, want 429 phone_locked_out", status, code)
	}
}

func TestSignInRejectsABadPhoneOrRole(t *testing.T) {
	base, _, stop := startAuthAPI(t)
	defer stop()

	status, code := errorBody(t, base+"/v1/auth/otp/request", `{"phone":"12345"}`, "")
	if status != http.StatusBadRequest || code != "invalid_phone" {
		t.Errorf("bad phone: status %d code %q", status, code)
	}

	status, code = errorBody(t, base+"/v1/auth/otp/verify",
		`{"phone":"`+uniquePhone(t)+`","code":"123456","role":"superuser","device":"x"}`, "")
	if status != http.StatusBadRequest || code != "invalid_role" {
		t.Errorf("bad role: status %d code %q", status, code)
	}
}

// TestTheResponseNeverContainsThePlaintextCode.
func TestTheOTPResponseRevealsNothing(t *testing.T) {
	base, tail, stop := startAuthAPI(t)
	defer stop()

	var body struct {
		Phone       string `json:"phone"`
		ExpiresIn   int64  `json:"expires_in"`
		ResendAfter int64  `json:"resend_after"`
	}
	phone := uniquePhone(t)
	status := postJSON(t, base+"/v1/auth/otp/request", `{"phone":"`+phone+`"}`, "", &body)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}

	code := tail.lastCode(t)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), code) {
		t.Errorf("the response contains the one-time code: %s", raw)
	}
	// The masked form must not carry the digits that identify the number.
	if strings.Contains(body.Phone, phone[4:9]) {
		t.Errorf("the response contains the identifying digits of the number: %q", body.Phone)
	}
	if body.ExpiresIn <= 0 {
		t.Errorf("expires_in = %d; the client needs it to run its countdown", body.ExpiresIn)
	}
}
