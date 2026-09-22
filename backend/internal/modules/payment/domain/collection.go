package domain

import (
	"errors"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// The rules a cash collection holds itself to.
var (
	ErrNoPartner        = errors.New("a collection must belong to a partner")
	ErrAlreadyRemitted  = errors.New("that cash has already been remitted")
	ErrNoRemittanceRef  = errors.New("a remittance needs a reference")
	ErrWrongPartner     = errors.New("that cash was not collected by this partner")
	ErrCollectionExists = errors.New("that order's cash has already been recorded")
)

// CollectionStatus is where a cash-on-delivery collection is in its life.
type CollectionStatus string

const (
	// StatusHeld is cash the partner is physically carrying.
	StatusHeld CollectionStatus = "held"
	// StatusRemitted is cash the partner has handed over to the platform.
	StatusRemitted CollectionStatus = "remitted"
)

// Collection is one COD order's worth of cash, from the moment a rider
// collects it to the moment they remit it.
//
// This is the platform's ledger of physical money it does not yet hold, which
// is what "the COD ledger reconciles" means: the sum of what every partner is
// carrying must always be an answerable question, not a hope.
type Collection struct {
	ID        string
	OrderID   string
	PartnerID string
	Amount    money.Money
	Status    CollectionStatus
	// RemittanceRef is set once the cash is handed over — a receipt number, a
	// bank deposit slip, whatever the operator recorded the handover against.
	RemittanceRef string

	CollectedAt time.Time
	RemittedAt  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewCollection records a rider taking cash for a delivered order.
//
// Created directly in StatusHeld: by the time this is called the money has
// already changed hands at the door. There is no "pending" state for cash —
// unlike a gateway, nobody has to confirm it arrived.
func NewCollection(id, orderID, partnerID string, amount money.Money, now time.Time) (Collection, error) {
	switch {
	case orderID == "":
		return Collection{}, ErrNoOrder
	case partnerID == "":
		return Collection{}, ErrNoPartner
	case amount.IsZero() || amount.IsNegative():
		return Collection{}, ErrNotPositive
	}
	return Collection{
		ID: id, OrderID: orderID, PartnerID: partnerID,
		Amount: amount, Status: StatusHeld,
		CollectedAt: now, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// Remit is the partner handing this cash over to the platform.
func (c *Collection) Remit(partnerID, reference string, at time.Time) error {
	if c.PartnerID != partnerID {
		return ErrWrongPartner
	}
	if c.Status != StatusHeld {
		return ErrAlreadyRemitted
	}
	if reference == "" {
		return ErrNoRemittanceRef
	}
	c.Status = StatusRemitted
	c.RemittanceRef = reference
	c.RemittedAt = at
	c.UpdatedAt = at
	return nil
}

// Ledger is a partner's cash position, summed from their collections.
//
// A pure function of the collections handed to it, not a stored figure — a
// ledger that could itself be wrong is worse than no ledger, so it is
// recomputed from the same rows an auditor would read, every time.
type Ledger struct {
	PartnerID   string
	Outstanding money.Money
	Remitted    money.Money
	// Held is the collections still owed, oldest first — what a "please remit"
	// screen lists.
	Held []Collection
}

// Summarize builds a partner's ledger from their collections.
func Summarize(partnerID string, collections []Collection) Ledger {
	ledger := Ledger{
		PartnerID:   partnerID,
		Outstanding: money.Zero(money.BDT),
		Remitted:    money.Zero(money.BDT),
	}
	for _, c := range collections {
		switch c.Status {
		case StatusHeld:
			ledger.Outstanding, _ = ledger.Outstanding.Add(c.Amount)
			ledger.Held = append(ledger.Held, c)
		case StatusRemitted:
			ledger.Remitted, _ = ledger.Remitted.Add(c.Amount)
		}
	}
	return ledger
}
