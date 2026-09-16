// Package config is the dispatch module's view of the config module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package config

import (
	"context"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
)

// Placement is where a decision is being made.
type Placement = cfgcontract.Placement

// Settings is a resolved configuration snapshot.
type Settings = cfgcontract.Settings

// The keys dispatch reads, all of them per-area business rules rather than
// deploy-time settings: a division with long distances and few riders needs a
// wider partner radius and a longer offer timeout than central Dhaka, and an
// operator must be able to say so without a redeploy.
const (
	ShortDistanceMax = cfgcontract.DispatchShortDistance
	LongDistanceMax  = cfgcontract.DispatchLongDistance
	PartnerRadius    = cfgcontract.DispatchPartnerRadius
	AssignTimeout    = cfgcontract.DispatchAssignTimeout
	MaxConcurrent    = cfgcontract.DispatchMaxConcurrent
)

// Service is the part of config dispatch depends on.
type Service interface {
	Settings(ctx context.Context, placement Placement) (Settings, error)
}
