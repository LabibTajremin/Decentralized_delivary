// Package fees implements ALG-05 — the delivery fee — from configuration.
//
// Provisional, and deliberately so. Pricing is P10; discovery is P08 and needs
// a fee on every merchant card the moment it can show one, because D2's whole
// point is that the customer sees expansion cost more *before* they expand.
// When pricing lands it owns ALG-05, and this package is deleted in favour of
// discovery's external/pricing service — the port it satisfies does not change,
// so no use case does either.
package fees

import (
	"context"
	"math"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/application/ports"
	cfg "github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Quoter prices a delivery from the configuration in force where it happens.
type Quoter struct {
	config cfg.Service
}

// NewQuoter wires the quoter.
func NewQuoter(config cfg.Service) *Quoter {
	return &Quoter{config: config}
}

// QuoteDelivery applies ALG-05: a base fee plus a per-kilometre rate over the
// straight-line distance, multiplied when the customer has widened the radius.
//
// O(1). Distance is banded to the whole kilometre, rounded up: a fee that
// changes by a few paisa as a phone's GPS drifts across a street looks broken,
// and the customer who sees ৳50 on the shop card must be charged ৳50.
func (q *Quoter) QuoteDelivery(ctx context.Context, p ports.Placement, req ports.DeliveryQuoteRequest) (ports.DeliveryQuote, error) {
	settings, err := q.config.Settings(ctx, cfg.Placement{
		AreaCode:     p.AreaCode,
		DistrictCode: p.DistrictCode,
		DivisionCode: p.DivisionCode,
	})
	if err != nil {
		return ports.DeliveryQuote{}, err
	}

	base, err := settings.Int(cfg.DeliveryBase)
	if err != nil {
		return ports.DeliveryQuote{}, err
	}
	perKm, err := settings.Int(cfg.DeliveryPerKm)
	if err != nil {
		return ports.DeliveryQuote{}, err
	}
	multiplier, err := settings.Ratio(cfg.ExpansionMult)
	if err != nil {
		return ports.DeliveryQuote{}, err
	}

	distance := req.DistanceM
	if distance < 0 {
		return ports.DeliveryQuote{}, errs.New(errs.KindInvalid, "invalid_distance",
			"We could not work out how far away that shop is.")
	}
	bands := int64(math.Ceil(distance / 1000))

	minor := base + perKm*bands
	expanded := req.ExpansionLevel > 0
	if expanded {
		// Rounded to the nearest paisa rather than truncated, so a 1.5×
		// multiplier on an odd amount does not quietly favour the platform.
		minor = int64(math.Round(float64(minor) * multiplier))
	}

	amount, err := money.Taka(minor).MustBeNonNegative()
	if err != nil {
		return ports.DeliveryQuote{}, errs.Wrap(err, errs.KindInternal, "invalid_delivery_fee",
			"We could not work out the delivery charge.")
	}

	return ports.DeliveryQuote{
		Minor:    amount.Minor(),
		Currency: string(amount.Currency()),
		Display:  amount.Display(),
		Expanded: expanded,
	}, nil
}
