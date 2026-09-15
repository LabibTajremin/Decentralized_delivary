// Package discovery is the cart module's view of the discovery module.
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

// Service is the part of discovery the cart depends on.
//
// One question: may this address still order from this shop. The cart asks it
// on every read rather than only at checkout, because a customer who switches
// to an address in another division should find out while the cart is small.
type Service interface {
	Reach(ctx context.Context, from Point, merchantID string) (Reach, error)
}
