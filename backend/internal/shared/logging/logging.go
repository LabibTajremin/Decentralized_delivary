// Package logging provides structured logging.
//
// Output is JSON so logs are queryable in production. Values known to be
// sensitive — phone numbers, tokens, OTPs — are redacted at the logger rather
// than trusted to every call site, because the call site is exactly where that
// discipline breaks down.
package logging

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

// Level names the verbosity of a logger.
type Level string

const (
	// LevelDebug is developer detail, off in production.
	LevelDebug Level = "debug"
	// LevelInfo is the production default.
	LevelInfo Level = "info"
	// LevelWarn marks recoverable trouble.
	LevelWarn Level = "warn"
	// LevelError marks a failure that needs attention.
	LevelError Level = "error"
)

// slogLevel maps a Level to slog's scale, defaulting to info for anything
// unrecognised so a typo in config cannot silence the logs.
func (l Level) slogLevel() slog.Level {
	switch strings.ToLower(string(l)) {
	case string(LevelDebug):
		return slog.LevelDebug
	case string(LevelWarn):
		return slog.LevelWarn
	case string(LevelError):
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// redactedKeys are attribute names whose values never reach the log.
var redactedKeys = map[string]bool{
	"password":      true,
	"otp":           true,
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"authorization": true,
	"phone":         true,
	"secret":        true,
	"api_key":       true,
}

// Redacted is the placeholder written in place of a sensitive value.
const Redacted = "[REDACTED]"

// redact replaces sensitive values, and is applied to every attribute including
// those nested inside groups.
func redact(_ []string, a slog.Attr) slog.Attr {
	if redactedKeys[strings.ToLower(a.Key)] {
		return slog.String(a.Key, Redacted)
	}
	return a
}

// New builds a JSON logger at the given level.
func New(w io.Writer, level Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level.slogLevel(),
		ReplaceAttr: redact,
	}))
}

// contextKey is unexported so no other package can collide with it.
type contextKey struct{}

// Into returns a context carrying the logger.
func Into(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// From retrieves the logger from a context, falling back to a discarding
// logger so call sites never need a nil check.
func From(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return New(io.Discard, LevelInfo)
}
