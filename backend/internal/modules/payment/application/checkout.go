package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// CheckoutUseCase starts collecting an order's total through a gateway.
type CheckoutUseCase struct {
	repo    ports.Repository
	gateway ports.Gateway
	order   orderx.Service
	clock   clock.Clock
	ids     id.Generator
}

// NewCheckoutUseCase wires the use case.
func NewCheckoutUseCase(repo ports.Repository, gateway ports.Gateway, order orderx.Service, c clock.Clock, ids id.Generator) *CheckoutUseCase {
	return &CheckoutUseCase{repo: repo, gateway: gateway, order: order, clock: c, ids: ids}
}

// Start begins, or resumes, paying for an order online.
//
// Idempotent on the order id in the same sense dispatch's Offer is: a customer
// who taps "pay" twice on a slow connection gets the same checkout back rather
// than a second attempt racing the first. A previous attempt that already
// failed does not block a new one — that is what a retry is.
func (uc *CheckoutUseCase) Start(ctx context.Context, customerID, orderID, lang string) (CheckoutView, error) {
	order, err := uc.order.Order(ctx, orderID)
	if err != nil {
		return CheckoutView{}, notFoundOr(err)
	}
	if order.CustomerID != customerID {
		return CheckoutView{}, notFound()
	}
	if order.PaymentMethod != "online" {
		return CheckoutView{}, errs.New(errs.KindConflict, "not_online_payment",
			"That order is not being paid for online.")
	}
	if order.Status != "pending_payment" {
		return CheckoutView{}, errs.New(errs.KindConflict, "not_awaiting_payment",
			"That order is not waiting for payment.")
	}

	if existing, found, err := uc.repo.PendingForOrder(ctx, orderID); err != nil {
		return CheckoutView{}, storageError(err)
	} else if found {
		return uc.viewOf(existing, lang), nil
	}

	now := uc.clock.Now()
	amount := money.Taka(order.TotalMinor)
	payment, err := domain.NewPayment(uc.ids.New("PAY"), orderID, customerID, uc.gateway.Name(), amount, now)
	if err != nil {
		return CheckoutView{}, paymentError(err)
	}

	result, err := uc.gateway.Checkout(ctx, payment.Reference, amount.Minor(), string(amount.Currency()))
	if err != nil {
		return CheckoutView{}, gatewayError(err)
	}
	payment.GatewayRef = result.GatewayRef

	if err := uc.repo.CreatePayment(ctx, payment); err != nil {
		// The unique guard on a pending attempt per order caught a race with
		// another request for the same checkout. The one that won is the
		// answer; this one never existed.
		if errs.CodeOf(err) == "checkout_in_progress" {
			if won, found, readErr := uc.repo.PendingForOrder(ctx, orderID); readErr == nil && found {
				return uc.viewOf(won, lang), nil
			}
		}
		return CheckoutView{}, storageError(err)
	}

	return checkoutViewOf(payment, result.RedirectURL, lang), nil
}
