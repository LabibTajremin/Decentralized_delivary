// Package domain holds the catalogue model: what a shop sells.
//
// The phase's acceptance criterion is that the per-merchant-type schema
// differences hold, and that is what shapes this package. A restaurant dish, a
// bag of rice and a strip of paracetamol are not the same kind of thing: one has
// add-ons and no stock count, one has a unit of sale and a shelf count, one has
// a strength and may need a prescription. Modelling them as one bag of optional
// fields would mean every consumer guessing which fields apply.
//
// So the differences live in one table — see Capabilities — and every rule that
// depends on them reads from it.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Errors returned when building catalogue entries.
var (
	// ErrEmptyMerchant means no shop was named.
	ErrEmptyMerchant = errors.New("a shop is required")
	// ErrEmptyName means the entry has no name.
	ErrEmptyName = errors.New("a name is required")
	// ErrNameTooLong means the name exceeds the limit.
	ErrNameTooLong = errors.New("that name is too long")
	// ErrTooLong means a free-text field exceeds its limit.
	ErrTooLong = errors.New("that text is too long")
	// ErrUnknownMerchantType means the shop type is not one we support.
	ErrUnknownMerchantType = errors.New("unknown shop type")
	// ErrNegativePrice means a price was below zero.
	ErrNegativePrice = errors.New("a price cannot be negative")
	// ErrCategoryNotFound means no such category belongs to this shop.
	ErrCategoryNotFound = errors.New("category not found")
	// ErrItemNotFound means no such item belongs to this shop.
	ErrItemNotFound = errors.New("item not found")
	// ErrComboNotFound means no such combo belongs to this shop.
	ErrComboNotFound = errors.New("combo not found")
	// ErrWrongCategory means the category belongs to another shop.
	ErrWrongCategory = errors.New("that category belongs to another shop")
	// ErrInvalidImageURL means the image location is not usable.
	ErrInvalidImageURL = errors.New("that image address is not valid")
)

const (
	maxNameLength        = 120
	maxDescriptionLength = 1000
	maxShortTextLength   = 60
)

// MerchantType is the kind of shop a catalogue belongs to.
//
// Mirrored from the merchant contract as a primitive rather than imported from
// merchant's domain, which 2.5 forbids. The values are the same three strings;
// ParseMerchantType is the one place that has to agree with them.
type MerchantType string

// The three shop types.
const (
	Restaurant MerchantType = "restaurant"
	Grocery    MerchantType = "grocery"
	Pharmacy   MerchantType = "pharmacy"
)

// AllMerchantTypes lists the types in a fixed order.
func AllMerchantTypes() []MerchantType { return []MerchantType{Restaurant, Grocery, Pharmacy} }

// ParseMerchantType reads a shop type.
func ParseMerchantType(s string) (MerchantType, error) {
	candidate := MerchantType(strings.TrimSpace(strings.ToLower(s)))
	for _, known := range AllMerchantTypes() {
		if candidate == known {
			return known, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownMerchantType, s)
}

// String returns the type code.
func (t MerchantType) String() string { return string(t) }

// Capabilities is what a catalogue of a given shop type may contain.
//
// This table *is* the acceptance criterion. Every per-type rule in the module
// reads from here rather than switching on the type itself, so adding a fourth
// kind of shop is a row in this function and not a hunt through the use cases
// for the switches that forgot it.
type Capabilities struct {
	// Variants are mutually-exclusive choices within one item: a size, a
	// strength, a pack. Every type has them, because every type sells the same
	// thing in more than one size.
	Variants bool
	// AddOns are extras bought alongside: extra cheese, a soft drink. Only a
	// restaurant has them — "extra cheese" on a box of paracetamol is not a
	// gap in the product, it is a category error, and allowing it would put a
	// meaningless control on every grocery item screen.
	AddOns bool
	// Combos bundle several items at a set price. Restaurants and groceries
	// run them; a pharmacy bundling medicines at a discount is something we
	// should not make easy.
	Combos bool
	// TracksStock means an item carries a shelf count. A grocer and a pharmacy
	// sell from a finite shelf and oversell if we do not count; a kitchen
	// cooks to order, and a count there would be a number nobody updates.
	TracksStock bool
	// RequiresUnit means an item must state how it is sold — per kg, per
	// piece, per litre. Without it "Rice 500" is a price with no meaning.
	RequiresUnit bool
	// Prescriptions means an item may be marked as requiring one.
	Prescriptions bool
}

// CapabilitiesFor returns what this kind of shop may put in its catalogue.
func CapabilitiesFor(t MerchantType) Capabilities {
	switch t {
	case Restaurant:
		return Capabilities{Variants: true, AddOns: true, Combos: true}
	case Grocery:
		return Capabilities{Variants: true, Combos: true, TracksStock: true, RequiresUnit: true}
	case Pharmacy:
		return Capabilities{Variants: true, TracksStock: true, RequiresUnit: true, Prescriptions: true}
	default:
		// A type we do not recognise gets nothing. Reached only through a
		// hand-built value — every entry point parses the type first — and it
		// fails closed rather than inheriting a restaurant's capabilities.
		return Capabilities{}
	}
}

// Category groups items on a shop's menu.
type Category struct {
	ID         string
	MerchantID string
	Name       string
	// SortOrder is the shop's own ordering. Decided by the owner and applied by
	// the server, so every client shows the menu in the same order rather than
	// each sorting by whatever field it happens to have.
	SortOrder int
	Active    bool
}

// NewCategory validates and builds a category.
func NewCategory(id, merchantID, name string, sortOrder int) (Category, error) {
	if strings.TrimSpace(merchantID) == "" {
		return Category{}, ErrEmptyMerchant
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, ErrEmptyName
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return Category{}, fmt.Errorf("%w: limit %d characters", ErrNameTooLong, maxNameLength)
	}
	if sortOrder < 0 {
		sortOrder = 0
	}
	return Category{
		ID: id, MerchantID: merchantID, Name: name,
		SortOrder: sortOrder, Active: true,
	}, nil
}

// WithName returns a copy under a new name.
func (c Category) WithName(name string) (Category, error) {
	renamed, err := NewCategory(c.ID, c.MerchantID, name, c.SortOrder)
	if err != nil {
		return Category{}, err
	}
	renamed.Active = c.Active
	return renamed, nil
}

// WithSortOrder returns a copy in a new position.
func (c Category) WithSortOrder(sortOrder int) Category {
	if sortOrder < 0 {
		sortOrder = 0
	}
	c.SortOrder = sortOrder
	return c
}

// WithActive returns a copy shown or hidden.
//
// Hidden rather than deleted: a category with items in it that is deleted takes
// the items' grouping with it, and an owner who hides a seasonal menu in
// February expects it back in December.
func (c Category) WithActive(active bool) Category {
	c.Active = active
	return c
}

// trimmedName validates a name shared by several entry types.
func trimmedName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrEmptyName
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return "", fmt.Errorf("%w: limit %d characters", ErrNameTooLong, maxNameLength)
	}
	return name, nil
}

// shortText validates an optional short field such as a brand or a strength.
func shortText(value string) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > maxShortTextLength {
		return "", fmt.Errorf("%w: limit %d characters", ErrTooLong, maxShortTextLength)
	}
	return value, nil
}

// normaliseImageURL accepts a path we serve or an https URL, and nothing else.
//
// http:// is refused rather than upgraded: an image loaded over plain HTTP turns
// every menu page into mixed content, and silently rewriting a URL an owner
// typed hides the problem instead of fixing it.
func normaliseImageURL(raw string) (string, error) {
	image := strings.TrimSpace(raw)
	switch {
	case image == "":
		return "", nil
	case strings.HasPrefix(image, "//"):
		// Protocol-relative, so it inherits the page's scheme — which is not
		// ours to promise.
		return "", fmt.Errorf("%w: %q", ErrInvalidImageURL, raw)
	case strings.HasPrefix(image, "/"):
		return image, nil
	case strings.HasPrefix(image, "https://") && len(image) > len("https://"):
		return image, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidImageURL, raw)
	}
}
