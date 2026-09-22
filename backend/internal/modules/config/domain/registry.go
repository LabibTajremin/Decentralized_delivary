package domain

import (
	"errors"
	"fmt"
	"sort"
)

// Errors returned when working with keys and definitions.
var (
	// ErrUnknownKey means the key is not in the registry. Configuration is a
	// closed set: a typo must fail rather than silently create a variable
	// nothing reads.
	ErrUnknownKey = errors.New("unknown configuration key")
	// ErrImmutable means the key may never be overridden, by anyone.
	ErrImmutable = errors.New("this variable cannot be changed")
	// ErrNotAutoTunable means the auto-tuner tried to change a variable it is
	// not allowed to touch.
	ErrNotAutoTunable = errors.New("this variable is not auto-tunable")
	// ErrOutOfBounds means a value is outside the admin-set range.
	ErrOutOfBounds = errors.New("configuration value is out of bounds")
	// ErrPinned means an admin has pinned the variable against auto-tuning.
	ErrPinned = errors.New("this variable is pinned in this scope")
)

// Key names a configuration variable.
type Key string

// The Appendix B keys. Every consumer names one of these constants rather than
// a string literal, so a renamed key is a compile error and not a silent
// fallback to the default.
const (
	DiscoveryBaseRadius     Key = "discovery.base_radius"
	DiscoveryExpansionStep  Key = "discovery.expansion_step"
	DiscoveryMaxExpansions  Key = "discovery.max_expansions"
	DiscoveryMinMerchants   Key = "discovery.min_merchants"
	DiscoveryAutoExpand     Key = "discovery.auto_expand"
	DiscoveryDivisionCeil   Key = "discovery.division_ceiling"
	PricingDeliveryBase     Key = "pricing.delivery_base"
	PricingDeliveryPerKm    Key = "pricing.delivery_per_km"
	PricingExpansionMult    Key = "pricing.expansion_multiplier"
	PricingFreeDelivery     Key = "pricing.free_delivery_threshold"
	DispatchShortDistance   Key = "dispatch.short_distance_max"
	DispatchLongDistance    Key = "dispatch.long_distance_max"
	DispatchPartnerRadius   Key = "dispatch.partner_radius"
	DispatchAssignTimeout   Key = "dispatch.assignment_timeout"
	DispatchMaxConcurrent   Key = "dispatch.max_concurrent_jobs"
	OrderCancellationWindow Key = "order.cancellation_window"
	OrderCODLimit           Key = "order.cod_limit"

	// Auth limits (P04). These are business rules, not deployment settings:
	// an operator seeing OTP abuse in one division must be able to tighten the
	// limit there without a redeploy, and without tightening it everywhere.
	AuthOTPRequestsPerHour Key = "auth.otp_requests_per_hour"
	AuthOTPVerifyAttempts  Key = "auth.otp_verify_attempts"
	AuthOTPLockoutWindow   Key = "auth.otp_lockout_window"
	AuthOTPTTL             Key = "auth.otp_ttl"
	// These two are flagged by gosec's hardcoded-credential rule because the
	// identifier contains "token" and the value is a literal. They are the
	// names of configuration keys, not secrets: the signing key lives in the
	// environment and never in the database (ADR 0004).
	AuthAccessTokenTTL  Key = "auth.access_token_ttl"  //nolint:gosec // a config key name, not a credential
	AuthRefreshTokenTTL Key = "auth.refresh_token_ttl" //nolint:gosec // a config key name, not a credential
	AuthMaxSessions     Key = "auth.max_sessions_per_user"
)

// Definition describes one variable.
type Definition struct {
	Key  Key
	Kind Kind
	// Default is what applies when nothing overrides it. Every key has one, so
	// resolution can never fail to produce a value.
	Default Value
	// AutoTunable marks a variable ALG-09 may adjust.
	AutoTunable bool
	// Immutable marks a variable nobody may override — not an admin, not the
	// tuner. Exactly one variable is immutable; see DiscoveryDivisionCeil.
	Immutable bool
	// Min and Max bound any override, admin or tuner. They are the same kind as
	// Default. Bools have no order and carry zero bounds.
	Min, Max Value
	Purpose  string
}

// The table below is written with these constructors rather than the validating
// public ones. Inside this package a Value can be built directly, and every
// literal in the table is a non-negative constant, so there is no error to
// handle and no init-time panic to reason about.
//
// The table's consistency — kinds matching, bounds ordered, defaults inside
// their own bounds — is asserted by TestRegistryMatchesAppendixB and its
// neighbours. A static table should be proven correct by a test before it
// ships, not by a panic when a server boots.
func count(n int64) Value    { return Value{kind: KindCount, whole: n} }
func ratio(f float64) Value  { return Value{kind: KindRatio, ratio: f} }
func money(n int64) Value    { return Value{kind: KindMoney, whole: n} }
func distance(n int64) Value { return Value{kind: KindDistance, whole: n} }
func duration(n int64) Value { return Value{kind: KindDuration, whole: n} }

// taka converts whole taka to minor units, so the table below reads in the
// units Appendix B is written in.
func taka(n int64) Value { return money(n * 100) }

// km converts kilometres to metres, for the same reason.
func km(n int64) Value { return distance(n * 1000) }

// definitions is the Appendix B table, in code.
//
// Bounds exist for every tunable variable. An auto-tuner with no upper bound on
// a delivery fee is one bad input away from quoting a customer a fee nobody
// would pay, and the admin-set range is what makes ALG-09 safe to run
// unattended.
var definitions = []Definition{
	{
		Key: DiscoveryBaseRadius, Kind: KindDistance, Default: km(5),
		AutoTunable: true, Min: km(1), Max: km(25),
		Purpose: "Initial search radius",
	},
	{
		Key: DiscoveryExpansionStep, Kind: KindDistance, Default: km(5),
		AutoTunable: true, Min: km(1), Max: km(10),
		Purpose: "Increment per expansion",
	},
	{
		Key: DiscoveryMaxExpansions, Kind: KindCount, Default: count(4),
		AutoTunable: false, Min: count(0), Max: count(10),
		Purpose: "Hard cap on expansion steps",
	},
	{
		Key: DiscoveryMinMerchants, Kind: KindCount, Default: count(5),
		AutoTunable: false, Min: count(1), Max: count(50),
		Purpose: "Threshold that triggers the expansion offer",
	},
	{
		Key: DiscoveryAutoExpand, Kind: KindBool, Default: Bool(false),
		AutoTunable: false,
		Purpose:     "Expand without asking when below the merchant threshold",
	},
	{
		Key: DiscoveryDivisionCeil, Kind: KindBool, Default: Bool(true),
		AutoTunable: false, Immutable: true,
		Purpose: "Enforces D3. The one variable nothing may change: not an " +
			"admin, not the auto-tuner, not a per-area override. It is in the " +
			"registry so it is visible and auditable, not so it is adjustable.",
	},
	{
		Key: PricingDeliveryBase, Kind: KindMoney, Default: taka(40),
		AutoTunable: true, Min: taka(10), Max: taka(200),
		Purpose: "Base delivery fee",
	},
	{
		Key: PricingDeliveryPerKm, Kind: KindMoney, Default: taka(10),
		AutoTunable: true, Min: taka(2), Max: taka(50),
		Purpose: "Distance rate per kilometre",
	},
	{
		Key: PricingExpansionMult, Kind: KindRatio, Default: ratio(1.5),
		AutoTunable: true, Min: ratio(1), Max: ratio(3),
		Purpose: "Surcharge multiplier when the radius is extended (D2). " +
			"The minimum is 1: a multiplier below 1 would make a longer " +
			"delivery cheaper than a shorter one.",
	},
	{
		Key: PricingFreeDelivery, Kind: KindMoney, Default: taka(500),
		AutoTunable: true, Min: taka(100), Max: taka(5000),
		Purpose: "Order value above which delivery is free",
	},
	{
		Key: DispatchShortDistance, Kind: KindDistance, Default: km(5),
		AutoTunable: true, Min: km(1), Max: km(15),
		Purpose: "Upper bound of a short-distance job (D4)",
	},
	{
		Key: DispatchLongDistance, Kind: KindDistance, Default: km(25),
		AutoTunable: true, Min: km(5), Max: km(100),
		Purpose: "Upper bound of a long-distance job",
	},
	{
		Key: DispatchPartnerRadius, Kind: KindDistance, Default: km(7),
		AutoTunable: true, Min: km(1), Max: km(30),
		Purpose: "Radius of a delivery partner's job feed",
	},
	{
		Key: DispatchAssignTimeout, Kind: KindDuration, Default: duration(30),
		AutoTunable: false, Min: duration(10), Max: duration(300),
		Purpose: "How long an offer stands before it is reoffered",
	},
	{
		Key: DispatchMaxConcurrent, Kind: KindCount, Default: count(3),
		AutoTunable: false, Min: count(1), Max: count(10),
		Purpose: "How many jobs one partner may hold at once",
	},
	{
		Key: OrderCancellationWindow, Kind: KindDuration, Default: duration(120),
		AutoTunable: false, Min: duration(0), Max: duration(900),
		Purpose: "Free cancellation period after placing an order",
	},
	{
		Key: OrderCODLimit, Kind: KindMoney, Default: taka(5000),
		AutoTunable: true, Min: taka(500), Max: taka(50000),
		Purpose: "Maximum value of a cash-on-delivery order",
	},
	{
		Key: AuthOTPRequestsPerHour, Kind: KindCount, Default: count(5),
		AutoTunable: false, Min: count(1), Max: count(20),
		Purpose: "OTP requests allowed per phone number per hour",
	},
	{
		Key: AuthOTPVerifyAttempts, Kind: KindCount, Default: count(5),
		AutoTunable: false, Min: count(3), Max: count(10),
		Purpose: "Wrong OTP entries before the number is locked out. " +
			"The minimum is 3: fewer would lock out people who simply mistype.",
	},
	{
		Key: AuthOTPLockoutWindow, Kind: KindDuration, Default: duration(900),
		AutoTunable: false, Min: duration(60), Max: duration(3600),
		Purpose: "How long a number stays locked out after too many wrong codes",
	},
	{
		Key: AuthOTPTTL, Kind: KindDuration, Default: duration(300),
		AutoTunable: false, Min: duration(60), Max: duration(600),
		Purpose: "How long an OTP stays valid. The maximum is deliberately " +
			"short: a code that lives for an hour is a code an attacker has an " +
			"hour to guess.",
	},
	{
		Key: AuthAccessTokenTTL, Kind: KindDuration, Default: duration(900),
		AutoTunable: false, Min: duration(300), Max: duration(3600),
		Purpose: "Access token lifetime. Access tokens are stateless, so this " +
			"is the blast radius of a revoked session — an hour is the most " +
			"anyone should accept.",
	},
	{
		Key: AuthRefreshTokenTTL, Kind: KindDuration, Default: duration(5_184_000),
		AutoTunable: false, Min: duration(86_400), Max: duration(15_552_000),
		Purpose: "Refresh token lifetime (60 days by default). This is how " +
			"long a user stays silently signed in without opening the app.",
	},
	{
		Key: AuthMaxSessions, Kind: KindCount, Default: count(5),
		AutoTunable: false, Min: count(1), Max: count(20),
		Purpose: "Active devices per user. The oldest session is evicted beyond this.",
	},
}

// registry indexes the definitions by key.
//
// A duplicate key would silently shrink this map, which is why the tests
// compare its size against the Appendix B table rather than trusting the
// literal above to be free of repeats.
var registry = func() map[Key]Definition {
	m := make(map[Key]Definition, len(definitions))
	for _, d := range definitions {
		m[d.Key] = d
	}
	return m
}()

// Lookup returns a definition.
func Lookup(key Key) (Definition, error) {
	d, ok := registry[key]
	if !ok {
		return Definition{}, fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}
	return d, nil
}

// AllKeys returns every key, sorted, so listings and documentation are stable.
func AllKeys() []Key {
	keys := make([]Key, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// AllDefinitions returns every definition in key order.
func AllDefinitions() []Definition {
	keys := AllKeys()
	out := make([]Definition, 0, len(keys))
	for _, k := range keys {
		out = append(out, registry[k])
	}
	return out
}

// InBounds reports whether a value is within a definition's range.
//
// A bool has no order, so its bounds are not checked — the kind check has
// already established it is one of two allowed values.
func (d Definition) InBounds(v Value) error {
	if v.Kind() != d.Kind {
		return fmt.Errorf("%w: %s expects %s, got %s",
			ErrKindMismatch, d.Key, d.Kind, v.Kind())
	}
	if d.Kind == KindBool {
		return nil
	}
	// The kinds are known equal by the check above, and the table guarantees
	// Min and Max share that kind, so the fields are compared directly. Going
	// through the public Compare would add an error path that nothing can
	// reach — and an unreachable error path is untestable by construction.
	outside := false
	if d.Kind == KindRatio {
		outside = v.ratio < d.Min.ratio || v.ratio > d.Max.ratio
	} else {
		outside = v.whole < d.Min.whole || v.whole > d.Max.whole
	}
	if outside {
		return fmt.Errorf("%w: %s must be between %s and %s, got %s",
			ErrOutOfBounds, d.Key, d.Min, d.Max, v)
	}
	return nil
}
