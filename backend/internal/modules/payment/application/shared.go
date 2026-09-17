// Package application holds payment's use cases: taking money through a
// gateway, recording cash a partner collects, and giving either back.
package application

import (
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// paymentError turns a domain rule about a gateway payment into a failure a
// caller can act on.
//
// ErrNotPending has no case here: every call site that could produce it —
// Capture and Fail inside webhook.go's apply — treats it as a payment that
// already moved rather than a fresh failure, and answers idempotently instead
// of reaching this function at all.
func paymentError(err error) error {
	switch {
	case errors.Is(err, domain.ErrAmountMismatch):
		return errs.Wrap(err, errs.KindConflict, "amount_mismatch",
			"The amount confirmed does not match what was asked for.")
	case errors.Is(err, domain.ErrNotCaptured):
		return errs.Wrap(err, errs.KindConflict, "not_captured",
			"Only a captured payment can be refunded.")
	case errors.Is(err, domain.ErrNoReason):
		return errs.Wrap(err, errs.KindInvalid, "reason_required",
			"Please say why.")
	default:
		return errs.Wrap(err, errs.KindInvalid, "invalid_payment",
			"We could not process that payment.")
	}
}

// collectionError turns a domain rule about a cash collection into a failure a
// caller can act on.
//
// ErrWrongPartner and ErrAlreadyRemitted have no case here: Reconcile enforces
// both through repo.Remit's own transaction rather than by calling
// Collection.Remit and mapping what it returns, so this function never sees
// them (application/collection.go).
func collectionError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNoRemittanceRef):
		return errs.Wrap(err, errs.KindInvalid, "reference_required",
			"A remittance needs a reference.")
	default:
		return errs.Wrap(err, errs.KindInvalid, "invalid_collection",
			"We could not update that collection.")
	}
}

// storageError reports a payment record we could not read or write.
func storageError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "payment_unavailable",
		"We could not reach payments just now. Please try again.")
}

// gatewayError reports a gateway we could not reach or that refused a call.
func gatewayError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "gateway_unavailable",
		"We could not reach the payment gateway just now. Please try again.")
}

// notFound is one payment that is not there.
func notFound() error {
	return errs.New(errs.KindNotFound, "payment_not_found", "We could not find that payment.")
}

// notFoundOr turns a missing record into a not-found and anything else into an
// outage.
func notFoundOr(err error) error {
	if errs.Is(err, errs.KindNotFound) {
		return notFound()
	}
	return storageError(err)
}

// settledReason is why a payment ended the way it did — a failure reason
// while failed, a refund reason once refunded, empty otherwise. One place to
// pick between the two so a view and a contract DTO cannot disagree about
// which reason a caller should see.
func settledReason(p domain.Payment) string {
	if p.Status == domain.StatusRefunded {
		return p.RefundReason
	}
	return p.FailureReason
}
