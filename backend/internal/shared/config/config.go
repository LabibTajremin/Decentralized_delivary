// Package config loads process configuration from the environment.
//
// This is deployment configuration only — where to bind, where Postgres and
// Redis live, how to sign tokens, what public URL the apps should call. It is
// read once at startup and changing it means a restart.
//
// Business variables (radius, fees, COD limits, commission) are NOT here. They
// live in the config module, resolve per area, and are tunable at runtime by an
// admin without a deploy. See docs/build/appendix-b-config.md and ADR 0004: a
// rule that needs a redeploy to change is a rule in the wrong place.
package config

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Lookup reads an environment variable. os.LookupEnv satisfies it; tests pass
// a map-backed function instead of mutating the real environment.
type Lookup func(key string) (string, bool)

// FromMap builds a Lookup over a fixed map, for tests.
func FromMap(m map[string]string) Lookup {
	return func(key string) (string, bool) {
		v, ok := m[key]
		return v, ok
	}
}

// EnvProduction is the value of APP_ENV that turns on the strict checks.
const EnvProduction = "production"

// devSigningKey is used when no key is configured outside production. It is a
// fixed, obviously fake value so a leaked development token is worthless and a
// production deployment cannot accidentally inherit a "random" key that happens
// to work.
const devSigningKey = "insecure-development-signing-key-do-not-use"

// devWebhookSecret is PaymentWebhookSecret's equivalent outside production.
const devWebhookSecret = "insecure-development-webhook-secret-do-not-use"

// Config is the process configuration.
type Config struct {
	// Env is "development", "staging" or "production". Production turns on the
	// strict checks below.
	Env string

	// APIAddr is the address the server binds, such as ":8080".
	APIAddr string

	// PublicBaseURL is the URL clients actually call, such as
	// "https://api.goklay.com". It differs from APIAddr behind a load balancer,
	// and the Flutter apps are built against it.
	PublicBaseURL string

	// DatabaseURL is the Postgres/PostGIS connection string.
	DatabaseURL string

	// RedisURL is the Redis connection string used for sessions, refresh
	// tokens and rate limiting (ADR 0005).
	RedisURL string

	// JWTSigningKey signs access tokens. Required in production; never stored
	// in the database, because anything in the database is readable by an admin
	// who should not be able to mint tokens.
	JWTSigningKey string

	// JWTIssuer is the `iss` claim, so tokens from another environment are
	// rejected rather than silently accepted.
	JWTIssuer string

	// PaymentWebhookSecret signs and verifies inbound payment gateway webhooks.
	// Same treatment as the JWT key and for the same reason: never stored in
	// the database, where an admin with read access could forge a "payment
	// captured" event for an order they did not pay for.
	PaymentWebhookSecret string

	// CORSAllowedOrigins lists the web origins allowed to call the API. Empty
	// means none, which is correct for a mobile-only deployment.
	CORSAllowedOrigins []string

	// TrackingStreamInterval is how often the live order-tracking endpoint
	// polls order and dispatch for a change to send. A deploy-time knob, not
	// an Appendix B business rule: it trades server load against how quickly
	// a customer's screen updates, not anything a division's operations team
	// would tune.
	TrackingStreamInterval time.Duration

	// LogLevel is debug, info, warn or error.
	LogLevel string

	// ShutdownGap is how long to let in-flight requests finish on SIGTERM.
	ShutdownGap time.Duration
}

// IsProduction reports whether the strict checks apply.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// Loader accumulates values and the errors found while reading them, so one
// pass reports every problem instead of failing on the first.
type Loader struct {
	lookup Lookup
	errs   []string
}

// NewLoader builds a Loader. A nil lookup reads the real environment.
func NewLoader(lookup Lookup) *Loader {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	return &Loader{lookup: lookup}
}

// String reads a string value, falling back to def when unset or empty.
func (l *Loader) String(key, def string) string {
	v, ok := l.lookup(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// Required reads a value that has no sensible default, recording an error when
// it is missing.
func (l *Loader) Required(key string) string {
	v, ok := l.lookup(key)
	if !ok || strings.TrimSpace(v) == "" {
		l.errs = append(l.errs, key+" is required")
		return ""
	}
	return v
}

// Duration reads a Go duration such as "10s", recording an error when the value
// is present but unparseable.
func (l *Loader) Duration(key string, def time.Duration) time.Duration {
	v, ok := l.lookup(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		l.errs = append(l.errs, fmt.Sprintf("%s is not a duration: %q", key, v))
		return def
	}
	return d
}

// Int reads an integer, recording an error when the value is unparseable.
func (l *Loader) Int(key string, def int) int {
	v, ok := l.lookup(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l.errs = append(l.errs, fmt.Sprintf("%s is not an integer: %q", key, v))
		return def
	}
	return n
}

// Bool reads a boolean, recording an error when the value is unparseable.
func (l *Loader) Bool(key string, def bool) bool {
	v, ok := l.lookup(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		l.errs = append(l.errs, fmt.Sprintf("%s is not a boolean: %q", key, v))
		return def
	}
	return b
}

// CSV reads a comma-separated list, trimming blanks. An unset key yields nil
// rather than a one-element slice containing "".
func (l *Loader) CSV(key string) []string {
	v, ok := l.lookup(key)
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// URL reads a value that must parse as an absolute http or https URL. A
// misconfigured base URL is worth catching at startup: the mobile apps are
// built against it, so a bad value ships to devices.
func (l *Loader) URL(key, def string) string {
	raw := l.String(key, def)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		l.errs = append(l.errs, fmt.Sprintf("%s must be an absolute http(s) URL: %q", key, raw))
		return def
	}
	return strings.TrimRight(raw, "/")
}

// Err returns every problem found, sorted, or nil when the load was clean.
func (l *Loader) Err() error {
	if len(l.errs) == 0 {
		return nil
	}
	sorted := append([]string(nil), l.errs...)
	sort.Strings(sorted)
	return fmt.Errorf("config: %s", strings.Join(sorted, "; "))
}

// Load reads the process configuration, reporting all problems at once.
//
// Every value here is set at deploy time. See .env.example for the full list
// with defaults.
func Load(lookup Lookup) (Config, error) {
	l := NewLoader(lookup)

	env := l.String("APP_ENV", "development")
	production := env == EnvProduction

	cfg := Config{
		Env:                    env,
		APIAddr:                l.String("API_ADDR", ":8080"),
		PublicBaseURL:          l.URL("PUBLIC_BASE_URL", "http://localhost:8080"),
		DatabaseURL:            l.Required("DATABASE_URL"),
		RedisURL:               l.Required("REDIS_URL"),
		JWTIssuer:              l.String("JWT_ISSUER", "goklay"),
		CORSAllowedOrigins:     l.CSV("CORS_ALLOWED_ORIGINS"),
		LogLevel:               l.String("LOG_LEVEL", "info"),
		ShutdownGap:            l.Duration("SHUTDOWN_TIMEOUT", 10*time.Second),
		TrackingStreamInterval: l.Duration("TRACKING_STREAM_INTERVAL", 3*time.Second),
	}

	// The signing key is required in production and defaulted elsewhere, so a
	// developer can run the stack with no secrets while a production deploy
	// cannot start without one.
	if production {
		cfg.JWTSigningKey = l.Required("JWT_SIGNING_KEY")
		if cfg.JWTSigningKey == devSigningKey {
			l.errs = append(l.errs, "JWT_SIGNING_KEY must not be the development key in production")
		}
		if len(cfg.JWTSigningKey) > 0 && len(cfg.JWTSigningKey) < 32 {
			l.errs = append(l.errs, "JWT_SIGNING_KEY must be at least 32 characters")
		}
		if strings.HasPrefix(cfg.PublicBaseURL, "http://") {
			l.errs = append(l.errs, "PUBLIC_BASE_URL must use https in production")
		}
		cfg.PaymentWebhookSecret = l.Required("PAYMENT_WEBHOOK_SECRET")
		if cfg.PaymentWebhookSecret == devWebhookSecret {
			l.errs = append(l.errs, "PAYMENT_WEBHOOK_SECRET must not be the development key in production")
		}
		if len(cfg.PaymentWebhookSecret) > 0 && len(cfg.PaymentWebhookSecret) < 32 {
			l.errs = append(l.errs, "PAYMENT_WEBHOOK_SECRET must be at least 32 characters")
		}
	} else {
		cfg.JWTSigningKey = l.String("JWT_SIGNING_KEY", devSigningKey)
		cfg.PaymentWebhookSecret = l.String("PAYMENT_WEBHOOK_SECRET", devWebhookSecret)
	}

	return cfg, l.Err()
}
