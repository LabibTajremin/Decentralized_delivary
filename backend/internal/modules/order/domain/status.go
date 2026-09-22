// Package domain is the order: what was bought, what it cost, and where it is
// in its life. It imports nothing outside itself except the shared money value
// object (ADR 0007).
//
// The lifecycle is the heart of it. An order moves through the world in one
// direction, and every party — the customer, the shop, the rider, an admin —
// can move it only in the ways that make sense from where they stand. All of
// that lives in one table below rather than scattered through the use cases,
// because a transition rule in two places is a transition rule that will
// disagree with itself.
package domain

import "errors"

// Status is where an order is.
type Status string

// The order lifecycle.
const (
	// StatusPendingPayment is a prepaid order waiting for the money. It exists
	// only on the prepaid path; a cash order is placed outright, because there
	// is nothing to wait for.
	StatusPendingPayment Status = "pending_payment"
	// StatusPlaced is an order the shop has been told about and not yet
	// answered. The free cancellation window runs from here.
	StatusPlaced Status = "placed"
	// StatusAccepted is the shop saying yes.
	StatusAccepted Status = "accepted"
	// StatusPreparing is the shop cooking or picking.
	StatusPreparing Status = "preparing"
	// StatusReady is waiting on the counter for a rider.
	StatusReady Status = "ready"
	// StatusPickedUp is with the rider.
	StatusPickedUp Status = "picked_up"
	// StatusDelivered is done, and terminal.
	StatusDelivered Status = "delivered"
	// StatusCancelled is called off by the customer or an admin, and terminal.
	StatusCancelled Status = "cancelled"
	// StatusRejected is the shop saying no, and terminal. Separate from
	// cancelled because they are different facts about different people, and a
	// shop that rejects often is a shop an admin needs to see.
	StatusRejected Status = "rejected"
	// StatusFailed is a delivery that could not be completed — nobody at the
	// address, a rider accident — and terminal.
	StatusFailed Status = "failed"
)

// Actor is who is making a transition.
//
// Part of the transition table rather than checked separately, because "who may
// do this" and "what may happen next" are the same question asked twice. A
// table that answered only the second would let a customer mark their own
// order delivered.
type Actor string

// The parties who can move an order.
const (
	// ActorCustomer is the person who placed it.
	ActorCustomer Actor = "customer"
	// ActorMerchant is the shop.
	ActorMerchant Actor = "merchant"
	// ActorPartner is the delivery rider.
	ActorPartner Actor = "partner"
	// ActorAdmin is platform operations.
	ActorAdmin Actor = "admin"
	// ActorSystem is the platform itself — a payment confirmed, a timeout
	// expired. Not a role anybody holds; a transition nobody chose.
	ActorSystem Actor = "system"
)

// The rules this package refuses to bend.
var (
	ErrIllegalTransition = errors.New("an order cannot move that way")
	ErrNotYourTransition = errors.New("that is not yours to do")
	ErrAlreadyFinished   = errors.New("that order is already finished")
)

// transition is one legal move: to a status, by whoever is listed.
type transition struct {
	to     Status
	actors []Actor
}

// transitions is the whole lifecycle, in one place.
//
// Read it as: from this status, these moves exist, and each may be made by
// these people. Everything else is refused — including, deliberately, a shop
// marking an order delivered and a customer marking one accepted.
var transitions = map[Status][]transition{
	StatusPendingPayment: {
		// The payment module confirms; nobody chooses this.
		{to: StatusPlaced, actors: []Actor{ActorSystem}},
		// An unpaid order is the customer's to abandon, and an admin's to
		// clear up. The shop is not involved: it has not been told yet.
		{to: StatusCancelled, actors: []Actor{ActorCustomer, ActorAdmin, ActorSystem}},
	},
	StatusPlaced: {
		{to: StatusAccepted, actors: []Actor{ActorMerchant, ActorAdmin}},
		{to: StatusRejected, actors: []Actor{ActorMerchant, ActorAdmin}},
		{to: StatusCancelled, actors: []Actor{ActorCustomer, ActorAdmin}},
	},
	StatusAccepted: {
		{to: StatusPreparing, actors: []Actor{ActorMerchant}},
		// A shop that has accepted can still discover it cannot deliver — the
		// last portion went, the cook did not come in. Rejecting late is worse
		// than rejecting early and better than a customer waiting for nothing.
		{to: StatusRejected, actors: []Actor{ActorMerchant, ActorAdmin}},
		// The customer's free window may still be open here on a fast-accepting
		// shop. Whether it actually is, is a clock question, answered in
		// cancel.go — this table only says the move exists.
		{to: StatusCancelled, actors: []Actor{ActorCustomer, ActorAdmin}},
	},
	StatusPreparing: {
		{to: StatusReady, actors: []Actor{ActorMerchant}},
		// No customer cancellation. Food is being cooked; somebody has already
		// paid for it in ingredients.
		{to: StatusCancelled, actors: []Actor{ActorAdmin}},
	},
	StatusReady: {
		{to: StatusPickedUp, actors: []Actor{ActorPartner}},
		{to: StatusCancelled, actors: []Actor{ActorAdmin}},
	},
	StatusPickedUp: {
		{to: StatusDelivered, actors: []Actor{ActorPartner}},
		{to: StatusFailed, actors: []Actor{ActorPartner, ActorAdmin}},
	},
	// The four terminal states have no rows at all, which is what makes them
	// terminal: there is nowhere to look up a move from.
	StatusDelivered: nil,
	StatusCancelled: nil,
	StatusRejected:  nil,
	StatusFailed:    nil,
}

// IsTerminal reports whether an order has finished, however it finished.
func (s Status) IsTerminal() bool {
	switch s {
	case StatusDelivered, StatusCancelled, StatusRejected, StatusFailed:
		return true
	default:
		return false
	}
}

// IsLive reports whether an order is still going, which is what a customer's
// "current orders" list means.
func (s Status) IsLive() bool { return !s.IsTerminal() && s != "" }

// Valid reports whether a string names a status.
func (s Status) Valid() bool {
	_, known := transitions[s]
	return known
}

// CanTransition reports whether an actor may move an order from one status to
// another, and says why not when they may not.
//
// Two different refusals, because they need two different answers. "That order
// is already finished" is a fact the customer can see for themselves; "that is
// not yours to do" means somebody is asking for something they should not be
// able to ask for, and is worth a different log line.
func CanTransition(from, to Status, actor Actor) error {
	if from.IsTerminal() {
		return ErrAlreadyFinished
	}
	moves, known := transitions[from]
	if !known {
		return ErrIllegalTransition
	}
	for _, move := range moves {
		if move.to != to {
			continue
		}
		for _, allowed := range move.actors {
			if allowed == actor {
				return nil
			}
		}
		return ErrNotYourTransition
	}
	return ErrIllegalTransition
}

// NextStatuses lists where an order can go from here, for an actor.
//
// The server answers "what can I do now" rather than the client working it out
// from the status: deciding which actions a user is allowed to take is a
// backend decision (2.9), and a client with its own copy of this table would
// show a shop an Accept button on an order somebody already cancelled.
func NextStatuses(from Status, actor Actor) []Status {
	out := make([]Status, 0, len(transitions[from]))
	for _, move := range transitions[from] {
		for _, allowed := range move.actors {
			if allowed == actor {
				out = append(out, move.to)
				break
			}
		}
	}
	return out
}
