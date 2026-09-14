package domain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Errors returned when building variant and add-on groups.
var (
	// ErrVariantsNotAllowed means this kind of shop does not use variants.
	ErrVariantsNotAllowed = errors.New("this kind of shop does not use variants")
	// ErrNoOptions means a group of choices contains none.
	ErrNoOptions = errors.New("a group of choices needs at least one option")
	// ErrTooManyOptions means a group holds more options than we allow.
	ErrTooManyOptions = errors.New("too many options in one group")
	// ErrInvalidChoiceRange means the min/max choices do not make sense.
	ErrInvalidChoiceRange = errors.New("those choice limits do not make sense")
	// ErrDuplicateOption means two options in one group share a name.
	ErrDuplicateOption = errors.New("two options in that group have the same name")
)

// maxOptionsPerGroup caps a group of choices.
//
// Twenty is well past any real menu — a pizza has three sizes, not twenty — and
// low enough that the group still renders on a low-end phone without scrolling
// becoming the interaction.
const maxOptionsPerGroup = 20

// VariantOption is one mutually-exclusive choice: a size, a strength, a pack.
type VariantOption struct {
	ID   string
	Name string
	// PriceDelta adjusts the item's base price. Signed, because a small size
	// legitimately costs less, and modelling that as a separate item would
	// double every menu.
	PriceDelta money.Money
	Available  bool
}

// NewVariantOption validates a choice.
func NewVariantOption(id, name string, delta money.Money) (VariantOption, error) {
	trimmed, err := trimmedName(name)
	if err != nil {
		return VariantOption{}, err
	}
	return VariantOption{ID: id, Name: trimmed, PriceDelta: delta, Available: true}, nil
}

// VariantGroup is a set of mutually-exclusive choices.
type VariantGroup struct {
	ID   string
	Name string
	// Required means the customer must choose before adding to a cart. A pizza
	// has no price until a size is picked, so leaving it optional would mean
	// charging for something nobody chose.
	Required bool
	// MinChoices and MaxChoices bound the selection. A plain single-choice
	// group is 1..1; "pick up to two sauces" is 0..2.
	MinChoices int
	MaxChoices int
	Options    []VariantOption
	SortOrder  int
}

// NewVariantGroup validates a group of choices.
func NewVariantGroup(id, name string, required bool, minChoices, maxChoices int, options []VariantOption) (VariantGroup, error) {
	trimmed, err := trimmedName(name)
	if err != nil {
		return VariantGroup{}, err
	}
	if err := checkOptionNames(len(options), optionNames(options)); err != nil {
		return VariantGroup{}, err
	}

	if maxChoices <= 0 {
		maxChoices = 1
	}
	if required && minChoices < 1 {
		minChoices = 1
	}
	if minChoices < 0 {
		minChoices = 0
	}
	if minChoices > maxChoices || maxChoices > len(options) {
		return VariantGroup{}, fmt.Errorf("%w: %d to %d of %d options",
			ErrInvalidChoiceRange, minChoices, maxChoices, len(options))
	}

	return VariantGroup{
		ID: id, Name: trimmed, Required: required,
		MinChoices: minChoices, MaxChoices: maxChoices, Options: options,
	}, nil
}

// AddOn is an extra bought alongside an item.
type AddOn struct {
	ID   string
	Name string
	// Price is what the add-on costs, not a delta: an extra is a thing with a
	// price, and expressing "extra cheese, ৳30" as an adjustment to the pizza's
	// price makes a receipt impossible to read.
	Price     money.Money
	Available bool
}

// NewAddOn validates an extra.
func NewAddOn(id, name string, price money.Money) (AddOn, error) {
	trimmed, err := trimmedName(name)
	if err != nil {
		return AddOn{}, err
	}
	if _, err := price.MustBeNonNegative(); err != nil {
		return AddOn{}, fmt.Errorf("%w: %s", ErrNegativePrice, price.Display())
	}
	return AddOn{ID: id, Name: trimmed, Price: price, Available: true}, nil
}

// AddOnGroup is a set of extras offered together.
type AddOnGroup struct {
	ID         string
	Name       string
	MinChoices int
	MaxChoices int
	Options    []AddOn
	SortOrder  int
}

// NewAddOnGroup validates a group of extras.
func NewAddOnGroup(id, name string, minChoices, maxChoices int, options []AddOn) (AddOnGroup, error) {
	trimmed, err := trimmedName(name)
	if err != nil {
		return AddOnGroup{}, err
	}
	names := make([]string, 0, len(options))
	for _, option := range options {
		names = append(names, option.Name)
	}
	if err := checkOptionNames(len(options), names); err != nil {
		return AddOnGroup{}, err
	}

	if maxChoices <= 0 || maxChoices > len(options) {
		maxChoices = len(options)
	}
	if minChoices < 0 {
		minChoices = 0
	}
	if minChoices > maxChoices {
		return AddOnGroup{}, fmt.Errorf("%w: %d to %d of %d options",
			ErrInvalidChoiceRange, minChoices, maxChoices, len(options))
	}

	return AddOnGroup{
		ID: id, Name: trimmed,
		MinChoices: minChoices, MaxChoices: maxChoices, Options: options,
	}, nil
}

// optionNames lists a variant group's option names.
func optionNames(options []VariantOption) []string {
	names := make([]string, 0, len(options))
	for _, option := range options {
		names = append(names, option.Name)
	}
	return names
}

// checkOptionNames applies the rules every group of choices shares.
//
// Duplicate names are refused because the name is what a customer picks by. Two
// options called "Large" on one pizza is a support call whichever one the
// kitchen makes.
func checkOptionNames(count int, names []string) error {
	if count == 0 {
		return ErrNoOptions
	}
	if count > maxOptionsPerGroup {
		return fmt.Errorf("%w: %d, limit %d", ErrTooManyOptions, count, maxOptionsPerGroup)
	}
	seen := make(map[string]bool, count)
	for _, name := range names {
		key := strings.ToLower(name)
		if seen[key] {
			return fmt.Errorf("%w: %q", ErrDuplicateOption, name)
		}
		seen[key] = true
	}
	return nil
}
