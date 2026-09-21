// Package domain holds tracking's one real rule: what belongs in a moment's
// view of a delivery, and whether two moments differ in any way a screen
// would show.
package domain

import "time"

// Partner is the rider on a live job, as tracking shows them.
type Partner struct {
	ID      string
	Name    string
	Phone   string
	Vehicle string
	Lat     float64
	Lng     float64
}

// Snapshot is one moment of an order's delivery, composed for a screen.
type Snapshot struct {
	OrderID string
	// Status is the order's raw lifecycle state, and StatusLabel is it
	// composed for a screen in the caller's language (2.9): a client renders,
	// it does not translate.
	Status      string
	StatusLabel string
	// Live is whether there is anything left to watch. A stream sends this
	// snapshot once and stops.
	Live bool
	// Partner is who is carrying it, or nil before a job exists or after one
	// no longer has a rider holding it.
	Partner   *Partner
	UpdatedAt time.Time
}

// Changed reports whether s differs from prev in anything a screen would
// show — the status, whether a rider is now carrying it or no longer is, or
// where that rider has moved to.
//
// A stream that sent every snapshot on every tick, whether or not anything
// moved, would waste every client's battery redrawing a screen that looks
// identical. This is the one check that decides whether a tick is worth
// sending.
func (s Snapshot) Changed(prev Snapshot) bool {
	if s.Status != prev.Status {
		return true
	}
	if (s.Partner == nil) != (prev.Partner == nil) {
		return true
	}
	if s.Partner != nil && prev.Partner != nil {
		if s.Partner.Lat != prev.Partner.Lat || s.Partner.Lng != prev.Partner.Lng {
			return true
		}
	}
	return false
}
