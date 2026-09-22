// Package domain holds payment's own rules, and nothing borrowed.
//
// This is the isolated module (2.6): it is Go today and .NET later, so its
// domain imports nothing from another module — not a type, not a constant —
// only the shared kernel every module may use (ADR 0007). A Payment or a
// Collection is built from primitives and shared/money, never from a type
// order, dispatch or any other module owns. The day this module becomes a
// separate service, that is what makes the move a deployment change rather
// than a rewrite.
package domain

import (
	"errors"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// The rules a payment holds itself to.
var (
	ErrNoOrder        = errors.New("a payment must be about an order")
	ErrNoCustomer     = errors.New("a payment must belong to somebody")
	ErrNoGateway      = errors.New("a payment must name the gateway that will take it")
	ErrNotPositive    = errors.New("a payment must be for a positive amount")
	ErrNotPending     = errors.New("that payment is not waiting to be settled")
	ErrNotCaptured    = errors.New("only a captured payment can be refunded")
	ErrNoReason       = errors.New("that needs a reason")
	ErrAmountMismatch = errors.New("the gateway's amount does not match the payment")
)

// Status is where a gateway payment is in its own life.
type Status string

// The gateway payment lifecycle.
const (
	// StatusPending is a checkout that has been started and not yet answered.
	StatusPending Status = "pending"
	// StatusCaptured is money the gateway confirms it took.
	StatusCaptured Status = "captured"
	// StatusFailed is a checkout that did not complete, and terminal for this
	// attempt — a customer who wants to try again starts a new one.
	StatusFailed Status = "failed"
	// StatusRefunded is captured money given back, and terminal.
	StatusRefunded Status = "refunded"
)

// IsTerminal reports whether a payment has finished moving.
func (s Status) IsTerminal() bool { return s == StatusFailed || s == StatusRefunded }

// Payment is one attempt to collect an order's total through a gateway.
//
// One attempt, not one order: an order whose first attempt failed gets a new
// Payment for the retry rather than a reused one, so the record of what
// actually happened — a failed try, then a captured one — is never overwritten.
type Payment struct {
	ID         string
	OrderID    string
	CustomerID string
	// Gateway names the adapter this attempt went through — "manual" until a
	// real provider is chosen (2.4).
	Gateway string
	// Reference is the identifier this module minted and handed to the
	// gateway as its own merchant reference, so a webhook can be matched back
	// to a payment before the gateway has assigned anything of its own.
	Reference string
	// GatewayRef is the gateway's own transaction id, set once it exists —
	// empty until the gateway has actually done something.
	GatewayRef string

	Amount money.Money
	Status Status
	// FailureReason is set only when Status is StatusFailed.
	FailureReason string
	// RefundReason is set only when Status is StatusRefunded — why the money
	// went back, for whoever has to answer for it later.
	RefundReason string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewPayment opens an attempt to collect an order online.
func NewPayment(id, orderID, customerID, gateway string, amount money.Money, now time.Time) (Payment, error) {
	switch {
	case orderID == "":
		return Payment{}, ErrNoOrder
	case customerID == "":
		return Payment{}, ErrNoCustomer
	case gateway == "":
		return Payment{}, ErrNoGateway
	case amount.IsZero() || amount.IsNegative():
		return Payment{}, ErrNotPositive
	}
	return Payment{
		ID: id, OrderID: orderID, CustomerID: customerID,
		Gateway: gateway, Reference: id,
		Amount: amount, Status: StatusPending,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// Capture is the gateway confirming the money arrived.
//
// The amount the gateway reports has to agree with what was asked for. A
// gateway confirming a different amount is not a payment succeeding with a
// rounding error; it is either a bug in an adapter or a forged webhook, and
// either way marking the order paid for the wrong sum is the one mistake this
// method exists to refuse.
func (p *Payment) Capture(gatewayRef string, amount money.Money, at time.Time) error {
	if p.Status != StatusPending {
		return ErrNotPending
	}
	if p.Amount.Compare(amount) != 0 {
		return ErrAmountMismatch
	}
	p.Status = StatusCaptured
	p.GatewayRef = gatewayRef
	p.UpdatedAt = at
	return nil
}

// Fail is the gateway reporting the checkout did not complete.
func (p *Payment) Fail(reason string, at time.Time) error {
	if p.Status != StatusPending {
		return ErrNotPending
	}
	if reason == "" {
		return ErrNoReason
	}
	p.Status = StatusFailed
	p.FailureReason = reason
	p.UpdatedAt = at
	return nil
}

// Refund gives captured money back.
func (p *Payment) Refund(reason string, at time.Time) error {
	if p.Status != StatusCaptured {
		return ErrNotCaptured
	}
	if reason == "" {
		return ErrNoReason
	}
	p.Status = StatusRefunded
	p.RefundReason = reason
	p.UpdatedAt = at
	return nil
}
