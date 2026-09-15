// Package discovery is the order module's view of the discovery module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package discovery

import (
	"context"

	discocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/contract"
)

// Point is a coordinate pair.
type Point = discocontract.Point

// Reach is whether one merchant is visible from one point.
type Reach = discocontract.Reach

// The reasons a shop may be out of reach.
const (
	ReasonOutsideDivision = discocontract.ReasonOutsideDivision
	ReasonBeyondMaxRadius = discocontract.ReasonBeyondMaxRadius
)

// Service is the part of discovery the order depends on.
//
// One call answers three things order needs and would otherwise have to
// assemble: whether D3 allows this delivery at all, how far it is, and which
// expansion rung the customer is on. Measuring the distance here rather than
// taking the cart's word for it is what makes the price order writes down
// authoritative — the cart's number was composed for a screen, and the customer
// controls when that screen was drawn.
type Service interface {
	Reach(ctx context.Context, from Point, merchantID string) (Reach, error)
}
