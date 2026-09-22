// Package dispatch is payment's view of the dispatch module.
//
// The only thing payment needs from dispatch: which partner a signed-in
// account is, so a rider asking "what cash am I carrying" is answered against
// their own ledger and nobody else's. No adapter struct here, the same as
// order/external/dispatch: dispatch's own application.Service already has a
// PartnerOfUser method of this exact shape — three primitives, nothing borrowed
// from another module's contract type — so it satisfies Service structurally
// and cmd/api wires it in directly.
package dispatch

import "context"

// Service is the part of the dispatch module payment depends on.
type Service interface {
	// PartnerOfUser resolves a signed-in account to its own partner id.
	PartnerOfUser(ctx context.Context, userID string) (partnerID string, found bool, err error)
}
