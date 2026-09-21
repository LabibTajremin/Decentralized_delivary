// Package order is the review module's view of the order module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5). Review depends on
// order's contract alone — not dispatch's, not catalogue's — which is why
// order.Line carries an ItemID and order.Event carries an ActorID: both were
// added so review can check a rating's subject against an order it already
// has, instead of a second cross-module dependency.
package order

import (
	"context"

	ordercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/order/contract"
)

// Order is an order as review sees it.
type Order = ordercontract.Order

// Service is the part of order review depends on.
//
// One method: an order already carries its customer, its merchant, its
// lines and its delivery history — everything review needs to decide who
// may rate what. No adapter struct here: order's own application.Service
// already has an Order method of this exact shape, so it satisfies Service
// structurally, the pattern established in P13 and reused in every phase
// since.
type Service interface {
	Order(ctx context.Context, orderID string) (Order, error)
}
