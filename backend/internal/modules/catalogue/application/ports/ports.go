// Package ports declares what the catalogue use cases need from the outside
// world. No use case ever touches a database handle (05-architecture.md 2.3).
package ports

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
)

// CategoryRepository stores a shop's menu sections.
type CategoryRepository interface {
	// Categories returns a shop's categories in the owner's own order.
	Categories(ctx context.Context, merchantID string) ([]domain.Category, error)

	// Category returns one category belonging to a shop.
	//
	// The merchant id is part of the lookup rather than a check afterwards: a
	// query that can return another shop's row is one careless call away from
	// a handler that forgets to compare.
	Category(ctx context.Context, merchantID, categoryID string) (domain.Category, error)

	// SaveCategory writes a category, creating it if absent.
	SaveCategory(ctx context.Context, c domain.Category) error

	// DeleteCategory removes a category. Refused by the use case while items
	// still point at it.
	DeleteCategory(ctx context.Context, merchantID, categoryID string) error
}

// ItemFilter narrows an item listing.
type ItemFilter struct {
	MerchantID string
	CategoryID string
	// ActiveOnly hides what the owner has taken down. Customer-facing reads set
	// it; the owner's own menu screen does not, because an owner needs to see
	// what they hid in order to unhide it.
	ActiveOnly bool
	// Search matches the name, case-insensitively.
	Search string
	Limit  int
	Offset int
}

// ItemRepository stores what a shop sells.
type ItemRepository interface {
	// Items returns items matching a filter, in the owner's own order.
	Items(ctx context.Context, f ItemFilter) ([]domain.Item, error)

	// Item returns one item belonging to a shop.
	Item(ctx context.Context, merchantID, itemID string) (domain.Item, error)

	// ItemsByID returns several items of one shop at once, which is what a
	// combo and a cart both need. Ids that do not exist are simply absent.
	ItemsByID(ctx context.Context, merchantID string, itemIDs []string) ([]domain.Item, error)

	// SaveItem writes an item and its option groups in one transaction.
	//
	// The whole item rather than a field at a time, because an item's variant
	// groups are part of what it *is*: a window where the price has changed and
	// the sizes have not is a window where a customer is charged for a size
	// that no longer exists.
	SaveItem(ctx context.Context, i domain.Item) error

	// SaveItems writes several items in one transaction, for a bulk update.
	SaveItems(ctx context.Context, items []domain.Item) error

	// DeleteItem removes an item and everything hanging off it.
	DeleteItem(ctx context.Context, merchantID, itemID string) error

	// CountItemsInCategory reports how many items point at a category.
	CountItemsInCategory(ctx context.Context, merchantID, categoryID string) (int, error)
}

// ComboRepository stores bundles.
type ComboRepository interface {
	// Combos returns a shop's combos in the owner's own order.
	Combos(ctx context.Context, merchantID string, activeOnly bool) ([]domain.Combo, error)

	// Combo returns one combo belonging to a shop.
	Combo(ctx context.Context, merchantID, comboID string) (domain.Combo, error)

	// SaveCombo writes a combo and its lines in one transaction.
	SaveCombo(ctx context.Context, c domain.Combo) error

	// DeleteCombo removes a combo.
	DeleteCombo(ctx context.Context, merchantID, comboID string) error
}

// Repository is the whole catalogue store.
//
// Composed rather than one flat interface so a consumer — and a test — can
// depend on the part it uses. The Postgres implementation satisfies all three.
type Repository interface {
	CategoryRepository
	ItemRepository
	ComboRepository
}
