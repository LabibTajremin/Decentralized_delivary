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
	cfg, err := config.Load(config.FromMap(map[string]string{
		"APP_ENV":          "production",
		"API_ADDR":         ":9000",
		"DATABASE_URL":     "postgres://db",
		"REDIS_URL":        "redis://cache",
		"LOG_LEVEL":        "warn",
		"SHUTDOWN_TIMEOUT": "45s",
	}))
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Env != "production" || cfg.APIAddr != ":9000" || cfg.LogLevel != "warn" {
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
