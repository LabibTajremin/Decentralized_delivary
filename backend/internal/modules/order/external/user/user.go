// Package user is the order module's view of the user module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package user

import (
	"context"

	usercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/user/contract"
)

// Address is one entry in a customer's address book.
type Address = usercontract.Address

// Profile is a customer's own details.
type Profile = usercontract.Profile

// Service is the part of user the order depends on.
//
// The order copies the address rather than referring to it. A customer who
// edits an address after ordering must not silently redirect a rider who is
// already on the road, and an address deleted next month must not erase where
// last month's order went.
type Service interface {
	Address(ctx context.Context, userID, addressID string) (Address, error)
	Profile(ctx context.Context, userID string) (Profile, error)
}
