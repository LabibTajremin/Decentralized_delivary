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
)

// RequestOTPUseCase sends a one-time code to a phone number.
type RequestOTPUseCase struct {
	otps    ports.OTPStore
	limiter ports.RateLimiter
	sms     ports.SMSSender
	cfg     cfgcontract.Service
	clock   clock.Clock
	entropy io.Reader
	logger  *slog.Logger
}

// NewRequestOTPUseCase wires the use case.
func NewRequestOTPUseCase(
	otps ports.OTPStore,
	limiter ports.RateLimiter,
	sms ports.SMSSender,
	cfg cfgcontract.Service,
	c clock.Clock,
	entropy io.Reader,
	logger *slog.Logger,
) *RequestOTPUseCase {
	return &RequestOTPUseCase{
		otps: otps, limiter: limiter, sms: sms, cfg: cfg,
		clock: c, entropy: entropy, logger: logger,
	}
}

// RequestOTPResult is what the caller learns.
//
// It carries no code and no indication of whether the number is registered.
// Both would be information leaks: the first obvious, the second letting anyone
// enumerate which phone numbers have accounts.
type RequestOTPResult struct {
	// MaskedPhone is shown on the verification screen so a user can see they
	// typed the right number, without the screen displaying it in full.
	MaskedPhone string
	// ExpiresIn is how long the code lasts, in seconds. The client counts down
	// with it rather than holding its own idea of the TTL (2.9).
	ExpiresIn int64
	// ResendAfter is how long before another code may be requested.
	ResendAfter int64

	// DemoCode is the code itself, and is empty in every deployment that
	// sends one.
	//
	// It is filled only when the configured sender implements
	// [ports.CodeRevealer] — that is, only in a demo deployment, which cannot
	// be a production one. A visitor to a demo has no phone that will ring,
	// so the alternative is a sign-in screen nobody can get past.
	DemoCode string
}

// Execute issues and sends a code.
func (uc *RequestOTPUseCase) Execute(ctx context.Context, rawPhone string, placement cfgcontract.Placement) (RequestOTPResult, error) {
	phone, err := domain.NewPhone(rawPhone)
	if err != nil {
		return RequestOTPResult{}, errs.Wrap(err, errs.KindInvalid, "invalid_phone",
			"Please enter a valid Bangladeshi mobile number.")
	}

	settings, err := loadAuthSettings(ctx, uc.cfg, placement)
	if err != nil {
		return RequestOTPResult{}, err
	}

	// Rate limit before generating or sending anything. Checking afterwards
	// would still cost an SMS for every blocked request, which is both the
	// expensive part and the part that harasses the number's owner.
	allowed, retryAfter, err := uc.limiter.Allow(ctx,
		"otp_request:"+phone.String(), settings.otpRequestsPerHour, time.Hour)
	if err != nil {
		return RequestOTPResult{}, errs.Wrap(err, errs.KindUnavailable, "rate_limiter_unavailable",
			"We could not send a code just now. Please try again.")
	}
	if !allowed {
		return RequestOTPResult{}, errs.New(errs.KindRateLimited, "otp_requests_exhausted",
			"Too many codes requested for this number. Please wait and try again.").
			With("retry_after_seconds", fmt.Sprintf("%d", int64(retryAfter.Seconds())))
	}

	// A locked-out number gets no new code. Issuing one would let an attacker
	// clear their own way out of a lockout by asking for a fresh code.
	attempts, err := uc.otps.FailedAttempts(ctx, phone)
	if err != nil {
		return RequestOTPResult{}, errs.Wrap(err, errs.KindUnavailable, "otp_store_unavailable",
			"We could not send a code just now. Please try again.")
	}
	if attempts >= settings.otpVerifyAttempts {
		return RequestOTPResult{}, errs.New(errs.KindRateLimited, "phone_locked_out",
			"Too many incorrect codes. Please wait before trying again.")
	}

	code, err := domain.GenerateOTP(uc.entropy)
	if err != nil {
		return RequestOTPResult{}, errs.Wrap(err, errs.KindInternal, "otp_generation_failed",
			"We could not send a code just now. Please try again.")
	}

	// Stored before sending. If the store succeeds and the SMS fails the user
	// simply asks again; if the SMS succeeded and the store had failed, a real
	// code would be in someone's hand that the server cannot recognise.
	if err := uc.otps.PutOTP(ctx, phone, code.Hash(phone), settings.otpTTL); err != nil {
		return RequestOTPResult{}, errs.Wrap(err, errs.KindUnavailable, "otp_store_unavailable",
			"We could not send a code just now. Please try again.")
	}

	if err := uc.sms.SendOTP(ctx, phone, code.String()); err != nil {
		// The stored code is left in place: it is harmless, it expires on its
		// own, and deleting it would race with a message that was actually
		// delivered despite the error.
		uc.logger.Error("otp delivery failed",
			"phone", phone.Masked(),
			"error", err)
		return RequestOTPResult{}, errs.Wrap(err, errs.KindUnavailable, "sms_delivery_failed",
			"We could not send the code. Please check the number and try again.")
	}

	uc.logger.Info("otp issued", "phone", phone.Masked(), "expires_in", settings.otpTTL.Seconds())

	// Asked of the sender rather than of a flag: the only thing that knows
	// whether a code was really delivered somewhere is whatever delivered it.
	var demoCode string
	if _, revealed := uc.sms.(ports.CodeRevealer); revealed {
		demoCode = code.String()
	}

	return RequestOTPResult{
		MaskedPhone: phone.Masked(),
		ExpiresIn:   int64(settings.otpTTL.Seconds()),
		DemoCode:    demoCode,
		// max guards against a divide by zero. Config bounds already forbid a
		// limit of zero, but a crash in the sign-in path is not a risk worth
		// leaving to another package's validation.
		ResendAfter: int64(time.Hour.Seconds()) / int64(max(settings.otpRequestsPerHour, 1)),
	}, nil
}
