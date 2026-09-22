package application

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// VerifyOTPUseCase exchanges a correct code for a session and a token pair.
type VerifyOTPUseCase struct {
	otps     ports.OTPStore
	sessions ports.SessionStore
	signer   ports.TokenSigner
	users    ports.UserDirectory
	limiter  ports.RateLimiter
	cfg      cfgcontract.Service
	clock    clock.Clock
	ids      id.Generator
	entropy  io.Reader
	logger   *slog.Logger
}

// NewVerifyOTPUseCase wires the use case.
func NewVerifyOTPUseCase(
	otps ports.OTPStore,
	sessions ports.SessionStore,
	signer ports.TokenSigner,
	users ports.UserDirectory,
	limiter ports.RateLimiter,
	cfg cfgcontract.Service,
	c clock.Clock,
	ids id.Generator,
	entropy io.Reader,
	logger *slog.Logger,
) *VerifyOTPUseCase {
	return &VerifyOTPUseCase{
		otps: otps, sessions: sessions, signer: signer, users: users,
		limiter: limiter, cfg: cfg, clock: c, ids: ids, entropy: entropy, logger: logger,
	}
}

// VerifyRequest is a sign-in attempt.
type VerifyRequest struct {
	Phone     string
	Code      string
	Role      domain.Role
	Device    string
	Placement cfgcontract.Placement
}

// VerifyResult is a successful sign-in.
type VerifyResult struct {
	Tokens domain.TokenPair
	// NewUser tells the client whether to collect a name, rather than the
	// client guessing from an empty profile.
	NewUser bool
}

// Execute verifies a code and issues a session.
func (uc *VerifyOTPUseCase) Execute(ctx context.Context, req VerifyRequest) (VerifyResult, error) {
	phone, err := domain.NewPhone(req.Phone)
	if err != nil {
		return VerifyResult{}, errs.Wrap(err, errs.KindInvalid, "invalid_phone",
			"Please enter a valid Bangladeshi mobile number.")
	}
	if !req.Role.IsValid() {
		return VerifyResult{}, errs.New(errs.KindInvalid, "invalid_role",
			"That sign-in type is not recognised.")
	}

	settings, err := loadAuthSettings(ctx, uc.cfg, req.Placement)
	if err != nil {
		return VerifyResult{}, err
	}

	// The lockout is checked before the code is even parsed, so a locked number
	// costs an attacker a rejected request rather than a comparison.
	attempts, err := uc.otps.FailedAttempts(ctx, phone)
	if err != nil {
		return VerifyResult{}, errs.Wrap(err, errs.KindUnavailable, "otp_store_unavailable",
			"We could not check that code. Please try again.")
	}
	if attempts >= settings.otpVerifyAttempts {
		return VerifyResult{}, lockedOut(settings.otpLockoutWindow)
	}

	code, err := domain.ParseOTP(req.Code)
	if err != nil {
		// A malformed code still counts as an attempt. Excluding it would let
		// an attacker probe timing and state for free by sending rubbish.
		if _, recordErr := uc.recordFailure(ctx, phone, settings); recordErr != nil {
			return VerifyResult{}, recordErr
		}
		return VerifyResult{}, errs.Wrap(err, errs.KindInvalid, "invalid_code",
			"That code is not correct.")
	}

	// Peeked, not taken: a mistyped digit must not destroy the real code the
	// user is still holding.
	stored, ok, err := uc.otps.PeekOTP(ctx, phone)
	if err != nil {
		return VerifyResult{}, errs.Wrap(err, errs.KindUnavailable, "otp_store_unavailable",
			"We could not check that code. Please try again.")
	}
	if !ok || !code.Matches(phone, stored) {
		remaining, recordErr := uc.recordFailure(ctx, phone, settings)
		if recordErr != nil {
			return VerifyResult{}, recordErr
		}
		if remaining <= 0 {
			return VerifyResult{}, lockedOut(settings.otpLockoutWindow)
		}
		// The same message whether the code was wrong or had expired: telling
		// them apart tells an attacker whether a code is currently live.
		return VerifyResult{}, errs.New(errs.KindUnauthorized, "invalid_code",
			"That code is not correct.").
			With("attempts_remaining", fmt.Sprintf("%d", remaining))
	}

	// Correct. Consume it atomically — a read-then-delete would let two
	// concurrent submissions of the same code both create a session.
	if _, consumed, takeErr := uc.otps.TakeOTP(ctx, phone); takeErr != nil {
		return VerifyResult{}, errs.Wrap(takeErr, errs.KindUnavailable, "otp_store_unavailable",
			"We could not check that code. Please try again.")
	} else if !consumed {
		// Another request took it first. That request is the one that gets the
		// session; this one is a replay of a spent code.
		return VerifyResult{}, errs.New(errs.KindUnauthorized, "invalid_code",
			"That code is not correct.")
	}

	if err := uc.otps.ClearAttempts(ctx, phone); err != nil {
		// Not fatal: the user has proven themselves, and the counter expires on
		// its own. Failing the sign-in here would punish them for our outage.
		uc.logger.Warn("could not clear otp attempts", "phone", phone.Masked(), "error", err)
	}

	userID, created, err := uc.users.EnsureUser(ctx, phone, req.Role)
	if err != nil {
		return VerifyResult{}, errs.Wrap(err, errs.KindUnavailable, "user_directory_unavailable",
			"We could not sign you in just now. Please try again.")
	}

	pair, err := uc.issueSession(ctx, userID, req.Role, req.Device, settings)
	if err != nil {
		return VerifyResult{}, err
	}

	uc.logger.Info("sign-in",
		"user_id", userID, "role", req.Role.String(),
		"session_id", pair.SessionID, "new_user", created)

	return VerifyResult{Tokens: pair, NewUser: created}, nil
}

// recordFailure increments the counter and returns how many attempts remain.
func (uc *VerifyOTPUseCase) recordFailure(ctx context.Context, phone domain.Phone, settings authSettings) (int, error) {
	used, err := uc.otps.RecordFailedAttempt(ctx, phone, settings.otpLockoutWindow)
	if err != nil {
		return 0, errs.Wrap(err, errs.KindUnavailable, "otp_store_unavailable",
			"We could not check that code. Please try again.")
	}
	remaining := settings.otpVerifyAttempts - used
	if remaining <= 0 {
		// On lockout the pending code is destroyed, so waiting out the window
		// does not leave a still-valid code to try again with.
		if err := uc.otps.DeleteOTP(ctx, phone); err != nil {
			uc.logger.Warn("could not clear otp on lockout", "phone", phone.Masked(), "error", err)
		}
		uc.logger.Warn("phone locked out after failed codes", "phone", phone.Masked(), "attempts", used)
	}
	return remaining, nil
}

func lockedOut(window time.Duration) error {
	return errs.New(errs.KindRateLimited, "phone_locked_out",
		"Too many incorrect codes. Please wait before trying again.").
		With("retry_after_seconds", fmt.Sprintf("%d", int64(window.Seconds())))
}

// issueSession creates the session and the first token pair, evicting the
// oldest session when the device limit is reached.
func (uc *VerifyOTPUseCase) issueSession(
	ctx context.Context,
	userID string,
	role domain.Role,
	device string,
	settings authSettings,
) (domain.TokenPair, error) {
	now := uc.clock.Now()

	// Enforce the device cap before creating another. Evicting the oldest
	// rather than refusing the new one is deliberate: a user signing in on a
	// new phone should not be blocked by a session on a device they no longer
	// own and cannot reach to sign out of.
	existing, err := uc.sessions.UserSessions(ctx, userID)
	if err != nil {
		return domain.TokenPair{}, errs.Wrap(err, errs.KindUnavailable, "session_store_unavailable",
			"We could not sign you in just now. Please try again.")
	}
	for len(existing) >= settings.maxSessions && len(existing) > 0 {
		oldest := existing[0]
		if err := uc.sessions.RevokeSession(ctx, oldest.ID); err != nil {
			return domain.TokenPair{}, errs.Wrap(err, errs.KindUnavailable, "session_store_unavailable",
				"We could not sign you in just now. Please try again.")
		}
		uc.logger.Info("evicted oldest session at device limit",
			"user_id", userID, "session_id", oldest.ID, "limit", settings.maxSessions)
		existing = existing[1:]
	}

	session := domain.Session{
		ID:         uc.ids.New("ses"),
		UserID:     userID,
		Role:       role,
		Device:     truncateDevice(device),
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(settings.refreshTokenTTL),
	}

	refresh, err := domain.GenerateRefreshToken(uc.entropy)
	if err != nil {
		return domain.TokenPair{}, errs.Wrap(err, errs.KindInternal, "token_generation_failed",
			"We could not sign you in just now. Please try again.")
	}
	if err := uc.sessions.CreateSession(ctx, session, refresh.Hash(), settings.refreshTokenTTL); err != nil {
		return domain.TokenPair{}, errs.Wrap(err, errs.KindUnavailable, "session_store_unavailable",
			"We could not sign you in just now. Please try again.")
	}

	access, err := signAccessToken(uc.signer, uc.ids, session, now, settings.accessTokenTTL)
	if err != nil {
		return domain.TokenPair{}, err
	}

	return domain.TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(settings.accessTokenTTL.Seconds()),
		SessionID:    session.ID,
		Role:         role,
		UserID:       userID,
	}, nil
}

// signAccessToken mints one access token for a session.
func signAccessToken(signer ports.TokenSigner, ids id.Generator, session domain.Session, now time.Time, ttl time.Duration) (string, error) {
	token, err := signer.Sign(domain.Claims{
		UserID:    session.UserID,
		Role:      session.Role,
		SessionID: session.ID,
		TokenID:   ids.New("jti"),
		IssuedAt:  now,
		ExpiresAt: now.Add(ttl),
	})
	if err != nil {
		return "", errs.Wrap(err, errs.KindInternal, "token_signing_failed",
			"We could not sign you in just now. Please try again.")
	}
	return token, nil
}

// maxDeviceLabel caps the device name a client may send.
//
// It is displayed in the active-devices list and stored per session, so an
// unbounded string is both a storage cost and a way to make that screen
// unreadable.
const maxDeviceLabel = 64

func truncateDevice(device string) string {
	if len(device) <= maxDeviceLabel {
		return device
	}
	return device[:maxDeviceLabel]
}
