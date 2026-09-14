package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/schedule"
)

// Errors returned when building an item.
var (
	// ErrAddOnsNotAllowed means this kind of shop does not have add-ons.
	ErrAddOnsNotAllowed = errors.New("this kind of shop does not use add-ons")
	// ErrUnitRequired means the item does not say how it is sold.
	ErrUnitRequired = errors.New("please say how this is sold — per kg, per piece, and so on")
	// ErrUnitNotAllowed means this kind of shop does not sell by unit.
	ErrUnitNotAllowed = errors.New("this kind of shop does not sell by unit")
	// ErrPrescriptionNotAllowed means only a pharmacy may require a prescription.
	ErrPrescriptionNotAllowed = errors.New("only a pharmacy can require a prescription")
	// ErrUnknownUnit means the unit of sale is not one we support.
	ErrUnknownUnit = errors.New("unknown unit of sale")
	// ErrStockNotTracked means a count was given for an item that has none.
	ErrStockNotTracked = errors.New("this kind of shop does not count stock")
	// ErrNegativeStock means a count was below zero.
	ErrNegativeStock = errors.New("a stock count cannot be negative")
	// ErrNoCategory means the item was not filed under one.
	ErrNoCategory = errors.New("please choose a category")
)

// Unit is how an item is sold.
type Unit string

// The units of sale.
const (
	UnitPiece      Unit = "piece"
	UnitKilogram   Unit = "kg"
	UnitGram       Unit = "g"
	UnitLitre      Unit = "litre"
	UnitMillilitre Unit = "ml"
	UnitPack       Unit = "pack"
	UnitStrip      Unit = "strip"
	UnitBottle     Unit = "bottle"
)

// AllUnits lists the units in a fixed order.
func AllUnits() []Unit {
	return []Unit{UnitPiece, UnitKilogram, UnitGram, UnitLitre, UnitMillilitre,
		UnitPack, UnitStrip, UnitBottle}
}

// ParseUnit reads a unit of sale.
func ParseUnit(s string) (Unit, error) {
	candidate := Unit(strings.TrimSpace(strings.ToLower(s)))
	for _, known := range AllUnits() {
		if candidate == known {
			return known, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownUnit, s)
}

// String returns the unit code.
func (u Unit) String() string { return string(u) }

// Stock is how many of an item the shop has.
//
// Tracked is not merely "Quantity > 0": a kitchen has no count at all, and
// representing that as zero would read as sold out. The two states are
// different and the difference is visible to a customer.
type Stock struct {
	Tracked  bool
	Quantity int
}

// Untracked is the stock of something made to order.
func Untracked() Stock { return Stock{Tracked: false} }

// NewStock builds a counted stock level.
func NewStock(quantity int) (Stock, error) {
	if quantity < 0 {
		return Stock{}, fmt.Errorf("%w: %d", ErrNegativeStock, quantity)
	}
	return Stock{Tracked: true, Quantity: quantity}, nil
}

// InStock reports whether the item can be bought.
func (s Stock) InStock() bool { return !s.Tracked || s.Quantity > 0 }

// Availability is when an item may be ordered.
//
// The zero value means always, which is what almost every item is. A breakfast
// menu that only exists until 11am is the exception, and it should be the thing
// that takes configuring.
type Availability struct {
	// Always is the default. When false, Windows decides.
	Always  bool
	Windows schedule.Weekly
}

// AlwaysAvailable is the default availability.
func AlwaysAvailable() Availability { return Availability{Always: true} }

// NewAvailability builds a scheduled availability.
//
// No cap on windows a day: unlike a shop's opening hours, where three shifts is
// already unusual, there is no natural ceiling on when an item may be sold, and
// inventing one would refuse a schedule for no reason.
func NewAvailability(days map[time.Weekday][]schedule.Window) (Availability, error) {
	week, err := schedule.New(days, 0)
	if err != nil {
		return Availability{}, err
	}
	if week.IsEmpty() {
		// An item available at no time is invisible in a way that looks like a
		// bug to its owner. Hiding it is what Active is for, and that at least
		// says so on the owner's screen.
		return Availability{}, fmt.Errorf("%w: it would never be available", schedule.ErrInvalidWindow)
	}
	return Availability{Windows: week}, nil
}

// AvailableAt reports whether the item may be ordered at an instant.
func (a Availability) AvailableAt(t time.Time) bool {
	if a.Always {
		return true
	}
	return a.Windows.Covers(t)
}

// Encode renders the availability for storage. An always-available item stores
// nothing, which is also how an older row with no schedule reads back.
func (a Availability) Encode() map[string][]string {
	if a.Always {
		return nil
	}
	return a.Windows.Encode()
}

// DecodeAvailability reads the stored form back.
func DecodeAvailability(encoded map[string][]string) (Availability, error) {
	if len(encoded) == 0 {
		return AlwaysAvailable(), nil
	}
	week, err := schedule.Decode(encoded, 0)
	if err != nil {
		return Availability{}, err
	}
	return Availability{Windows: week}, nil
}

// Attributes are the fields that differ by shop type.
//
// One struct with per-type fields rather than three item types, because every
// consumer — cart, order, discovery — handles an item the same way and only the
// owner's screen cares about the difference. Which fields are allowed is
// decided by Capabilities, checked once when the item is built, so a consumer
// reading Strength on a restaurant dish gets an empty string rather than
// nonsense.
type Attributes struct {
	// Restaurant.
	IsVegetarian       bool
	PreparationMinutes int

	// Grocery and pharmacy.
	Unit     Unit
	PackSize string
	Brand    string

	// Pharmacy.
	GenericName          string
	Strength             string
	RequiresPrescription bool
}

// Item is one thing a shop sells.
type Item struct {
	ID           string
	MerchantID   string
	MerchantType MerchantType
	CategoryID   string
	Name         string
	Description  string
	ImageURL     string
	// Price is the base price. A variant may add to or subtract from it; the
	// sum is computed by pricing (P10) and never by a client (2.9).
	Price      money.Money
	Attributes Attributes
	Stock      Stock
	// Active is the owner's own switch. Distinct from being out of stock and
	// from being outside its availability window: three different reasons an
	// item is not orderable, and a customer is owed the right one.
	Active        bool
	Availability  Availability
	VariantGroups []VariantGroup
	AddOnGroups   []AddOnGroup
	SortOrder     int
}

// ItemDraft is everything an owner supplies about an item.
type ItemDraft struct {
	CategoryID  string
	Name        string
	Description string
	ImageURL    string
	Price       money.Money
	Attributes  Attributes
	// Stock is applied only for shop types that count it.
	Stock     Stock
	SortOrder int
}

// NewItem validates a draft into an item.
//
// The per-type rules are enforced here, once, against the Capabilities table.
// Doing it at the edge of the domain rather than in each use case means an item
// that exists is an item whose shape is already right, and no consumer has to
// ask whether a grocery item might have add-ons.
func NewItem(id, merchantID string, t MerchantType, draft ItemDraft) (Item, error) {
	if strings.TrimSpace(merchantID) == "" {
		return Item{}, ErrEmptyMerchant
	}
	kind, err := ParseMerchantType(t.String())
	if err != nil {
		return Item{}, err
	}
	if strings.TrimSpace(draft.CategoryID) == "" {
		return Item{}, ErrNoCategory
	}

	name, err := trimmedName(draft.Name)
	if err != nil {
		return Item{}, err
	}

	description := strings.TrimSpace(draft.Description)
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		return Item{}, fmt.Errorf("%w: limit %d characters", ErrTooLong, maxDescriptionLength)
	}

	image, err := normaliseImageURL(draft.ImageURL)
	if err != nil {
		return Item{}, err
	}

	if _, err := draft.Price.MustBeNonNegative(); err != nil {
		return Item{}, fmt.Errorf("%w: %s", ErrNegativePrice, draft.Price.Display())
	}

	attributes, err := validateAttributes(kind, draft.Attributes)
	if err != nil {
		return Item{}, err
	}

	stock, err := validateStock(kind, draft.Stock)
	if err != nil {
		return Item{}, err
	}

	sortOrder := draft.SortOrder
	if sortOrder < 0 {
		sortOrder = 0
	}

	return Item{
		ID: id, MerchantID: merchantID, MerchantType: kind,
		CategoryID:   strings.TrimSpace(draft.CategoryID),
		Name:         name,
		Description:  description,
		ImageURL:     image,
		Price:        draft.Price,
		Attributes:   attributes,
		Stock:        stock,
		Active:       true,
		Availability: AlwaysAvailable(),
		SortOrder:    sortOrder,
	}, nil
}

// validateAttributes keeps an item's shape honest for its shop type.
func validateAttributes(t MerchantType, a Attributes) (Attributes, error) {
	capabilities := CapabilitiesFor(t)
	out := Attributes{}

	if capabilities.RequiresUnit {
		unit, err := ParseUnit(a.Unit.String())
		if err != nil {
			return Attributes{}, ErrUnitRequired
		}
		out.Unit = unit

		packSize, err := shortText(a.PackSize)
		if err != nil {
			return Attributes{}, err
		}
		brand, err := shortText(a.Brand)
		if err != nil {
			return Attributes{}, err
		}
		out.PackSize, out.Brand = packSize, brand
	} else if a.Unit != "" {
		// Refused rather than dropped: an owner who typed a unit expects to see
		// it, and silently discarding it is how "the app lost my changes"
		// tickets start.
		return Attributes{}, fmt.Errorf("%w: %q", ErrUnitNotAllowed, a.Unit)
	}

	if capabilities.Prescriptions {
		generic, err := shortText(a.GenericName)
		if err != nil {
			return Attributes{}, err
		}
		strength, err := shortText(a.Strength)
		if err != nil {
			return Attributes{}, err
		}
		out.GenericName, out.Strength = generic, strength
		out.RequiresPrescription = a.RequiresPrescription
	} else if a.RequiresPrescription {
		return Attributes{}, ErrPrescriptionNotAllowed
	}

	if t == Restaurant {
		out.IsVegetarian = a.IsVegetarian
		if a.PreparationMinutes > 0 {
			out.PreparationMinutes = a.PreparationMinutes
		}
	}
	return out, nil
}

// validateStock applies the shop type's stock policy.
func validateStock(t MerchantType, s Stock) (Stock, error) {
	if !CapabilitiesFor(t).TracksStock {
		if s.Tracked {
			return Stock{}, ErrStockNotTracked
		}
		return Untracked(), nil
	}
	if !s.Tracked {
		// A counted shop with no count given starts at zero — out of stock —
		// rather than unlimited. Guessing wrong in the other direction sells
		// something the shop does not have.
		return NewStock(0)
	}
	return NewStock(s.Quantity)
}

// WithStock returns a copy at a new stock level.
func (i Item) WithStock(s Stock) (Item, error) {
	stock, err := validateStock(i.MerchantType, s)
	if err != nil {
		return Item{}, err
	}
	i.Stock = stock
	return i, nil
}

// WithActive returns a copy shown or hidden.
func (i Item) WithActive(active bool) Item {
	i.Active = active
	return i
}

// WithAvailability returns a copy on a new schedule.
func (i Item) WithAvailability(a Availability) Item {
	i.Availability = a
	return i
}

// WithPrice returns a copy at a new price.
func (i Item) WithPrice(price money.Money) (Item, error) {
	if _, err := price.MustBeNonNegative(); err != nil {
		return Item{}, fmt.Errorf("%w: %s", ErrNegativePrice, price.Display())
	}
	i.Price = price
	return i, nil
}

// WithVariantGroups returns a copy with new variant groups.
func (i Item) WithVariantGroups(groups []VariantGroup) (Item, error) {
	if len(groups) > 0 && !CapabilitiesFor(i.MerchantType).Variants {
		return Item{}, ErrVariantsNotAllowed
	}
	i.VariantGroups = groups
	return i, nil
}

// WithAddOnGroups returns a copy with new add-on groups.
func (i Item) WithAddOnGroups(groups []AddOnGroup) (Item, error) {
	if len(groups) > 0 && !CapabilitiesFor(i.MerchantType).AddOns {
		return Item{}, ErrAddOnsNotAllowed
	}
	i.AddOnGroups = groups
	return i, nil
}

// Orderable reports whether a customer may add this item to a cart now.
//
// The three reasons it might not be are kept apart on purpose — see
// UnavailableReason — because "sold out", "not on the menu right now" and
// "the shop took it down" are different things to tell a customer.
func (i Item) Orderable(now time.Time) bool {
	return i.Active && i.Stock.InStock() && i.Availability.AvailableAt(now)
}

// UnavailableReason explains why an item cannot be ordered, as a code the
// client renders. Empty when it can be.
func (i Item) UnavailableReason(now time.Time) string {
	switch {
	case !i.Active:
		return "unavailable"
	case !i.Stock.InStock():
		return "out_of_stock"
	case !i.Availability.AvailableAt(now):
		return "not_available_now"
	default:
		return ""
	}
}
