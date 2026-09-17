package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application/ports"
)

// ReadUseCase answers "what is the state of this order's payment" — the poll a
// customer's app makes after a redirect, and what support and admin tooling
// look at.
type ReadUseCase struct {
	repo ports.Repository
}

// NewReadUseCase wires the use case.
func NewReadUseCase(repo ports.Repository) *ReadUseCase {
	return &ReadUseCase{repo: repo}
}

// ForOrder returns the most recent payment attempt for an order, scoped to
// whoever is allowed to see it.
//
// A customer sees only their own order's payment — asking about another
// order's is answered the same as one that does not exist, exactly like every
// other caller-scoped read in this system (order/application/transitions.go
// mayTouch). An admin sees any order's.
func (uc *ReadUseCase) ForOrder(ctx context.Context, orderID, callerCustomerID string, admin bool, lang string) (PaymentView, error) {
	payment, found, err := uc.repo.LatestForOrder(ctx, orderID)
	if err != nil {
		return PaymentView{}, storageError(err)
	}
	if !found {
		return PaymentView{}, notFound()
	}
	if !admin && payment.CustomerID != callerCustomerID {
		return PaymentView{}, notFound()
	}
	return paymentViewOf(payment, lang), nil
}
