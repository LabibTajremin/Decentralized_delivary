package domain

import "time"

// The free cancellation window (`order.cancellation_window`, Appendix B).
//
// Two questions live here, and keeping them apart matters. The transition table
// says whether a cancellation is *possible* from where the order is; this says
// whether it is *free*. An order that has been accepted and is four minutes old
// can be cancelled by the table and refused by the clock, and the customer
// deserves to be told which.

// CancelDecision is whether a customer may call an order off, and why not.
type CancelDecision struct {
	// Allowed is whether to go ahead.
	Allowed bool
	// Reason names the refusal: "already_finished", "too_late",
	// "already_preparing", or empty when allowed.
	Reason string
	// FreeUntil is when the window closes, so a client can show a countdown
	// rather than a button that stops working without explanation.
	FreeUntil time.Time
	// SecondsLeft is the same fact as a number, because a phone's clock and the
	// server's disagree and the server's is the one that decides.
	SecondsLeft int
}

// The reasons a customer cancellation is refused.
const (
	// ReasonAlreadyFinished is a terminal order.
	ReasonAlreadyFinished = "already_finished"
	// ReasonTooLate is the window having closed.
	ReasonTooLate = "too_late"
	// ReasonUnderway is the shop already cooking. The transition table refuses
	// it too; this names it so the customer is told about the food rather than
	// about a state machine.
	ReasonUnderway = "already_preparing"
)

// MayCustomerCancel decides whether a customer may call this order off now.
//
// The window runs from placement rather than from acceptance, deliberately. A
// customer who changes their mind does so about the decision they made, and a
// shop that takes six minutes to accept should not thereby shorten — or
// silently extend — the time the customer had.
func (o Order) MayCustomerCancel(window time.Duration, now time.Time) CancelDecision {
	freeUntil := o.PlacedAt.Add(window)
	left := int(freeUntil.Sub(now).Seconds())
	if left < 0 {
		left = 0
	}
	decision := CancelDecision{FreeUntil: freeUntil, SecondsLeft: left}

	switch {
	case o.Status.IsTerminal():
		decision.Reason = ReasonAlreadyFinished
		return decision
	case CanTransition(o.Status, StatusCancelled, ActorCustomer) != nil:
		// The table says no from here — the shop is already preparing, or
		// further on.
		decision.Reason = ReasonUnderway
		return decision
	case !now.Before(freeUntil):
		decision.Reason = ReasonTooLate
		return decision
	}

	decision.Allowed = true
	return decision
}
