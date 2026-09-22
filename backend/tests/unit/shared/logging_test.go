package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/logging"
)

// decode reads the single JSON log record written to buf.
func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var out map[string]any
	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("no log record written")
	}
	if err := json.Unmarshal([]byte(line), &out); err != nil {
		t.Fatalf("log line is not JSON: %v (%q)", err, line)
	}
	return out
}

func TestNewWritesJSON(t *testing.T) {
	var buf bytes.Buffer
	logging.New(&buf, logging.LevelInfo).Info("order placed", "order_id", "GK-1")
	rec := decode(t, &buf)

	if rec["msg"] != "order placed" {
		t.Errorf("msg = %v, want %q", rec["msg"], "order placed")
	}
	if rec["order_id"] != "GK-1" {
		t.Errorf("order_id = %v, want GK-1", rec["order_id"])
	}
}

func TestSensitiveValuesAreRedacted(t *testing.T) {
	sensitive := []string{"password", "otp", "token", "access_token", "refresh_token", "authorization", "phone", "secret", "api_key"}
	for _, key := range sensitive {
		var buf bytes.Buffer
		logging.New(&buf, logging.LevelInfo).Info("attempt", key, "the-real-value")
		rec := decode(t, &buf)
		if rec[key] != logging.Redacted {
			t.Errorf("%s = %v, want %q", key, rec[key], logging.Redacted)
		}
	}
}

func TestRedactionIsCaseInsensitive(t *testing.T) {
	var buf bytes.Buffer
	logging.New(&buf, logging.LevelInfo).Info("attempt", "OTP", "123456", "Phone", "+8801700000000")
	rec := decode(t, &buf)
	if rec["OTP"] != logging.Redacted || rec["Phone"] != logging.Redacted {
		t.Errorf("record = %v, want both redacted regardless of case", rec)
	}
}

func TestRedactionReachesInsideGroups(t *testing.T) {
	var buf bytes.Buffer
	logging.New(&buf, logging.LevelInfo).
		With(slog.Group("auth", "otp", "999111")).
		Info("verify")
	if strings.Contains(buf.String(), "999111") {
		t.Errorf("a grouped sensitive value leaked: %s", buf.String())
	}
}

func TestNonSensitiveValuesSurvive(t *testing.T) {
	var buf bytes.Buffer
	logging.New(&buf, logging.LevelInfo).Info("ok", "zone", "Zone 4")
	if rec := decode(t, &buf); rec["zone"] != "Zone 4" {
		t.Errorf("zone = %v, want it kept", rec["zone"])
	}
}

func TestLevelFiltering(t *testing.T) {
	cases := []struct {
		level      logging.Level
		debugShown bool
		warnShown  bool
	}{
		{logging.LevelDebug, true, true},
		{logging.LevelInfo, false, true},
		{logging.LevelWarn, false, true},
		{logging.LevelError, false, false},
		{logging.Level("nonsense"), false, true}, // unknown falls back to info
	}
	for _, c := range cases {
		var buf bytes.Buffer
		l := logging.New(&buf, c.level)
		l.Debug("d")
		if got := strings.Contains(buf.String(), `"msg":"d"`); got != c.debugShown {
			t.Errorf("level %q debug shown = %v, want %v", c.level, got, c.debugShown)
		}
		buf.Reset()
		l.Warn("w")
		if got := strings.Contains(buf.String(), `"msg":"w"`); got != c.warnShown {
			t.Errorf("level %q warn shown = %v, want %v", c.level, got, c.warnShown)
		}
	}
}

func TestContextRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := logging.New(&buf, logging.LevelInfo)
	ctx := logging.Into(context.Background(), want)
	if got := logging.From(ctx); got != want {
		t.Error("From must return the logger put in by Into")
	}
}

func TestFromReturnsUsableLoggerWhenAbsent(t *testing.T) {
	// No panic and no output: call sites never need a nil check.
	logging.From(context.Background()).Info("dropped")
}

func TestFromIgnoresNilLoggerInContext(t *testing.T) {
	ctx := logging.Into(context.Background(), nil)
	if logging.From(ctx) == nil {
		t.Error("From must never return nil")
	}
	logging.From(ctx).Info("still safe")
}

func TestNewAcceptsDiscardWriter(t *testing.T) {
	logging.New(io.Discard, logging.LevelError).Error("gone")
}
