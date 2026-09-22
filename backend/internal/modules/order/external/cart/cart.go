// Package cart is the order module's view of the cart module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package cart

import (
	"context"

	cartcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/contract"
)

// Cart is a whole cart, revalidated.
type Cart = cartcontract.Cart

// Line is one thing in it.
type Line = cartcontract.Line

// Service is the part of cart the order depends on.
//
// Current returns the cart *revalidated*, at today's prices — an order frozen
// from a stale snapshot is an order the shop disputes. Clear is asked rather
// than done: the cart owns its storage, and order deleting rows it does not own
// is exactly the coupling 2.5 exists to prevent.
type Service interface {
	Current(ctx context.Context, userID string) (Cart, bool, error)
	Clear(ctx context.Context, userID string) error
}
