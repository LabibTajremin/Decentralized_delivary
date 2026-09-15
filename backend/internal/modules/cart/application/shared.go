// Package application holds the cart's use cases: holding what a customer
// chose, and telling them the truth about it every time they look.
package application

import (
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// cartError turns a domain rule into a failure a customer can act on.
//
// The message is the point. "cart: ErrDifferentMerchant" is a log line; "Your
// cart has items from another shop" is something a person can do something
// about, and the app shows it verbatim (2.9).
func cartError(err error) error {
	switch {
	case errors.Is(err, domain.ErrDifferentMerchant):
		return errs.Wrap(err, errs.KindConflict, "different_merchant",
			"Your cart already has items from another shop. Empty it first to order from this one.")
	case errors.Is(err, domain.ErrQuantityRange):
		return errs.Wrap(err, errs.KindInvalid, "invalid_quantity",
			"You can order between one and twenty of an item at a time.")
	case errors.Is(err, domain.ErrTooManyLines):
		return errs.Wrap(err, errs.KindInvalid, "cart_full",
			"Your cart is full. Please order what is in it before adding more.")
	case errors.Is(err, domain.ErrNoSuchLine):
		return errs.Wrap(err, errs.KindNotFound, "line_not_found",
			"That item is no longer in your cart.")
	case errors.Is(err, domain.ErrNoTarget):
		return errs.Wrap(err, errs.KindInvalid, "invalid_line",
			"We could not tell what you were adding.")
	case errors.Is(err, domain.ErrNoteTooLong):
		return errs.Wrap(err, errs.KindInvalid, "note_too_long",
			"That note is too long.")
	default:
		// ErrNoUser and ErrNoMerchant land here, and so would a rule added
		// later without a sentence of its own. One message covers all of them
		// honestly: whatever went wrong, the cart could not be opened, and the
		// customer's next step is the same.
		return errs.Wrap(err, errs.KindInvalid, "invalid_cart",
			"We could not open a cart just now. Please try again.")
	}
}

// storageError reports a cart we could not read or write.
func storageError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "cart_unavailable",
		"We could not reach your cart just now. Please try again.")
}

// fromMinor rebuilds a money value from the contract's primitives.
//
// Only the integer is read. The display string that came with it is a
// rendering, and recomputing it from the integer here means every amount this
// module emits was formatted by the same code.
func fromMinor(minor int64) money.Money { return money.Taka(minor) }
