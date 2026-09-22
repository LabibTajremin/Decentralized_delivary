package application

import (
	"context"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	merchantext "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/external/merchant"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// maxGroupsPerItem caps how many groups of choices one item may carry.
//
// Five is already a long form on a phone. Past that the item is really several
// items, and saying so is better than rendering a screen nobody scrolls to the
// end of.
const maxGroupsPerItem = 5

// OptionUseCase manages an item's variants and add-ons.
//
// Groups are replaced wholesale rather than edited one option at a time. A group
// has invariants across its members — how many may be chosen, no two names alike
// — and patching one option at a time means every intermediate state has to be
// legal, which the "pick exactly one of" rule cannot survive while the last
// option is being removed.
type OptionUseCase struct {
	repo      ports.ItemRepository
	merchants merchantext.Service
	ids       id.Generator
}

// NewOptionUseCase wires the use case.
func NewOptionUseCase(repo ports.ItemRepository, merchants merchantext.Service, ids id.Generator) *OptionUseCase {
	return &OptionUseCase{repo: repo, merchants: merchants, ids: ids}
}

// OptionRequest is one choice within a group.
type OptionRequest struct {
	// ID is kept when an owner edits an existing option, so a cart holding it
	// still resolves. Empty for a new one.
	ID   string
	Name string
	// PriceMinor is a delta for a variant and a price for an add-on. Which it
	// means is decided by the group it is in, not by the field.
	PriceMinor int64
	Available  *bool
}

// GroupRequest is a group of choices.
type GroupRequest struct {
	ID         string
	Name       string
	Required   bool
	MinChoices int
	MaxChoices int
	SortOrder  int
	Options    []OptionRequest
}

// SetVariantGroups replaces an item's variant groups.
func (uc *OptionUseCase) SetVariantGroups(ctx context.Context, ownerUserID, merchantID, itemID string, requested []GroupRequest) (domain.Item, error) {
	item, err := uc.ownedItem(ctx, ownerUserID, merchantID, itemID)
	if err != nil {
		return domain.Item{}, err
	}
	if err := uc.checkGroupCount(len(requested)); err != nil {
		return domain.Item{}, err
	}

	groups := make([]domain.VariantGroup, 0, len(requested))
	for _, req := range requested {
		options := make([]domain.VariantOption, 0, len(req.Options))
		for _, option := range req.Options {
			built, buildErr := domain.NewVariantOption(
				uc.optionID(option.ID, "vop"), option.Name, money.Taka(option.PriceMinor))
			if buildErr != nil {
				return domain.Item{}, entryError(buildErr)
			}
			if option.Available != nil {
				built.Available = *option.Available
			}
			options = append(options, built)
		}

		group, groupErr := domain.NewVariantGroup(uc.optionID(req.ID, "vgr"),
			req.Name, req.Required, req.MinChoices, req.MaxChoices, options)
		if groupErr != nil {
			return domain.Item{}, entryError(groupErr)
		}
		group.SortOrder = req.SortOrder
		groups = append(groups, group)
	}

	// No capability check here, unlike SetAddOnGroups: every shop type has
	// variants, so there is nothing this could refuse, and a branch no input
	// can reach is a branch no test can cover. The domain setter still guards
	// a hand-built item — domain.Item is an exported struct — and that path is
	// covered by the domain's own tests, so discarding its error here is
	// deliberate rather than careless. A future shop type without variants
	// says so in domain.Capabilities, and this is where its check belongs.
	updated, _ := item.WithVariantGroups(groups)
	return uc.save(ctx, updated)
}

// SetAddOnGroups replaces an item's add-on groups.
func (uc *OptionUseCase) SetAddOnGroups(ctx context.Context, ownerUserID, merchantID, itemID string, requested []GroupRequest) (domain.Item, error) {
	item, err := uc.ownedItem(ctx, ownerUserID, merchantID, itemID)
	if err != nil {
		return domain.Item{}, err
	}
	if err := uc.checkGroupCount(len(requested)); err != nil {
		return domain.Item{}, err
	}

	groups := make([]domain.AddOnGroup, 0, len(requested))
	for _, req := range requested {
		options := make([]domain.AddOn, 0, len(req.Options))
		for _, option := range req.Options {
			built, buildErr := domain.NewAddOn(
				uc.optionID(option.ID, "aop"), option.Name, money.Taka(option.PriceMinor))
			if buildErr != nil {
				return domain.Item{}, entryError(buildErr)
			}
			if option.Available != nil {
				built.Available = *option.Available
			}
			options = append(options, built)
		}

		group, groupErr := domain.NewAddOnGroup(uc.optionID(req.ID, "agr"),
			req.Name, req.MinChoices, req.MaxChoices, options)
		if groupErr != nil {
			return domain.Item{}, entryError(groupErr)
		}
		group.SortOrder = req.SortOrder
		groups = append(groups, group)
	}

	// Add-ons are a restaurant's alone, so this one can and does refuse.
	if !domain.CapabilitiesFor(item.MerchantType).AddOns {
		return domain.Item{}, entryError(domain.ErrAddOnsNotAllowed)
	}
	updated, _ := item.WithAddOnGroups(groups) // cannot fail: checked above
	return uc.save(ctx, updated)
}

// checkGroupCount refuses an item with more groups than a phone can show.
func (uc *OptionUseCase) checkGroupCount(n int) error {
	if n > maxGroupsPerItem {
		return errs.Newf(errs.KindInvalid, "too_many_groups",
			"Please use at most %d groups of choices on one item.", maxGroupsPerItem)
	}
	return nil
}

// optionID keeps an existing id or mints a new one.
//
// Keeping it matters: a cart holds the option id a customer picked, and minting
// a fresh one every time the owner saves the group would invalidate every cart
// in flight the moment a typo was fixed.
func (uc *OptionUseCase) optionID(existing, prefix string) string {
	if existing != "" {
		return existing
	}
	return uc.ids.New(prefix)
}

// save writes an item back.
func (uc *OptionUseCase) save(ctx context.Context, item domain.Item) (domain.Item, error) {
	if err := uc.repo.SaveItem(ctx, item); err != nil {
		return domain.Item{}, unavailable(err, "We could not save those choices just now. Please try again.")
	}
	return item, nil
}

// ownedItem resolves the shop, checks ownership, and loads the item.
func (uc *OptionUseCase) ownedItem(ctx context.Context, ownerUserID, merchantID, itemID string) (domain.Item, error) {
	found, _, err := shop(ctx, uc.merchants, merchantID)
	if err != nil {
		return domain.Item{}, err
	}
	if err := ownedBy(found, ownerUserID); err != nil {
		return domain.Item{}, err
	}

	item, err := uc.repo.Item(ctx, merchantID, itemID)
	if errors.Is(err, domain.ErrItemNotFound) {
		return domain.Item{}, notFound(err, "item_not_found",
			"We could not find that item.", "item_id", itemID)
	}
	if err != nil {
		return domain.Item{}, unavailable(err, "We could not load that item just now. Please try again.")
	}
	return item, nil
}
