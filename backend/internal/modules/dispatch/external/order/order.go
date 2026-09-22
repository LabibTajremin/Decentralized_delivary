// Package order is the dispatch module's view of the order module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package order

import (
	"context"

	ordercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/order/contract"
)

// Order is an order as dispatch sees it.
type Order = ordercontract.Order

// Service is the part of order dispatch depends on.
//
// Advance is the way in the order module left for exactly this: a party that
// owns a later stage of an order moving it, with the state machine still
// applied. Dispatch never writes an order's status directly, so a rider tapping
// "delivered" on a job whose order was cancelled underneath them is refused by
// the same table that refuses everybody else.
type Service interface {
	Order(ctx context.Context, orderID string) (Order, error)
	Advance(ctx context.Context, orderID, to, actor, actorID, reason string) error
}
