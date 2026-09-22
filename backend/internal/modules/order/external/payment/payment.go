// Package payment is the order module's view of the payment module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5). No adapter struct
// here, the same as external/dispatch: payment's own application.Service
// already has a method of this shape, so it satisfies Service structurally and
// cmd/api wires it in directly.
package payment

import "context"

// Service is the part of payment the order module depends on: the delivery
// hook for a cash order.
//
// Best-effort, the same as the dispatch hooks: a payment outage must not stop
// a rider marking a delivery done. The ledger entry it would have written is
// recoverable — an operator can see a delivered cash order with no collection
// and add one — while a delivery a rider could not mark complete is not.
type Service interface {
	RecordCashCollection(ctx context.Context, orderID, partnerID string, amountMinor int64) error
}
