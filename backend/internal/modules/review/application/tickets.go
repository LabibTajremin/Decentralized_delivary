package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/external/payment"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// RaiseTicketUseCase opens a support ticket about an order.
type RaiseTicketUseCase struct {
	repo  ports.TicketRepository
	order order.Service
	clock clock.Clock
	ids   id.Generator
}

// NewRaiseTicketUseCase wires the use case.
func NewRaiseTicketUseCase(repo ports.TicketRepository, o order.Service, c clock.Clock, ids id.Generator) *RaiseTicketUseCase {
	return &RaiseTicketUseCase{repo: repo, order: o, clock: c, ids: ids}
}

// Execute opens a ticket, once the order behind it says the raiser owns it.
//
// Unlike a review, a ticket needs no delivered status: "this never arrived"
// is exactly the complaint a ticket exists to raise about an order that is
// still out for delivery, or that failed.
func (uc *RaiseTicketUseCase) Execute(ctx context.Context, orderID, raisedBy, subject string) (domain.Ticket, error) {
	ord, err := uc.order.Order(ctx, orderID)
	if err != nil {
		return domain.Ticket{}, notFoundOr(err)
	}
	if ord.CustomerID != raisedBy {
		return domain.Ticket{}, errs.New(errs.KindForbidden, "not_your_order",
			"You can only raise a ticket about your own orders.")
	}

	ticket, err := domain.NewTicket(uc.ids.New("tkt"), orderID, raisedBy, subject, uc.clock.Now())
	if err != nil {
		return domain.Ticket{}, ticketError(err)
	}
	if err := uc.repo.SaveTicket(ctx, ticket); err != nil {
		return domain.Ticket{}, storageError(err)
	}
	return ticket, nil
}

// ResolveTicketUseCase is a support agent closing a ticket.
type ResolveTicketUseCase struct {
	repo    ports.TicketRepository
	payment payment.Service
	clock   clock.Clock
}

// NewResolveTicketUseCase wires the use case.
func NewResolveTicketUseCase(repo ports.TicketRepository, p payment.Service, c clock.Clock) *ResolveTicketUseCase {
	return &ResolveTicketUseCase{repo: repo, payment: p, clock: c}
}

// Execute resolves a ticket. When the resolution is a refund, the refund is
// asked for before the resolution is saved: a ticket recorded as "refunded"
// whose refund never actually went through would tell a customer money is
// coming that will never arrive. Refund is idempotent on the order id, so a
// resolution that fails after the refund succeeded and is retried does not
// ask the gateway to give the money back twice.
func (uc *ResolveTicketUseCase) Execute(ctx context.Context, ticketID, agentID string, resolution domain.Resolution, note string) (domain.Ticket, error) {
	ticket, found, err := uc.repo.Ticket(ctx, ticketID)
	if err != nil {
		return domain.Ticket{}, storageError(err)
	}
	if !found {
		return domain.Ticket{}, notFound()
	}
	if err := ticket.Resolve(agentID, resolution, note, uc.clock.Now()); err != nil {
		return domain.Ticket{}, ticketError(err)
	}

	if resolution == domain.ResolutionRefunded {
		if err := uc.payment.Refund(ctx, ticket.OrderID, "support ticket "+ticket.ID); err != nil {
			return domain.Ticket{}, refundError(err)
		}
	}

	if err := uc.repo.SaveTicket(ctx, ticket); err != nil {
		return domain.Ticket{}, storageError(err)
	}
	return ticket, nil
}

// MyTicketsUseCase serves a customer's own support tickets.
type MyTicketsUseCase struct {
	repo ports.TicketRepository
}

// NewMyTicketsUseCase wires the use case.
func NewMyTicketsUseCase(repo ports.TicketRepository) *MyTicketsUseCase {
	return &MyTicketsUseCase{repo: repo}
}

// For lists a customer's own tickets, newest first.
func (uc *MyTicketsUseCase) For(ctx context.Context, userID string) ([]domain.Ticket, error) {
	tickets, err := uc.repo.TicketsRaisedBy(ctx, userID)
	if err != nil {
		return nil, storageError(err)
	}
	return tickets, nil
}

// OpenTicketsUseCase serves the support queue: tickets nobody has resolved
// yet, for an agent to work through.
type OpenTicketsUseCase struct {
	repo ports.TicketRepository
}

// NewOpenTicketsUseCase wires the use case.
func NewOpenTicketsUseCase(repo ports.TicketRepository) *OpenTicketsUseCase {
	return &OpenTicketsUseCase{repo: repo}
}

// Execute lists open tickets, oldest first, capped the same way every other
// unbounded list in this system is.
func (uc *OpenTicketsUseCase) Execute(ctx context.Context, limit int) ([]domain.Ticket, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	tickets, err := uc.repo.OpenTickets(ctx, limit)
	if err != nil {
		return nil, storageError(err)
	}
	return tickets, nil
}
