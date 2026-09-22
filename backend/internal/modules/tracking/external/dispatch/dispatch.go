// Package dispatch is the tracking module's view of the dispatch module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package dispatch

import (
	"context"

	dispatchcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/contract"
)

// Job is a delivery as tracking sees it.
type Job = dispatchcontract.Job

// Partner is the rider on a job, as tracking sees them.
type Partner = dispatchcontract.Partner

// Service is the part of dispatch tracking depends on.
//
// No adapter struct here, the same as payment/external/dispatch: dispatch's
// own application.Service already has both methods of this exact shape, so
// it satisfies Service structurally and cmd/api wires it in directly.
type Service interface {
	// JobForOrder is where the rider is, when there is one — dispatch already
	// carries the partner's last reported location on the job (P12), so
	// tracking never reaches into dispatch's tables for it.
	JobForOrder(ctx context.Context, orderID string) (Job, bool, error)

	// PartnerOfUser resolves the caller to their own partner id, so a rider
	// asking to watch a delivery is only let in when it is theirs to carry.
	PartnerOfUser(ctx context.Context, userID string) (partnerID string, found bool, err error)
}
