// Package application resolves a tariff and turns a quote into words.
//
// Pricing has one use case and no storage: the rule is in the domain and the
// numbers are in config. What lives here is the boundary — reading the six
// settings, and composing the sentences a receipt is made of (2.9).
package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/domain"
	cfg "github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Service implements contract.PricingContract.
type Service struct {
	config cfg.Service
}

// NewService wires the public service.
func NewService(config cfg.Service) *Service { return &Service{config: config} }

// Tariff resolves the pricing configuration in force at a placement.
func (s *Service) Tariff(ctx context.Context, p contract.Placement) (contract.Tariff, error) {
	settings, err := s.config.Settings(ctx, cfg.Placement{
		AreaCode:     p.AreaCode,
		DistrictCode: p.DistrictCode,
		DivisionCode: p.DivisionCode,
	})
	if err != nil {
		return nil, configError(err)
	}

	base, err := settings.Int(cfg.DeliveryBase)
	if err != nil {
		return nil, configError(err)
	}
	perKm, err := settings.Int(cfg.DeliveryPerKm)
	if err != nil {
		return nil, configError(err)
	}
	multiplier, err := settings.Ratio(cfg.ExpansionMult)
	if err != nil {
		return nil, configError(err)
	}
	freeAbove, err := settings.Int(cfg.FreeDeliveryAt)
	if err != nil {
		return nil, configError(err)
	}

	tariff, err := domain.NewTariff(
		money.Taka(base), money.Taka(perKm), multiplier, money.Taka(freeAbove),
	)
	if err != nil {
		return nil, tariffError(err)
	}
	return resolved{tariff: tariff}, nil
}

// resolved is a tariff snapshot, holding the domain value behind the contract
// interface so a consumer never links pricing's domain.
type resolved struct {
	tariff domain.Tariff
}

// DeliveryFee is the fee alone, for a shop card.
func (r resolved) DeliveryFee(req contract.QuoteRequest) (contract.Fee, error) {
	expanded := req.ExpansionLevel > 0
	amount, surcharge, err := r.tariff.DeliveryFee(req.DistanceM, expanded)
	if err != nil {
		return contract.Fee{}, quoteError(err)
	}
	return contract.Fee{
		Amount:    toMoney(amount),
		Expanded:  expanded,
		Surcharge: toMoney(surcharge),
	}, nil
}

// Quote is the whole receipt.
func (r resolved) Quote(req contract.QuoteRequest) (contract.Quote, error) {
	quote, err := r.tariff.Price(money.Taka(req.SubtotalMinor), req.DistanceM, req.ExpansionLevel > 0)
	if err != nil {
		return contract.Quote{}, quoteError(err)
	}

	rows := make([]contract.Row, 0, len(quote.Rows))
	for _, row := range quote.Rows {
		rows = append(rows, contract.Row{
			Key:    string(row.Key),
			Label:  label(row.Key, req.Lang),
			Amount: toMoney(row.Amount),
		})
	}

	return contract.Quote{
		Subtotal:             toMoney(quote.Subtotal),
		Delivery:             toMoney(quote.Delivery),
		DeliveryBeforeWaiver: toMoney(quote.DeliveryBeforeWaiver),
		ExpansionSurcharge:   toMoney(quote.ExpansionSurcharge),
		FreeDelivery:         quote.FreeDelivery,
		FreeDeliveryAt:       toMoney(quote.FreeDeliveryAt),
		AwayFromFreeDelivery: toMoney(quote.AwayFromFreeDelivery),
		Total:                toMoney(quote.Total),
		DistanceM:            quote.DistanceM,
		Expanded:             quote.Expanded,
		Rows:                 rows,
		Notice:               notice(quote, req.Lang),
	}, nil
}

func toMoney(m money.Money) contract.Money {
	return contract.Money{Minor: m.Minor(), Currency: string(m.Currency()), Display: m.Display()}
}

// label is a receipt row in words. Bengali-first (1.4).
func label(key domain.RowKey, lang string) string {
	bengali := lang != "en"
	switch key {
	case domain.RowSubtotal:
		if bengali {
			return "পণ্যের মোট"
		}
		return "Items"
	case domain.RowDelivery:
		if bengali {
			return "ডেলিভারি চার্জ"
		}
		return "Delivery"
	case domain.RowExpansionSurcharge:
		if bengali {
			return "দূরত্বের জন্য অতিরিক্ত"
		}
		return "Extra distance charge"
	case domain.RowFreeDelivery:
		if bengali {
			return "ফ্রি ডেলিভারি"
		}
		return "Free delivery"
	default:
		if bengali {
			return "সর্বমোট"
		}
		return "Total"
	}
}

// notice is the one line under the total.
//
// Three different things worth saying, and nothing when there is nothing. A
// notice on every quote is a notice nobody reads.
func notice(quote domain.Quote, lang string) string {
	bengali := lang != "en"
	switch {
	case quote.FreeDelivery:
		if bengali {
			return "এই অর্ডারে ডেলিভারি ফ্রি।"
		}
		return "Delivery is free on this order."
	case !quote.AwayFromFreeDelivery.IsZero():
		if bengali {
			return "আর " + quote.AwayFromFreeDelivery.Display() + " কিনলে ডেলিভারি ফ্রি।"
		}
		return quote.AwayFromFreeDelivery.Display() + " more for free delivery."
	case quote.Expanded && !quote.ExpansionSurcharge.IsZero():
		if bengali {
			return "দূরের দোকান বেছে নেওয়ায় ডেলিভারি চার্জ " +
				quote.ExpansionSurcharge.Display() + " বেশি।"
		}
		return "Delivery costs " + quote.ExpansionSurcharge.Display() +
			" more because this shop is outside your usual area."
	default:
		return ""
	}
}

// configError reports configuration that could not be read. A fee that
// silently fell back to a hard-coded ৳40 would be a misconfigured division
// nobody noticed until somebody checked a receipt by hand.
func configError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "pricing_config_unavailable",
		"We could not work out the delivery charge just now. Please try again.")
}

// tariffError reports configuration that is present but impossible.
func tariffError(err error) error {
	return errs.Wrap(err, errs.KindInternal, "pricing_config_invalid",
		"Delivery is unavailable in this area. Please try again later.")
}

// quoteError reports a question pricing cannot answer.
func quoteError(err error) error {
	return errs.Wrap(err, errs.KindInvalid, "invalid_quote_request",
		"We could not work out the delivery charge for that order.")
}
