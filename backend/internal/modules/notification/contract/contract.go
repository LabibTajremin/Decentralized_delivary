// Package contract is the notification module's only public surface.
//
// Consumed by: order (Appendix A) — order tells a customer their delivery
// moved; a later phase (dispatch offering a rider work, identity confirming a
// sign-in) can reach this contract the same way once it needs to.
package contract

import "context"

// NotificationContract is the notification module's public interface.
type NotificationContract interface {
	// Notify delivers an already-composed message (2.9 — the caller knows
	// what happened and in which language; this module only knows how to
	// reach the person), push first, falling back to SMS.
	Notify(ctx context.Context, userID, title, body string) error
}
