// Package log stands in for a push provider until one is chosen.
//
// A real adapter (FCM, APNs) belongs here alongside Sender, behind the same
// interface. It is not written yet because no provider has been chosen, and a
// speculative adapter for the wrong API is worse than none — the seam is what
// matters, and it exists.
package log

import (
	"context"
	"errors"
	"log/slog"
)

// ErrNotForProduction is returned when the logging sender is constructed in
// a production deployment.
var ErrNotForProduction = errors.New("the logging push sender must not be used in production")

// Sender writes a push notification to the log instead of sending it.
//
// This is how the stack runs locally with no push account. It refuses to be
// constructed in production — the same guarantee identity's log SMS sender
// makes for OTP codes, for the same reason: a message written to the log is
// readable by anyone with log access, and in a real deployment that is a
// great many more people than should see what was sent to whom.
type Sender struct {
	logger *slog.Logger
}

// New builds a development sender.
//
// The production flag is passed in rather than read from the environment
// here, so the refusal is testable and there is exactly one place that
// decides what "production" means.
func New(logger *slog.Logger, production bool) (*Sender, error) {
	if production {
		return nil, ErrNotForProduction
	}
	return &Sender{logger: logger}, nil
}

// Send logs the message instead of pushing it to a device.
func (s *Sender) Send(_ context.Context, token, title, body string) error {
	s.logger.Warn("push is not configured; the notification is being written to the log. "+
		"This must never happen in production.",
		"token", token, "title", title, "body", body,
	)
	return nil
}
