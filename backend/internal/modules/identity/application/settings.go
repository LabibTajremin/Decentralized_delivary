// Package application holds the identity use cases.
package application

import (
	"context"
	"time"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// authSettings is the resolved auth configuration for one request.
//
// Read once and passed down rather than fetched per use: an OTP written with a
// five-minute TTL and validated against a ten-minute one is the sort of
// inconsistency that only shows up under load.
type authSettings struct {
	otpRequestsPerHour int
	otpVerifyAttempts  int
	otpLockoutWindow   time.Duration
	otpTTL             time.Duration
	accessTokenTTL     time.Duration
	refreshTokenTTL    time.Duration
	maxSessions        int
}

// loadAuthSettings resolves the auth limits for a placement.
//
// Auth limits are per-area like every other business rule, so an operator
// seeing OTP abuse in one division can tighten it there alone. A placement is
// not always known — someone signing in has not chosen an address yet — and an
// empty placement resolves to the global values, which is the right answer.
func loadAuthSettings(ctx context.Context, cfg cfgcontract.Service, placement cfgcontract.Placement) (authSettings, error) {
	settings, err := cfg.Settings(ctx, placement)
	if err != nil {
		return authSettings{}, errs.Wrap(err, errs.KindUnavailable, "auth_config_unavailable",
			"We could not sign you in just now. Please try again.")
	}

	read := func(key string) (int64, error) {
		v, readErr := settings.Int(key)
		if readErr != nil {
			return 0, errs.Wrap(readErr, errs.KindInternal, "auth_config_invalid",
				"We could not sign you in just now.").With("key", key)
		}
		return v, nil
	}

	var out authSettings
	for _, field := range []struct {
		key    string
		assign func(int64)
	}{
		{cfgcontract.AuthOTPRequestsPerHour, func(v int64) { out.otpRequestsPerHour = int(v) }},
		{cfgcontract.AuthOTPVerifyAttempts, func(v int64) { out.otpVerifyAttempts = int(v) }},
		{cfgcontract.AuthOTPLockoutWindow, func(v int64) { out.otpLockoutWindow = time.Duration(v) * time.Second }},
		{cfgcontract.AuthOTPTTL, func(v int64) { out.otpTTL = time.Duration(v) * time.Second }},
		{cfgcontract.AuthAccessTokenTTL, func(v int64) { out.accessTokenTTL = time.Duration(v) * time.Second }},
		{cfgcontract.AuthRefreshTokenTTL, func(v int64) { out.refreshTokenTTL = time.Duration(v) * time.Second }},
		{cfgcontract.AuthMaxSessions, func(v int64) { out.maxSessions = int(v) }},
	} {
		value, readErr := read(field.key)
		if readErr != nil {
			return authSettings{}, readErr
		}
		field.assign(value)
	}
	return out, nil
}
