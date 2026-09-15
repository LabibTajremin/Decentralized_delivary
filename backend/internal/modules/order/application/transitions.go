package application

import (
	"context"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	cfg "github.com/rootlogic-lab/delivery/backend/internal/modules/order/external/config"
	merchantx "github.com/rootlogic-lab/delivery/backend/internal/modules/order/external/merchant"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// TransitionUseCase moves an order along its life.
//
// One use case for every party, because they are all asking the same question —
// may this order go from here to there, and is it mine to move — and the answer
// lives in one table (domain/status.go). Separate use cases per role would be
// four copies of the same check, and the copies would drift.
type TransitionUseCase struct {
	repo     ports.Repository
	merchant merchantx.Service
	config   cfg.Service
	clock    clock.Clock
	ids      id.Generator
}

// NewTransitionUseCase wires the use case.
func NewTransitionUseCase(
	repo ports.Repository,
	merchants merchantx.Service,
	config cfg.Service,
	c clock.Clock,
	ids id.Generator,
) *TransitionUseCase {
	return &TransitionUseCase{repo: repo, merchant: merchants, config: config, clock: c, ids: ids}
}

// Caller is who is asking, and on whose behalf.
type Caller struct {
	Actor domain.Actor
	// ID is the user id, the merchant id, or the partner id — whichever the
	// actor means. Recorded on the event, so "who cancelled this" has an
	// answer.
	ID string
	// MerchantIDs are the shops this caller owns, for a merchant actor. An
	// owner may move their own shop's orders and nobody else's, and checking
	// that here rather than in the transport means it cannot be skipped by a
	// second endpoint later.
	MerchantIDs []string
	Lang        string
}

// Execute applies a transition.
func (uc *TransitionUseCase) Execute(ctx context.Context, orderID string, to domain.Status, reason string, caller Caller) (View, error) {
	order, err := uc.repo.Order(ctx, orderID)
	if err != nil {
		return View{}, notFoundOr(err)
	}
	if err := uc.mayTouch(order, caller); err != nil {
		return View{}, err
	}

	now := uc.clock.Now()

	// A customer cancellation is two questions, and the clock answers the
	// second. The state machine says the move exists from here; the window says
	// whether it is still theirs to make.
	if caller.Actor == domain.ActorCustomer && to == domain.StatusCancelled {
		decision, decisionErr := uc.cancelDecision(ctx, order, now)
		if decisionErr != nil {
			return View{}, decisionErr
		}
		if !decision.Allowed {
			return View{}, errs.New(errs.KindConflict, "cannot_cancel",
				cancelText(decision.Reason, caller.Lang)).With("reason", decision.Reason)
		}
	}

	if to == domain.StatusRejected && reason == "" {
		// A shop refusing an order owes the customer a reason. It is the only
		// thing they will have to go on.
		return View{}, errs.New(errs.KindInvalid, "reason_required",
			"Please say why you cannot take this order.")
	}

	from := order.Status
	if err := order.Move(to, caller.Actor, caller.ID, reason, now); err != nil {
		return View{}, orderError(err)
	}

	event := order.Events[len(order.Events)-1]
	event.ID = uc.ids.New("OEV")
	if err := uc.repo.AppendTransition(ctx, order.ID, from, event); err != nil {
		return View{}, storageError(err)
	}
	order.Events[len(order.Events)-1] = event

	return uc.render(ctx, order, caller, now), nil
}

// CancelStatus reports whether the customer may still call an order off,
// without changing anything.
//
// Its own endpoint because a countdown needs refreshing and a client must not
// learn the answer by trying it.
func (uc *TransitionUseCase) CancelStatus(ctx context.Context, orderID string, caller Caller) (CancelView, error) {
	order, err := uc.repo.Order(ctx, orderID)
	if err != nil {
		return CancelView{}, notFoundOr(err)
	}
	if err := uc.mayTouch(order, caller); err != nil {
		return CancelView{}, err
	}
	decision, err := uc.cancelDecision(ctx, order, uc.clock.Now())
	if err != nil {
		return CancelView{}, err
	}
	return withCancelText(cancelViewOf(decision), caller.Lang), nil
}

// cancelDecision reads the window in force where the order is going and applies
// it.
func (uc *TransitionUseCase) cancelDecision(ctx context.Context, order domain.Order, now time.Time) (domain.CancelDecision, error) {
	settings, err := uc.config.Settings(ctx, cfg.Placement{AreaCode: order.Destination.AreaCode})
	if err != nil {
		return domain.CancelDecision{}, configError(err)
	}
	seconds, err := settings.Int(cfg.CancellationWindow)
	if err != nil {
		return domain.CancelDecision{}, configError(err)
	}
	return order.MayCustomerCancel(time.Duration(seconds)*time.Second, now), nil
}

// mayTouch decides whether this caller has anything to do with this order.
//
// Separate from the transition table, which decides what an actor of a *kind*
// may do. This decides whether they are that actor *for this order* — a
// merchant may accept orders, but only their own shop's.
func (uc *TransitionUseCase) mayTouch(order domain.Order, caller Caller) error {
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
	case domain.ActorAdmin, domain.ActorPartner, domain.ActorSystem:
		// An admin sees everything. A partner reaches orders through dispatch
		// (P12), which checks the assignment before it calls; this module's
		// job is the state machine, and a second assignment check here would be
		// a copy of dispatch's that could disagree with it.
		return nil
	}
	return nil
}

// render composes the screen after a change, from the point of view of whoever
// made it.
func (uc *TransitionUseCase) render(ctx context.Context, order domain.Order, caller Caller, now time.Time) View {
	cancel := CancelView{}
	if caller.Actor == domain.ActorCustomer {
		if decision, err := uc.cancelDecision(ctx, order, now); err == nil {
			cancel = withCancelText(cancelViewOf(decision), caller.Lang)
		}
	}
	return viewFor(order, caller.Actor, cancel, caller.Lang)
}

// notFoundOr turns a missing order into a not-found and anything else into an
// outage.
func notFoundOr(err error) error {
	if errs.Is(err, errs.KindNotFound) {
		return notFound()
	}
	return storageError(err)
}
