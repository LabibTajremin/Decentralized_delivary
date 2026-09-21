// Package payment is the review module's view of the payment module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package payment

import "context"

// Service is the part of payment review depends on.
//
// One method: a support agent resolving a ticket as "refunded" needs exactly
// what PaymentContract.Refund already does — give the money back, once, no
// matter how many times it is asked. No adapter struct here: payment's own
// application.Service already has a Refund method of this exact shape.
type Service interface {
	Refund(ctx context.Context, orderID, reason string) error
}
