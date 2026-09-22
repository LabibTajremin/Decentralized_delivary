package identity

import (
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application/ports"
	identityredis "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/redis"
)

// These parsers read replies from this package's own Lua scripts, so a
// malformed reply means the script has been replaced or corrupted. That is
// exactly when a silent misreading is worst — a corrupted rotation script must
// not be read as "rotation succeeded" — so the guards are asserted directly.

func TestParsingARotationReply(t *testing.T) {
	cases := []struct {
		name      string
		reply     any
		outcome   ports.RotationOutcome
		sessionID string
		wantErr   bool
	}{
		{"success", []any{"ok", "ses_1"}, ports.RotationOK, "ses_1", false},
		{"reuse", []any{"reuse", "ses_1"}, ports.RotationReuse, "ses_1", false},
		{"unknown token", []any{"unknown"}, ports.RotationUnknownToken, "", false},
		{"not a list", "ok", ports.RotationUnknownToken, "", true},
		{"empty list", []any{}, ports.RotationUnknownToken, "", true},
		{"nil", nil, ports.RotationUnknownToken, "", true},
		// A success with no session id cannot be acted on: there is nothing to
		// issue a token against, and treating it as success would hand the
		// caller an empty identity.
		{"success with no session", []any{"ok"}, ports.RotationUnknownToken, "", true},
		{"success with a non-string session", []any{"ok", 42}, ports.RotationUnknownToken, "", true},
		// A reuse with no session still reports reuse: the token is refused
		// either way, which is the part that matters.
		{"reuse with no session", []any{"reuse"}, ports.RotationReuse, "", false},
	}

	for _, c := range cases {
		outcome, sessionID, err := identityredis.ParseRotateReply(c.reply)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: error = %v, wantErr %v", c.name, err, c.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if outcome != c.outcome || sessionID != c.sessionID {
			t.Errorf("%s: outcome = %v, session = %q; want %v, %q",
				c.name, outcome, sessionID, c.outcome, c.sessionID)
		}
	}
}

func TestParsingARateLimitReply(t *testing.T) {
	window := time.Minute

	allowed, retryAfter, err := identityredis.ParseAllowReply([]any{int64(3), int64(30)}, 5, window)
	if err != nil || !allowed {
		t.Errorf("within the limit: %v, %v", allowed, err)
	}
	if retryAfter != 30*time.Second {
		t.Errorf("retry-after = %v, want the key's TTL", retryAfter)
	}

	allowed, _, err = identityredis.ParseAllowReply([]any{int64(6), int64(30)}, 5, window)
	if err != nil || allowed {
		t.Errorf("over the limit: %v, %v", allowed, err)
	}

	// The edge: the limit itself is allowed, the one after is not.
	if allowed, _, _ := identityredis.ParseAllowReply([]any{int64(5), int64(30)}, 5, window); !allowed {
		t.Error("the fifth of five was refused")
	}

	// A key with no expiry must not become an unbounded lockout.
	_, retryAfter, err = identityredis.ParseAllowReply([]any{int64(6), int64(-1)}, 5, window)
	if err != nil {
		t.Fatalf("missing TTL: %v", err)
	}
	if retryAfter != window {
		t.Errorf("retry-after = %v, want the full window when the key has no expiry", retryAfter)
	}

	for _, bad := range []any{nil, "not a list", []any{int64(1)}} {
		allowed, _, err := identityredis.ParseAllowReply(bad, 5, window)
		if err == nil {
			t.Errorf("reply %v was accepted", bad)
		}
		if allowed {
			t.Errorf("reply %v allowed the request; a limiter that fails open is not a limiter", bad)
		}
	}
}
