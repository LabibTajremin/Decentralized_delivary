// Package sms delivers one-time codes.
//
// A real gateway adapter belongs here alongside LogSender, behind the same
// interface. It is not written yet because no provider has been chosen, and a
// speculative adapter for the wrong API is worse than none — the seam is what
// matters, and it exists.
package sms

import (
	"context"
	"log/slog"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
)

// LogSender writes the code to the log instead of sending it.
//
// This is how a developer signs in locally without an SMS account. It refuses
// to be constructed in production, because a code printed to the application
// log is a code readable by anyone with log access — which in a real deployment
// is a great many more people than should be able to sign in as a customer.
type LogSender struct {
	logger *slog.Logger
}

// NewLogSender builds a development sender.
//
// The production flag is passed in rather than read from the environment here,
// so the refusal is testable and so there is exactly one place that decides what
// "production" means.
func NewLogSender(logger *slog.Logger, production bool) (*LogSender, error) {
	if production {
		return nil, ErrNotForProduction
	}
	return &LogSender{logger: logger}, nil
}

// SendOTP logs the code.
func (s *LogSender) SendOTP(_ context.Context, phone domain.Phone, code string) error {
	s.logger.Warn("SMS is not configured; the one-time code is being written to the log. "+
		"This must never happen in production.",
		"phone", phone.Masked(),
		"code", code,
	)
	return nil
}
