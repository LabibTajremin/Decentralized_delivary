package application

import (
	"context"
	"errors"
	"strings"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// PlaceMerchantUseCase records where a merchant is, for the radius search.
//
// The merchant module owns the merchant; geo owns the index over its location.
// The split is deliberate: the division a merchant sits in decides who can ever
// see it (D3), and deriving that here — in the module that owns the boundaries
// — means no caller can register a merchant into the wrong division by sending
// a division code of its own.
type PlaceMerchantUseCase struct {
	repo ports.GeoRepository
}

// NewPlaceMerchantUseCase wires the use case.
func NewPlaceMerchantUseCase(repo ports.GeoRepository) *PlaceMerchantUseCase {
	return &PlaceMerchantUseCase{repo: repo}
}

// Execute places a merchant at a coordinate.
//
// A merchant outside every division is refused. This is not a contradiction of
// D1 — a merchant may register from anywhere *in Bangladesh*, and a point that
// falls in no division is not in Bangladesh. Accepting it would create a row
// that no search can ever return, which looks to the merchant like a successful
// registration and an empty order book.
func (uc *PlaceMerchantUseCase) Execute(ctx context.Context, merchantID string, c domain.Coordinate, active bool) error {
	if strings.TrimSpace(merchantID) == "" {
		return errs.New(errs.KindInvalid, "merchant_id_required", "A merchant id is required.")
	}

	division, err := uc.repo.DivisionContaining(ctx, c)
	if err != nil {
		if errors.Is(err, domain.ErrUnknownDivision) {
			return errs.Wrap(err, errs.KindNotFound, "outside_service_area",
				"That address is outside the area we serve.")
		}
		return errs.Wrap(err, errs.KindUnavailable, "geo_lookup_failed",
			"We could not check this location. Please try again.")
	}

	// The area is best-effort. A shop can sit inside a division but outside
	// every area we have drawn, and refusing it would make whether a merchant
	// may join depend on how finely we have mapped their upazila.
	var areaCode string
	if area, areaErr := uc.repo.AreaContaining(ctx, c); areaErr == nil {
		areaCode = area.Code
	}

	if err := uc.repo.UpsertMerchantLocation(ctx, ports.MerchantPoint{
		MerchantID: merchantID,
		Location:   c,
		Division:   division.Code,
		AreaCode:   areaCode,
		Active:     active,
	}); err != nil {
		return errs.Wrap(err, errs.KindUnavailable, "merchant_place_failed",
			"We could not save the shop location just now. Please try again.")
	}
	return nil
}

// Remove drops a merchant from the spatial index.
func (uc *PlaceMerchantUseCase) Remove(ctx context.Context, merchantID string) error {
	if err := uc.repo.DeleteMerchantLocation(ctx, merchantID); err != nil {
		return errs.Wrap(err, errs.KindUnavailable, "merchant_place_failed",
			"We could not update the shop location just now. Please try again.")
	}
	return nil
}
