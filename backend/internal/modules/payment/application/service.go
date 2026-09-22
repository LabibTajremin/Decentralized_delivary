package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
)

// Service implements contract.PaymentContract.
//
// A thin adapter over the use cases, and the only thing another module ever
// imports of this one (2.6).
type Service struct {
	repo        ports.Repository
	collections *CollectionUseCase
	refunds     *RefundUseCase
}

// NewService wires the public service.
func NewService(repo ports.Repository, collections *CollectionUseCase, refunds *RefundUseCase) *Service {
	return &Service{repo: repo, collections: collections, refunds: refunds}
}

// PaymentFor returns the most recent gateway attempt for an order.
func (s *Service) PaymentFor(ctx context.Context, orderID string) (contract.Payment, bool, error) {
	payment, found, err := s.repo.LatestForOrder(ctx, orderID)
	if err != nil {
		return contract.Payment{}, false, storageError(err)
	}
	if !found {
		return contract.Payment{}, false, nil
	}
	return toContract(payment), true, nil
}

// Refund gives a captured payment back.
func (s *Service) Refund(ctx context.Context, orderID, reason string) error {
	return s.refunds.Refund(ctx, orderID, reason)
}

// RecordCashCollection is order's delivery hook.
func (s *Service) RecordCashCollection(ctx context.Context, orderID, partnerID string, amountMinor int64) error {
	return s.collections.Record(ctx, orderID, partnerID, amountMinor)
}

// toContract restates a payment for another module.
func toContract(p domain.Payment) contract.Payment {
	return contract.Payment{
		ID: p.ID, OrderID: p.OrderID, Status: string(p.Status),
		Amount: contract.Money{
			Minor: p.Amount.Minor(), Currency: string(p.Amount.Currency()),
			Display: p.Amount.Display(),
		},
		Reason: settledReason(p),
	}
}
