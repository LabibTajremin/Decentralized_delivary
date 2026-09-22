package identity

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/sms"
)

// TestTheLoggingSenderRefusesProduction is the guard that matters. A one-time
// code written to the application log is readable by everyone with log access,
// which in a real deployment is far more people than should be able to sign in
// as a customer.
func TestTheLoggingSenderRefusesProduction(t *testing.T) {
	sender, err := sms.NewLogSender(slog.Default(), true)
	if !errors.Is(err, sms.ErrNotForProduction) {
		t.Fatalf("error = %v, want ErrNotForProduction", err)
	}
	if sender != nil {
		t.Error("a sender was returned for production")
	}
}

// Outside production it works, because it is how a developer signs in locally
// without an SMS account.
func TestTheLoggingSenderWritesTheCodeWithAWarning(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	sender, err := sms.NewLogSender(logger, false)
	if err != nil {
		t.Fatalf("NewLogSender: %v", err)
	}
	if err := sender.SendOTP(context.Background(), phone(t, "01712345678"), "123456"); err != nil {
		t.Fatalf("SendOTP: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "123456") {
		t.Errorf("the code was not logged, so a developer cannot sign in: %s", output)
	}
	if !strings.Contains(output, "WARN") {
		t.Errorf("the code was logged without a warning: %s", output)
	}
	// The number is masked even here: the log is the one place this code is
	// least private, and pairing a full number with a live code is worse than
	// either alone.
	if strings.Contains(output, "12345678") {
		t.Errorf("the full phone number was logged alongside the code: %s", output)
	}
}

// TestTheDemoSenderRefusesProduction. It is the one adapter in this codebase
// that deliberately weakens authentication, so the refusal is the first thing
// asserted about it.
func TestTheDemoSenderRefusesProduction(t *testing.T) {
	sender, err := sms.NewDemoSender(slog.Default(), true)
	if !errors.Is(err, sms.ErrDemoNotForProduction) {
		t.Fatalf("error = %v, want ErrDemoNotForProduction", err)
	}
	if sender != nil {
		t.Error("a demo sender was returned for production")
	}
	// Its own error rather than LogSender's, so a startup failure tells an
	// operator which of the two they configured.
	if errors.Is(err, sms.ErrNotForProduction) {
		t.Error("the demo refusal is indistinguishable from the logging sender's")
	}
}

// Outside production it sends nothing, says so loudly, and does not write the
// code anywhere. The code reaches the caller through the challenge response,
// which is a place that lives for sixty seconds rather than for as long as the
// logs are kept.
func TestTheDemoSenderSendsNothingAndLogsNoCode(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	sender, err := sms.NewDemoSender(logger, false)
	if err != nil {
		t.Fatalf("NewDemoSender: %v", err)
	}
	if err := sender.SendOTP(context.Background(), phone(t, "01712345678"), "123456"); err != nil {
		t.Fatalf("SendOTP: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "123456") {
		t.Errorf("the demo sender logged the code: %s", output)
	}
	if !strings.Contains(output, "WARN") {
		t.Errorf("a deployment showing codes on screen did so quietly: %s", output)
	}
	if !strings.Contains(output, "DEMO_MODE") {
		t.Errorf("the warning does not name the setting that caused it: %s", output)
	}
	if strings.Contains(output, "12345678") {
		t.Errorf("the full phone number was logged: %s", output)
	}
}

// RevealsCode is a marker, and what it marks is the type satisfying the port
// the use case asks for. A test that called the method would prove nothing;
// this one proves the assertion in RequestOTPUseCase.Execute will succeed.
func TestOnlyTheDemoSenderRevealsItsCodes(t *testing.T) {
	demo, err := sms.NewDemoSender(slog.Default(), false)
	if err != nil {
		t.Fatalf("NewDemoSender: %v", err)
	}
	if _, ok := any(demo).(ports.CodeRevealer); !ok {
		t.Error("the demo sender does not satisfy CodeRevealer, so no code is ever shown")
	}
	demo.RevealsCode()

	logging, err := sms.NewLogSender(slog.Default(), false)
	if err != nil {
		t.Fatalf("NewLogSender: %v", err)
	}
	if _, ok := any(logging).(ports.CodeRevealer); ok {
		t.Error("the logging sender claims its codes may be displayed")
	}
}
