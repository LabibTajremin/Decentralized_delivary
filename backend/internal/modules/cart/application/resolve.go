package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/external/catalogue"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Resolving what the customer tapped into a line the cart can hold.
//
// The client sends ids: an item and the option ids it chose. Everything else —
// the name, the price, whether that combination is even allowed — is read from
// the catalogue here. A request that carried a price would be a request that
// could carry a different one (2.9).

// resolve turns an add request into a line, at today's prices.
func (uc *CartUseCase) resolve(ctx context.Context, req AddRequest) (domain.Line, error) {
	if req.Quantity <= 0 {
		return domain.Line{}, cartError(domain.ErrQuantityRange)
	}

	switch domain.Kind(req.Kind) {
	case domain.KindCombo:
		return uc.resolveCombo(ctx, req)
	case domain.KindItem:
		return uc.resolveItem(ctx, req)
	default:
		return domain.Line{}, cartError(domain.ErrNoTarget)
	}
}

func (uc *CartUseCase) resolveCombo(ctx context.Context, req AddRequest) (domain.Line, error) {
	combo, err := uc.catalogue.Combo(ctx, req.MerchantID, req.TargetID)
	if err != nil {
		return domain.Line{}, err
	}
	if !combo.Orderable {
		return domain.Line{}, errs.New(errs.KindConflict, "not_orderable",
			"That bundle is not available right now.")
	}
	// A combo has no options: its contents are the shop's choice, at a price
	// that is not the sum of its parts. Accepting choices here and ignoring
	// them would be worse than refusing them.
	if len(req.Choices) > 0 {
		return domain.Line{}, errs.New(errs.KindInvalid, "combo_has_no_options",
			"That bundle comes as it is.")
	}
	return domain.Line{
		ID:        uc.ids.New("CLN"),
		Kind:      domain.KindCombo,
		TargetID:  combo.ID,
		Name:      combo.Name,
		UnitPrice: fromMinor(combo.Price.Minor),
		Quantity:  req.Quantity,
		Note:      req.Note,
	}, nil
}

func (uc *CartUseCase) resolveItem(ctx context.Context, req AddRequest) (domain.Line, error) {
	item, err := uc.catalogue.Item(ctx, req.MerchantID, req.TargetID)
	if err != nil {
		return domain.Line{}, err
	}
	if !item.Orderable {
		return domain.Line{}, errs.New(errs.KindConflict, "not_orderable",
			"That item is not available right now.").
			With("reason", item.UnavailableReason)
	}

	options, err := chooseOptions(item, req.Choices)
	if err != nil {
		return domain.Line{}, err
	}
	return domain.Line{
		ID:        uc.ids.New("CLN"),
		Kind:      domain.KindItem,
		TargetID:  item.ID,
		Name:      item.Name,
		UnitPrice: fromMinor(item.Price.Minor),
		Options:   options,
		Quantity:  req.Quantity,
		Note:      req.Note,
	}, nil
}

// chooseOptions resolves the chosen option ids against the item's groups and
// checks the selection is one the shop allows.
//
// The min/max rules belong to catalogue, which is why the groups are read from
// there rather than restated. What happens here is enforcement at the moment it
// matters: an item added without its required size is an order the kitchen
// cannot make, and finding that out at checkout is finding it out too late.
func chooseOptions(item catalogue.Item, choices []Choice) ([]domain.Option, error) {
	groups := map[string]catalogue.OptionGroup{}
	for _, g := range append(append([]catalogue.OptionGroup{}, item.VariantGroups...), item.AddOnGroups...) {
		groups[g.ID] = g
	}

	picked := map[string]int{}
	options := make([]domain.Option, 0, len(choices))
	seen := map[string]bool{}

	for _, choice := range choices {
		group, ok := groups[choice.GroupID]
		if !ok {
			return nil, errs.New(errs.KindInvalid, "unknown_option_group",
				"One of the choices is not offered on that item.")
		}
		key := choice.GroupID + "/" + choice.OptionID
		if seen[key] {
			return nil, errs.New(errs.KindInvalid, "duplicate_option",
				"The same choice was made twice.")
		}
		seen[key] = true

		option, found := optionIn(group, choice.OptionID)
		if !found {
			return nil, errs.New(errs.KindInvalid, "unknown_option",
				"One of the choices is not offered on that item.")
		}
		if !option.Available {
			return nil, errs.New(errs.KindConflict, "option_unavailable",
				"One of the choices is not available right now.").
				With("option", option.Name)
		}
		picked[choice.GroupID]++
		options = append(options, domain.Option{
			GroupID:  group.ID,
			OptionID: option.ID,
			Name:     option.Name,
			Price:    fromMinor(option.Price.Minor),
		})
	}

	for _, group := range groups {
		if err := checkGroup(group, picked[group.ID]); err != nil {
			return nil, err
		}
	}
	return options, nil
}

// checkGroup enforces one group's min and max against what was picked.
func checkGroup(group catalogue.OptionGroup, picked int) error {
	if group.Required && picked == 0 {
		return errs.New(errs.KindInvalid, "option_required",
			"Please choose "+group.Name+".").With("group", group.Name)
	}
	if picked == 0 {
		// An optional group nobody touched. Its minimum applies to a customer
		// who is using it, not to one who ignored it: a "choose at least two
		// toppings" group must not force toppings onto someone who wanted none.
		return nil
	}
	if group.MinChoices > 0 && picked < group.MinChoices {
		return errs.New(errs.KindInvalid, "too_few_options",
			"Please choose more from "+group.Name+".").With("group", group.Name)
	}
	if group.MaxChoices > 0 && picked > group.MaxChoices {
		return errs.New(errs.KindInvalid, "too_many_options",
			"You have chosen too many from "+group.Name+".").With("group", group.Name)
	}
	return nil
}

func optionIn(group catalogue.OptionGroup, optionID string) (catalogue.Option, bool) {
	for _, option := range group.Options {
		if option.ID == optionID {
			return option, true
		}
	}
	return catalogue.Option{}, false
}
