package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Errors returned when building a combo.
var (
	// ErrCombosNotAllowed means this kind of shop does not run combos.
	ErrCombosNotAllowed = errors.New("this kind of shop does not offer combos")
	// ErrEmptyCombo means the bundle contains nothing.
	ErrEmptyCombo = errors.New("a combo needs at least two items")
	// ErrComboTooLarge means the bundle holds more lines than we allow.
	ErrComboTooLarge = errors.New("that combo has too many items")
	// ErrInvalidQuantity means a line quantity is not a count.
	ErrInvalidQuantity = errors.New("that quantity is not valid")
	// ErrDuplicateComboLine means one item appears twice in a bundle.
	ErrDuplicateComboLine = errors.New("that item is in the combo twice")
)

const (
	// minComboLines is two, because a "combo" of one item is a price change.
	minComboLines = 2
	// maxComboLines caps a bundle at something a person can read on a phone.
	maxComboLines = 20
	// maxComboQuantity caps one line. Ten of anything in a meal deal is already
	// a catering order, which is a different product.
	maxComboQuantity = 10
)

// ComboLine is one item in a bundle.
type ComboLine struct {
	ItemID   string
	Quantity int
}

// Combo is several items sold together at a set price.
//
// The price is stated, not derived from the members. A combo exists to be
// cheaper than its parts, and computing it from them would mean the discount
// moved every time one member's price changed — including upward.
type Combo struct {
	ID           string
	MerchantID   string
	MerchantType MerchantType
	Name         string
	Description  string
	ImageURL     string
	Price        money.Money
	Lines        []ComboLine
	Active       bool
	Availability Availability
	SortOrder    int
}

// ComboDraft is everything an owner supplies about a combo.
type ComboDraft struct {
	Name        string
	Description string
	ImageURL    string
	Price       money.Money
	Lines       []ComboLine
	SortOrder   int
}

// NewCombo validates a draft into a combo.
//
// It does not check that the member items exist — that needs the repository, so
// the use case does it. What is checked here is everything decidable from the
// draft alone, which is most of it.
func NewCombo(id, merchantID string, t MerchantType, draft ComboDraft) (Combo, error) {
	if strings.TrimSpace(merchantID) == "" {
		return Combo{}, ErrEmptyMerchant
	}
	kind, err := ParseMerchantType(t.String())
	if err != nil {
		return Combo{}, err
	}
	if !CapabilitiesFor(kind).Combos {
		return Combo{}, ErrCombosNotAllowed
	}

	name, err := trimmedName(draft.Name)
	if err != nil {
		return Combo{}, err
	}

	description := strings.TrimSpace(draft.Description)
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		return Combo{}, fmt.Errorf("%w: limit %d characters", ErrTooLong, maxDescriptionLength)
	}

	image, err := normaliseImageURL(draft.ImageURL)
	if err != nil {
		return Combo{}, err
	}

	if _, err := draft.Price.MustBeNonNegative(); err != nil {
		return Combo{}, fmt.Errorf("%w: %s", ErrNegativePrice, draft.Price.Display())
	}

	lines, err := validateComboLines(draft.Lines)
	if err != nil {
		return Combo{}, err
	}

	sortOrder := draft.SortOrder
	if sortOrder < 0 {
		sortOrder = 0
	}

	return Combo{
		ID: id, MerchantID: merchantID, MerchantType: kind,
		Name: name, Description: description, ImageURL: image,
		Price: draft.Price, Lines: lines, Active: true,
		Availability: AlwaysAvailable(), SortOrder: sortOrder,
	}, nil
}

// validateComboLines checks the bundle's membership.
func validateComboLines(lines []ComboLine) ([]ComboLine, error) {
	if len(lines) < minComboLines {
		return nil, ErrEmptyCombo
	}
	if len(lines) > maxComboLines {
		return nil, fmt.Errorf("%w: %d, limit %d", ErrComboTooLarge, len(lines), maxComboLines)
	}

	seen := make(map[string]bool, len(lines))
	out := make([]ComboLine, 0, len(lines))
	for _, line := range lines {
		itemID := strings.TrimSpace(line.ItemID)
		if itemID == "" {
			return nil, ErrItemNotFound
		}
		// Two lines for the same item would be ambiguous against a quantity:
		// the owner means "two of these", and the way to say that is the
		// quantity field.
		if seen[itemID] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateComboLine, itemID)
		}
		seen[itemID] = true

		if line.Quantity < 1 || line.Quantity > maxComboQuantity {
			return nil, fmt.Errorf("%w: %d for item %s", ErrInvalidQuantity, line.Quantity, itemID)
		}
		out = append(out, ComboLine{ItemID: itemID, Quantity: line.Quantity})
	}
	return out, nil
}

// ItemIDs lists the items a combo is built from.
func (c Combo) ItemIDs() []string {
	out := make([]string, 0, len(c.Lines))
	for _, line := range c.Lines {
		out = append(out, line.ItemID)
	}
	return out
}

// WithActive returns a copy shown or hidden.
func (c Combo) WithActive(active bool) Combo {
	c.Active = active
	return c
}

// WithAvailability returns a copy on a new schedule.
func (c Combo) WithAvailability(a Availability) Combo {
	c.Availability = a
	return c
}

// Orderable reports whether a combo may be bought at an instant.
//
// Whether its member items are orderable is decided by the use case, which has
// them to hand. A combo whose members are sold out is not orderable however
// active it says it is, and deciding that here would need the domain to hold
// the whole item set.
func (c Combo) Orderable(now time.Time) bool {
	return c.Active && c.Availability.AvailableAt(now)
}
