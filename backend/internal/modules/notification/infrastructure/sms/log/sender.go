// Package log stands in for an SMS provider until one is chosen.
//
// A real adapter belongs here alongside Sender, behind the same interface.
// It is not written yet because no provider has been chosen — the seam is
// what matters, and it exists.
package log

import (
	"context"
	"errors"
	"log/slog"
)

// ErrNotForProduction is returned when the logging sender is constructed in
// a production deployment.
var ErrNotForProduction = errors.New("the logging SMS sender must not be used in production")

// Sender writes an SMS to the log instead of sending it.
//
// This is push's fallback channel, so it carries the same refusal identity's
// own OTP sender does: a text written to the log is readable by anyone with
// log access, which in a real deployment must never include what was texted
// to a customer's phone.
type Sender struct {
	logger *slog.Logger
}

// New builds a development sender.
func New(logger *slog.Logger, production bool) (*Sender, error) {
	if production {
		return nil, ErrNotForProduction
	}
	return &Sender{logger: logger}, nil
}

// Send logs the message instead of texting it.
func (s *Sender) Send(_ context.Context, phone, body string) error {
	s.logger.Warn("SMS is not configured; the fallback notification is being written to the log. "+
		"This must never happen in production.",
		"phone", phone, "body", body,
	)
	return nil
}
