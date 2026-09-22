package application

import (
	"context"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// WebhookUseCase applies a gateway's callback to a payment.
type WebhookUseCase struct {
	repo    ports.Repository
	gateway ports.Gateway
	order   orderx.Service
	clock   clock.Clock
}

// NewWebhookUseCase wires the use case.
func NewWebhookUseCase(repo ports.Repository, gateway ports.Gateway, order orderx.Service, c clock.Clock) *WebhookUseCase {
	return &WebhookUseCase{repo: repo, gateway: gateway, order: order, clock: c}
}

// Handle applies one gateway callback.
//
// Idempotent by construction rather than by a deduplication table: a
// gateway's own retry policy guarantees at-least-once delivery, never
// exactly-once, and a payment moves out of pending exactly once no matter how
// many times the same event arrives, because Capture and Fail both refuse to
// run from anywhere but pending. A replay after the first success is not an
// error — it is answered the same way the first one was, with nothing written
// twice.
func (uc *WebhookUseCase) Handle(ctx context.Context, payload []byte, signature string) error {
	event, err := uc.gateway.VerifyWebhook(ctx, payload, signature)
	if err != nil {
		return errs.Wrap(err, errs.KindInvalid, "invalid_webhook",
			"That webhook could not be verified.")
	}
	return uc.apply(ctx, event)
}

// Simulate applies an outcome directly, without a signed callback — the
// manual gateway's stand-in for "the bank confirmed", used by the dev-only
// completion route and nowhere else.
//
// Restricted three ways, each of which would be enough on its own and none of
// which is trusted to be:
//
//   - to the manual gateway, by name. A route omitted from production wiring
//     is one omission away from a way to mark any order paid for free.
//   - to a deployment that mounts the route at all, which production does not.
//   - to the customer whose payment it is. That is the one added here when the
//     customer app began calling this route on a demo: before, any signed-in
//     caller could settle any payment they knew the id of, which was
//     tolerable while only a developer with curl could reach it and is not
//     once it is a button in a shipped app.
//
// An id belonging to somebody else answers not-found, the same as an id that
// never existed — the same choice the rest of this API makes.
func (uc *WebhookUseCase) Simulate(ctx context.Context, paymentID, customerID string, succeeded bool, reason string) error {
	if uc.gateway.Name() != ManualGateway {
		return errs.New(errs.KindForbidden, "dev_only",
			"This action is only available with the manual gateway.")
	}
	payment, err := uc.repo.Payment(ctx, paymentID)
	if err != nil {
		return notFoundOr(err)
	}
	if payment.CustomerID != customerID {
		return notFound()
	}
	return uc.apply(ctx, ports.WebhookEvent{
		Reference: payment.Reference, GatewayRef: payment.Reference,
		Succeeded: succeeded, Reason: reason, AmountMinor: payment.Amount.Minor(),
	})
}

// apply is the one place a gateway's word on a payment is turned into a
// state change — whether that word arrived as a verified webhook or, in
// development, as Simulate standing in for one.
func (uc *WebhookUseCase) apply(ctx context.Context, event ports.WebhookEvent) error {
	if !event.Succeeded && event.Reason == "" {
		return errs.New(errs.KindInvalid, "reason_required",
			"A failed payment needs a reason.")
	}

	payment, err := uc.repo.ByReference(ctx, event.Reference)
	if err != nil {
		if errs.Is(err, errs.KindNotFound) {
			// A callback for a reference we never minted is not this module's
			// mistake to fix by writing something — it is refused, loudly, so
			// whatever produced it can be investigated.
			return errs.New(errs.KindNotFound, "payment_not_found",
				"That reference does not match a payment we started.")
		}
		return storageError(err)
	}

	now := uc.clock.Now()
	expected := payment.Status
	if event.Succeeded {
		if err := payment.Capture(event.GatewayRef, money.Taka(event.AmountMinor), now); err != nil {
			if errors.Is(err, domain.ErrAmountMismatch) {
				return paymentError(err)
			}
			// Anything else means this payment already left pending — a
			// replay of an event already applied, or a late arrival after a
			// different one won. Either way it is answered as success, with
			// nothing written twice.
			return nil
		}
	} else {
		if err := payment.Fail(event.Reason, now); err != nil {
			return nil
		}
	}

	if err := uc.repo.Save(ctx, payment, expected); err != nil {
		if errs.CodeOf(err) == "payment_moved" {
			// Somebody else's webhook won the race. Its outcome stands.
			return nil
		}
		return storageError(err)
	}

	if event.Succeeded {
		if err := uc.order.MarkPaid(ctx, payment.OrderID); err != nil {
			return err
		}
		return nil
	}
	return uc.order.MarkPaymentFailed(ctx, payment.OrderID, event.Reason)
}
