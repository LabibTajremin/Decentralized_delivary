// Package order is the tracking module's view of the order module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package order

import (
	"context"

	ordercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/order/contract"
)

// Order is an order as tracking sees it.
type Order = ordercontract.Order

// Service is the part of order tracking depends on.
//
// One method: order already composed everything a status timeline needs —
// CustomerID to scope who may watch, Status for the moment, Events for a
// history a later screen might want. No adapter struct here: order's own
// application.Service already has an Order method of this exact shape, so it
// satisfies Service structurally and cmd/api wires it in directly.
type Service interface {
	Order(ctx context.Context, orderID string) (Order, error)
}
