// Package config loads process configuration from the environment.
//
// This is bootstrap configuration only — the address to bind, where Postgres
// and Redis live. Business variables (radius, fees, COD limits) are NOT here:
// they live in the config module, are resolved per area and are tunable at
// runtime by an admin. See docs/build/appendix-b-config.md.
package config

import (
	"fmt"
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

// Config is the process configuration.
type Config struct {
	Env         string
	APIAddr     string
	DatabaseURL string
	RedisURL    string
	LogLevel    string
	ShutdownGap time.Duration
}

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
func Load(lookup Lookup) (Config, error) {
	l := NewLoader(lookup)
	cfg := Config{
		Env:         l.String("APP_ENV", "development"),
		APIAddr:     l.String("API_ADDR", ":8080"),
		DatabaseURL: l.Required("DATABASE_URL"),
		RedisURL:    l.Required("REDIS_URL"),
		LogLevel:    l.String("LOG_LEVEL", "info"),
		ShutdownGap: l.Duration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
	return cfg, l.Err()
}
