// Package identity is the notification module's view of the identity module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package identity

import "context"

// Service is the part of identity notification depends on.
//
// One lookup: the number behind an account id, so a push that could not be
// delivered has somewhere else to go. No adapter struct here, the same as
// payment/external/dispatch: identity's own application.Service already has
// a PhoneFor method of this exact shape, so it satisfies Service
// structurally and cmd/api wires it in directly.
type Service interface {
	PhoneFor(ctx context.Context, userID string) (phone string, found bool, err error)
}
