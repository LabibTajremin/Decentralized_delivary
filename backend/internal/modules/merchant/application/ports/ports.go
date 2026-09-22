// Package ports declares what the merchant use cases need from the outside
// world. No use case ever touches a database handle (05-architecture.md 2.3).
package ports

import (
	"context"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
)

// Repository stores merchants, their documents and their status history.
type Repository interface {
	// Create stores a new merchant, returning domain.ErrAlreadyRegistered if
	// the owner already has one.
	Create(ctx context.Context, m domain.Merchant) error

	// Merchant returns one merchant by id, or domain.ErrMerchantNotFound.
	Merchant(ctx context.Context, merchantID string) (domain.Merchant, error)

	// ByOwner returns the merchant an account owns, or
	// domain.ErrMerchantNotFound.
	ByOwner(ctx context.Context, ownerUserID string) (domain.Merchant, error)

	// Save writes an existing merchant, documents included.
	Save(ctx context.Context, m domain.Merchant) error

	// Delete removes a merchant and everything hanging off it. Only ever
	// called for a registration that was never approved — see Withdraw.
	Delete(ctx context.Context, merchantID string) error

	// List returns merchants matching a filter, newest first.
	List(ctx context.Context, f Filter) ([]domain.Merchant, error)

	// RecordStatusChange appends to the audit trail.
	//
	// Separate from Save rather than derived from it: the history is what an
	// operator reads when a merchant asks why they were suspended, and deriving
	// it from writes would record the changes that happened to go through Save
	// rather than the decisions somebody made.
	RecordStatusChange(ctx context.Context, e StatusChange) error

	// StatusHistory returns a merchant's decisions, newest first.
	StatusHistory(ctx context.Context, merchantID string, limit int) ([]StatusChange, error)
}

// Filter narrows a merchant listing.
//
// Every field is optional; a zero Filter lists everything, page by page. The
// division filter exists because an operator works a region (D3) and a
// nationwide list is unreadable by the time the product works.
type Filter struct {
	Status       domain.Status
	Type         domain.Type
	DivisionCode string
	Limit        int
	Offset       int
}

// StatusChange is one decision in a merchant's history.
type StatusChange struct {
	ID          string
	MerchantID  string
	From        domain.Status
	To          domain.Status
	ActorUserID string
	Note        string
	At          time.Time
}
