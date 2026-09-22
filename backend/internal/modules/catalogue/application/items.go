package application

import (
	"context"
	"errors"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	merchantext "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/external/merchant"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/paging"
)

// maxBulkItems caps one bulk update.
//
// A shop re-pricing its whole menu sends one request; a shop sending five
// hundred items is either a mistake or an import, and an import is a different
// feature with a different failure story.
const maxBulkItems = 200

// ItemUseCase manages what a shop sells.
type ItemUseCase struct {
	repo       ports.ItemRepository
	categories ports.CategoryRepository
	merchants  merchantext.Service
	clock      clock.Clock
	ids        id.Generator
}

// NewItemUseCase wires the use case.
func NewItemUseCase(
	repo ports.ItemRepository,
	categories ports.CategoryRepository,
	merchants merchantext.Service,
	c clock.Clock,
	ids id.Generator,
) *ItemUseCase {
	return &ItemUseCase{repo: repo, categories: categories, merchants: merchants, clock: c, ids: ids}
}

// ListRequest narrows an item listing.
type ListRequest struct {
	CategoryID string
	ActiveOnly bool
	Search     string
	Limit      int
	Offset     int
}

// List returns a shop's items.
func (uc *ItemUseCase) List(ctx context.Context, merchantID string, req ListRequest) ([]domain.Item, error) {
	page, err := paging.NewPage(req.Limit, req.Offset)
	if err != nil {
		return nil, errs.Wrap(err, errs.KindInvalid, "invalid_page", "That page is not valid.")
	}

	items, err := uc.repo.Items(ctx, ports.ItemFilter{
		MerchantID: merchantID,
		CategoryID: req.CategoryID,
		ActiveOnly: req.ActiveOnly,
		Search:     req.Search,
		Limit:      page.Limit,
		Offset:     page.Offset,
	})
	if err != nil {
		return nil, unavailable(err, "We could not load the menu just now. Please try again.")
	}
	return items, nil
}

// Get returns one item.
func (uc *ItemUseCase) Get(ctx context.Context, merchantID, itemID string) (domain.Item, error) {
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

// ItemRequest is a new or edited item.
//
// PriceMinor rather than a money.Money, because this is the boundary the
// transport hands values across and the currency is the server's to decide, not
// the client's (2.9).
type ItemRequest struct {
	CategoryID  string
	Name        string
	Description string
	ImageURL    string
	PriceMinor  int64
	SortOrder   int

	// Attributes. Which of these are allowed is decided by the shop's type; the
	// domain refuses the ones that are not, rather than dropping them.
	IsVegetarian         bool
	PreparationMinutes   int
	Unit                 string
	PackSize             string
	Brand                string
	GenericName          string
	Strength             string
	RequiresPrescription bool

	// StockQuantity applies only where the shop type counts stock.
	StockQuantity *int
}

// Create adds an item to a shop's menu.
func (uc *ItemUseCase) Create(ctx context.Context, ownerUserID, merchantID string, req ItemRequest) (domain.Item, error) {
	_, kind, err := uc.owned(ctx, ownerUserID, merchantID)
	if err != nil {
		return domain.Item{}, err
	}
	if err := uc.categoryExists(ctx, merchantID, req.CategoryID); err != nil {
		return domain.Item{}, err
	}

	draft, err := uc.toDraft(req)
	if err != nil {
		return domain.Item{}, err
	}

	item, err := domain.NewItem(uc.ids.New("itm"), merchantID, kind, draft)
	if err != nil {
		return domain.Item{}, entryError(err)
	}
	if err := uc.repo.SaveItem(ctx, item); err != nil {
		return domain.Item{}, unavailable(err, "We could not save that item just now. Please try again.")
	}
	return item, nil
}

// Update edits an item, keeping its option groups and availability.
func (uc *ItemUseCase) Update(ctx context.Context, ownerUserID, merchantID, itemID string, req ItemRequest) (domain.Item, error) {
	existing, kind, err := uc.ownedItem(ctx, ownerUserID, merchantID, itemID)
	if err != nil {
		return domain.Item{}, err
	}
	if err := uc.categoryExists(ctx, merchantID, req.CategoryID); err != nil {
		return domain.Item{}, err
	}

	draft, err := uc.toDraft(req)
	if err != nil {
		return domain.Item{}, err
	}

	// Rebuilt through NewItem rather than patched field by field, so an edit is
	// validated by exactly the rules a creation is. A patch path with its own
	// weaker checks is how an item ends up in a shape creation would refuse.
	updated, err := domain.NewItem(existing.ID, merchantID, kind, draft)
	if err != nil {
		return domain.Item{}, entryError(err)
	}
	updated.Active = existing.Active
	updated.Availability = existing.Availability
	updated.VariantGroups = existing.VariantGroups
	updated.AddOnGroups = existing.AddOnGroups
	if req.StockQuantity == nil {
		// An edit that says nothing about stock leaves the shelf count alone.
		// Resetting it to zero would take a shop's whole inventory offline
		// because somebody fixed a typo in a name.
		updated.Stock = existing.Stock
	}

	if err := uc.repo.SaveItem(ctx, updated); err != nil {
		return domain.Item{}, unavailable(err, "We could not save that item just now. Please try again.")
	}
	return updated, nil
}

// SetActive shows or hides an item.
func (uc *ItemUseCase) SetActive(ctx context.Context, ownerUserID, merchantID, itemID string, active bool) (domain.Item, error) {
	item, _, err := uc.ownedItem(ctx, ownerUserID, merchantID, itemID)
	if err != nil {
		return domain.Item{}, err
	}
	return uc.save(ctx, item.WithActive(active))
}

// SetStock updates a shelf count.
func (uc *ItemUseCase) SetStock(ctx context.Context, ownerUserID, merchantID, itemID string, quantity int) (domain.Item, error) {
	item, _, err := uc.ownedItem(ctx, ownerUserID, merchantID, itemID)
	if err != nil {
		return domain.Item{}, err
	}

	stock, err := domain.NewStock(quantity)
	if err != nil {
		return domain.Item{}, entryError(err)
	}
	updated, err := item.WithStock(stock)
	if err != nil {
		return domain.Item{}, entryError(err)
	}
	return uc.save(ctx, updated)
}

// AvailabilityRequest sets when an item may be ordered.
type AvailabilityRequest struct {
	// Always, or the windows below. Keyed by weekday number with Sunday as "0",
	// each window written "07:00-11:00".
	Always bool
	Days   map[string][]string
}

// SetAvailability puts an item on a schedule, or takes it off one.
func (uc *ItemUseCase) SetAvailability(ctx context.Context, ownerUserID, merchantID, itemID string, req AvailabilityRequest) (domain.Item, error) {
	item, _, err := uc.ownedItem(ctx, ownerUserID, merchantID, itemID)
	if err != nil {
		return domain.Item{}, err
	}

	availability := domain.AlwaysAvailable()
	if !req.Always {
		// Checked before decoding: an empty map decodes to "always available",
		// which is the opposite of what a caller asking for a schedule meant.
		if len(req.Days) == 0 {
			return domain.Item{}, errs.New(errs.KindInvalid, "availability_required",
				"Please choose when this is available, or mark it always available.")
		}
		decoded, decodeErr := domain.DecodeAvailability(req.Days)
		if decodeErr != nil {
			return domain.Item{}, errs.Wrap(decodeErr, errs.KindInvalid, "invalid_availability",
				"Please write availability times like 07:00-11:00.")
		}
		availability = decoded
	}
	return uc.save(ctx, item.WithAvailability(availability))
}

// Delete removes an item.
func (uc *ItemUseCase) Delete(ctx context.Context, ownerUserID, merchantID, itemID string) error {
	if _, _, err := uc.ownedItem(ctx, ownerUserID, merchantID, itemID); err != nil {
		return err
	}
	if err := uc.repo.DeleteItem(ctx, merchantID, itemID); err != nil {
		return unavailable(err, "We could not remove that item just now. Please try again.")
	}
	return nil
}

// BulkChange is one line of a bulk update.
//
// Every field is a pointer so "leave this alone" and "set this" are different
// instructions. Without that, a shop changing prices would have to send stock
// counts back too, and one that forgot would zero its inventory.
type BulkChange struct {
	ItemID     string
	PriceMinor *int64
	Stock      *int
	Active     *bool
}

// BulkUpdate applies a set of changes in one transaction.
//
// One transaction because a half-applied re-pricing is worse than none: a menu
// where the first thirty items moved and the rest did not is a shop selling at
// two price lists, and the owner cannot tell where the boundary fell.
func (uc *ItemUseCase) BulkUpdate(ctx context.Context, ownerUserID, merchantID string, changes []BulkChange) ([]domain.Item, error) {
	if _, _, err := uc.owned(ctx, ownerUserID, merchantID); err != nil {
		return nil, err
	}
	if len(changes) == 0 {
		return nil, errs.New(errs.KindInvalid, "no_changes", "There is nothing to update.")
	}
	if len(changes) > maxBulkItems {
		return nil, errs.Newf(errs.KindInvalid, "too_many_changes",
			"Please update at most %d items at a time.", maxBulkItems)
	}

	ids := make([]string, 0, len(changes))
	for _, change := range changes {
		ids = append(ids, change.ItemID)
	}
	found, err := uc.repo.ItemsByID(ctx, merchantID, ids)
	if err != nil {
		return nil, unavailable(err, "We could not update those items just now. Please try again.")
	}

	byID := make(map[string]domain.Item, len(found))
	for _, item := range found {
		byID[item.ID] = item
	}

	updated := make([]domain.Item, 0, len(changes))
	for _, change := range changes {
		item, ok := byID[change.ItemID]
		if !ok {
			// The whole request is refused rather than the missing lines
			// skipped. A bulk update that silently applied to 28 of 30 items
			// and reported success is a shop that thinks it re-priced its menu.
			return nil, notFound(domain.ErrItemNotFound, "item_not_found",
				"One of those items is not on this menu.", "item_id", change.ItemID)
		}

		if change.PriceMinor != nil {
			item, err = item.WithPrice(money.Taka(*change.PriceMinor))
			if err != nil {
				return nil, entryError(err)
			}
		}
		if change.Stock != nil {
			stock, stockErr := domain.NewStock(*change.Stock)
			if stockErr != nil {
				return nil, entryError(stockErr)
			}
			item, err = item.WithStock(stock)
			if err != nil {
				return nil, entryError(err)
			}
		}
		if change.Active != nil {
			item = item.WithActive(*change.Active)
		}
		updated = append(updated, item)
	}

	if err := uc.repo.SaveItems(ctx, updated); err != nil {
		return nil, unavailable(err, "We could not update those items just now. Please try again.")
	}
	return updated, nil
}

// Now is the clock the use case decides availability against, exposed so
// transport renders "orderable" against the same instant the rules used.
func (uc *ItemUseCase) Now() time.Time { return uc.clock.Now() }

// toDraft turns a request into a draft.
//
// It validates only what is decidable without knowing the shop's type — the
// unit has to parse, the stock count has to be a count. Which of those fields
// this kind of shop may carry at all is decided by domain.NewItem against the
// Capabilities table, in one place, rather than half here and half there.
func (uc *ItemUseCase) toDraft(req ItemRequest) (domain.ItemDraft, error) {
	attributes := domain.Attributes{
		IsVegetarian:         req.IsVegetarian,
		PreparationMinutes:   req.PreparationMinutes,
		PackSize:             req.PackSize,
		Brand:                req.Brand,
		GenericName:          req.GenericName,
		Strength:             req.Strength,
		RequiresPrescription: req.RequiresPrescription,
	}
	if req.Unit != "" {
		unit, err := domain.ParseUnit(req.Unit)
		if err != nil {
			return domain.ItemDraft{}, entryError(err)
		}
		attributes.Unit = unit
	}

	draft := domain.ItemDraft{
		CategoryID:  req.CategoryID,
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		Price:       money.Taka(req.PriceMinor),
		Attributes:  attributes,
		SortOrder:   req.SortOrder,
	}
	if req.StockQuantity != nil {
		stock, err := domain.NewStock(*req.StockQuantity)
		if err != nil {
			return domain.ItemDraft{}, entryError(err)
		}
		draft.Stock = stock
	}
	return draft, nil
}

// categoryExists checks the item is filed under a section of this shop.
func (uc *ItemUseCase) categoryExists(ctx context.Context, merchantID, categoryID string) error {
	if categoryID == "" {
		return entryError(domain.ErrNoCategory)
	}
	_, err := uc.categories.Category(ctx, merchantID, categoryID)
	if errors.Is(err, domain.ErrCategoryNotFound) {
		return notFound(err, "category_not_found",
			"We could not find that section.", "category_id", categoryID)
	}
	if err != nil {
		return unavailable(err, "We could not check that section just now. Please try again.")
	}
	return nil
}

// save writes an item back.
func (uc *ItemUseCase) save(ctx context.Context, item domain.Item) (domain.Item, error) {
	if err := uc.repo.SaveItem(ctx, item); err != nil {
		return domain.Item{}, unavailable(err, "We could not save that item just now. Please try again.")
	}
	return item, nil
}

// owned resolves the shop and checks the caller owns it.
func (uc *ItemUseCase) owned(ctx context.Context, ownerUserID, merchantID string) (merchantext.Merchant, domain.MerchantType, error) {
	found, kind, err := shop(ctx, uc.merchants, merchantID)
	if err != nil {
		return merchantext.Merchant{}, "", err
	}
	if err := ownedBy(found, ownerUserID); err != nil {
		return merchantext.Merchant{}, "", err
	}
	return found, kind, nil
}

// ownedItem resolves the shop, checks ownership, and loads the item.
func (uc *ItemUseCase) ownedItem(ctx context.Context, ownerUserID, merchantID, itemID string) (domain.Item, domain.MerchantType, error) {
	_, kind, err := uc.owned(ctx, ownerUserID, merchantID)
	if err != nil {
		return domain.Item{}, "", err
	}
	item, err := uc.Get(ctx, merchantID, itemID)
	if err != nil {
		return domain.Item{}, "", err
	}
	return item, kind, nil
}
