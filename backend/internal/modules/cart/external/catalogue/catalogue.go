// Package catalogue is the cart module's view of the catalogue module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package catalogue

import (
	"context"

	catcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/contract"
)

// Item is one thing a shop sells.
type Item = catcontract.Item

// Combo is several items sold together at a set price.
type Combo = catcontract.Combo

// OptionGroup is a set of choices on an item.
type OptionGroup = catcontract.OptionGroup

// Option is one choice within a group.
type Option = catcontract.Option

// Service is the part of catalogue the cart depends on.
//
// Items in the plural is the point: a cart revalidating eight lines must not
// make eight round trips, and a cart that made one call per line would get
// slower exactly as it got more valuable.
type Service interface {
	Item(ctx context.Context, merchantID, itemID string) (Item, error)
	Items(ctx context.Context, merchantID string, itemIDs []string) ([]Item, error)
	Combo(ctx context.Context, merchantID, comboID string) (Combo, error)
}
