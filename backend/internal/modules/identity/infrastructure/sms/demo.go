package sms

import (
	"context"
	"log/slog"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
)

// DemoSender delivers nothing and allows the code to be shown to whoever asked
// for it.
//
// This is what makes a published demonstration possible at all. A visitor who
// types a phone number they do not own — or a demo number nobody owns — cannot
// receive an SMS, so a demo with a real sender is a demo nobody can sign in
// to, and a demo with LogSender is one where only whoever can read the server
// log can sign in.
//
// It is the one adapter in this codebase that deliberately weakens
// authentication, so it is fenced three ways: it refuses to be constructed in
// production; the configuration that selects it refuses to be set in
// production; and the code it reveals is revealed only through
// [ports.CodeRevealer], which nothing but this type implements.
//
// What it does not do is weaken anything else. The code is still generated
// with the same entropy, still stored as a hash, still rate limited, still
// locked out after too many wrong attempts, and still expires. A demo visitor
// signs in through exactly the path a real user would; they are simply handed
// the code instead of being sent it.
type DemoSender struct {
	logger *slog.Logger
}

// NewDemoSender builds the demo sender.
//
// The production flag is passed in for the same reason LogSender's is: so the
// refusal is testable, and so one place decides what production means.
func NewDemoSender(logger *slog.Logger, production bool) (*DemoSender, error) {
	if production {
		return nil, ErrDemoNotForProduction
	}
	return &DemoSender{logger: logger}, nil
}

// SendOTP sends nothing. The code reaches the caller through the challenge
// response instead — see [DemoSender.RevealsCode].
func (s *DemoSender) SendOTP(_ context.Context, phone domain.Phone, _ string) error {
	s.logger.Warn("DEMO_MODE is on: no SMS was sent and the one-time code is "+
		"being shown to whoever asked for it. This must never happen in production.",
		"phone", phone.Masked(),
	)
	return nil
}

// RevealsCode marks this sender as one whose codes may be displayed.
//
// A marker rather than a getter: the code itself is already in the use case's
// hand at the moment it decides, and passing it back out through the sender
// would mean the sender held a credential it has no reason to keep.
func (s *DemoSender) RevealsCode() {}

// The code is deliberately not logged. LogSender writes it because a developer
// reading their own terminal is the delivery mechanism; here the delivery
// mechanism is the response, and writing it to the log as well would put it
// somewhere with a much longer life than the sixty seconds it is good for.
