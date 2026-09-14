package application

import (
	"context"
	"errors"
	"strings"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	geoext "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/external/geo"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// maxListLimit caps an admin listing page.
const maxListLimit = 100

// defaultListLimit is the page size when a caller does not ask.
const defaultListLimit = 20

// ModerationUseCase is an admin deciding whether a shop may trade.
//
// This is the half of the phase's acceptance criterion that gates visibility.
// Every decision here ends in two writes that must agree: the merchant's status,
// which the owner and the admin console read, and the active flag on geo's
// spatial row, which decides whether a customer's radius search can return the
// shop at all. Keeping both behind this one use case is what stops them drifting
// into a shop that looks approved and is invisible, or worse, the reverse.
type ModerationUseCase struct {
	repo  ports.Repository
	geo   geoext.Service
	clock clock.Clock
	ids   id.Generator
}

// NewModerationUseCase wires the use case.
func NewModerationUseCase(repo ports.Repository, geo geoext.Service, c clock.Clock, ids id.Generator) *ModerationUseCase {
	return &ModerationUseCase{repo: repo, geo: geo, clock: c, ids: ids}
}

// Approve lets a shop trade.
func (uc *ModerationUseCase) Approve(ctx context.Context, adminUserID, merchantID, note string) (domain.Merchant, error) {
	return uc.decide(ctx, adminUserID, merchantID, domain.StatusApproved, note, false)
}

// Reject refuses a shop, with a reason the owner will read.
//
// The reason is mandatory. A rejection with no reason is a support ticket we
// answer by hand, and the usual cause — an unreadable licence photo — is
// something the owner can fix in a minute if we say so.
func (uc *ModerationUseCase) Reject(ctx context.Context, adminUserID, merchantID, reason string) (domain.Merchant, error) {
	return uc.decide(ctx, adminUserID, merchantID, domain.StatusRejected, reason, true)
}

// Suspend pulls an approved shop out of the listings.
func (uc *ModerationUseCase) Suspend(ctx context.Context, adminUserID, merchantID, reason string) (domain.Merchant, error) {
	return uc.decide(ctx, adminUserID, merchantID, domain.StatusSuspended, reason, true)
}

// Reinstate puts a suspended shop back.
func (uc *ModerationUseCase) Reinstate(ctx context.Context, adminUserID, merchantID, note string) (domain.Merchant, error) {
	return uc.decide(ctx, adminUserID, merchantID, domain.StatusApproved, note, false)
}

// decide applies one status change and keeps the search index in step.
func (uc *ModerationUseCase) decide(
	ctx context.Context,
	adminUserID, merchantID string,
	next domain.Status,
	note string,
	noteRequired bool,
) (domain.Merchant, error) {
	if noteRequired && strings.TrimSpace(note) == "" {
		return domain.Merchant{}, errs.New(errs.KindInvalid, "reason_required",
			"Please give a reason. The shop owner will see it.")
	}

	merchant, err := uc.load(ctx, merchantID)
	if err != nil {
		return domain.Merchant{}, err
	}

	updated, err := merchant.WithStatus(next, note)
	if err != nil {
		return domain.Merchant{}, transitionError(err, merchant.Status)
	}

	if err := uc.repo.Save(ctx, updated); err != nil {
		return domain.Merchant{}, unavailable(err, "We could not record that decision just now. Please try again.")
	}

	// The search index is updated after the status, and a failure here is
	// reported rather than swallowed. The two can disagree for the moment
	// between the writes; what must not happen is that they disagree silently
	// and permanently, which is what logging-and-continuing would produce.
	if err := uc.geo.PlaceMerchant(ctx, geoext.Placement{
		MerchantID: updated.ID,
		Lat:        updated.Pin.Lat,
		Lng:        updated.Pin.Lng,
		Active:     updated.Status == domain.StatusApproved,
	}); err != nil {
		return domain.Merchant{}, err
	}

	if err := uc.repo.RecordStatusChange(ctx, ports.StatusChange{
		ID:          uc.ids.New("mse"),
		MerchantID:  updated.ID,
		From:        merchant.Status,
		To:          updated.Status,
		ActorUserID: adminUserID,
		Note:        strings.TrimSpace(note),
		At:          uc.clock.Now(),
	}); err != nil {
		return domain.Merchant{}, unavailable(err, "We could not record that decision just now. Please try again.")
	}
	return updated, nil
}

// Get returns one merchant for an admin.
func (uc *ModerationUseCase) Get(ctx context.Context, merchantID string) (domain.Merchant, error) {
	return uc.load(ctx, merchantID)
}

// ListRequest narrows an admin listing.
type ListRequest struct {
	Status       string
	Type         string
	DivisionCode string
	Limit        int
	Offset       int
}

// List returns merchants matching a filter.
func (uc *ModerationUseCase) List(ctx context.Context, req ListRequest) ([]domain.Merchant, error) {
	filter := ports.Filter{
		DivisionCode: strings.TrimSpace(req.DivisionCode),
		Limit:        req.Limit,
		Offset:       req.Offset,
	}

	if req.Status != "" {
		status, err := domain.ParseStatus(req.Status)
		if err != nil {
			return nil, errs.Wrap(err, errs.KindInvalid, "unknown_status", "That is not a shop status we use.")
		}
		filter.Status = status
	}
	if req.Type != "" {
		kind, err := domain.ParseType(req.Type)
		if err != nil {
			return nil, errs.Wrap(err, errs.KindInvalid, "unknown_merchant_type",
				"Please choose restaurant, grocery or pharmacy.")
		}
		filter.Type = kind
	}

	if filter.Limit <= 0 || filter.Limit > maxListLimit {
		filter.Limit = defaultListLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	found, err := uc.repo.List(ctx, filter)
	if err != nil {
		return nil, unavailable(err, "We could not load the shop list just now. Please try again.")
	}
	return found, nil
}

// History returns a merchant's decisions, newest first.
func (uc *ModerationUseCase) History(ctx context.Context, merchantID string, limit int) ([]ports.StatusChange, error) {
	if limit <= 0 || limit > maxListLimit {
		limit = defaultListLimit
	}
	events, err := uc.repo.StatusHistory(ctx, merchantID, limit)
	if err != nil {
		return nil, unavailable(err, "We could not load that history just now. Please try again.")
	}
	return events, nil
}

// load fetches a merchant by id.
func (uc *ModerationUseCase) load(ctx context.Context, merchantID string) (domain.Merchant, error) {
	merchant, err := uc.repo.Merchant(ctx, merchantID)
	if errors.Is(err, domain.ErrMerchantNotFound) {
		return domain.Merchant{}, errs.Wrap(err, errs.KindNotFound, "merchant_not_found",
			"We could not find that shop.").With("merchant_id", merchantID)
	}
	if err != nil {
		return domain.Merchant{}, unavailable(err, "We could not load that shop just now. Please try again.")
	}
	return merchant, nil
}
