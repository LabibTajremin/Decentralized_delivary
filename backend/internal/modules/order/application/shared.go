// Package application holds the order's use cases: turning a cart into an
// agreement, and moving that agreement through its life.
package application

import (
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// orderError turns a domain rule into a failure a customer can act on.
func orderError(err error) error {
	switch {
	case errors.Is(err, domain.ErrIllegalTransition):
		return errs.Wrap(err, errs.KindConflict, "illegal_transition",
			"That order cannot be changed that way.")
	case errors.Is(err, domain.ErrNotYourTransition):
		return errs.Wrap(err, errs.KindForbidden, "not_your_transition",
			"You cannot do that to this order.")
	case errors.Is(err, domain.ErrAlreadyFinished):
		return errs.Wrap(err, errs.KindConflict, "order_finished",
			"That order has already finished.")
	case errors.Is(err, domain.ErrCODLimit):
		return errs.Wrap(err, errs.KindConflict, "cod_limit_exceeded",
			"This order is too large to pay for in cash. Please pay online.")
	case errors.Is(err, domain.ErrUnknownPayment):
		return errs.Wrap(err, errs.KindInvalid, "invalid_payment_method",
			"Please choose how you would like to pay.")
	case errors.Is(err, domain.ErrNoLines):
		return errs.Wrap(err, errs.KindInvalid, "empty_order",
			"There is nothing in your cart to order.")
	case errors.Is(err, domain.ErrNoAddress):
		return errs.Wrap(err, errs.KindInvalid, "no_address",
			"Please choose a delivery address.")
	default:
		// Everything else is a draft that could not become an order —
		// ErrNoCustomer, ErrNoMerchant, ErrNoEventID, an amount below zero, or
		// a rule added later without a sentence here. One message covers all
		// of them honestly: whatever went wrong, the order was not placed.
		return errs.Wrap(err, errs.KindInvalid, "invalid_order",
			"We could not place that order. Please try again.")
	}
}

// storageError reports an order we could not read or write.
func storageError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "order_unavailable",
		"We could not reach your orders just now. Please try again.")
}

// notFound is one order that is not there — or not the caller's, which is the
// same answer on purpose.
//
// An endpoint that said "that order exists but is not yours" would let anybody
// with a list of ids find out which ones are real.
func notFound() error {
	return errs.New(errs.KindNotFound, "order_not_found", "We could not find that order.")
}

// taka rebuilds a money value from the contract's primitives.
func taka(minor int64) money.Money { return money.Taka(minor) }
