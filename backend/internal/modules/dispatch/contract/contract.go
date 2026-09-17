// Package contract is the dispatch module's only public surface.
//
// Consumed by: order, tracking (Appendix A).
//
// Order calls Offer when an order becomes ready to collect, and Withdraw when
// one is called off. Tracking asks where the rider is. Neither may read a job
// table.
package contract

import (
	"context"
	"time"
)

// Place is one end of a delivery, in primitives.
type Place struct {
	Name       string
	Phone      string
	SingleLine string
	Lat        float64
	Lng        float64
}

// OfferRequest is an order that needs a rider.
//
// Everything dispatch needs to put the job on a board, with no second call back
// into order. The distance is passed rather than measured because the order
// already had it measured — by geo, at placement, which is the number the
// customer was charged against.
type OfferRequest struct {
	OrderID     string
	Code        string
	Pickup      Place
	Destination Place
	DistanceM   float64
	// AreaCode places the job for configuration: the distance bands and the
	// offer timeout are per-area (Appendix B).
	AreaCode     string
	DistrictCode string
	DivisionCode string
}

// Partner is the rider on a job, as another module sees them.
type Partner struct {
	ID      string
	Name    string
	Phone   string
	Vehicle string
	// Lat and Lng are where they last reported being, so tracking can draw a
	// dot without reaching into dispatch's tables.
	Lat float64
	Lng float64
}

// Job is a delivery as another module sees it.
type Job struct {
	ID      string
	OrderID string
	// Status is "waiting", "offered", "assigned", "collected", "delivered",
	// "failed" or "cancelled".
	Status string
	Live   bool
	// Band is "short", "long" or "beyond" — D4's distance classification.
	Band      string
	DistanceM float64
	// Partner is who has it, or the zero value while nobody does.
	Partner   Partner
	Attempts  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DispatchContract is the dispatch module's public interface.
type DispatchContract interface {
	// Offer puts an order on the board. Idempotent on the order id: an order
	// that reaches `ready` twice — a retry, a re-delivery of an event — must
	// not become two jobs for two riders.
	Offer(ctx context.Context, req OfferRequest) (Job, error)

	// Withdraw takes a job off the board because the order behind it went
	// away. Not an error when there is no job: an order cancelled before it
	// was ever ready never had one.
	Withdraw(ctx context.Context, orderID, reason string) error

	// JobForOrder returns the delivery for an order, if there is one. The bool
	// is false before an order is ready, which is most of its life.
	JobForOrder(ctx context.Context, orderID string) (Job, bool, error)

	// PartnerOfUser resolves a signed-in account to its own partner id, or
	// false when the account is not a partner.
	//
	// Payment's COD ledger is keyed on the partner id dispatch already uses —
	// the same one recorded as the actor on an order's "delivered" event — so a
	// rider asking "what do I owe" needs this one lookup rather than a second
	// copy of the partner registry.
	PartnerOfUser(ctx context.Context, userID string) (partnerID string, found bool, err error)
}
