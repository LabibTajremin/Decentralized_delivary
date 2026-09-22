// Package contract is the config module's only public surface.
//
// Pricing, discovery, dispatch and order all read configuration. They depend on
// this interface through their own external/ package and never on config's
// domain (05-architecture.md 2.5).
//
// Consumed by: discovery, pricing, dispatch, order, admin (Appendix A).
package contract

import "context"

// Placement is where a decision is being made. It is the output of geo's
// ResolveArea, restated in primitives so config and geo stay independent of
// each other's types.
type Placement struct {
	AreaCode     string
	DistrictCode string
	DivisionCode string
}

// The configuration keys, as consumers name them.
//
// Exported here so a consumer writes contract.PricingDeliveryBase rather than a
// string literal. A renamed key then breaks the build instead of silently
// falling back to a default, which is the failure that would otherwise go
// unnoticed until someone checked a fee by hand.
const (
	DiscoveryBaseRadius     = "discovery.base_radius"
	DiscoveryExpansionStep  = "discovery.expansion_step"
	DiscoveryMaxExpansions  = "discovery.max_expansions"
	DiscoveryMinMerchants   = "discovery.min_merchants"
	DiscoveryAutoExpand     = "discovery.auto_expand"
	DiscoveryDivisionCeil   = "discovery.division_ceiling"
	PricingDeliveryBase     = "pricing.delivery_base"
	PricingDeliveryPerKm    = "pricing.delivery_per_km"
	PricingExpansionMult    = "pricing.expansion_multiplier"
	PricingFreeDelivery     = "pricing.free_delivery_threshold"
	DispatchShortDistance   = "dispatch.short_distance_max"
	DispatchLongDistance    = "dispatch.long_distance_max"
	DispatchPartnerRadius   = "dispatch.partner_radius"
	DispatchAssignTimeout   = "dispatch.assignment_timeout"
	DispatchMaxConcurrent   = "dispatch.max_concurrent_jobs"
	OrderCancellationWindow = "order.cancellation_window"
	OrderCODLimit           = "order.cod_limit"

	AuthOTPRequestsPerHour = "auth.otp_requests_per_hour"
	AuthOTPVerifyAttempts  = "auth.otp_verify_attempts"
	AuthOTPLockoutWindow   = "auth.otp_lockout_window"
	AuthOTPTTL             = "auth.otp_ttl"
	// These two are flagged by gosec's hardcoded-credential rule because the
	// identifier contains "token" and the value is a literal. They are the
	// names of configuration keys, not secrets: the signing key lives in the
	// environment and never in the database (ADR 0004).
	AuthAccessTokenTTL  = "auth.access_token_ttl"  //nolint:gosec // a config key name, not a credential
	AuthRefreshTokenTTL = "auth.refresh_token_ttl" //nolint:gosec // a config key name, not a credential
	AuthMaxSessions     = "auth.max_sessions_per_user"
)

// Settings is a resolved configuration snapshot.
//
// Taken once per request and passed down, rather than each use case fetching
// what it needs: a fee calculated with a 5 km radius and a distance measured
// against a 10 km one is the kind of inconsistency that only appears under load
// and is almost impossible to reproduce.
type Settings interface {
	// Int returns a count, money amount in minor units, distance in metres, or
	// duration in seconds. Money is never a float (2.9); distance is always
	// metres so no caller has to remember a scale factor.
	Int(key string) (int64, error)

	// Bool returns a switch.
	Bool(key string) (bool, error)

	// Ratio returns a multiplier.
	Ratio(key string) (float64, error)
}

// ConfigContract is the config module's public interface.
type ConfigContract interface {
	// Settings resolves the effective configuration for a placement, applying
	// overrides area → district → division → global.
	Settings(ctx context.Context, placement Placement) (Settings, error)
}
