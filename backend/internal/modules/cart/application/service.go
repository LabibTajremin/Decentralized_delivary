package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/pricing"
)

// Service implements contract.CartContract.
//
// A thin adapter over the use case. Order calls it to turn a cart into an
// order and then to clear it — and it gets the *revalidated* cart, at today's
// prices, because an order frozen from a stale snapshot is an order the shop
// will dispute.
type Service struct {
	carts *CartUseCase
}

// NewService wires the public service.
func NewService(carts *CartUseCase) *Service { return &Service{carts: carts} }

// Current returns the customer's cart, revalidated.
//
// The language is fixed rather than taken as an argument: the caller is another
// module, not a request handler, and the sentences in a contract response are
// for order's own logic rather than a screen.
func (s *Service) Current(ctx context.Context, userID string) (contract.Cart, bool, error) {
	view, found, err := s.carts.Current(ctx, userID, "")
	if err != nil || !found {
		return contract.Cart{}, found, err
	}
	return toContract(userID, view), true, nil
}

// Clear empties the customer's cart once its contents have become an order.
func (s *Service) Clear(ctx context.Context, userID string) error {
	return s.carts.Clear(ctx, userID)
}

func toContract(userID string, view View) contract.Cart {
	lines := make([]contract.Line, 0, len(view.Lines))
	for _, l := range view.Lines {
		options := make([]contract.Option, 0, len(l.Options))
		for _, o := range l.Options {
			options = append(options, contract.Option{
				GroupID: o.GroupID, OptionID: o.OptionID, Name: o.Name, Price: toContractMoney(o.Price),
			})
		}
		lines = append(lines, contract.Line{
			ID: l.ID, Kind: l.Kind, TargetID: l.TargetID, Name: l.Name,
			UnitPrice: toContractMoney(l.UnitPrice),
			Options:   options, Quantity: l.Quantity, Note: l.Note,
			LineTotal: toContractMoney(l.LineTotal),
		})
	}
	return contract.Cart{
		ID: view.ID, UserID: userID, MerchantID: view.MerchantID,
		Delivery:  toContractDelivery(view.Pricing),
		AddressID: view.AddressID,
		Lat:       view.Lat,
		Lng:       view.Lng,
		Lines:     lines,
		Subtotal:  toContractMoney(view.Subtotal),
		Orderable: view.Orderable, Blocker: view.Blocker,
	}
}

// toContractDelivery restates the quote in the cart's own primitives.
func toContractDelivery(quote *pricing.Quote) *contract.Delivery {
	if quote == nil {
		return nil
	}
	return &contract.Delivery{
		Fee:                fromPricingMoney(quote.Delivery),
		ExpansionSurcharge: fromPricingMoney(quote.ExpansionSurcharge),
		Total:              fromPricingMoney(quote.Total),
		FreeDelivery:       quote.FreeDelivery,
		Expanded:           quote.Expanded,
		DistanceM:          quote.DistanceM,
	}
}

func fromPricingMoney(m pricing.Money) contract.Money {
	return contract.Money{Minor: m.Minor, Currency: m.Currency, Display: m.Display}
}

func toContractMoney(m Money) contract.Money {
	return contract.Money{Minor: m.Minor, Currency: m.Currency, Display: m.Display}
}
