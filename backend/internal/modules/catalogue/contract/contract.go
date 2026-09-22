// Package contract is the catalogue module's only public surface.
//
// Consumed by: discovery, cart, order (Appendix A).
//
// Discovery shows a shop's menu; cart resolves what a customer picked and needs
// the option prices to validate it; order freezes what was bought. None of them
// may read a catalogue table, which is why everything they need is a primitive
// here — and why money crosses as both a minor-unit integer and a preformatted
// string (2.9).
package contract

import "context"

// Money is an amount as it crosses a module boundary and reaches a client.
//
// Both forms, always. The integer is what cart and pricing compute with; the
// string is what the app renders, because a client that formats money itself
// will eventually format it differently from a receipt.
type Money struct {
	Minor    int64
	Currency string
	Display  string
}

// Option is one choice within a group.
type Option struct {
	ID   string
	Name string
	// Price is a delta for a variant and a price for an add-on. Which it means
	// is decided by the group it sits in.
	Price     Money
	Available bool
}

// OptionGroup is a set of choices on an item.
type OptionGroup struct {
	ID         string
	Name       string
	Required   bool
	MinChoices int
	MaxChoices int
	Options    []Option
}

// Item is one thing a shop sells.
type Item struct {
	ID          string
	MerchantID  string
	CategoryID  string
	Name        string
	Description string
	ImageURL    string
	Price       Money

	// Orderable is whether a customer may add this to a cart right now. The
	// server decides it, from the item's own switch, its stock and its
	// availability window — three different reasons, which is why the code
	// below says which one applied.
	Orderable bool
	// UnavailableReason is "out_of_stock", "not_available_now", "unavailable",
	// or empty when the item is orderable.
	UnavailableReason string

	// The per-shop-type fields. Which of these carry anything is decided by the
	// shop's type; the rest are the zero value, never nonsense.
	Unit                 string
	PackSize             string
	Brand                string
	GenericName          string
	Strength             string
	RequiresPrescription bool
	IsVegetarian         bool
	PreparationMinutes   int

	VariantGroups []OptionGroup
	AddOnGroups   []OptionGroup
}

// Category groups items on a shop's menu.
type Category struct {
	ID        string
	Name      string
	SortOrder int
}

// ComboLine is one item in a bundle.
type ComboLine struct {
	ItemID   string
	Name     string
	Quantity int
}

// Combo is several items sold together at a set price.
type Combo struct {
	ID          string
	MerchantID  string
	Name        string
	Description string
	ImageURL    string
	Price       Money
	Lines       []ComboLine
	Orderable   bool
	// Savings is what the bundle saves against buying its parts separately,
	// computed by the server. A client that worked it out itself would be doing
	// arithmetic on money, which 2.9 forbids outright.
	Savings Money
}

// Menu is a shop's whole catalogue, as a customer sees it.
type Menu struct {
	MerchantID string
	Categories []Category
	Items      []Item
	Combos     []Combo
}

// CatalogueContract is the catalogue module's public interface.
type CatalogueContract interface {
	// Menu returns everything a customer may see from a shop right now.
	Menu(ctx context.Context, merchantID string) (Menu, error)

	// Item returns one item, whatever its state. Cart needs to resolve an item
	// it already holds even after the shop hides it, so it can say so.
	Item(ctx context.Context, merchantID, itemID string) (Item, error)

	// Items returns several items of one shop at once, which is what a cart
	// revalidating its lines needs without a call per line.
	Items(ctx context.Context, merchantID string, itemIDs []string) ([]Item, error)

	// Combo returns one bundle.
	Combo(ctx context.Context, merchantID, comboID string) (Combo, error)
}
