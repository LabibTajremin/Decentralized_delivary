package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Service implements contract.OrderContract.
//
// It reads the repository directly for the read path — the use cases above
// check who is asking, which is exactly wrong when the caller is dispatch
// rather than a person. For the write path it goes through the transition use
// case, so a module reaching in cannot skip the state machine.
type Service struct {
	repo        ports.Repository
	transitions *TransitionUseCase
}

// NewService wires the public service.
func NewService(repo ports.Repository, transitions *TransitionUseCase) *Service {
	return &Service{repo: repo, transitions: transitions}
}

// Order returns one order, whatever its status.
func (s *Service) Order(ctx context.Context, orderID string) (contract.Order, error) {
	order, err := s.repo.Order(ctx, orderID)
	if err != nil {
		return contract.Order{}, notFoundOr(err)
	}
	return toContract(order), nil
}

// MarkPaid moves a prepaid order from pending_payment to placed.
//
// A named method rather than a general "set status": a contract that let any
// consumer write any status would put the lifecycle back in the hands of every
// module that imports it.
func (s *Service) MarkPaid(ctx context.Context, orderID string) error {
	_, err := s.transitions.Execute(ctx, orderID, domain.StatusPlaced, "",
		Caller{Actor: domain.ActorSystem, ID: "payment"})
	return err
}

// MarkPaymentFailed cancels an order whose payment never arrived.
func (s *Service) MarkPaymentFailed(ctx context.Context, orderID, reason string) error {
	_, err := s.transitions.Execute(ctx, orderID, domain.StatusCancelled, reason,
		Caller{Actor: domain.ActorSystem, ID: "payment"})
	return err
}

// Advance moves an order on behalf of a party that owns a later stage of it.
//
// The transition is still checked against the state machine: this is a way in,
// not a way round.
func (s *Service) Advance(ctx context.Context, orderID, to, actor, actorID, reason string) error {
	status := domain.Status(to)
	if !status.Valid() {
		return errs.New(errs.KindInvalid, "unknown_status", "That is not a state an order can be in.")
	}
	who := domain.Actor(actor)
	switch who {
	case domain.ActorPartner, domain.ActorAdmin, domain.ActorSystem:
	default:
		// Only the three parties that reach an order through another module.
		//
		// Not the customer: their cancellation is bound by a window checked on
		// their own path, and a consumer claiming to be them would sidestep it.
		// Not the merchant either: a shop's authority over an order is "this
		// shop is yours", which the HTTP layer establishes from the merchant
		// record. A contract caller has no such claim to make, and accepting
		// one here would mean any consumer could accept anybody's order.
		return errs.New(errs.KindForbidden, "unknown_actor",
			"That is not a party that can move an order.")
	}
	_, err := s.transitions.Execute(ctx, orderID, status, reason, Caller{Actor: who, ID: actorID})
	return err
}

func toContract(o domain.Order) contract.Order {
	lines := make([]contract.Line, 0, len(o.Lines))
	for _, l := range o.Lines {
		lines = append(lines, contract.Line{Name: l.Name, Quantity: l.Quantity})
	}
	events := make([]contract.Event, 0, len(o.Events))
	for _, e := range o.Events {
		events = append(events, contract.Event{
			Status: string(e.Status), Actor: string(e.Actor), Reason: e.Reason, At: e.At,
		})
	}
	return contract.Order{
		ID: o.ID, Code: o.Code,
		CustomerID: o.CustomerID, MerchantID: o.MerchantID,
		Status: string(o.Status), Live: o.Status.IsLive(),
		PaymentMethod: string(o.Payment),
		Lines:         lines, Count: o.Count(),
		Subtotal: contractMoney(o.Charges.Subtotal),
		Delivery: contractMoney(o.Charges.Delivery),
		Total:    contractMoney(o.Charges.Total),
		Expanded: o.Charges.Expanded, DistanceM: o.Charges.DistanceM,
		Pickup: contract.Place{
			Name: o.Pickup.Name, Phone: o.Pickup.Phone,
			SingleLine: o.Pickup.SingleLine, Lat: o.Pickup.Lat, Lng: o.Pickup.Lng,
		},
		Destination: contract.Place{
			Name: o.Destination.Recipient, Phone: o.Destination.Phone,
			SingleLine: o.Destination.SingleLine,
			Lat:        o.Destination.Lat, Lng: o.Destination.Lng,
		},
		Events: events, PlacedAt: o.PlacedAt, UpdatedAt: o.UpdatedAt,
	}
}

func contractMoney(m money.Money) contract.Money {
	return contract.Money{Minor: m.Minor(), Currency: string(m.Currency()), Display: m.Display()}
}
