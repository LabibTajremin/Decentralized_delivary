package shared

import (
	"strings"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/config"
)

func TestLoadUsesDefaults(t *testing.T) {
	cfg, err := config.Load(config.FromMap(map[string]string{
		"DATABASE_URL": "postgres://x",
		"REDIS_URL":    "redis://y",
	}))
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
	if cfg.APIAddr != ":8080" {
		t.Errorf("APIAddr = %q, want :8080", cfg.APIAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.ShutdownGap != 10*time.Second {
		t.Errorf("ShutdownGap = %v, want 10s", cfg.ShutdownGap)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	// Staging, not production: the production path has its own strict checks
	// covered by TestProductionRequiresRealSecrets and friends.
	cfg, err := config.Load(config.FromMap(map[string]string{
		"APP_ENV":          "staging",
		"API_ADDR":         ":9000",
		"DATABASE_URL":     "postgres://db",
		"REDIS_URL":        "redis://cache",
		"LOG_LEVEL":        "warn",
		"SHUTDOWN_TIMEOUT": "45s",
	}))
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Env != "staging" || cfg.APIAddr != ":9000" || cfg.LogLevel != "warn" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
	if cfg.ShutdownGap != 45*time.Second {
		t.Errorf("ShutdownGap = %v, want 45s", cfg.ShutdownGap)
	}
}

func TestLoadReportsEveryMissingRequiredValueAtOnce(t *testing.T) {
	_, err := config.Load(config.FromMap(map[string]string{}))
	if err == nil {
		t.Fatal("Load must fail when required values are missing")
	}
	msg := err.Error()
	for _, want := range []string{"DATABASE_URL is required", "REDIS_URL is required"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q is missing %q — one pass should report every problem", msg, want)
		}
	}
}

func TestRequiredTreatsBlankAsMissing(t *testing.T) {
	_, err := config.Load(config.FromMap(map[string]string{
		"DATABASE_URL": "   ",
		"REDIS_URL":    "redis://y",
	}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Errorf("error = %v, want a whitespace-only value treated as missing", err)
	}
}

func TestLoaderStringFallsBackOnBlank(t *testing.T) {
	l := config.NewLoader(config.FromMap(map[string]string{"K": "  "}))
	if got := l.String("K", "fallback"); got != "fallback" {
		t.Errorf("String = %q, want fallback", got)
	}
	if got := l.String("MISSING", "fallback"); got != "fallback" {
		t.Errorf("String(missing) = %q, want fallback", got)
	}
	if err := l.Err(); err != nil {
		t.Errorf("String must not record errors, got %v", err)
	}
}

func TestLoaderInt(t *testing.T) {
	l := config.NewLoader(config.FromMap(map[string]string{"N": "7", "BAD": "seven", "BLANK": ""}))
	if got := l.Int("N", 1); got != 7 {
		t.Errorf("Int = %d, want 7", got)
	}
	if got := l.Int("MISSING", 3); got != 3 {
		t.Errorf("Int(missing) = %d, want the default 3", got)
	}
	if got := l.Int("BLANK", 4); got != 4 {
		t.Errorf("Int(blank) = %d, want the default 4", got)
	}
	if got := l.Int("BAD", 5); got != 5 {
		t.Errorf("Int(bad) = %d, want the default 5", got)
	}
	if err := l.Err(); err == nil || !strings.Contains(err.Error(), "BAD is not an integer") {
		t.Errorf("Err = %v, want a report about BAD", err)
	}
}

func TestLoaderBool(t *testing.T) {
	l := config.NewLoader(config.FromMap(map[string]string{"T": "true", "BAD": "yesish", "BLANK": ""}))
	if !l.Bool("T", false) {
		t.Error("Bool = false, want true")
	}
	if l.Bool("MISSING", false) {
		t.Error("Bool(missing) should return the default")
	}
	if !l.Bool("BLANK", true) {
		t.Error("Bool(blank) should return the default")
	}
	if l.Bool("BAD", false) {
		t.Error("Bool(bad) should return the default")
	}
	if err := l.Err(); err == nil || !strings.Contains(err.Error(), "BAD is not a boolean") {
		t.Errorf("Err = %v, want a report about BAD", err)
	}
}

func TestLoaderDuration(t *testing.T) {
	l := config.NewLoader(config.FromMap(map[string]string{"D": "2m", "BAD": "soon", "BLANK": ""}))
	if got := l.Duration("D", time.Second); got != 2*time.Minute {
		t.Errorf("Duration = %v, want 2m", got)
	}
	if got := l.Duration("MISSING", time.Second); got != time.Second {
		t.Errorf("Duration(missing) = %v, want the default", got)
	}
	if got := l.Duration("BLANK", 3*time.Second); got != 3*time.Second {
		t.Errorf("Duration(blank) = %v, want the default", got)
	}
	if got := l.Duration("BAD", 4*time.Second); got != 4*time.Second {
		t.Errorf("Duration(bad) = %v, want the default", got)
	}
	if err := l.Err(); err == nil || !strings.Contains(err.Error(), "BAD is not a duration") {
		t.Errorf("Err = %v, want a report about BAD", err)
	}
}

func TestLoaderErrIsSortedAndStable(t *testing.T) {
	l := config.NewLoader(config.FromMap(map[string]string{"Z": "x", "A": "x"}))
	l.Int("Z", 0)
	l.Int("A", 0)
	err := l.Err()
	if err == nil {
		t.Fatal("expected errors")
	}
	msg := err.Error()
	iA, iZ := strings.Index(msg, "A is not an integer"), strings.Index(msg, "Z is not an integer")
	if iA < 0 || iZ < 0 {
		t.Fatalf("Err = %q, want both problems reported", msg)
	}
	if iA > iZ {
		t.Errorf("Err = %q, want problems sorted by key", msg)
	}
}

func TestNewLoaderNilLookupReadsRealEnvironment(t *testing.T) {
	t.Setenv("P01_PROBE", "present")
	l := config.NewLoader(nil)
	if got := l.String("P01_PROBE", "fallback"); got != "present" {
		t.Errorf("String = %q, want the value from the real environment", got)
	}
}

func TestLoadDeploymentDefaults(t *testing.T) {
	cfg, err := config.Load(config.FromMap(map[string]string{
		"DATABASE_URL": "postgres://x",
		"REDIS_URL":    "redis://y",
	}))
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.PublicBaseURL != "http://localhost:8080" {
		t.Errorf("PublicBaseURL = %q, want the local default", cfg.PublicBaseURL)
	}
	if cfg.JWTIssuer != "goklay" {
		t.Errorf("JWTIssuer = %q, want goklay", cfg.JWTIssuer)
	}
	if cfg.JWTSigningKey == "" {
		t.Error("development must get a signing key so the stack runs with no secrets")
	}
	if cfg.PaymentWebhookSecret == "" {
		t.Error("development must get a webhook secret so the stack runs with no secrets")
	}
	if cfg.CORSAllowedOrigins != nil {
		t.Errorf("CORSAllowedOrigins = %v, want nil for a mobile-only deployment", cfg.CORSAllowedOrigins)
	}
	if cfg.IsProduction() {
		t.Error("default env must not be production")
	}
}

func TestLoadTrimsTrailingSlashFromBaseURL(t *testing.T) {
	cfg, err := config.Load(config.FromMap(map[string]string{
		"DATABASE_URL":    "postgres://x",
		"REDIS_URL":       "redis://y",
		"PUBLIC_BASE_URL": "https://api.goklay.com/",
	}))
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.PublicBaseURL != "https://api.goklay.com" {
		t.Errorf("PublicBaseURL = %q, want the trailing slash trimmed", cfg.PublicBaseURL)
	}
}

func TestLoadRejectsBadBaseURL(t *testing.T) {
	for _, bad := range []string{"not-a-url", "ftp://api.goklay.com", "/relative/path"} {
		_, err := config.Load(config.FromMap(map[string]string{
			"DATABASE_URL":    "postgres://x",
			"REDIS_URL":       "redis://y",
			"PUBLIC_BASE_URL": bad,
		}))
		if err == nil || !strings.Contains(err.Error(), "PUBLIC_BASE_URL must be an absolute http(s) URL") {
			t.Errorf("PUBLIC_BASE_URL=%q: error = %v, want a rejection", bad, err)
		}
	}
}

func TestLoadParsesCORSOrigins(t *testing.T) {
	cfg, err := config.Load(config.FromMap(map[string]string{
		"DATABASE_URL":         "postgres://x",
		"REDIS_URL":            "redis://y",
		"CORS_ALLOWED_ORIGINS": "https://admin.goklay.com, https://ops.goklay.com ,",
	}))
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	want := []string{"https://admin.goklay.com", "https://ops.goklay.com"}
	if len(cfg.CORSAllowedOrigins) != len(want) {
		t.Fatalf("origins = %v, want %v", cfg.CORSAllowedOrigins, want)
	}
	for i := range want {
		if cfg.CORSAllowedOrigins[i] != want[i] {
			t.Errorf("origin[%d] = %q, want %q", i, cfg.CORSAllowedOrigins[i], want[i])
		}
	}
}

func TestCSVOfOnlySeparatorsIsNil(t *testing.T) {
	l := config.NewLoader(config.FromMap(map[string]string{"K": " , , "}))
	if got := l.CSV("K"); got != nil {
		t.Errorf("CSV = %v, want nil when every entry is blank", got)
	}
}

// TestProductionRequiresRealSecrets is the deployment guard: a production
// process must not start with development defaults.
func TestProductionRequiresRealSecrets(t *testing.T) {
	_, err := config.Load(config.FromMap(map[string]string{
		"APP_ENV":      "production",
		"DATABASE_URL": "postgres://x",
		"REDIS_URL":    "redis://y",
	}))
	if err == nil {
		t.Fatal("production must not start without a signing key")
	}
	if !strings.Contains(err.Error(), "JWT_SIGNING_KEY is required") {
		t.Errorf("error = %v, want a missing signing key", err)
	}
	if !strings.Contains(err.Error(), "PAYMENT_WEBHOOK_SECRET is required") {
		t.Errorf("error = %v, want a missing webhook secret", err)
	}
	if !strings.Contains(err.Error(), "PUBLIC_BASE_URL must use https in production") {
		t.Errorf("error = %v, want the http base URL rejected in production", err)
	}
}

// TestProductionRejectsTheDevelopmentWebhookSecret mirrors the signing-key
// guard: a webhook secret an attacker can read out of this repository is a
// secret that signs nothing.
func TestProductionRejectsTheDevelopmentWebhookSecret(t *testing.T) {
	_, err := config.Load(config.FromMap(map[string]string{
		"APP_ENV":                "production",
		"DATABASE_URL":           "postgres://x",
		"REDIS_URL":              "redis://y",
		"PUBLIC_BASE_URL":        "https://api.goklay.com",
		"JWT_SIGNING_KEY":        strings.Repeat("k", 48),
		"PAYMENT_WEBHOOK_SECRET": "insecure-development-webhook-secret-do-not-use",
	}))
	if err == nil || !strings.Contains(err.Error(), "PAYMENT_WEBHOOK_SECRET must not be the development key") {
		t.Errorf("error = %v, want the development webhook secret rejected", err)
	}
}

func TestProductionRejectsAShortWebhookSecret(t *testing.T) {
	_, err := config.Load(config.FromMap(map[string]string{
		"APP_ENV":                "production",
		"DATABASE_URL":           "postgres://x",
		"REDIS_URL":              "redis://y",
		"PUBLIC_BASE_URL":        "https://api.goklay.com",
		"JWT_SIGNING_KEY":        strings.Repeat("k", 48),
		"PAYMENT_WEBHOOK_SECRET": "tooshort",
	}))
	if err == nil || !strings.Contains(err.Error(), "PAYMENT_WEBHOOK_SECRET") || !strings.Contains(err.Error(), "at least 32 characters") {
		t.Errorf("error = %v, want a short webhook secret rejected", err)
	}
}

func TestProductionRejectsTheDevelopmentKey(t *testing.T) {
	_, err := config.Load(config.FromMap(map[string]string{
		"APP_ENV":         "production",
		"DATABASE_URL":    "postgres://x",
		"REDIS_URL":       "redis://y",
		"PUBLIC_BASE_URL": "https://api.goklay.com",
		"JWT_SIGNING_KEY": "insecure-development-signing-key-do-not-use",
	}))
	if err == nil || !strings.Contains(err.Error(), "must not be the development key") {
		t.Errorf("error = %v, want the development key rejected", err)
	}
}

func TestProductionRejectsShortKey(t *testing.T) {
	_, err := config.Load(config.FromMap(map[string]string{
		"APP_ENV":         "production",
		"DATABASE_URL":    "postgres://x",
		"REDIS_URL":       "redis://y",
		"PUBLIC_BASE_URL": "https://api.goklay.com",
		"JWT_SIGNING_KEY": "tooshort",
	}))
	if err == nil || !strings.Contains(err.Error(), "at least 32 characters") {
		t.Errorf("error = %v, want a short key rejected", err)
	}
}

func TestProductionAcceptsAProperDeployment(t *testing.T) {
	cfg, err := config.Load(config.FromMap(map[string]string{
		"APP_ENV":                "production",
		"API_ADDR":               ":8080",
		"PUBLIC_BASE_URL":        "https://api.goklay.com",
		"DATABASE_URL":           "postgres://user:pass@db:5432/delivery",
		"REDIS_URL":              "redis://cache:6379/0",
		"JWT_SIGNING_KEY":        strings.Repeat("k", 48),
		"JWT_ISSUER":             "goklay-prod",
		"PAYMENT_WEBHOOK_SECRET": strings.Repeat("w", 40),
		"CORS_ALLOWED_ORIGINS":   "https://admin.goklay.com",
		"LOG_LEVEL":              "warn",
		"SHUTDOWN_TIMEOUT":       "30s",
	}))
	if err != nil {
		t.Fatalf("a complete production config must load: %v", err)
	}
	if !cfg.IsProduction() {
		t.Error("IsProduction() = false, want true")
	}
	if cfg.JWTIssuer != "goklay-prod" || cfg.PublicBaseURL != "https://api.goklay.com" {
		t.Errorf("config = %+v", cfg)
	}
	if cfg.PaymentWebhookSecret != strings.Repeat("w", 40) {
		t.Errorf("PaymentWebhookSecret = %q", cfg.PaymentWebhookSecret)
	}
}

func TestLoaderURLWithNoValueAndNoDefault(t *testing.T) {
	l := config.NewLoader(config.FromMap(map[string]string{}))
	if got := l.URL("MISSING", ""); got != "" {
		t.Errorf("URL = %q, want empty when neither a value nor a default is set", got)
	}
	if err := l.Err(); err != nil {
		t.Errorf("an absent optional URL must not be an error, got %v", err)
	}
}
