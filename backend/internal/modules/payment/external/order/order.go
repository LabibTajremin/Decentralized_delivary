// Package order is payment's view of the order module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5). Payment's isolation
// goes one step further (2.6): everything read out of ordercontract.Order here
// is copied into Order below, a type this package owns, rather than carried
// past this file as order's own contract type. The rest of the payment module
// never imports ordercontract — only this file does.
package order

import (
	"context"

	ordercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/order/contract"
)

// Order is an order as payment needs to see it: who is paying, how much, and
// which lifecycle state it is in.
type Order struct {
	ID            string
	CustomerID    string
	Status        string
	PaymentMethod string
	TotalMinor    int64
	Currency      string
}

// Service is the part of the order module payment depends on.
type Service interface {
	// Order returns one order, whatever its status.
	Order(ctx context.Context, orderID string) (Order, error)

	// MarkPaid moves a prepaid order from pending_payment to placed. Payment
	// calls this and nothing else does (order/contract/contract.go).
	MarkPaid(ctx context.Context, orderID string) error

	// MarkPaymentFailed cancels an order whose payment never arrived.
	MarkPaymentFailed(ctx context.Context, orderID, reason string) error
}

// service adapts ordercontract.OrderContract to Service.
type service struct {
	order ordercontract.OrderContract
}

// New wires the seam.
func New(o ordercontract.OrderContract) Service { return &service{order: o} }

func (s *service) Order(ctx context.Context, orderID string) (Order, error) {
	o, err := s.order.Order(ctx, orderID)
	if err != nil {
		return Order{}, err
	}
	return Order{
		ID: o.ID, CustomerID: o.CustomerID, Status: o.Status,
		PaymentMethod: o.PaymentMethod,
		TotalMinor:    o.Total.Minor, Currency: o.Total.Currency,
	}, nil
}

func (s *service) MarkPaid(ctx context.Context, orderID string) error {
	return s.order.MarkPaid(ctx, orderID)
}

func (s *service) MarkPaymentFailed(ctx context.Context, orderID, reason string) error {
	return s.order.MarkPaymentFailed(ctx, orderID, reason)
}
