// Package ports declares what payment's use cases need from the outside
// world. No use case ever touches a database handle or a gateway SDK
// directly (05-architecture.md 2.3, 2.4).
package ports

import (
	"context"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
)

// PaymentRepository stores gateway payment attempts.
type PaymentRepository interface {
	// CreatePayment writes a new attempt. Refused with a conflict, translated
	// by the repository, when a pending attempt already exists for this
	// order; the caller re-reads it instead.
	CreatePayment(ctx context.Context, p domain.Payment) error

	// Payment returns one attempt by its own id.
	Payment(ctx context.Context, id string) (domain.Payment, error)

	// ByReference finds the attempt a webhook is answering, by the merchant
	// reference this module minted and handed to the gateway.
	ByReference(ctx context.Context, reference string) (domain.Payment, error)

	// PendingForOrder returns the open attempt for an order, if there is one —
	// what Checkout re-reads instead of starting a second one.
	PendingForOrder(ctx context.Context, orderID string) (domain.Payment, bool, error)

	// LatestForOrder returns the most recent attempt for an order, whatever
	// its state — a retried checkout after a failure is a second row, and this
	// is which one is authoritative to show.
	LatestForOrder(ctx context.Context, orderID string) (domain.Payment, bool, error)

	// Save writes a payment, checking it is still where the caller thought it
	// was. The other half of idempotent webhooks: a replayed event finds the
	// payment already moved and updates nothing.
	Save(ctx context.Context, p domain.Payment, expected domain.Status) error
}

// CollectionRepository stores cash-on-delivery collections.
type CollectionRepository interface {
	// CreateCollection records a rider taking cash for a delivered order.
	// Refused with a conflict when the order already has one — the hook that
	// calls this can be retried and must not double-count a delivery.
	CreateCollection(ctx context.Context, c domain.Collection) error

	// Collection returns one record by its own id.
	Collection(ctx context.Context, id string) (domain.Collection, error)

	// ForPartner lists a partner's collections, held first then remitted, for
	// the ledger.
	ForPartner(ctx context.Context, partnerID string) ([]domain.Collection, error)

	// Remit marks a batch of a partner's held collections as remitted, inside
	// one transaction: every id must belong to this partner and still be
	// held, or none of them move. Reconciling five collections and getting
	// three remitted is a ledger an operator can no longer trust, so this is
	// all-or-nothing rather than a loop of individual writes.
	//
	// A collection that fails the check is reported as a *errs.Error of
	// errs.KindConflict, distinguishable from an unexpected storage failure.
	Remit(ctx context.Context, partnerID string, collectionIDs []string, reference string, now time.Time) error
}

// Repository is the whole payment store.
type Repository interface {
	PaymentRepository
	CollectionRepository
}

// CheckoutResult is what starting a checkout gets back from the gateway.
type CheckoutResult struct {
	// GatewayRef is the gateway's own reference for this attempt, if it
	// assigns one immediately. Empty until the gateway says otherwise is
	// normal for some flows.
	GatewayRef string
	// RedirectURL is where the customer's app sends them to pay.
	RedirectURL string
}

// WebhookEvent is a gateway's callback, decoded and verified.
type WebhookEvent struct {
	// Reference is the merchant reference this module minted — how the event
	// is matched back to a Payment.
	Reference string
	// GatewayRef is the gateway's own transaction id.
	GatewayRef string
	// Succeeded is the outcome. A gateway that reports neither success nor a
	// reason for failure is not a gateway a caller can act on, so Reason is
	// required whenever Succeeded is false.
	Succeeded bool
	Reason    string
	// AmountMinor is what the gateway says it collected, checked against what
	// was asked for before anything is marked paid.
	AmountMinor int64
}

// RefundResult is what asking a gateway to give money back gets.
type RefundResult struct {
	RefundRef string
}

// Gateway is the swappable boundary to a payment provider (2.4).
//
// One adapter exists today — infrastructure/gateway/manual, a stand-in that
// refuses to run in production exactly as identity's LogSender does. A real
// provider implements the same three methods and nothing above this
// interface changes.
type Gateway interface {
	// Name identifies which gateway this is, stored on the Payment so a mixed
	// history — one provider retired, another taking over — stays readable.
	Name() string

	// Checkout starts collecting an amount, addressed by the merchant
	// reference this module minted.
	Checkout(ctx context.Context, reference string, amountMinor int64, currency string) (CheckoutResult, error)

	// VerifyWebhook checks a callback's signature and decodes it. An
	// unverifiable payload is refused before anything in it is trusted.
	VerifyWebhook(ctx context.Context, payload []byte, signature string) (WebhookEvent, error)

	// Refund gives captured money back through the gateway that took it.
	Refund(ctx context.Context, gatewayRef string, amountMinor int64) (RefundResult, error)
}
