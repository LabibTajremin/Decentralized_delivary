// Package contract is payment's only public surface, and the whole of the
// boundary 2.6 requires for this module.
//
// Consumed by: order (the COD collection hook), review/support (P16's refund
// workflow). Every type here is built from primitives — no shared Go types
// with any other module, because this module is Go today and a separate .NET
// service tomorrow, and the day it moves, PaymentContract's shape is the
// wire contract, unchanged.
package contract

import "context"

// Money is an amount as it crosses this module's boundary: the integer to
// compute with and the string to render (2.9).
type Money struct {
	Minor    int64
	Currency string
	Display  string
}

// Payment is a gateway payment attempt as another module sees it.
type Payment struct {
	ID      string
	OrderID string
	// Status is "pending", "captured", "failed" or "refunded".
	Status string
	Amount Money
	Reason string
}

// PaymentContract is the payment module's public interface.
type PaymentContract interface {
	// PaymentFor returns the most recent gateway attempt for an order, if
	// there is one. False for a cash order, which never has a gateway
	// payment at all.
	PaymentFor(ctx context.Context, orderID string) (Payment, bool, error)

	// Refund gives a captured payment back. Idempotent: refunding an already
	// refunded payment succeeds without asking the gateway twice.
	Refund(ctx context.Context, orderID, reason string) error

	// RecordCashCollection is order's delivery hook: a partner has just
	// handed over goods paid for in cash, and is now carrying that cash.
	// Idempotent on the order id.
	RecordCashCollection(ctx context.Context, orderID, partnerID string, amountMinor int64) error
}
