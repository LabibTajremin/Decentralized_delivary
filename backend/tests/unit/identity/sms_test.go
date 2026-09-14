package identity

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

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
