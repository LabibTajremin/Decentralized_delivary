// Package application holds the catalogue use cases.
package application

import (
	"context"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	merchantext "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/external/merchant"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// shop loads the shop a catalogue belongs to and returns its type.
//
// Every write goes through here, because the shop's type decides the shape of
// everything in its catalogue (domain.Capabilities) and an item built against
// the wrong type is an item with fields that mean nothing.
func shop(ctx context.Context, service merchantext.Service, merchantID string) (merchantext.Merchant, domain.MerchantType, error) {
	found, err := service.Merchant(ctx, merchantID)
	if err != nil {
		// Merchant already distinguishes "no such shop" from "we could not
		// look" and words both for a person. Passing the error through keeps
		// one wording rather than inventing a second here.
		return merchantext.Merchant{}, "", err
	}
	kind, err := domain.ParseMerchantType(string(found.Type))
	if err != nil {
		// A shop whose type we cannot read is a schema drift between two
		// modules, not something an owner can fix. Reported rather than
		// defaulted, because defaulting would silently give a pharmacy a
		// restaurant's rules.
		return merchantext.Merchant{}, "", errs.Wrap(err, errs.KindInternal, "unknown_merchant_type",
			"We could not read that shop's details.")
	}
	return found, kind, nil
}

// ownedBy checks that the caller owns the shop they are editing.
//
// Ownership is checked against the merchant record rather than trusted from the
// request: the catalogue is the merchant app's main screen, and an endpoint that
// took a merchant id on trust would let any signed-in account rewrite any menu
// in the country.
func ownedBy(found merchantext.Merchant, ownerUserID string) error {
	if found.OwnerUserID != ownerUserID {
		return errs.New(errs.KindForbidden, "not_your_shop",
			"You can only change your own shop's menu.")
	}
	return nil
}

// unavailable wraps a dependency failure.
func unavailable(err error, message string) error {
	return errs.Wrap(err, errs.KindUnavailable, "catalogue_unavailable", message)
}

// notFound turns a repository miss into a client-facing refusal.
func notFound(err error, code, message, field, id string) error {
	return errs.Wrap(err, errs.KindNotFound, code, message).With(field, id)
}

// entryError turns a domain refusal into something an owner can act on.
//
// The per-type refusals are the interesting ones: an owner who added add-ons to
// a grocery item has misunderstood the product, and "some of those details are
// not valid" would leave them guessing which.
func entryError(err error) error {
	switch {
	case errors.Is(err, domain.ErrEmptyName):
		return errs.Wrap(err, errs.KindInvalid, "name_required", "Please enter a name.")
	case errors.Is(err, domain.ErrNameTooLong), errors.Is(err, domain.ErrTooLong):
		return errs.Wrap(err, errs.KindInvalid, "text_too_long", "Some of that text is too long.")
	case errors.Is(err, domain.ErrNoCategory):
		return errs.Wrap(err, errs.KindInvalid, "category_required", "Please choose a category.")
	case errors.Is(err, domain.ErrNegativePrice):
		return errs.Wrap(err, errs.KindInvalid, "invalid_price", "A price cannot be negative.")
	case errors.Is(err, domain.ErrInvalidImageURL):
		return errs.Wrap(err, errs.KindInvalid, "invalid_image_url", "That image address is not valid.")

	// The per-shop-type rules.
	case errors.Is(err, domain.ErrAddOnsNotAllowed):
		return errs.Wrap(err, errs.KindInvalid, "addons_not_allowed",
			"Add-ons are for restaurant items.")
	// ErrVariantsNotAllowed has no case: every shop type has variants, so
	// nothing an owner can send produces it. The domain still raises it for a
	// hand-built item, which would land on the fallback below. A shop type
	// without variants would need a case here and a check in
	// OptionUseCase.SetVariantGroups — see the comment there.
	case errors.Is(err, domain.ErrCombosNotAllowed):
		return errs.Wrap(err, errs.KindInvalid, "combos_not_allowed",
			"This kind of shop does not offer combos.")
	case errors.Is(err, domain.ErrUnitRequired), errors.Is(err, domain.ErrUnknownUnit):
		return errs.Wrap(err, errs.KindInvalid, "unit_required",
			"Please say how this is sold — per kg, per piece, and so on.")
	case errors.Is(err, domain.ErrUnitNotAllowed):
		return errs.Wrap(err, errs.KindInvalid, "unit_not_allowed",
			"A restaurant item is not sold by unit.")
	case errors.Is(err, domain.ErrPrescriptionNotAllowed):
		return errs.Wrap(err, errs.KindInvalid, "prescription_not_allowed",
			"Only a pharmacy can mark an item as needing a prescription.")
	case errors.Is(err, domain.ErrStockNotTracked):
		return errs.Wrap(err, errs.KindInvalid, "stock_not_tracked",
			"This kind of shop does not count stock.")
	case errors.Is(err, domain.ErrNegativeStock):
		return errs.Wrap(err, errs.KindInvalid, "invalid_stock", "A stock count cannot be negative.")

	// Option groups.
	case errors.Is(err, domain.ErrNoOptions):
		return errs.Wrap(err, errs.KindInvalid, "options_required",
			"Please add at least one option to that group.")
	case errors.Is(err, domain.ErrTooManyOptions):
		return errs.Wrap(err, errs.KindInvalid, "too_many_options", "That group has too many options.")
	case errors.Is(err, domain.ErrDuplicateOption):
		return errs.Wrap(err, errs.KindInvalid, "duplicate_option",
			"Two options in that group have the same name.")
	case errors.Is(err, domain.ErrInvalidChoiceRange):
		return errs.Wrap(err, errs.KindInvalid, "invalid_choice_range",
			"Those choice limits do not make sense.")

	// Combos.
	case errors.Is(err, domain.ErrEmptyCombo):
		return errs.Wrap(err, errs.KindInvalid, "combo_too_small", "A combo needs at least two items.")
	case errors.Is(err, domain.ErrComboTooLarge):
		return errs.Wrap(err, errs.KindInvalid, "combo_too_large", "That combo has too many items.")
	case errors.Is(err, domain.ErrDuplicateComboLine):
		return errs.Wrap(err, errs.KindInvalid, "duplicate_combo_item",
			"That item is in the combo twice. Use the quantity instead.")
	case errors.Is(err, domain.ErrInvalidQuantity):
		return errs.Wrap(err, errs.KindInvalid, "invalid_quantity", "That quantity is not valid.")

	default:
		return errs.Wrap(err, errs.KindInvalid, "invalid_entry", "Some of those details are not valid.")
	}
}
