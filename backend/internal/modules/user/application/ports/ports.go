// Package ports declares what the user use cases need from the outside world.
package ports

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/domain"
)

// ProfileRepository stores profiles.
type ProfileRepository interface {
	// Profile returns a user's profile, or domain.ErrProfileNotFound.
	Profile(ctx context.Context, userID string) (domain.Profile, error)

	// SaveProfile writes a profile, creating it if absent.
	SaveProfile(ctx context.Context, profile domain.Profile) error
}

// AddressRepository stores addresses.
type AddressRepository interface {
	// Addresses returns a user's addresses, oldest first.
	Addresses(ctx context.Context, userID string) ([]domain.Address, error)

	// Address returns one address belonging to a user.
	//
	// The user id is part of the lookup rather than a check afterwards: a query
	// that can return another user's address is one call away from a handler
	// that forgets to compare.
	Address(ctx context.Context, userID, addressID string) (domain.Address, error)

	// DefaultAddress returns the user's default, or domain.ErrNoDefaultAddress.
	DefaultAddress(ctx context.Context, userID string) (domain.Address, error)

	// SaveAddresses writes a user's whole address list in one transaction.
	//
	// The whole list rather than one row, because "exactly one default" is a
	// property of the set. Writing rows individually means a window where two
	// are default or none is, and a concurrent read in that window gets a wrong
	// delivery address.
	SaveAddresses(ctx context.Context, userID string, addresses []domain.Address) error

	// DeleteAddress removes one address.
	DeleteAddress(ctx context.Context, userID, addressID string) error
}
