package domain

import (
	"errors"
	"time"
)

// TicketStatus is where a support ticket stands.
type TicketStatus string

// A ticket has exactly two states: open, and resolved. Reopening a resolved
// ticket is a new ticket, not a mutation of the record an agent's decision
// already made.
const (
	TicketOpen     TicketStatus = "open"
	TicketResolved TicketStatus = "resolved"
)

// Resolution is what a support agent decided.
type Resolution string

// The two outcomes a ticket can resolve to.
const (
	ResolutionRefunded Resolution = "refunded"
	ResolutionRejected Resolution = "rejected"
)

// Errors NewTicket and Resolve return.
var (
	ErrNoTicketOrder = errors.New("a support ticket needs the order it is about")
	ErrNoRaisedBy    = errors.New("a support ticket needs who raised it")
	ErrNoSubject     = errors.New("a support ticket needs a reason")
	ErrTicketClosed  = errors.New("that ticket is already resolved")
	ErrNoResolution  = errors.New("a resolution must say whether the customer was refunded")
	ErrNoAgent       = errors.New("a resolution needs who decided it")
)

// Ticket is one customer's complaint about one order, and how it was closed.
type Ticket struct {
	ID       string
	OrderID  string
	RaisedBy string
	// Subject is the free-text reason a customer gave — "wrong item",
	// "never arrived" — not one of the three review subjects. A ticket is
	// about what went wrong, not what is being rated.
	Subject    string
	Status     TicketStatus
	Resolution Resolution
	Note       string
	AgentID    string
	CreatedAt  time.Time
	ResolvedAt time.Time
}

// NewTicket opens a ticket.
func NewTicket(id, orderID, raisedBy, subject string, now time.Time) (Ticket, error) {
	if orderID == "" {
		return Ticket{}, ErrNoTicketOrder
	}
	if raisedBy == "" {
		return Ticket{}, ErrNoRaisedBy
	}
	if subject == "" {
		return Ticket{}, ErrNoSubject
	}
	return Ticket{
		ID: id, OrderID: orderID, RaisedBy: raisedBy, Subject: subject,
		Status: TicketOpen, CreatedAt: now,
	}, nil
}

// Resolve records a support agent's decision. Once resolved, a ticket stays
// resolved: the record of what an agent decided must not silently change
// underneath a customer who was already told the outcome.
func (t *Ticket) Resolve(agentID string, resolution Resolution, note string, now time.Time) error {
	if t.Status != TicketOpen {
		return ErrTicketClosed
	}
	if agentID == "" {
		return ErrNoAgent
	}
	switch resolution {
	case ResolutionRefunded, ResolutionRejected:
	default:
		return ErrNoResolution
	}
	t.Status = TicketResolved
	t.Resolution = resolution
	t.Note = note
	t.AgentID = agentID
	t.ResolvedAt = now
	return nil
}
