// Package contract is the user module's only public surface.
//
// Not in the Appendix A table, which lists user only as a consumer of geo. It
// is needed all the same: order must know where to deliver, and dispatch must
// know who to hand the parcel to. Adding it here rather than letting order read
// the user tables keeps the rule that a module never reaches into another's
// storage — see docs/technical/user.md.
//
// Consumed by: order, dispatch, tracking.
package contract

import "context"

// Address is a delivery destination, in primitives.
//
// It carries the preformatted single line as well as the parts. Every surface
// that shows an address — the app, a receipt, the rider's screen — shows the
// same string, because a client that joins the parts itself will eventually
// join them differently (2.9).
type Address struct {
	ID             string
	Label          string
	RecipientName  string
	RecipientPhone string
	Line1          string
	Line2          string
	Instructions   string
	SingleLine     string
	Lat            float64
	Lng            float64

	// The placement, so a consumer can resolve config or price without a second
	// call into geo.
	AreaCode     string
	AreaName     string
	DistrictCode string
	DivisionCode string
}

// Profile is what a person has told us about themselves.
type Profile struct {
	UserID string
	Name   string
	// DisplayName is never empty: the server decides the fallback so two clients
	// do not invent two different greetings.
	DisplayName string
	Email       string
	Language    string
}

// UserContract is the user module's public interface.
type UserContract interface {
	// Profile returns a user's profile, inventing an empty one if they have
	// never saved anything.
	Profile(ctx context.Context, userID string) (Profile, error)

	// DefaultAddress returns where to deliver by default.
	DefaultAddress(ctx context.Context, userID string) (Address, error)

	// Address returns one of the user's addresses.
	//
	// The user id is part of the lookup, not a check afterwards: an accessor
	// that can return another user's address is one careless call away from
	// leaking a home address and a phone number.
	Address(ctx context.Context, userID, addressID string) (Address, error)
}
