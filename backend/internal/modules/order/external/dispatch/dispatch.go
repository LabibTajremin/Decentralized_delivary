// Package dispatch is the order module's view of the dispatch module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package dispatch

import (
	"context"

	dispatchcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/contract"
)

// OfferRequest is an order that needs a rider.
type OfferRequest = dispatchcontract.OfferRequest

// Place is one end of a delivery.
type Place = dispatchcontract.Place

// Job is a delivery as the order module sees it.
type Job = dispatchcontract.Job

// Service is the part of dispatch the order depends on.
//
// Called when an order changes state, not on a timer: an order reaching `ready`
// should make a rider's phone buzz while the food is hot, and one being
// cancelled should take the job off the board before somebody rides to a shop
// for nothing.
//
// Both calls are best-effort from the order's point of view. A dispatch outage
// must not stop a shop marking food ready — the job is created by the next
// call or by an operator's sweep, and an order stuck at `ready` with no rider
// is visible; an order the shop could not mark ready is not.
type Service interface {
	Offer(ctx context.Context, req OfferRequest) (Job, error)
	Withdraw(ctx context.Context, orderID, reason string) error
}
