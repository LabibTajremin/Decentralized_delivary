package application

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Service implements contract.CatalogueContract.
//
// It reads the repository directly rather than going through the owner-facing
// use cases: those check that the caller owns the shop, which is exactly wrong
// for a customer browsing somebody else's menu.
type Service struct {
	repo  ports.Repository
	clock clock.Clock
}

// NewService wires the public service.
func NewService(repo ports.Repository, c clock.Clock) *Service {
	return &Service{repo: repo, clock: c}
}

// Menu returns everything a customer may see from a shop right now.
//
// One call rather than three, because a menu screen needs all of it and three
// round trips on a village 2G connection is the difference between a shop that
// opens and one the customer gives up on.
func (s *Service) Menu(ctx context.Context, merchantID string) (contract.Menu, error) {
	now := s.clock.Now()

	categories, err := s.repo.Categories(ctx, merchantID)
	if err != nil {
		return contract.Menu{}, unavailable(err, "We could not load that menu just now. Please try again.")
	}

	items, err := s.repo.Items(ctx, ports.ItemFilter{
		MerchantID: merchantID,
		ActiveOnly: true,
		Limit:      maxMenuItems,
	})
	if err != nil {
		return contract.Menu{}, unavailable(err, "We could not load that menu just now. Please try again.")
	}

	combos, err := s.repo.Combos(ctx, merchantID, true)
	if err != nil {
		return contract.Menu{}, unavailable(err, "We could not load that menu just now. Please try again.")
	}

	menu := contract.Menu{MerchantID: merchantID}
	for _, category := range categories {
		// A hidden section takes its items off the menu with it, so an owner
		// who hides "Winter specials" does not have to hide each dish too.
		if !category.Active {
			continue
		}
		menu.Categories = append(menu.Categories, contract.Category{
			ID: category.ID, Name: category.Name, SortOrder: category.SortOrder,
		})
	}

	visible := make(map[string]bool, len(menu.Categories))
	for _, category := range menu.Categories {
		visible[category.ID] = true
	}

	byID := make(map[string]domain.Item, len(items))
	for _, item := range items {
		byID[item.ID] = item
		if !visible[item.CategoryID] {
			continue
		}
		menu.Items = append(menu.Items, ToContractItem(item, now))
	}

	for _, combo := range combos {
		menu.Combos = append(menu.Combos, s.toContractCombo(combo, byID, now))
	}
	return menu, nil
}

// maxMenuItems caps a whole-menu read. A shop with more than this is a
// supermarket, and a supermarket's catalogue is browsed by category rather than
// handed over in one response.
const maxMenuItems = 500

// Item returns one item, whatever its state.
func (s *Service) Item(ctx context.Context, merchantID, itemID string) (contract.Item, error) {
	item, err := s.repo.Item(ctx, merchantID, itemID)
	if errors.Is(err, domain.ErrItemNotFound) {
		return contract.Item{}, notFound(err, "item_not_found",
			"We could not find that item.", "item_id", itemID)
	}
	if err != nil {
		return contract.Item{}, unavailable(err, "We could not load that item just now. Please try again.")
	}
	return ToContractItem(item, s.clock.Now()), nil
}

// Items returns several items of one shop at once.
//
// Ids that no longer exist are simply absent rather than failing the batch: the
// caller is a cart revalidating lines it has held since before the shop edited
// its menu, and it needs to know which survived — not to be told the whole
// cart is broken.
func (s *Service) Items(ctx context.Context, merchantID string, itemIDs []string) ([]contract.Item, error) {
	found, err := s.repo.ItemsByID(ctx, merchantID, itemIDs)
	if err != nil {
		return nil, unavailable(err, "We could not load those items just now. Please try again.")
	}

	now := s.clock.Now()
	byID := make(map[string]domain.Item, len(found))
	for _, item := range found {
		byID[item.ID] = item
	}

	// Returned in the order asked for, so a caller holding a cart can zip the
	// result against its own lines without a second index.
	out := make([]contract.Item, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		if item, ok := byID[itemID]; ok {
			out = append(out, ToContractItem(item, now))
		}
	}
	return out, nil
}

// Combo returns one bundle.
func (s *Service) Combo(ctx context.Context, merchantID, comboID string) (contract.Combo, error) {
	combo, err := s.repo.Combo(ctx, merchantID, comboID)
	if errors.Is(err, domain.ErrComboNotFound) {
		return contract.Combo{}, notFound(err, "combo_not_found",
			"We could not find that combo.", "combo_id", comboID)
	}
	if err != nil {
		return contract.Combo{}, unavailable(err, "We could not load that combo just now. Please try again.")
	}

	members, err := s.repo.ItemsByID(ctx, merchantID, combo.ItemIDs())
	if err != nil {
		return contract.Combo{}, unavailable(err, "We could not load that combo just now. Please try again.")
	}
	byID := make(map[string]domain.Item, len(members))
	for _, item := range members {
		byID[item.ID] = item
	}
	return s.toContractCombo(combo, byID, s.clock.Now()), nil
}

// toContractCombo renders a bundle, including what it saves.
//
// A combo is orderable only when every item in it is: a meal deal whose drink is
// sold out is a meal deal the kitchen cannot make, and letting it be ordered
// moves the disappointment from the menu screen to the doorstep.
func (s *Service) toContractCombo(combo domain.Combo, members map[string]domain.Item, now time.Time) contract.Combo {
	out := contract.Combo{
		ID:          combo.ID,
		MerchantID:  combo.MerchantID,
		Name:        combo.Name,
		Description: combo.Description,
		ImageURL:    combo.ImageURL,
		Price:       toContractMoney(combo.Price),
		Orderable:   combo.Orderable(now),
	}

	parts := money.Zero(combo.Price.Currency())
	for _, line := range combo.Lines {
		member, ok := members[line.ItemID]
		if !ok || !member.Orderable(now) {
			out.Orderable = false
		}
		out.Lines = append(out.Lines, contract.ComboLine{
			ItemID: line.ItemID, Name: member.Name, Quantity: line.Quantity,
		})
		if ok {
			// The sum cannot fail: every price in a shop is in the same
			// currency, which money.Taka guarantees at the only place prices
			// are built. The error is dropped rather than propagated because
			// there is no branch for a caller to handle.
			parts, _ = parts.Add(member.Price.Times(line.Quantity))
		}
	}

	savings, _ := parts.Sub(combo.Price)
	if savings.IsNegative() {
		// A "combo" priced above its parts saves nothing. Reporting a negative
		// saving would render as "save -৳40", which is worse than saying
		// nothing.
		savings = money.Zero(combo.Price.Currency())
	}
	out.Savings = toContractMoney(savings)
	return out
}

// ToContractItem converts an item to its public form.
//
// Exported because transport renders the same shape: one conversion means the
// item a consuming module receives and the item the app shows are assembled by
// the same code, so they cannot drift apart.
func ToContractItem(i domain.Item, now time.Time) contract.Item {
	out := contract.Item{
		ID:                   i.ID,
		MerchantID:           i.MerchantID,
		CategoryID:           i.CategoryID,
		Name:                 i.Name,
		Description:          i.Description,
		ImageURL:             i.ImageURL,
		Price:                toContractMoney(i.Price),
		Orderable:            i.Orderable(now),
		UnavailableReason:    i.UnavailableReason(now),
		Unit:                 i.Attributes.Unit.String(),
		PackSize:             i.Attributes.PackSize,
		Brand:                i.Attributes.Brand,
		GenericName:          i.Attributes.GenericName,
		Strength:             i.Attributes.Strength,
		RequiresPrescription: i.Attributes.RequiresPrescription,
		IsVegetarian:         i.Attributes.IsVegetarian,
		PreparationMinutes:   i.Attributes.PreparationMinutes,
	}

	for _, group := range sortedVariantGroups(i.VariantGroups) {
		options := make([]contract.Option, 0, len(group.Options))
		for _, option := range group.Options {
			options = append(options, contract.Option{
				ID: option.ID, Name: option.Name,
				Price: toContractMoney(option.PriceDelta), Available: option.Available,
			})
		}
		out.VariantGroups = append(out.VariantGroups, contract.OptionGroup{
			ID: group.ID, Name: group.Name, Required: group.Required,
			MinChoices: group.MinChoices, MaxChoices: group.MaxChoices, Options: options,
		})
	}

	for _, group := range sortedAddOnGroups(i.AddOnGroups) {
		options := make([]contract.Option, 0, len(group.Options))
		for _, option := range group.Options {
			options = append(options, contract.Option{
				ID: option.ID, Name: option.Name,
				Price: toContractMoney(option.Price), Available: option.Available,
			})
		}
		out.AddOnGroups = append(out.AddOnGroups, contract.OptionGroup{
			ID: group.ID, Name: group.Name,
			MinChoices: group.MinChoices, MaxChoices: group.MaxChoices, Options: options,
		})
	}
	return out
}

// sortedVariantGroups returns the groups in the owner's order, without editing
// the item's own slice.
func sortedVariantGroups(groups []domain.VariantGroup) []domain.VariantGroup {
	out := make([]domain.VariantGroup, len(groups))
	copy(out, groups)
	sort.SliceStable(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out
}

// sortedAddOnGroups does the same for add-ons.
func sortedAddOnGroups(groups []domain.AddOnGroup) []domain.AddOnGroup {
	out := make([]domain.AddOnGroup, len(groups))
	copy(out, groups)
	sort.SliceStable(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out
}

// ToContractMoney renders an amount in both the forms a consumer needs.
func ToContractMoney(m money.Money) contract.Money { return toContractMoney(m) }

func toContractMoney(m money.Money) contract.Money {
	return contract.Money{
		Minor:    m.Minor(),
		Currency: string(m.Currency()),
		Display:  m.Display(),
	}
}
