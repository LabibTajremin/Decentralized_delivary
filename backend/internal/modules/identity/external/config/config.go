// Package config is identity's view of the config module.
//
// Cross-module access goes through a module's own external/ package, which
// depends only on the other module's contract (05-architecture.md 2.5). The
// indirection is what makes extracting either module into its own service a
// change to this one file rather than to every caller.
package config

import (
	"context"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
)

// The auth configuration keys identity reads, re-exported so the use cases
// never import the config module directly.
const (
	AuthOTPRequestsPerHour = cfgcontract.AuthOTPRequestsPerHour
	AuthOTPVerifyAttempts  = cfgcontract.AuthOTPVerifyAttempts
	AuthOTPLockoutWindow   = cfgcontract.AuthOTPLockoutWindow
	AuthOTPTTL             = cfgcontract.AuthOTPTTL
	AuthAccessTokenTTL     = cfgcontract.AuthAccessTokenTTL
	AuthRefreshTokenTTL    = cfgcontract.AuthRefreshTokenTTL
	AuthMaxSessions        = cfgcontract.AuthMaxSessions
)

// Placement is where a decision is being made.
type Placement = cfgcontract.Placement

// Settings is a resolved configuration snapshot.
type Settings = cfgcontract.Settings

// Service is the part of the config module identity depends on.
//
// Narrower than cfgcontract.ConfigContract on purpose: identity needs to read
// settings and nothing else, and an interface that only declares what is used
// cannot quietly grow a dependency.
type Service interface {
	Settings(ctx context.Context, placement Placement) (Settings, error)
}
