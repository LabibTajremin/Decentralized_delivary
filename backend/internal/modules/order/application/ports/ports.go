// Package ports declares what the order use cases need from the outside world.
// No use case ever touches a database handle (05-architecture.md 2.3).
package ports

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
)

// Filter narrows a listing of orders.
type Filter struct {
	// CustomerID and MerchantID scope the list. Exactly one is set by the
	// caller: a customer sees their own orders and a shop sees its own, and a
	// query that could be given neither is a query that would return
	// everybody's.
	CustomerID string
	MerchantID string
	// Statuses narrows to particular states. Empty means all.
	Statuses []domain.Status
	// LiveOnly is the "current orders" tab — everything not yet finished.
	LiveOnly bool
	Limit    int
	Offset   int
}

// Repository stores orders.
type Repository interface {
	// Create writes a whole order — lines, options and its first event — in one
	// transaction. An order half-written is an order the customer paid for and
	// the shop never saw.
	Create(ctx context.Context, o domain.Order) error

	// Order returns one order with its lines and history.
	Order(ctx context.Context, orderID string) (domain.Order, error)

	// Orders lists orders matching a filter, newest first, with the total
	// before paging.
	Orders(ctx context.Context, f Filter) ([]domain.Order, int, error)

	// AppendTransition writes a status change and its event atomically.
	//
	// The expected status is passed and checked in the UPDATE itself, so two
	// riders tapping "picked up" at the same moment cannot both succeed. The
	// state machine in the domain says what may happen; this says it happened
	// once.
	AppendTransition(ctx context.Context, orderID string, from domain.Status, e domain.Event) error

	// ByIdempotencyKey returns the order a key already created, if any. The key
	// is scoped to the customer: two customers retrying with the same client
	// library and the same generated key must not collide.
	ByIdempotencyKey(ctx context.Context, customerID, key string) (domain.Order, bool, error)

	// ClaimIdempotencyKey records that a key belongs to an order, failing if it
	// is already taken. Written in the same transaction as the order, so there
	// is no window where the key exists and the order does not.
	ClaimIdempotencyKey(ctx context.Context, customerID, key, orderID string) error
}

// CodeGenerator makes the short human reference a customer reads out.
type CodeGenerator interface {
	// Code returns a short, unambiguous reference. A port rather than a
	// function so the format can change — and be tested — without touching a
	// use case.
	Code() string
}
