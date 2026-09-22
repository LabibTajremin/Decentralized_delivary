// Package contract is the tracking module's only public surface.
//
// Consumed by: order, notification (Appendix A) — a later phase's admin live
// order view (P15) is the shape of caller either would use; nothing in this
// phase calls it yet, the same position identity's own IdentityContract was
// in until payment needed PartnerOfUser's cousin. The interface exists so
// that day is a change to one caller, not a new one.
package contract

import "context"

// Money is not carried here — tracking shows movement, not charges. A
// Snapshot is deliberately smaller than order's own Order: only what changes
// while a delivery is in flight.

// Partner is the rider on a live job, as another module would see it.
type Partner struct {
	ID      string
	Name    string
	Phone   string
	Vehicle string
	Lat     float64
	Lng     float64
}

// Snapshot is one moment of an order's delivery.
type Snapshot struct {
	OrderID     string
	Status      string
	StatusLabel string
	Live        bool
	Partner     *Partner
}

// TrackingContract is the tracking module's public interface.
type TrackingContract interface {
	// Snapshot composes one moment of a delivery. Unscoped: a caller reaching
	// this contract is another backend module, already trusted, not an end
	// user — the ownership check tracking's own transport applies lives on
	// the HTTP path, not here.
	Snapshot(ctx context.Context, orderID, lang string) (Snapshot, error)
}
