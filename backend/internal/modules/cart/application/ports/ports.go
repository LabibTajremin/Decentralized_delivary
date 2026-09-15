// Package ports declares what the cart use cases need from the outside world.
// No use case ever touches a database handle (05-architecture.md 2.3).
package ports

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
)

// Repository stores carts.
//
// Whole carts, not lines. A cart is one thing a customer is looking at: saving
// a line at a time leaves a window where the quantity has changed and the note
// has not, and the customer refreshing in that window sees a cart that was
// never true.
type Repository interface {
	// OfUser returns the customer's open cart. The second return is false when
	// they have none, which is not an error — it is most customers, most of
	// the time.
	OfUser(ctx context.Context, userID string) (domain.Cart, bool, error)

	// Save writes a cart and its lines in one transaction, creating it if
	// absent.
	Save(ctx context.Context, c domain.Cart) error

	// Delete removes a cart entirely, for the customer who abandons it or
	// switches shops.
	Delete(ctx context.Context, cartID string) error
}
