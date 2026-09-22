package identity

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	cfgext "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

var signInAt = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

const testPhone = "01712345678"

func ctx() context.Context { return context.Background() }

// rig holds a wired set of use cases over shared fakes, so a test can drive a
// whole sign-in and then inspect what the stores ended up holding.
type rig struct {
	otps      *fakeOTPStore
	sessions  *fakeSessionStore
	signer    *fakeSigner
	directory *fakeDirectory
	limiter   *fakeLimiter
	sms       *fakeSMS
	cfg       *fakeConfig

	request  *application.RequestOTPUseCase
	verify   *application.VerifyOTPUseCase
	refresh  *application.RefreshSessionUseCase
	logout   *application.LogoutUseCase
	sessList *application.ListSessionsUseCase
}

func newRig() *rig { return newRigWithEntropy(&countingReader{}) }

// newRigWithEntropy wires the same rig over a chosen entropy source, so a test
// can make token generation fail the way a broken OS RNG would.
func newRigWithEntropy(entropy io.Reader) *rig {
	r := &rig{
		otps:      newOTPStore(),
		sessions:  newSessionStore(),
		signer:    &fakeSigner{},
		directory: newDirectory(),
		limiter:   &fakeLimiter{},
		sms:       &fakeSMS{},
		cfg:       newConfig(),
	}
	clk := fixedClock{at: signInAt}
	ids := &seqIDs{}
	logger := quietLogger()

	r.request = application.NewRequestOTPUseCase(r.otps, r.limiter, r.sms, r.cfg, clk, entropy, logger)
	r.verify = application.NewVerifyOTPUseCase(r.otps, r.sessions, r.signer, r.directory, r.limiter, r.cfg, clk, ids, entropy, logger)
	r.refresh = application.NewRefreshSessionUseCase(r.sessions, r.signer, r.limiter, r.cfg, clk, ids, entropy, logger)
	r.logout = application.NewLogoutUseCase(r.sessions, logger)
	r.sessList = application.NewListSessionsUseCase(r.sessions)
	return r
}

var noPlacement = cfgext.Placement{}

// signIn runs a complete request-then-verify and returns the token pair.
func (r *rig) signIn(t *testing.T) domain.TokenPair {
	t.Helper()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request otp: %v", err)
	}
	if len(r.sms.codes) == 0 {
		t.Fatal("no code was sent")
	}
	result, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone:  testPhone,
		Code:   r.sms.codes[len(r.sms.codes)-1],
		Role:   domain.RoleCustomer,
		Device: "Pixel 8",
	})
	if err != nil {
		t.Fatalf("verify otp: %v", err)
	}
	return result.Tokens
}

func TestRequestingACodeSendsOneAndStoresOnlyItsHash(t *testing.T) {
	r := newRig()

	result, err := r.request.Execute(ctx(), testPhone, noPlacement)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(r.sms.codes) != 1 {
		t.Fatalf("sent %d codes, want 1", len(r.sms.codes))
	}

	// The response tells the user nothing they should not know.
	if strings.Contains(result.MaskedPhone, "12345") {
		t.Errorf("the response shows the full number: %q", result.MaskedPhone)
	}
	if result.ExpiresIn != 300 {
		t.Errorf("expires_in = %d, want the configured 300", result.ExpiresIn)
	}

	// What is stored is a hash, not the code.
	stored := r.otps.hashes["+8801712345678"]
	if stored == "" {
		t.Fatal("nothing was stored")
	}
	if strings.Contains(stored, r.sms.codes[0]) {
		t.Errorf("the stored value %q contains the plaintext code", stored)
	}
}

// TestTheCodeIsStoredBeforeItIsSent: if the store succeeded and the SMS failed,
// the user simply asks again. The other order would put a real code in
// someone's hand that the server cannot recognise.
func TestAFailedSMSStillLeavesTheStoredCode(t *testing.T) {
	r := newRig()
	r.sms.err = errors.New("gateway timeout")

	_, err := r.request.Execute(ctx(), testPhone, noPlacement)
	if errs.CodeOf(err) != "sms_delivery_failed" {
		t.Fatalf("error = %v, want sms_delivery_failed", err)
	}
	if r.otps.hashes["+8801712345678"] == "" {
		t.Error("the code was removed after a delivery failure; a message that did arrive would then be useless")
	}
}

// The rate limit is applied before anything is generated or sent, so a blocked
// request costs no SMS — the expensive part, and the part that harasses the
// number's owner.
func TestTheRateLimitStopsTheSMSBeforeItIsSent(t *testing.T) {
	r := newRig()
	r.limiter.deny = true

	_, err := r.request.Execute(ctx(), testPhone, noPlacement)
	if errs.KindOf(err) != errs.KindRateLimited {
		t.Fatalf("error = %v, want rate limited", err)
	}
	if len(r.sms.codes) != 0 {
		t.Error("an SMS was sent for a rate-limited request")
	}
	if r.otps.hashes["+8801712345678"] != "" {
		t.Error("a code was stored for a rate-limited request")
	}
}

// TestALockedNumberGetsNoNewCode: issuing one would let an attacker clear their
// own way out of a lockout just by asking again.
func TestALockedNumberGetsNoNewCode(t *testing.T) {
	r := newRig()
	r.otps.attempts["+8801712345678"] = 5

	_, err := r.request.Execute(ctx(), testPhone, noPlacement)
	if errs.CodeOf(err) != "phone_locked_out" {
		t.Fatalf("error = %v, want phone_locked_out", err)
	}
	if len(r.sms.codes) != 0 {
		t.Error("a code was sent to a locked-out number")
	}
}

func TestRequestingACodeForANonsenseNumberIsRejected(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), "not a phone", noPlacement); errs.CodeOf(err) != "invalid_phone" {
		t.Errorf("error = %v, want invalid_phone", err)
	}
}

func TestRequestSurfacesStoreAndConfigFailures(t *testing.T) {
	limiterFail := newRig()
	limiterFail.limiter.err = errStore
	if _, err := limiterFail.request.Execute(ctx(), testPhone, noPlacement); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("limiter failure = %v", err)
	}

	countFail := newRig()
	countFail.otps.countErr = errStore
	if _, err := countFail.request.Execute(ctx(), testPhone, noPlacement); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("attempt read failure = %v", err)
	}

	putFail := newRig()
	putFail.otps.putErr = errStore
	if _, err := putFail.request.Execute(ctx(), testPhone, noPlacement); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("store failure = %v", err)
	}

	cfgFail := newRig()
	cfgFail.cfg.err = errStore
	if _, err := cfgFail.request.Execute(ctx(), testPhone, noPlacement); errs.CodeOf(err) != "auth_config_unavailable" {
		t.Errorf("config failure = %v", err)
	}
}

func TestASuccessfulSignInIssuesBothTokens(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)

	if pair.AccessToken == "" || pair.RefreshToken.String() == "" {
		t.Fatalf("pair = %+v", pair)
	}
	if pair.ExpiresIn != 900 {
		t.Errorf("expires_in = %d, want the configured 900", pair.ExpiresIn)
	}
	if pair.Role != domain.RoleCustomer || pair.UserID != "usr_1" {
		t.Errorf("pair = %+v", pair)
	}

	// The session exists and knows its device.
	session, err := r.sessions.Session(ctx(), pair.SessionID)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if session.Device != "Pixel 8" || session.UserID != "usr_1" {
		t.Errorf("session = %+v", session)
	}
	if !session.ExpiresAt.Equal(signInAt.Add(5_184_000 * time.Second)) {
		t.Errorf("session expiry = %v, want the configured 60 days", session.ExpiresAt)
	}
}

// TestAWrongCodeDoesNotBurnTheRealOne: the store is peeked, not taken, so a
// mistyped digit leaves the code the user is holding still usable.
func TestAWrongCodeDoesNotBurnTheRealOne(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	real := r.sms.codes[0]

	wrong := "000000"
	if wrong == real {
		wrong = "111111"
	}
	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: wrong, Role: domain.RoleCustomer,
	}); err == nil {
		t.Fatal("a wrong code was accepted")
	}

	// The real code still works.
	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: real, Role: domain.RoleCustomer,
	}); err != nil {
		t.Errorf("the real code stopped working after one wrong attempt: %v", err)
	}
}

// TestTheCodeIsSingleUse: a correct code creates exactly one session.
func TestTheCodeIsSingleUse(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	code := r.sms.codes[0]

	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: code, Role: domain.RoleCustomer,
	}); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: code, Role: domain.RoleCustomer,
	}); err == nil {
		t.Error("the same code created a second session")
	}
}

// TestBruteForceIsStoppedByTheAttemptCap. Six digits is a million
// possibilities; on its own that is weak, and this cap is what makes it not.
func TestBruteForceIsStoppedByTheAttemptCap(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	real := r.sms.codes[0]

	guesses := []string{"000001", "000002", "000003", "000004", "000005"}
	for i, guess := range guesses {
		if guess == real {
			guess = "999999"
		}
		_, err := r.verify.Execute(ctx(), application.VerifyRequest{
			Phone: testPhone, Code: guess, Role: domain.RoleCustomer,
		})
		if err == nil {
			t.Fatalf("guess %d was accepted", i)
		}
		if i < len(guesses)-1 && errs.CodeOf(err) != "invalid_code" {
			t.Errorf("guess %d: error = %v, want invalid_code", i, err)
		}
	}

	// The fifth failure locks the number, and the real code no longer works.
	_, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: real, Role: domain.RoleCustomer,
	})
	if errs.CodeOf(err) != "phone_locked_out" {
		t.Errorf("after the cap: error = %v, want phone_locked_out", err)
	}
}

// On lockout the pending code is destroyed, so waiting out the window does not
// leave a still-valid code to try again with.
func TestLockoutDestroysThePendingCode(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	for i := 0; i < 5; i++ {
		_, _ = r.verify.Execute(ctx(), application.VerifyRequest{
			Phone: testPhone, Code: "000000", Role: domain.RoleCustomer,
		})
	}
	if r.otps.hashes["+8801712345678"] != "" {
		t.Error("the pending code survived a lockout")
	}
}

// A wrong code and an expired one give the same message: telling them apart
// tells an attacker whether a code is currently live.
func TestAWrongCodeAndNoCodeAreIndistinguishable(t *testing.T) {
	r := newRig()
	// No code has been requested at all.
	_, noCode := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: "123456", Role: domain.RoleCustomer,
	})

	r2 := newRig()
	if _, err := r2.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	wrong := "000000"
	if wrong == r2.sms.codes[0] {
		wrong = "111111"
	}
	_, wrongCode := r2.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: wrong, Role: domain.RoleCustomer,
	})

	if errs.CodeOf(noCode) != errs.CodeOf(wrongCode) {
		t.Errorf("no code gives %q but a wrong code gives %q; the difference tells an attacker whether a code is live",
			errs.CodeOf(noCode), errs.CodeOf(wrongCode))
	}
	if errs.MessageOf(noCode) != errs.MessageOf(wrongCode) {
		t.Errorf("messages differ: %q vs %q", errs.MessageOf(noCode), errs.MessageOf(wrongCode))
	}
}

// A malformed code still counts as an attempt; excluding it would let an
// attacker probe state for free by sending rubbish.
func TestAMalformedCodeCountsAsAnAttempt(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: "abc", Role: domain.RoleCustomer,
	}); err == nil {
		t.Fatal("a malformed code was accepted")
	}
	if r.otps.attempts["+8801712345678"] != 1 {
		t.Errorf("attempts = %d, want the malformed submission counted", r.otps.attempts["+8801712345678"])
	}
}

func TestVerifyRejectsAnUnknownRole(t *testing.T) {
	r := newRig()
	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: "123456", Role: "superuser",
	}); errs.CodeOf(err) != "invalid_role" {
		t.Errorf("error = %v, want invalid_role", err)
	}
}

func TestVerifyRejectsANonsenseNumber(t *testing.T) {
	r := newRig()
	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: "nope", Code: "123456", Role: domain.RoleCustomer,
	}); errs.CodeOf(err) != "invalid_phone" {
		t.Errorf("error = %v, want invalid_phone", err)
	}
}

// TestTheOldestSessionIsEvictedAtTheDeviceLimit: a user signing in on a new
// phone must not be blocked by a session on a device they no longer own.
func TestTheOldestSessionIsEvictedAtTheDeviceLimit(t *testing.T) {
	r := newRig()
	r.cfg.values["auth.max_sessions_per_user"] = 2

	var ids []string
	for i := 0; i < 3; i++ {
		if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		result, err := r.verify.Execute(ctx(), application.VerifyRequest{
			Phone: testPhone, Code: r.sms.codes[len(r.sms.codes)-1],
			Role: domain.RoleCustomer, Device: "device",
		})
		if err != nil {
			t.Fatalf("verify %d: %v", i, err)
		}
		ids = append(ids, result.Tokens.SessionID)
	}

	live, err := r.sessions.UserSessions(ctx(), "usr_1")
	if err != nil {
		t.Fatalf("UserSessions: %v", err)
	}
	if len(live) != 2 {
		t.Fatalf("live sessions = %d, want the limit of 2", len(live))
	}
	for _, s := range live {
		if s.ID == ids[0] {
			t.Error("the oldest session survived past the device limit")
		}
	}
}

func TestVerifySurfacesInfrastructureFailures(t *testing.T) {
	cases := map[string]func(*rig){
		"otp read":       func(r *rig) { r.otps.peekErr = errStore },
		"attempt count":  func(r *rig) { r.otps.countErr = errStore },
		"directory":      func(r *rig) { r.directory.err = errStore },
		"session create": func(r *rig) { r.sessions.createErr = errStore },
		"session list":   func(r *rig) { r.sessions.listErr = errStore },
	}
	for name, brk := range cases {
		r := newRig()
		if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
			t.Fatalf("%s: request: %v", name, err)
		}
		code := r.sms.codes[0]
		brk(r)

		_, err := r.verify.Execute(ctx(), application.VerifyRequest{
			Phone: testPhone, Code: code, Role: domain.RoleCustomer,
		})
		if err == nil {
			t.Errorf("%s: a broken store did not fail the sign-in", name)
			continue
		}
		if kind := errs.KindOf(err); kind != errs.KindUnavailable && kind != errs.KindInternal {
			t.Errorf("%s: kind = %v, want unavailable or internal", name, kind)
		}
	}
}

// The attempt counter is only touched when a code is wrong, so its failure is
// asserted on that path rather than on a successful sign-in.
func TestAFailureToRecordAnAttemptFailsTheVerification(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	r.otps.incrErr = errStore

	_, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: "000000", Role: domain.RoleCustomer,
	})
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable; a counter that cannot be written means the cap is not being enforced", err)
	}
}

// A signing failure must fail the sign-in rather than return an empty token.
func TestVerifySurfacesASigningFailure(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	r.signer.signErr = errors.New("key unavailable")

	_, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: r.sms.codes[0], Role: domain.RoleCustomer,
	})
	if errs.CodeOf(err) != "token_signing_failed" {
		t.Errorf("error = %v, want token_signing_failed", err)
	}
}

// A failure to clear the attempt counter must not fail a sign-in the user has
// already earned; the counter expires on its own.
func TestAFailureToClearAttemptsDoesNotFailTheSignIn(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	r.otps.clearErr = errStore

	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: r.sms.codes[0], Role: domain.RoleCustomer,
	}); err != nil {
		t.Errorf("the sign-in failed because a counter could not be cleared: %v", err)
	}
}

// ---------------------------------------------------------------- refresh

// TestRefreshRotatesTheToken is auto-login: the app presents what it stored and
// gets a fresh pair, with no login screen.
func TestRefreshRotatesTheToken(t *testing.T) {
	r := newRig()
	first := r.signIn(t)

	second, err := r.refresh.Execute(ctx(), first.RefreshToken.String(), noPlacement)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if second.RefreshToken.String() == first.RefreshToken.String() {
		t.Error("the refresh token was not rotated")
	}
	if second.SessionID != first.SessionID {
		t.Errorf("session changed from %s to %s; a refresh keeps the session", first.SessionID, second.SessionID)
	}
	if second.UserID != first.UserID || second.Role != first.Role {
		t.Errorf("identity changed across a refresh: %+v", second)
	}
}

// TestARefreshTokenWorksExactlyOnce, and the second use is treated as theft.
func TestPresentingASpentTokenRevokesTheWholeSession(t *testing.T) {
	r := newRig()
	first := r.signIn(t)

	second, err := r.refresh.Execute(ctx(), first.RefreshToken.String(), noPlacement)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	// Replay the original.
	_, err = r.refresh.Execute(ctx(), first.RefreshToken.String(), noPlacement)
	if !errors.Is(err, domain.ErrTokenReuseDetected) {
		t.Fatalf("error = %v, want ErrTokenReuseDetected", err)
	}
	if errs.CodeOf(err) != "token_reuse_detected" {
		t.Errorf("code = %q", errs.CodeOf(err))
	}

	// The whole family is gone, including the token the honest client holds.
	// Which of the two is the thief cannot be known, so both lose access:
	// signing the real user out is recoverable, leaving an attacker signed in
	// is not.
	if _, err := r.refresh.Execute(ctx(), second.RefreshToken.String(), noPlacement); err == nil {
		t.Error("the current token still worked after a reuse detection")
	}
	if len(r.sessions.revoked) == 0 {
		t.Error("no session was revoked on reuse detection")
	}
}

// TestConcurrentRefreshesProduceOneRotation is the race the Lua script exists
// for: two in-flight refreshes from one device must not both mint a token, and
// must not raise a false theft alarm against an innocent user.
func TestConcurrentRefreshesProduceOneRotation(t *testing.T) {
	r := newRig()
	first := r.signIn(t)

	const attempts = 8
	var wg sync.WaitGroup
	results := make([]error, attempts)
	pairs := make([]domain.TokenPair, attempts)

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pairs[i], results[i] = r.refresh.Execute(ctx(), first.RefreshToken.String(), noPlacement)
		}(i)
	}
	wg.Wait()

	succeeded := 0
	reuse := 0
	for i, err := range results {
		switch {
		case err == nil:
			succeeded++
			if pairs[i].RefreshToken.String() == first.RefreshToken.String() {
				t.Error("a concurrent refresh returned the token it was given")
			}
		case errors.Is(err, domain.ErrTokenReuseDetected):
			reuse++
		}
	}
	if succeeded != 1 {
		t.Errorf("%d of %d concurrent refreshes succeeded, want exactly 1", succeeded, attempts)
	}
}

func TestAnUnknownRefreshTokenIsRejected(t *testing.T) {
	r := newRig()
	// Well-formed but never issued.
	unknown, err := domain.GenerateRefreshToken(&countingReader{})
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	_, err = r.refresh.Execute(ctx(), unknown.String(), noPlacement)
	if errs.CodeOf(err) != "invalid_refresh_token" {
		t.Errorf("error = %v, want invalid_refresh_token", err)
	}
}

// The shape is checked before Redis is touched, so a flood of garbage costs a
// length check rather than a round trip each.
func TestAMalformedRefreshTokenIsRejectedWithoutHittingTheStore(t *testing.T) {
	r := newRig()
	r.sessions.rotateErr = errors.New("the store must not be reached")

	for _, bad := range []string{"", "   ", "not-base64!!", "c2hvcnQ"} {
		if _, err := r.refresh.Execute(ctx(), bad, noPlacement); errs.CodeOf(err) != "invalid_refresh_token" {
			t.Errorf("token %q: error = %v", bad, err)
		}
	}
}

func TestAnExpiredSessionIsRejectedAndRevoked(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)

	// Age the session past its expiry without changing the clock the use case
	// reads, which is what an abandoned app looks like.
	session := r.sessions.sessions[pair.SessionID]
	session.ExpiresAt = signInAt.Add(-time.Hour)
	r.sessions.sessions[pair.SessionID] = session

	_, err := r.refresh.Execute(ctx(), pair.RefreshToken.String(), noPlacement)
	if errs.CodeOf(err) != "session_expired" {
		t.Fatalf("error = %v, want session_expired", err)
	}
	if len(r.sessions.revoked) == 0 {
		t.Error("an expired session was not revoked")
	}
}

func TestRefreshIsRateLimitedByToken(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)
	r.limiter.deny = true

	_, err := r.refresh.Execute(ctx(), pair.RefreshToken.String(), noPlacement)
	if errs.CodeOf(err) != "refresh_rate_exceeded" {
		t.Fatalf("error = %v, want refresh_rate_exceeded", err)
	}

	// Limited by token hash, not by IP: many users share an IP behind carrier
	// NAT, and limiting by IP would lock out a neighbourhood for one device.
	found := false
	for _, key := range r.limiter.seenKeys {
		if strings.HasPrefix(key, "refresh:") {
			found = true
		}
		if strings.Contains(key, "ip") {
			t.Errorf("the refresh limiter keyed on %q, which looks like an address", key)
		}
	}
	if !found {
		t.Error("the refresh limit was not keyed on the token")
	}
}

func TestRefreshSurfacesStoreFailures(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)
	r.sessions.rotateErr = errStore

	if _, err := r.refresh.Execute(ctx(), pair.RefreshToken.String(), noPlacement); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable", err)
	}
}

// ---------------------------------------------------------------- logout

func TestLogoutRevokesTheCurrentSessionOnly(t *testing.T) {
	r := newRig()
	first := r.signIn(t)
	// A second device for the same user.
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	secondResult, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: r.sms.codes[len(r.sms.codes)-1],
		Role: domain.RoleCustomer, Device: "laptop",
	})
	if err != nil {
		t.Fatalf("second sign-in: %v", err)
	}

	principal := domain.Principal{UserID: first.UserID, Role: first.Role, SessionID: first.SessionID}
	if err := r.logout.Execute(ctx(), principal); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if _, err := r.refresh.Execute(ctx(), first.RefreshToken.String(), noPlacement); err == nil {
		t.Error("the signed-out session's refresh token still works")
	}
	if _, err := r.refresh.Execute(ctx(), secondResult.Tokens.RefreshToken.String(), noPlacement); err != nil {
		t.Errorf("signing out one device signed out another: %v", err)
	}
}

// TestLogoutOfSomebodyElsesSessionIsRefused: without this, a valid token for
// any account could sign out any other account given a session id.
func TestLogoutOfSomebodyElsesSessionIsRefused(t *testing.T) {
	r := newRig()
	victim := r.signIn(t)

	attacker := domain.Principal{UserID: "usr_attacker", Role: domain.RoleCustomer, SessionID: victim.SessionID}
	err := r.logout.Execute(ctx(), attacker)
	if errs.KindOf(err) != errs.KindForbidden {
		t.Fatalf("error = %v, want forbidden", err)
	}
	if _, err := r.sessions.Session(ctx(), victim.SessionID); err != nil {
		t.Error("the victim's session was revoked")
	}
}

// Logout is idempotent: reporting an error for an already-gone session would
// make a client retry something that has already succeeded.
func TestLoggingOutTwiceIsNotAnError(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)
	principal := domain.Principal{UserID: pair.UserID, Role: pair.Role, SessionID: pair.SessionID}

	if err := r.logout.Execute(ctx(), principal); err != nil {
		t.Fatalf("first logout: %v", err)
	}
	if err := r.logout.Execute(ctx(), principal); err != nil {
		t.Errorf("second logout: %v", err)
	}
}

func TestLogoutRequiresAnIdentity(t *testing.T) {
	r := newRig()
	if err := r.logout.Execute(ctx(), domain.Principal{}); errs.KindOf(err) != errs.KindUnauthorized {
		t.Errorf("error = %v, want unauthorized", err)
	}
	if err := r.logout.Execute(ctx(), domain.Principal{UserID: "usr_1"}); errs.KindOf(err) != errs.KindUnauthorized {
		t.Errorf("a principal with no session = %v, want unauthorized", err)
	}
	if err := r.logout.ExecuteAll(ctx(), domain.Principal{}); errs.KindOf(err) != errs.KindUnauthorized {
		t.Errorf("logout-all with no identity = %v", err)
	}
}

// TestLogoutAllClearsEveryDevice is the control someone reaches for when they
// think their account is compromised, so it must not need to know which device
// was stolen.
func TestLogoutAllClearsEveryDevice(t *testing.T) {
	r := newRig()
	var pairs []domain.TokenPair
	for i := 0; i < 3; i++ {
		if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		result, err := r.verify.Execute(ctx(), application.VerifyRequest{
			Phone: testPhone, Code: r.sms.codes[len(r.sms.codes)-1],
			Role: domain.RoleCustomer, Device: "device",
		})
		if err != nil {
			t.Fatalf("verify %d: %v", i, err)
		}
		pairs = append(pairs, result.Tokens)
	}

	principal := domain.Principal{UserID: pairs[0].UserID, Role: pairs[0].Role, SessionID: pairs[0].SessionID}
	if err := r.logout.ExecuteAll(ctx(), principal); err != nil {
		t.Fatalf("logout all: %v", err)
	}
	for i, pair := range pairs {
		if _, err := r.refresh.Execute(ctx(), pair.RefreshToken.String(), noPlacement); err == nil {
			t.Errorf("device %d still has a working token after logout-all", i)
		}
	}
}

func TestLogoutSurfacesStoreFailures(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)
	principal := domain.Principal{UserID: pair.UserID, Role: pair.Role, SessionID: pair.SessionID}

	r.sessions.revokeErr = errStore
	if err := r.logout.Execute(ctx(), principal); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("logout = %v, want unavailable", err)
	}
	if err := r.logout.ExecuteAll(ctx(), principal); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("logout all = %v, want unavailable", err)
	}

	r2 := newRig()
	pair2 := r2.signIn(t)
	r2.sessions.readErr = errStore
	principal2 := domain.Principal{UserID: pair2.UserID, Role: pair2.Role, SessionID: pair2.SessionID}
	if err := r2.logout.Execute(ctx(), principal2); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("session read failure = %v, want unavailable", err)
	}
}

// ---------------------------------------------------------------- sessions

func TestListingDevicesMarksTheCurrentOne(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)

	principal := domain.Principal{UserID: pair.UserID, Role: pair.Role, SessionID: pair.SessionID}
	sessions, err := r.sessList.Execute(ctx(), principal)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(sessions))
	}
	if !sessions[0].Current {
		t.Error("the session making the request is not marked current")
	}
	if sessions[0].Device != "Pixel 8" {
		t.Errorf("device = %q", sessions[0].Device)
	}
}

// The list carries no token and no hash: it exists so a user can recognise and
// revoke a device, which needs a label and a time, not a credential.
func TestTheDeviceListCarriesNoCredentials(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)
	principal := domain.Principal{UserID: pair.UserID, Role: pair.Role, SessionID: pair.SessionID}

	sessions, err := r.sessList.Execute(ctx(), principal)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, s := range sessions {
		rendered := s.SessionID + s.Device + s.CreatedAt + s.LastSeenAt
		if strings.Contains(rendered, pair.RefreshToken.String()) {
			t.Error("the device list contains a refresh token")
		}
		if strings.Contains(rendered, pair.AccessToken) {
			t.Error("the device list contains an access token")
		}
	}
}

func TestListingDevicesRequiresAnIdentity(t *testing.T) {
	r := newRig()
	if _, err := r.sessList.Execute(ctx(), domain.Principal{}); errs.KindOf(err) != errs.KindUnauthorized {
		t.Errorf("error = %v, want unauthorized", err)
	}
}

func TestListingDevicesSurfacesAStoreFailure(t *testing.T) {
	r := newRig()
	r.sessions.listErr = errStore
	_, err := r.sessList.Execute(ctx(), domain.Principal{UserID: "usr_1", Role: domain.RoleCustomer, SessionID: "ses_a"})
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable", err)
	}
}

// ------------------------------------------------- infrastructure failures
//
// Each of these is a branch that only runs when something underneath has
// broken. They matter because the wrong behaviour on any of them is silent:
// a sign-in that half-succeeds, a lockout that does not lock, or a revoked
// session that is not revoked.

func TestRefreshSurfacesAConfigFailure(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)
	r.cfg.err = errStore

	if _, err := r.refresh.Execute(ctx(), pair.RefreshToken.String(), noPlacement); errs.CodeOf(err) != "auth_config_unavailable" {
		t.Errorf("error = %v, want auth_config_unavailable", err)
	}
}

func TestRefreshSurfacesALimiterFailure(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)
	r.limiter.err = errStore

	if _, err := r.refresh.Execute(ctx(), pair.RefreshToken.String(), noPlacement); errs.CodeOf(err) != "rate_limiter_unavailable" {
		t.Errorf("error = %v, want rate_limiter_unavailable", err)
	}
}

// A broken entropy source must fail the refresh rather than issue a predictable
// token — a refresh token that can be guessed is worse than no refresh at all.
func TestRefreshFailsRatherThanIssueAWeakToken(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)

	broken := newRigWithEntropy(emptyReader{})
	broken.sessions = r.sessions
	broken.refresh = application.NewRefreshSessionUseCase(
		r.sessions, broken.signer, broken.limiter, broken.cfg,
		fixedClock{at: signInAt}, &seqIDs{}, emptyReader{}, quietLogger())

	if _, err := broken.refresh.Execute(ctx(), pair.RefreshToken.String(), noPlacement); errs.CodeOf(err) != "token_generation_failed" {
		t.Errorf("error = %v, want token_generation_failed", err)
	}
}

func TestRefreshSurfacesASigningFailure(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)
	r.signer.signErr = errors.New("key unavailable")

	if _, err := r.refresh.Execute(ctx(), pair.RefreshToken.String(), noPlacement); errs.CodeOf(err) != "token_signing_failed" {
		t.Errorf("error = %v, want token_signing_failed", err)
	}
}

// A revocation that fails after reuse is logged, not returned: the caller is
// already being refused, and reporting a store error instead of the theft
// would hide why.
func TestAFailedRevocationAfterReuseStillReportsTheTheft(t *testing.T) {
	r := newRig()
	first := r.signIn(t)
	if _, err := r.refresh.Execute(ctx(), first.RefreshToken.String(), noPlacement); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	r.sessions.revokeErr = errStore

	_, err := r.refresh.Execute(ctx(), first.RefreshToken.String(), noPlacement)
	if !errors.Is(err, domain.ErrTokenReuseDetected) {
		t.Errorf("error = %v, want the reuse still reported", err)
	}
}

func TestAFailedRevocationOfAnExpiredSessionStillRefuses(t *testing.T) {
	r := newRig()
	pair := r.signIn(t)

	session := r.sessions.sessions[pair.SessionID]
	session.ExpiresAt = signInAt.Add(-time.Hour)
	r.sessions.sessions[pair.SessionID] = session
	r.sessions.revokeErr = errStore

	if _, err := r.refresh.Execute(ctx(), pair.RefreshToken.String(), noPlacement); errs.CodeOf(err) != "session_expired" {
		t.Errorf("error = %v, want session_expired", err)
	}
}

func TestVerifySurfacesAConfigFailure(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	code := r.sms.codes[0]
	r.cfg.err = errStore

	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: code, Role: domain.RoleCustomer,
	}); errs.CodeOf(err) != "auth_config_unavailable" {
		t.Errorf("error = %v, want auth_config_unavailable", err)
	}
}

// A config snapshot missing a key is an internal fault, not the caller's: it
// means the registry and this module disagree.
func TestVerifySurfacesAnIncompleteConfig(t *testing.T) {
	r := newRig()
	delete(r.cfg.values, "auth.otp_ttl")

	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); errs.CodeOf(err) != "auth_config_invalid" {
		t.Errorf("error = %v, want auth_config_invalid", err)
	}
}

// A malformed code counts as an attempt, so a store that cannot record it must
// fail the request — otherwise the cap is not being enforced.
func TestAMalformedCodeSurfacesARecordFailure(t *testing.T) {
	r := newRig()
	r.otps.incrErr = errStore

	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: "abc", Role: domain.RoleCustomer,
	}); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable", err)
	}
}

func TestVerifySurfacesAConsumeFailure(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	code := r.sms.codes[0]
	r.otps.takeErr = errStore

	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: code, Role: domain.RoleCustomer,
	}); errs.CodeOf(err) != "otp_store_unavailable" {
		t.Errorf("error = %v, want otp_store_unavailable", err)
	}
}

// TestLosingTheRaceToConsumeACodeIsRefused: when two requests submit the same
// correct code, the one that takes it gets the session and the other is a
// replay of something already spent.
func TestLosingTheRaceToConsumeACodeIsRefused(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	code := r.sms.codes[0]

	// Peek will succeed but the code is gone by the time it is taken, which is
	// exactly what the loser of the race sees.
	stored := r.otps.hashes["+8801712345678"]
	r.otps.peekErr = nil
	delete(r.otps.hashes, "+8801712345678")
	r.otps.peekHash = stored

	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: code, Role: domain.RoleCustomer,
	}); errs.CodeOf(err) != "invalid_code" {
		t.Errorf("error = %v, want invalid_code", err)
	}
}

// A store that cannot clear the code on lockout is logged, not fatal: the
// lockout itself has already taken effect.
func TestAFailureToClearTheCodeOnLockoutIsNotFatal(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	r.otps.deleteErr = errStore

	for i := 0; i < 5; i++ {
		_, err := r.verify.Execute(ctx(), application.VerifyRequest{
			Phone: testPhone, Code: "000000", Role: domain.RoleCustomer,
		})
		if err == nil {
			t.Fatalf("guess %d was accepted", i)
		}
	}
	// Still locked out, despite the cleanup failing.
	if _, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: "000000", Role: domain.RoleCustomer,
	}); errs.CodeOf(err) != "phone_locked_out" {
		t.Errorf("error = %v, want phone_locked_out", err)
	}
}

// If the oldest session cannot be evicted, the sign-in fails rather than
// quietly exceeding the device limit.
func TestAFailedEvictionFailsTheSignIn(t *testing.T) {
	r := newRig()
	r.cfg.values["auth.max_sessions_per_user"] = 1
	r.signIn(t)

	r.sessions.revokeErr = errStore
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	_, err := r.verify.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: r.sms.codes[len(r.sms.codes)-1],
		Role: domain.RoleCustomer, Device: "second",
	})
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("error = %v, want unavailable", err)
	}
}

// A broken entropy source must fail the request rather than issue a guessable
// code. A predictable OTP is worse than no OTP: the user would never know.
func TestRequestingACodeFailsRatherThanIssueAWeakOne(t *testing.T) {
	r := newRigWithEntropy(emptyReader{})
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); errs.CodeOf(err) != "otp_generation_failed" {
		t.Errorf("error = %v, want otp_generation_failed", err)
	}
	if len(r.sms.codes) != 0 {
		t.Error("a code was sent despite generation failing")
	}
}

// The same for the refresh token issued at sign-in.
func TestSignInFailsRatherThanIssueAWeakRefreshToken(t *testing.T) {
	r := newRig()
	if _, err := r.request.Execute(ctx(), testPhone, noPlacement); err != nil {
		t.Fatalf("request: %v", err)
	}
	code := r.sms.codes[0]

	broken := application.NewVerifyOTPUseCase(
		r.otps, r.sessions, r.signer, r.directory, r.limiter, r.cfg,
		fixedClock{at: signInAt}, &seqIDs{}, emptyReader{}, quietLogger())

	if _, err := broken.Execute(ctx(), application.VerifyRequest{
		Phone: testPhone, Code: code, Role: domain.RoleCustomer,
	}); errs.CodeOf(err) != "token_generation_failed" {
		t.Errorf("error = %v, want token_generation_failed", err)
	}
}

// TestARequestRevealsItsCodeOnlyToADemoSender is the whole of demo mode on
// this side: the code comes back in the response when the sender says its
// codes may be shown, and not otherwise.
//
// It matters that this is asked of the *sender* rather than of a flag. A flag
// could be set by a deployment whose sender really sends an SMS, and the code
// would then be handed to whoever asked for it as well as to the number's
// owner.
func TestARequestRevealsItsCodeOnlyToADemoSender(t *testing.T) {
	ctx := context.Background()

	ordinary := newRig()
	result, err := ordinary.request.Execute(ctx, "01712345678", noPlacement)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.DemoCode != "" {
		t.Errorf("an ordinary sender handed the code back: %q", result.DemoCode)
	}

	demo := newRig()
	revealing := &revealingSMS{}
	demo.request = application.NewRequestOTPUseCase(
		demo.otps, demo.limiter, revealing, demo.cfg,
		fixedClock{at: signInAt}, &countingReader{}, quietLogger())

	demoResult, err := demo.request.Execute(ctx, "01712345678", noPlacement)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if demoResult.DemoCode == "" {
		t.Fatal("a demo sender revealed nothing, so nobody can sign in to the demo")
	}
	// The code revealed is the code that was issued — not a second one, and
	// not a placeholder. The sender saw the same string.
	if len(revealing.codes) != 1 || revealing.codes[0] != demoResult.DemoCode {
		t.Errorf("revealed %q, sent %v", demoResult.DemoCode, revealing.codes)
	}
	// And it is a real code: verifying with it works, which is what proves
	// nothing about the stored hash changed.
	if _, err := demo.verify.Execute(ctx, application.VerifyRequest{
		Phone: "01712345678", Code: demoResult.DemoCode,
		Role: domain.RoleCustomer, Device: "demo", Placement: noPlacement,
	}); err != nil {
		t.Fatalf("the revealed code did not verify: %v", err)
	}
}
