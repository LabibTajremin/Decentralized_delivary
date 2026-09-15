package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/catalogue"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/discovery"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/merchant"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// revalidator re-asks the world about a cart.
//
// Split out from the use case because it is what every single read does. A cart
// is never returned from storage as-is: the shop has been editing its menu, and
// a cart shown from the database alone is a cart that lies.
type revalidator struct {
	catalogue catalogue.Service
	merchant  merchant.Service
	discovery discovery.Service
}

// Execute checks a cart against the shop, the menu and the delivery address.
//
// Three calls regardless of how many lines there are: one merchant read, one
// batched item read, and one reachability check. Combos are the exception —
// they have no batch form, and a cart rarely holds more than one or two.
func (r revalidator) Execute(ctx context.Context, cart domain.Cart) (domain.Check, merchant.Merchant, error) {
	shop, err := r.merchant.Merchant(ctx, cart.MerchantID)
	if err != nil {
		return domain.Check{}, merchant.Merchant{}, err
	}

	reach, err := r.reachOf(ctx, cart)
	if err != nil {
		return domain.Check{}, merchant.Merchant{}, err
	}

	lines, err := r.checkLines(ctx, cart)
	if err != nil {
		return domain.Check{}, merchant.Merchant{}, err
	}

	return domain.Decide(cart,
		domain.Shop{Listed: shop.IsListed, Open: shop.IsOpenNow},
		reach, lines), shop, nil
}

// reachOf asks discovery whether this address can still order from this shop.
//
// Skipped entirely when there is no address: there is nothing to ask about, and
// Decide already reports the missing address as the blocker.
func (r revalidator) reachOf(ctx context.Context, cart domain.Cart) (domain.Reach, error) {
	if !cart.HasAddress() {
		return domain.Reach{}, nil
	}
	reach, err := r.discovery.Reach(ctx, discovery.Point{Lat: cart.Lat, Lng: cart.Lng}, cart.MerchantID)
	if err != nil {
		return domain.Reach{}, err
	}
	return domain.Reach{Reachable: reach.Reachable, Reason: reach.Reason}, nil
}

// checkLines compares every line against what the shop sells now.
func (r revalidator) checkLines(ctx context.Context, cart domain.Cart) ([]domain.CheckedLine, error) {
	if cart.IsEmpty() {
		return nil, nil
	}

	itemIDs := make([]string, 0, len(cart.Lines))
	for _, l := range cart.Lines {
		if l.Kind == domain.KindItem {
			itemIDs = append(itemIDs, l.TargetID)
		}
	}

	items := map[string]catalogue.Item{}
	if len(itemIDs) > 0 {
		found, err := r.catalogue.Items(ctx, cart.MerchantID, itemIDs)
		if err != nil {
			return nil, err
		}
		for _, item := range found {
			items[item.ID] = item
		}
	}

	checked := make([]domain.CheckedLine, 0, len(cart.Lines))
	for _, line := range cart.Lines {
		checked = append(checked, domain.CheckLine(line, r.currentOf(ctx, cart.MerchantID, line, items)))
	}
	return checked, nil
}

// currentOf is what the shop says about one line right now.
//
// Total, with no error return. Every way a line can fail to resolve — deleted,
// turned off, an option withdrawn — is a fact about the line rather than an
// outage, and each has its own Issue. A read that failed here would take the
// rest of a good cart down with it.
func (r revalidator) currentOf(
	ctx context.Context,
	merchantID string,
	line domain.Line,
	items map[string]catalogue.Item,
) domain.Current {
	if line.Kind == domain.KindCombo {
		combo, err := r.catalogue.Combo(ctx, merchantID, line.TargetID)
		if err != nil {
			// A combo that has been deleted reads as "removed" rather than as
			// an outage. The customer's answer is the same either way — it is
			// not coming.
			return domain.Current{}
		}
		return domain.Current{
			Found:     true,
			Orderable: combo.Orderable,
			Reason:    "unavailable",
			UnitPrice: fromMinor(combo.Price.Minor),
		}
	}

	item, ok := items[line.TargetID]
	if !ok {
		return domain.Current{}
	}

	price, chosen := currentUnitPrice(item, line)
	if !chosen {
		// A variant or add-on the customer picked is gone. The line cannot be
		// priced, so it cannot be ordered — and saying "unavailable" is more
		// honest than repricing it without the choice they made.
		return domain.Current{Found: true, Reason: "unavailable"}
	}
	return domain.Current{
		Found:     true,
		Orderable: item.Orderable,
		Reason:    item.UnavailableReason,
		UnitPrice: price,
	}
}

// currentUnitPrice reprices one of a line against the item's option groups as
// they stand. The bool is false when any chosen option has gone or been turned
// off.
func currentUnitPrice(item catalogue.Item, line domain.Line) (money.Money, bool) {
	available := map[string]int64{}
	for _, group := range append(append([]catalogue.OptionGroup{}, item.VariantGroups...), item.AddOnGroups...) {
		for _, option := range group.Options {
			if !option.Available {
				continue
			}
			available[group.ID+"/"+option.ID] = option.Price.Minor
		}
	}

	total := fromMinor(item.Price.Minor)
	for _, chosen := range line.Options {
		minor, ok := available[chosen.GroupID+"/"+chosen.OptionID]
		if !ok {
			return money.Money{}, false
		}
		total = money.Taka(total.Minor() + minor)
	}
	return total, true
}
