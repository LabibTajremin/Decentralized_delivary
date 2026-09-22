// Package config is the pricing module's view of the config module.
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

// The keys pricing reads. Named constants rather than string literals, so a
// renamed key breaks the build instead of silently falling back to a default.
const (
	DeliveryBase   = cfgcontract.PricingDeliveryBase
	DeliveryPerKm  = cfgcontract.PricingDeliveryPerKm
	ExpansionMult  = cfgcontract.PricingExpansionMult
	FreeDeliveryAt = cfgcontract.PricingFreeDelivery
)

// Service is the part of config pricing depends on.
type Service interface {
	Settings(ctx context.Context, placement Placement) (Settings, error)
}
