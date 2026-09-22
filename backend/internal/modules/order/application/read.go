package application

import (
	"context"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	cfg "github.com/rootlogic-lab/delivery/backend/internal/modules/order/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
)

// maxPage caps a listing. A customer with four hundred orders gets them a page
// at a time, because a low-end phone rendering four hundred receipts is a phone
// that stops responding (2.9).
const maxPage = 50

// ReadUseCase answers "what have I ordered" and "what is happening to it".
type ReadUseCase struct {
	repo   ports.Repository
	config cfg.Service
	clock  clock.Clock
}

// NewReadUseCase wires the use case.
func NewReadUseCase(repo ports.Repository, config cfg.Service, c clock.Clock) *ReadUseCase {
	return &ReadUseCase{repo: repo, config: config, clock: c}
}

// ListQuery is one page of somebody's orders.
type ListQuery struct {
	// LiveOnly is the "current orders" tab.
	LiveOnly bool
	Limit    int
	Offset   int
	Lang     string
}

// List is a page of orders with the count before paging.
type List struct {
	Orders []View
	Total  int
}

// OfCustomer lists a customer's own orders.
func (uc *ReadUseCase) OfCustomer(ctx context.Context, customerID string, q ListQuery) (List, error) {
	return uc.list(ctx, ports.Filter{
		CustomerID: customerID, LiveOnly: q.LiveOnly,
		Limit: pageSize(q.Limit), Offset: q.Offset,
	}, domain.ActorCustomer, q.Lang)
}

// OfMerchant lists one shop's orders.
func (uc *ReadUseCase) OfMerchant(ctx context.Context, merchantID string, q ListQuery) (List, error) {
	return uc.list(ctx, ports.Filter{
		MerchantID: merchantID, LiveOnly: q.LiveOnly,
		Limit: pageSize(q.Limit), Offset: q.Offset,
	}, domain.ActorMerchant, q.Lang)
}

func (uc *ReadUseCase) list(ctx context.Context, f ports.Filter, actor domain.Actor, lang string) (List, error) {
	orders, total, err := uc.repo.Orders(ctx, f)
	if err != nil {
		return List{}, storageError(err)
	}
	views := make([]View, 0, len(orders))
	for _, o := range orders {
		views = append(views, viewFor(o, actor, CancelView{}, lang))
	}
	return List{Orders: views, Total: total}, nil
}

// One returns a single order, from the point of view of whoever is asking.
//
// The caller's identity decides both whether they may see it and what they are
// told they can do next. A customer also gets the cancellation countdown, which
// is the one part of the screen that has to be right to the second.
func (uc *ReadUseCase) One(ctx context.Context, orderID string, caller Caller) (View, error) {
	order, err := uc.repo.Order(ctx, orderID)
	if err != nil {
		return View{}, notFoundOr(err)
	}
	if err := uc.mayRead(order, caller); err != nil {
		return View{}, err
	}

	cancel := CancelView{}
	if caller.Actor == domain.ActorCustomer {
		if decision, cancelErr := uc.cancelDecision(ctx, order); cancelErr == nil {
			cancel = withCancelText(cancelViewOf(decision), caller.Lang)
		}
	}
	return viewFor(order, caller.Actor, cancel, caller.Lang), nil
}

func (uc *ReadUseCase) cancelDecision(ctx context.Context, order domain.Order) (domain.CancelDecision, error) {
	settings, err := uc.config.Settings(ctx, cfg.Placement{AreaCode: order.Destination.AreaCode})
	if err != nil {
		return domain.CancelDecision{}, configError(err)
	}
	seconds, err := settings.Int(cfg.CancellationWindow)
	if err != nil {
		return domain.CancelDecision{}, configError(err)
	}
	return order.MayCustomerCancel(time.Duration(seconds)*time.Second, uc.clock.Now()), nil
}

// mayRead decides whether this caller may see this order at all.
//
// The refusal is a not-found rather than a forbidden, deliberately. Saying
// "that order exists but is not yours" would let anybody with a list of ids
// find out which ones are real.
func (uc *ReadUseCase) mayRead(order domain.Order, caller Caller) error {
	switch caller.Actor {
	case domain.ActorCustomer:
		if !order.BelongsTo(caller.ID) {
			return notFound()
		}
	case domain.ActorMerchant:
		for _, id := range caller.MerchantIDs {
			if id == order.MerchantID {
				return nil
			}
		}
		return notFound()
	}
	return nil
}

// pageSize clamps a requested page rather than refusing one. A client asking
// for a thousand orders wants "as many as you will give me".
func pageSize(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > maxPage {
		return maxPage
	}
	return limit
}
