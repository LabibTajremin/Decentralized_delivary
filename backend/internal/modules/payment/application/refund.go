package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
)

// RefundUseCase gives captured money back through the gateway that took it.
//
// The primitive P16's refund workflow calls through PaymentContract: a support
// agent deciding a refund is owed is a decision this module has no opinion on,
// and does not need to — it only has to make "give this money back" a call
// that either truly happens or fails loudly.
type RefundUseCase struct {
	repo    ports.Repository
	gateway ports.Gateway
	clock   clock.Clock
}

// NewRefundUseCase wires the use case.
func NewRefundUseCase(repo ports.Repository, gateway ports.Gateway, c clock.Clock) *RefundUseCase {
	return &RefundUseCase{repo: repo, gateway: gateway, clock: c}
}

// Refund reverses the captured payment for an order.
//
// The reason is checked before the gateway is ever called: asking a gateway to
// move money and only then discovering the request cannot be recorded would
// leave money moved with nothing to show for it.
//
// Idempotent: a payment already refunded returns success without asking the
// gateway again, because asking a gateway to refund the same transaction twice
// is the kind of thing that either errors confusingly or, worse, does not.
func (uc *RefundUseCase) Refund(ctx context.Context, orderID, reason string) error {
	if reason == "" {
		return paymentError(domain.ErrNoReason)
	}

	payment, found, err := uc.repo.LatestForOrder(ctx, orderID)
	if err != nil {
		return storageError(err)
	}
	if !found {
		return notFound()
	}
	if payment.Status == domain.StatusRefunded {
		return nil
	}
	expected := payment.Status
	if err := payment.Refund(reason, uc.clock.Now()); err != nil {
		return paymentError(err)
	}

	if _, err := uc.gateway.Refund(ctx, payment.GatewayRef, payment.Amount.Minor()); err != nil {
		return gatewayError(err)
	}

	if err := uc.repo.Save(ctx, payment, expected); err != nil {
		return storageError(err)
	}
	return nil
}
