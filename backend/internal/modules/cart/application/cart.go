package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/catalogue"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/discovery"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/merchant"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/pricing"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// CartUseCase is everything a customer does to their cart.
//
// One use case rather than six, because every operation ends the same way: load
// the cart, change it, save it, and revalidate the result against a shop that
// may have changed underneath. Splitting that into six types would give six
// copies of the ending.
type CartUseCase struct {
	repo        ports.Repository
	catalogue   catalogue.Service
	pricing     pricing.Service
	revalidator revalidator
	clock       clock.Clock
	ids         id.Generator
}

// NewCartUseCase wires the use case.
func NewCartUseCase(
	repo ports.Repository,
	cat catalogue.Service,
	merchants merchant.Service,
	disco discovery.Service,
	prices pricing.Service,
	c clock.Clock,
	ids id.Generator,
) *CartUseCase {
	return &CartUseCase{
		repo:      repo,
		catalogue: cat,
		pricing:   prices,
		revalidator: revalidator{
			catalogue: cat, merchant: merchants, discovery: disco,
		},
		clock: c,
		ids:   ids,
	}
}

// Choice is one option the customer picked when adding a line.
type Choice struct {
	GroupID  string
	OptionID string
}

// AddRequest is a customer adding something.
type AddRequest struct {
	MerchantID string
	// Kind is "item" or "combo".
	Kind     string
	TargetID string
	Quantity int
	Choices  []Choice
	Note     string
	Lang     string
}

// Current returns the customer's cart, revalidated. The second return is false
// when they have no cart, which is not an error.
func (uc *CartUseCase) Current(ctx context.Context, userID, lang string) (View, bool, error) {
	cart, found, err := uc.repo.OfUser(ctx, userID)
	if err != nil {
		return View{}, false, storageError(err)
	}
	if !found {
		return View{}, false, nil
	}
	view, err := uc.render(ctx, cart, lang)
	return view, true, err
}

// Add puts something in the cart, opening one if the customer has none.
//
// Single-merchant is enforced here and refused rather than resolved: emptying
// somebody's cart because they tapped an item in a different shop is a
// destructive act, and it is theirs to take. The client gets a conflict with a
// sentence explaining it, and a Replace endpoint to take it deliberately.
func (uc *CartUseCase) Add(ctx context.Context, userID string, req AddRequest) (View, error) {
	cart, found, err := uc.repo.OfUser(ctx, userID)
	if err != nil {
		return View{}, storageError(err)
	}
	if !found {
		cart, err = domain.NewCart(uc.ids.New("CRT"), userID, req.MerchantID, uc.clock.Now())
		if err != nil {
			return View{}, cartError(err)
		}
	}

	line, err := uc.resolve(ctx, req)
	if err != nil {
		return View{}, err
	}
	if err := cart.Add(req.MerchantID, line); err != nil {
		return View{}, cartError(err)
	}
	return uc.saveAndRender(ctx, cart, req.Lang)
}

// Replace empties the cart and starts a new one at another shop. The deliberate
// version of what Add refuses to do quietly.
func (uc *CartUseCase) Replace(ctx context.Context, userID string, req AddRequest) (View, error) {
	existing, found, err := uc.repo.OfUser(ctx, userID)
	if err != nil {
		return View{}, storageError(err)
	}
	if found {
		if err := uc.repo.Delete(ctx, existing.ID); err != nil {
			return View{}, storageError(err)
		}
	}
	return uc.Add(ctx, userID, req)
}

// SetQuantity changes how many of a line. Zero removes it.
func (uc *CartUseCase) SetQuantity(ctx context.Context, userID, lineID string, quantity int, lang string) (View, error) {
	cart, err := uc.mine(ctx, userID)
	if err != nil {
		return View{}, err
	}
	if err := cart.SetQuantity(lineID, quantity); err != nil {
		return View{}, cartError(err)
	}
	return uc.saveAndRender(ctx, cart, lang)
}

// Remove takes a line out.
func (uc *CartUseCase) Remove(ctx context.Context, userID, lineID, lang string) (View, error) {
	cart, err := uc.mine(ctx, userID)
	if err != nil {
		return View{}, err
	}
	if err := cart.Remove(lineID); err != nil {
		return View{}, cartError(err)
	}
	return uc.saveAndRender(ctx, cart, lang)
}

// SetAddress binds the cart to a delivery point.
//
// This is the operation the second acceptance criterion turns on: the next
// read re-asks discovery, and a customer who has moved their delivery address
// into another division is told so while the cart is still small, rather than
// at checkout.
func (uc *CartUseCase) SetAddress(ctx context.Context, userID, addressID string, lat, lng float64, lang string) (View, error) {
	cart, err := uc.mine(ctx, userID)
	if err != nil {
		return View{}, err
	}
	if addressID == "" {
		return View{}, errs.New(errs.KindInvalid, "invalid_address",
			"We could not tell which address you chose.")
	}
	cart.PlaceAt(addressID, lat, lng)
	return uc.saveAndRender(ctx, cart, lang)
}

// Clear deletes the cart entirely.
//
// Deleted rather than emptied: an empty cart still bound to a shop would make
// the next Add at a different shop a conflict about a cart with nothing in it,
// which is a rule with no purpose left.
func (uc *CartUseCase) Clear(ctx context.Context, userID string) error {
	cart, found, err := uc.repo.OfUser(ctx, userID)
	if err != nil {
		return storageError(err)
	}
	if !found {
		return nil
	}
	if err := uc.repo.Delete(ctx, cart.ID); err != nil {
		return storageError(err)
	}
	return nil
}

// mine loads the caller's cart, refusing when there is none.
func (uc *CartUseCase) mine(ctx context.Context, userID string) (domain.Cart, error) {
	cart, found, err := uc.repo.OfUser(ctx, userID)
	if err != nil {
		return domain.Cart{}, storageError(err)
	}
	if !found {
		return domain.Cart{}, errs.New(errs.KindNotFound, "cart_not_found", "You do not have a cart yet.")
	}
	return cart, nil
}

// saveAndRender writes the cart and returns it revalidated.
func (uc *CartUseCase) saveAndRender(ctx context.Context, cart domain.Cart, lang string) (View, error) {
	cart.UpdatedAt = uc.clock.Now()
	if err := uc.repo.Save(ctx, cart); err != nil {
		return View{}, storageError(err)
	}
	return uc.render(ctx, cart, lang)
}

// render revalidates a cart, prices it, and composes the screen.
func (uc *CartUseCase) render(ctx context.Context, cart domain.Cart, lang string) (View, error) {
	got, err := uc.revalidator.Execute(ctx, cart)
	if err != nil {
		return View{}, err
	}

	view := viewOf(cart, got.shop, got.check, lang)

	quote, priced, err := uc.price(ctx, got, view.Subtotal.Minor, lang)
	if err != nil {
		return View{}, err
	}
	if priced {
		view.Pricing = &quote
	}
	return view, nil
}

// price quotes the delivery, when there is anything true to quote.
//
// Skipped when the address cannot reach the shop, and when there is no address
// at all: a delivery charge for a journey that cannot happen is a number the
// customer would reasonably take for a promise. The blocker already says why,
// and a cart screen with no total under an "another division" message reads
// correctly.
func (uc *CartUseCase) price(ctx context.Context, got revalidated, subtotalMinor int64, lang string) (pricing.Quote, bool, error) {
	if !got.reach.Reachable {
		return pricing.Quote{}, false, nil
	}
	tariff, err := uc.pricing.Tariff(ctx, pricing.Placement{
		AreaCode:     got.reach.AreaCode,
		DistrictCode: got.reach.DistrictCode,
		DivisionCode: got.reach.DivisionCode,
	})
	if err != nil {
		return pricing.Quote{}, false, err
	}
	quote, err := tariff.Quote(pricing.QuoteRequest{
		SubtotalMinor:  subtotalMinor,
		DistanceM:      got.reach.DistanceM,
		ExpansionLevel: got.reach.RequiredLevel,
		Lang:           lang,
	})
	if err != nil {
		return pricing.Quote{}, false, err
	}
	return quote, true, nil
}
