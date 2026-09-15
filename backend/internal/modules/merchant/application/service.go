package application

import (
	"context"
	"errors"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Service implements contract.MerchantContract.
//
// It reads the repository directly rather than going through the owner-facing
// use cases: those check that the caller owns the shop, which is exactly wrong
// for a consumer module asking about somebody else's.
type Service struct {
	repo  ports.Repository
	clock clock.Clock
}

// NewService wires the public service.
func NewService(repo ports.Repository, c clock.Clock) *Service {
	return &Service{repo: repo, clock: c}
}

// Merchant returns one shop.
func (s *Service) Merchant(ctx context.Context, merchantID string) (contract.Merchant, error) {
	merchant, err := s.repo.Merchant(ctx, merchantID)
	if err != nil {
		return contract.Merchant{}, notFound(err, merchantID)
	}
	return ToContract(merchant, s.clock.Now(), ""), nil
}

// OwnedBy returns the ids of the shops an account owns.
//
// An account with no shop is not an error: most accounts have none, and a
// consumer asking "which shops are yours" about a customer should get an empty
// answer rather than a failure it has to special-case.
func (s *Service) OwnedBy(ctx context.Context, ownerUserID string) ([]string, error) {
	merchant, err := s.repo.ByOwner(ctx, ownerUserID)
	if err != nil {
		if errors.Is(err, domain.ErrMerchantNotFound) {
			return nil, nil
		}
		return nil, errs.Wrap(err, errs.KindUnavailable, "merchant_unavailable",
			"We could not check your shop just now. Please try again.")
	}
	return []string{merchant.ID}, nil
}

// Listed returns the subset of these ids a customer may see, in order.
func (s *Service) Listed(ctx context.Context, merchantIDs []string) ([]contract.Merchant, error) {
	now := s.clock.Now()
	out := make([]contract.Merchant, 0, len(merchantIDs))
	for _, merchantID := range merchantIDs {
		merchant, err := s.repo.Merchant(ctx, merchantID)
		if err != nil {
			// A missing id is skipped rather than failing the batch. The caller
			// is discovery, holding ids from a spatial index that can be a
			// moment behind a withdrawal; failing the whole page would turn one
			// stale row into an empty search result.
			continue
		}
		if !merchant.IsListed(now) {
			continue
		}
		out = append(out, ToContract(merchant, now, ""))
	}
	return out, nil
}

// IsAcceptingOrders reports whether a shop can take an order right now.
func (s *Service) IsAcceptingOrders(ctx context.Context, merchantID string) (bool, error) {
	merchant, err := s.repo.Merchant(ctx, merchantID)
	if err != nil {
		return false, notFound(err, merchantID)
	}
	return merchant.IsOpenAt(s.clock.Now()), nil
}

// ToContract converts a merchant to its public form.
//
// Exported because transport renders the same shape: one conversion means the
// shop a consuming module sees and the shop the app shows are assembled by the
// same code, so they cannot drift apart.
func ToContract(m domain.Merchant, now time.Time, lang string) contract.Merchant {
	return contract.Merchant{
		ID:           m.ID,
		OwnerUserID:  m.OwnerUserID,
		Name:         m.Name,
		Type:         contract.Type(m.Type),
		Phone:        m.Phone,
		LogoURL:      m.LogoURL,
		Line1:        m.Line1,
		Line2:        m.Line2,
		SingleLine:   m.SingleLine(),
		Lat:          m.Pin.Lat,
		Lng:          m.Pin.Lng,
		AreaCode:     m.Placement.AreaCode,
		AreaName:     m.Placement.AreaName,
		DistrictCode: m.Placement.DistrictCode,
		DivisionCode: m.Placement.DivisionCode,
		IsListed:     m.IsListed(now),
		IsOpenNow:    m.IsOpenAt(now),
		OpenStatus:   m.OpenStatus(now, lang),
	}
}

// notFound turns a repository miss into a client-facing refusal.
func notFound(err error, merchantID string) error {
	if errors.Is(err, domain.ErrMerchantNotFound) {
		return errs.Wrap(err, errs.KindNotFound, "merchant_not_found",
			"We could not find that shop.").With("merchant_id", merchantID)
	}
	return unavailable(err, "We could not load that shop just now. Please try again.")
}
