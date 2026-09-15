package application

import (
	"context"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	cartx "github.com/rootlogic-lab/delivery/backend/internal/modules/order/external/cart"
	cfg "github.com/rootlogic-lab/delivery/backend/internal/modules/order/external/config"
	disco "github.com/rootlogic-lab/delivery/backend/internal/modules/order/external/discovery"
	merchantx "github.com/rootlogic-lab/delivery/backend/internal/modules/order/external/merchant"
	pricingx "github.com/rootlogic-lab/delivery/backend/internal/modules/order/external/pricing"
	userx "github.com/rootlogic-lab/delivery/backend/internal/modules/order/external/user"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// PlaceUseCase turns a cart into an agreement.
//
// Everything it is given by the client is an id or an intention. Everything
// that decides an outcome — what the goods cost, how far it is, whether the
// shop is open, whether cash is allowed at this amount — is read here, at the
// moment of writing. A price the customer was shown is not a price the system
// promised.
type PlaceUseCase struct {
	repo     ports.Repository
	codes    ports.CodeGenerator
	cart     cartx.Service
	pricing  pricingx.Service
	merchant merchantx.Service
	user     userx.Service
	disco    disco.Service
	config   cfg.Service
	clock    clock.Clock
	ids      id.Generator
}

// NewPlaceUseCase wires the use case.
func NewPlaceUseCase(
	repo ports.Repository,
	codes ports.CodeGenerator,
	cart cartx.Service,
	pricing pricingx.Service,
	merchants merchantx.Service,
	users userx.Service,
	discovery disco.Service,
	config cfg.Service,
	c clock.Clock,
	ids id.Generator,
) *PlaceUseCase {
	return &PlaceUseCase{
		repo: repo, codes: codes, cart: cart, pricing: pricing,
		merchant: merchants, user: users, disco: discovery, config: config,
		clock: c, ids: ids,
	}
}

// PlaceRequest is a customer checking out.
//
// Three fields, and none of them is a price. The cart says what is being
// bought, the address book says where it is going, and everything else the
// server works out (2.9).
type PlaceRequest struct {
	// AddressID is which saved address to deliver to. Taken from the request
	// rather than the cart because the customer may change it on the checkout
	// screen, and the copy written onto the order is read from the address book
	// here either way.
	AddressID string
	// Payment is "cash" or "online".
	Payment string
	// IdempotencyKey is the client's own reference for this attempt. A second
	// request with the same key returns the first order rather than making a
	// second one — which is what a customer on a village 2G connection tapping
	// "Place order" twice actually needs.
	IdempotencyKey string
	Lang           string
}

// Execute places the order.
func (uc *PlaceUseCase) Execute(ctx context.Context, customerID string, req PlaceRequest) (View, error) {
	if req.IdempotencyKey != "" {
		existing, found, err := uc.repo.ByIdempotencyKey(ctx, customerID, req.IdempotencyKey)
		if err != nil {
			return View{}, storageError(err)
		}
		if found {
			// The same answer as the first time, including the same order id.
			// A retry that created a second order would have the customer
			// paying twice and the shop cooking twice.
			return viewOf(existing, CancelView{}, req.Lang), nil
		}
	}

	payment := domain.PaymentMethod(req.Payment)
	if !payment.Valid() {
		return View{}, orderError(domain.ErrUnknownPayment)
	}

	basket, found, err := uc.cart.Current(ctx, customerID)
	if err != nil {
		return View{}, err
	}
	if !found || len(basket.Lines) == 0 {
		return View{}, orderError(domain.ErrNoLines)
	}
	// The cart's own verdict, re-read rather than taken from the client. It
	// already asked the shop, the menu and discovery; repeating all of that
	// here would be a second implementation of revalidation.
	if !basket.Orderable {
		return View{}, errs.New(errs.KindConflict, "cart_not_orderable",
			"Your cart cannot be ordered right now. Please open it to see why.").
			With("blocker", basket.Blocker)
	}

	address, err := uc.user.Address(ctx, customerID, req.AddressID)
	if err != nil {
		return View{}, err
	}
	profile, err := uc.user.Profile(ctx, customerID)
	if err != nil {
		return View{}, err
	}

	shop, err := uc.merchant.Merchant(ctx, basket.MerchantID)
	if err != nil {
		return View{}, err
	}
	accepting, err := uc.merchant.IsAcceptingOrders(ctx, basket.MerchantID)
	if err != nil {
		return View{}, err
	}
	if !accepting {
		return View{}, errs.New(errs.KindConflict, "shop_not_accepting",
			"That shop is not taking orders right now.")
	}

	// D3 and the distance, measured now against the address being ordered to —
	// which may not be the one the cart was held against.
	reach, err := uc.disco.Reach(ctx, disco.Point{Lat: address.Lat, Lng: address.Lng}, basket.MerchantID)
	if err != nil {
		return View{}, err
	}
	if !reach.Reachable {
		return View{}, errs.New(errs.KindConflict, "not_deliverable",
			deliverabilityMessage(reach.Reason, req.Lang)).
			With("reason", reach.Reason)
	}

	quote, err := uc.quote(ctx, reach, basket.Subtotal.Minor, req.Lang)
	if err != nil {
		return View{}, err
	}

	// One settings read for both of the order's rules. Taken together rather
	// than one at a time so the ceiling an order is checked against and the
	// window the customer is given come from the same snapshot.
	rules, err := uc.rules(ctx, reach)
	if err != nil {
		return View{}, err
	}

	now := uc.clock.Now()
	order, err := domain.NewOrder(domain.Draft{
		ID:         uc.ids.New("ORD"),
		EventID:    uc.ids.New("OEV"),
		Code:       uc.codes.Code(),
		CustomerID: customerID,
		MerchantID: basket.MerchantID,
		Payment:    payment,
		Lines:      linesOf(basket, uc.ids),
		Charges: domain.Charges{
			Subtotal:           taka(quote.Subtotal.Minor),
			Delivery:           taka(quote.Delivery.Minor),
			ExpansionSurcharge: taka(quote.ExpansionSurcharge.Minor),
			Total:              taka(quote.Total.Minor),
			FreeDelivery:       quote.FreeDelivery,
			Expanded:           quote.Expanded,
			DistanceM:          quote.DistanceM,
		},
		Destination:    destinationOf(address, profile),
		Pickup:         pickupOf(shop),
		ExpansionLevel: reach.RequiredLevel,
		CODLimit:       taka(rules.codLimitMinor),
		Now:            now,
	})
	if err != nil {
		return View{}, orderError(err)
	}

	if err := uc.repo.Create(ctx, order); err != nil {
		return View{}, storageError(err)
	}
	if req.IdempotencyKey != "" {
		if err := uc.repo.ClaimIdempotencyKey(ctx, customerID, req.IdempotencyKey, order.ID); err != nil {
			return View{}, storageError(err)
		}
	}

	// The cart goes last and its failure is not the customer's problem: the
	// order exists, and a cart that lingers is a nuisance rather than a
	// double charge. Asked rather than deleted, because the cart owns its
	// storage (2.5).
	_ = uc.cart.Clear(ctx, customerID)

	return viewOf(order,
		withCancelText(cancelViewOf(order.MayCustomerCancel(rules.cancellationWindow, now)), req.Lang),
		req.Lang), nil
}

// quote re-prices the order at the moment of writing.
func (uc *PlaceUseCase) quote(ctx context.Context, reach disco.Reach, subtotalMinor int64, lang string) (pricingx.Quote, error) {
	tariff, err := uc.pricing.Tariff(ctx, pricingx.Placement{
		AreaCode:     reach.AreaCode,
		DistrictCode: reach.DistrictCode,
		DivisionCode: reach.DivisionCode,
	})
	if err != nil {
		return pricingx.Quote{}, err
	}
	return tariff.Quote(pricingx.QuoteRequest{
		SubtotalMinor:  subtotalMinor,
		DistanceM:      reach.DistanceM,
		ExpansionLevel: reach.RequiredLevel,
		Lang:           lang,
	})
}

// orderRules is the configuration an order is bound by, read once.
type orderRules struct {
	codLimitMinor      int64
	cancellationWindow time.Duration
}

// rules reads the cash ceiling and the cancellation window in force where the
// order is going.
//
// Both from one snapshot, and both before the order is written. Reading the
// window afterwards would mean a placed order could still fail to describe
// itself, and reading them separately would let an area retuned between the two
// reads check an order against one configuration and offer a countdown from
// another.
func (uc *PlaceUseCase) rules(ctx context.Context, reach disco.Reach) (orderRules, error) {
	settings, err := uc.config.Settings(ctx, cfg.Placement{
		AreaCode:     reach.AreaCode,
		DistrictCode: reach.DistrictCode,
		DivisionCode: reach.DivisionCode,
	})
	if err != nil {
		return orderRules{}, configError(err)
	}
	limit, err := settings.Int(cfg.CODLimit)
	if err != nil {
		return orderRules{}, configError(err)
	}
	window, err := settings.Int(cfg.CancellationWindow)
	if err != nil {
		return orderRules{}, configError(err)
	}
	return orderRules{
		codLimitMinor:      limit,
		cancellationWindow: time.Duration(window) * time.Second,
	}, nil
}

// linesOf copies the cart's lines onto the order, at the prices the cart just
// revalidated.
func linesOf(basket cartx.Cart, ids id.Generator) []domain.Line {
	lines := make([]domain.Line, 0, len(basket.Lines))
	for _, l := range basket.Lines {
		options := make([]domain.Option, 0, len(l.Options))
		for _, o := range l.Options {
			options = append(options, domain.Option{
				GroupID: o.GroupID, OptionID: o.OptionID, Name: o.Name,
				Price: taka(o.Price.Minor),
			})
		}
		lines = append(lines, domain.Line{
			ID: ids.New("OLN"), Kind: l.Kind, TargetID: l.TargetID, Name: l.Name,
			UnitPrice: taka(l.UnitPrice.Minor), Options: options,
			Quantity: l.Quantity, Note: l.Note,
		})
	}
	return lines
}

// destinationOf copies the address onto the order, falling back to the
// customer's own name when the address names no separate recipient.
func destinationOf(a userx.Address, p userx.Profile) domain.Destination {
	recipient := a.RecipientName
	if recipient == "" {
		recipient = p.DisplayName
	}
	return domain.Destination{
		AddressID: a.ID, Label: a.Label, Recipient: recipient,
		Phone: a.RecipientPhone, Line1: a.Line1, Line2: a.Line2,
		SingleLine: a.SingleLine, Lat: a.Lat, Lng: a.Lng,
		AreaCode: a.AreaCode, AreaName: a.AreaName, Directions: a.Instructions,
	}
}

// pickupOf copies the shop onto the order.
func pickupOf(m merchantx.Merchant) domain.Pickup {
	return domain.Pickup{
		MerchantID: m.ID, Name: m.Name, Phone: m.Phone,
		SingleLine: m.SingleLine, Lat: m.Lat, Lng: m.Lng,
	}
}

// deliverabilityMessage says why this address cannot be delivered to, in words.
func deliverabilityMessage(reason, lang string) string {
	bengali := lang != "en"
	if reason == disco.ReasonOutsideDivision {
		if bengali {
			return "এই ঠিকানা অন্য বিভাগে। এই দোকান থেকে সেখানে ডেলিভারি করা যাবে না।"
		}
		return "That address is in another division, so this shop cannot deliver to it."
	}
	if bengali {
		return "এই ঠিকানা দোকান থেকে অনেক দূরে।"
	}
	return "That address is too far from this shop."
}

// configError reports configuration that could not be read.
func configError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "order_config_unavailable",
		"We could not place that order just now. Please try again.")
}
