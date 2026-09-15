// Package config is the order module's view of the config module.
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

// The keys the order reads. Both are business rules an operator tunes per area
// rather than deploy-time settings: a division seeing cash-handling trouble
// must be able to lower its COD ceiling without a redeploy and without lowering
// it everywhere.
const (
	CODLimit           = cfgcontract.OrderCODLimit
	CancellationWindow = cfgcontract.OrderCancellationWindow
)

// Service is the part of config the order depends on.
type Service interface {
	Settings(ctx context.Context, placement Placement) (Settings, error)
}
