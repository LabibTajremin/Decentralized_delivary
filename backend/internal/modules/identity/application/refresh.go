package application

import (
	"context"
	"io"
	"log/slog"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// RefreshSessionUseCase rotates a refresh token and issues a new pair.
//
// This is what makes silent auto-login work: the app starts, presents the token
// it kept in secure storage, and the user lands on Home without seeing a login
// screen. It is also the most security-sensitive path in the system, because a
// refresh token is a sixty-day bearer credential.
type RefreshSessionUseCase struct {
	sessions ports.SessionStore
	signer   ports.TokenSigner
	limiter  ports.RateLimiter
	cfg      cfgcontract.Service
	clock    clock.Clock
	ids      id.Generator
	entropy  io.Reader
	logger   *slog.Logger
}

// NewRefreshSessionUseCase wires the use case.
func NewRefreshSessionUseCase(
	sessions ports.SessionStore,
	signer ports.TokenSigner,
	limiter ports.RateLimiter,
	cfg cfgcontract.Service,
	c clock.Clock,
	ids id.Generator,
	entropy io.Reader,
	logger *slog.Logger,
) *RefreshSessionUseCase {
	return &RefreshSessionUseCase{
		sessions: sessions, signer: signer, limiter: limiter, cfg: cfg,
		clock: c, ids: ids, entropy: entropy, logger: logger,
	}
}

// refreshRateLimit caps refreshes per token hash.
//
// Generous, because a legitimate client refreshes about once every fifteen
// minutes, and deliberately not unlimited: without a cap, a stolen token can be
// used to mint access tokens as fast as the network allows.
const (
	refreshRateLimit  = 60
	refreshRateWindow = 60 // seconds
)

// Execute rotates a refresh token.
//
// Three outcomes, and the middle one is the important one:
//
//   - The token is current — rotate it, extend the session, return a new pair.
//   - The token was already rotated away — someone is presenting a spent token,
//     which means a copy exists. Revoke the whole session.
//   - The token is unknown — reject it without saying more.
func (uc *RefreshSessionUseCase) Execute(ctx context.Context, rawToken string, placement cfgcontract.Placement) (domain.TokenPair, error) {
	// Shape first: a flood of garbage then costs a length check rather than a
	// round trip to Redis each.
	presented, err := domain.ParseRefreshToken(rawToken)
	if err != nil {
		return domain.TokenPair{}, errs.Wrap(err, errs.KindUnauthorized, "invalid_refresh_token",
			"Please sign in again.")
	}

	settings, err := loadAuthSettings(ctx, uc.cfg, placement)
	if err != nil {
		return domain.TokenPair{}, err
	}

	oldHash := presented.Hash()

	// Limited by token hash, not by IP: many users share an IP behind carrier
	// NAT, and limiting by IP would lock out a whole neighbourhood because one
	// device misbehaved.
	allowed, _, err := uc.limiter.Allow(ctx, "refresh:"+oldHash, refreshRateLimit, refreshRateWindow*1e9)
	if err != nil {
		return domain.TokenPair{}, errs.Wrap(err, errs.KindUnavailable, "rate_limiter_unavailable",
			"We could not sign you in just now. Please try again.")
	}
	if !allowed {
		return domain.TokenPair{}, errs.New(errs.KindRateLimited, "refresh_rate_exceeded",
			"Too many attempts. Please try again shortly.")
	}

	replacement, err := domain.GenerateRefreshToken(uc.entropy)
	if err != nil {
		return domain.TokenPair{}, errs.Wrap(err, errs.KindInternal, "token_generation_failed",
			"We could not sign you in just now. Please try again.")
	}

	now := uc.clock.Now()

	// One atomic swap. Two refreshes racing from the same device must produce
	// one rotation — not two valid tokens, and not a false theft alarm that
	// signs an innocent user out.
	outcome, session, err := uc.sessions.Rotate(ctx, oldHash, replacement.Hash(), settings.refreshTokenTTL, now)
	if err != nil {
		return domain.TokenPair{}, errs.Wrap(err, errs.KindUnavailable, "session_store_unavailable",
			"We could not sign you in just now. Please try again.")
	}

	switch outcome {
	case ports.RotationReuse:
		// A token that was already rotated has been presented again. The
		// legitimate client replaced its copy, so whoever sent this has a copy
		// they should not have. Which of the two is the thief cannot be known,
		// so the whole session goes: signing the real user out is recoverable,
		// leaving an attacker signed in is not.
		uc.logger.Warn("refresh token reuse detected; revoking session",
			"session_id", session.ID, "user_id", session.UserID)
		if err := uc.sessions.RevokeSession(ctx, session.ID); err != nil {
			uc.logger.Error("could not revoke session after reuse",
				"session_id", session.ID, "error", err)
		}
		return domain.TokenPair{}, errs.Wrap(domain.ErrTokenReuseDetected, errs.KindUnauthorized,
			"token_reuse_detected", "For your security, please sign in again.")

	case ports.RotationUnknownToken:
		// Deliberately indistinguishable from an expired session: saying which
		// would tell an attacker whether a guessed token ever existed.
		return domain.TokenPair{}, errs.Wrap(domain.ErrInvalidRefreshToken, errs.KindUnauthorized,
			"invalid_refresh_token", "Please sign in again.")

	case ports.RotationOK:
	}

	if session.IsExpired(now) {
		if err := uc.sessions.RevokeSession(ctx, session.ID); err != nil {
			uc.logger.Error("could not revoke expired session", "session_id", session.ID, "error", err)
		}
		return domain.TokenPair{}, errs.Wrap(domain.ErrSessionNotFound, errs.KindUnauthorized,
			"session_expired", "Please sign in again.")
	}

	access, err := signAccessToken(uc.signer, uc.ids, session, now, settings.accessTokenTTL)
	if err != nil {
		return domain.TokenPair{}, err
	}

	return domain.TokenPair{
		AccessToken:  access,
		RefreshToken: replacement,
		ExpiresIn:    int64(settings.accessTokenTTL.Seconds()),
		SessionID:    session.ID,
		Role:         session.Role,
		UserID:       session.UserID,
	}, nil
}
