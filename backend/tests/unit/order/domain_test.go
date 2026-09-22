package order

import (
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// The phase's first acceptance criterion: the lifecycle rejects illegal
// transitions. Every refusal below is a thing somebody could plausibly try.

func TestTheLifecycleRefusesIllegalMoves(t *testing.T) {
	cases := []struct {
		name     string
		from, to domain.Status
		actor    domain.Actor
		want     error
	}{
		// Skipping ahead. A shop cannot mark an order delivered; a rider
		// cannot pick up something the kitchen has not finished.
		{"placed straight to delivered", domain.StatusPlaced, domain.StatusDelivered, domain.ActorMerchant, domain.ErrIllegalTransition},
		{"accepted straight to picked up", domain.StatusAccepted, domain.StatusPickedUp, domain.ActorPartner, domain.ErrIllegalTransition},
		{"preparing straight to delivered", domain.StatusPreparing, domain.StatusDelivered, domain.ActorPartner, domain.ErrIllegalTransition},

		// Going backwards.
		{"ready back to preparing", domain.StatusReady, domain.StatusPreparing, domain.ActorMerchant, domain.ErrIllegalTransition},
		{"picked up back to ready", domain.StatusPickedUp, domain.StatusReady, domain.ActorPartner, domain.ErrIllegalTransition},

		// Moving something that has finished.
		{"cancelling a delivered order", domain.StatusDelivered, domain.StatusCancelled, domain.ActorAdmin, domain.ErrAlreadyFinished},
		{"reviving a rejected order", domain.StatusRejected, domain.StatusAccepted, domain.ActorMerchant, domain.ErrAlreadyFinished},
		{"re-delivering a failed order", domain.StatusFailed, domain.StatusDelivered, domain.ActorPartner, domain.ErrAlreadyFinished},

		// The right move by the wrong party. These are the ones that matter:
		// the move exists, so a table that only listed moves would allow them.
		{"a customer accepting their own order", domain.StatusPlaced, domain.StatusAccepted, domain.ActorCustomer, domain.ErrNotYourTransition},
		{"a customer marking it delivered", domain.StatusPickedUp, domain.StatusDelivered, domain.ActorCustomer, domain.ErrNotYourTransition},
		{"a shop marking its own order picked up", domain.StatusReady, domain.StatusPickedUp, domain.ActorMerchant, domain.ErrNotYourTransition},
		{"a rider rejecting an order", domain.StatusPlaced, domain.StatusRejected, domain.ActorPartner, domain.ErrNotYourTransition},
		{"a customer cancelling a cooking order", domain.StatusPreparing, domain.StatusCancelled, domain.ActorCustomer, domain.ErrNotYourTransition},
		{"a shop cancelling an unpaid order it has not seen", domain.StatusPendingPayment, domain.StatusCancelled, domain.ActorMerchant, domain.ErrNotYourTransition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := domain.CanTransition(tc.from, tc.to, tc.actor); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestTheLifecycleAllowsTheRealPath(t *testing.T) {
	cases := []struct {
		from, to domain.Status
		actor    domain.Actor
	}{
		{domain.StatusPendingPayment, domain.StatusPlaced, domain.ActorSystem},
		{domain.StatusPlaced, domain.StatusAccepted, domain.ActorMerchant},
		{domain.StatusAccepted, domain.StatusPreparing, domain.ActorMerchant},
		{domain.StatusPreparing, domain.StatusReady, domain.ActorMerchant},
		{domain.StatusReady, domain.StatusPickedUp, domain.ActorPartner},
		{domain.StatusPickedUp, domain.StatusDelivered, domain.ActorPartner},
		{domain.StatusPickedUp, domain.StatusFailed, domain.ActorPartner},
		{domain.StatusPlaced, domain.StatusCancelled, domain.ActorCustomer},
		{domain.StatusPlaced, domain.StatusRejected, domain.ActorMerchant},
		// A shop that has accepted can still discover it cannot deliver.
		{domain.StatusAccepted, domain.StatusRejected, domain.ActorMerchant},
		// An admin can clear up anything that has not finished.
		{domain.StatusPreparing, domain.StatusCancelled, domain.ActorAdmin},
		{domain.StatusReady, domain.StatusCancelled, domain.ActorAdmin},
	}
	for _, tc := range cases {
		if err := domain.CanTransition(tc.from, tc.to, tc.actor); err != nil {
			t.Errorf("%s → %s by %s: %v", tc.from, tc.to, tc.actor, err)
		}
	}
}

func TestTerminalStatesAreTerminal(t *testing.T) {
	for _, s := range []domain.Status{
		domain.StatusDelivered, domain.StatusCancelled,
		domain.StatusRejected, domain.StatusFailed,
	} {
		if !s.IsTerminal() || s.IsLive() {
			t.Errorf("%s: terminal = %v, live = %v", s, s.IsTerminal(), s.IsLive())
		}
		if len(domain.NextStatuses(s, domain.ActorAdmin)) != 0 {
			t.Errorf("%s offers a move out", s)
		}
	}
	for _, s := range []domain.Status{
		domain.StatusPendingPayment, domain.StatusPlaced, domain.StatusAccepted,
		domain.StatusPreparing, domain.StatusReady, domain.StatusPickedUp,
	} {
		if s.IsTerminal() || !s.IsLive() {
			t.Errorf("%s: terminal = %v, live = %v", s, s.IsTerminal(), s.IsLive())
		}
	}
	// The empty status is neither, which is what an unset field should be.
	if domain.Status("").IsLive() {
		t.Error("the zero status reported itself live")
	}
}

func TestStatusValidity(t *testing.T) {
	if !domain.StatusPlaced.Valid() {
		t.Error("placed is not a status")
	}
	if domain.Status("shipped").Valid() {
		t.Error("shipped is a status")
	}
	// An unknown status has no moves out of it either.
	if err := domain.CanTransition("shipped", domain.StatusDelivered, domain.ActorAdmin); !errors.Is(err, domain.ErrIllegalTransition) {
		t.Errorf("err = %v", err)
	}
}

// The server answers "what can I do now" rather than the client working it out
// from the status. Each party gets a different answer from the same state.
func TestNextStatusesIsPerActor(t *testing.T) {
	cases := []struct {
		actor domain.Actor
		want  []domain.Status
	}{
		{domain.ActorMerchant, []domain.Status{domain.StatusAccepted, domain.StatusRejected}},
		{domain.ActorCustomer, []domain.Status{domain.StatusCancelled}},
		{domain.ActorPartner, nil},
		{domain.ActorAdmin, []domain.Status{domain.StatusAccepted, domain.StatusRejected, domain.StatusCancelled}},
	}
	for _, tc := range cases {
		got := domain.NextStatuses(domain.StatusPlaced, tc.actor)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got %v, want %v", tc.actor, got, tc.want)
		}
		for i, want := range tc.want {
			if got[i] != want {
				t.Errorf("%s: got %v, want %v", tc.actor, got, tc.want)
			}
		}
	}
}

// ------------------------------------------------------------------ orders

func draft() domain.Draft {
	return domain.Draft{
		ID: "ORD-1", EventID: "OEV-1", Code: "ABC234",
		CustomerID: "USR-1", MerchantID: "MER-1",
		Payment: domain.PaymentCash,
		Lines: []domain.Line{{
			ID: "OLN-1", Kind: "item", TargetID: "ITM-1", Name: "Biryani",
			UnitPrice: money.Taka(25000), Quantity: 2,
		}},
		Charges: domain.Charges{
			Subtotal: money.Taka(50000), Delivery: money.Taka(7000),
			Total: money.Taka(57000),
		},
		Destination: domain.Destination{AddressID: "ADR-1", Lat: 23.7, Lng: 90.4},
		Pickup:      domain.Pickup{MerchantID: "MER-1", Name: "Star Kabab"},
		CODLimit:    money.Taka(500000),
		Now:         placedAt,
	}
}

func TestNewOrderFreezesTheDraft(t *testing.T) {
	order, err := domain.NewOrder(draft())
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	if order.Status != domain.StatusPlaced || order.Count() != 2 {
		t.Fatalf("order = %+v", order)
	}
	// Cash is placed outright: there is nothing to wait for.
	if order.Payment != domain.PaymentCash {
		t.Errorf("payment = %q", order.Payment)
	}
	// The first event is the placement itself, so the history is complete from
	// the beginning rather than starting at the first change.
	if len(order.Events) != 1 || order.Events[0].Status != domain.StatusPlaced ||
		order.Events[0].Actor != domain.ActorCustomer {
		t.Fatalf("events = %+v", order.Events)
	}
	if !order.BelongsTo("USR-1") || order.BelongsTo("USR-2") {
		t.Error("ownership is wrong")
	}
}

// A prepaid order does not reach the shop until the money does. A shop that
// started cooking on an unpaid order would be carrying the platform's risk.
func TestAPrepaidOrderWaitsForTheMoney(t *testing.T) {
	d := draft()
	d.Payment = domain.PaymentOnline
	order, err := domain.NewOrder(d)
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	if order.Status != domain.StatusPendingPayment {
		t.Fatalf("status = %q", order.Status)
	}
	// And the shop cannot see or touch it yet.
	if err := domain.CanTransition(order.Status, domain.StatusAccepted, domain.ActorMerchant); err == nil {
		t.Error("a shop could accept an unpaid order")
	}
}

func TestTheCODLimit(t *testing.T) {
	d := draft()
	d.Charges.Total = money.Taka(500001)
	if _, err := domain.NewOrder(d); !errors.Is(err, domain.ErrCODLimit) {
		t.Fatalf("a cash order past the ceiling was accepted: %v", err)
	}

	// Exactly at the limit is allowed. "Maximum COD order value" reads as "up
	// to and including" to everybody who is not writing the comparison.
	d.Charges.Total = money.Taka(500000)
	if _, err := domain.NewOrder(d); err != nil {
		t.Fatalf("an order at exactly the ceiling was refused: %v", err)
	}

	// The ceiling is about cash. The same amount online is fine — nobody is
	// carrying it.
	d.Charges.Total = money.Taka(500001)
	d.Payment = domain.PaymentOnline
	if _, err := domain.NewOrder(d); err != nil {
		t.Fatalf("a large prepaid order was refused: %v", err)
	}

	// A limit of zero switches the check off rather than refusing every cash
	// order, for the same reason a zero free-delivery threshold switches that
	// promotion off: nobody configures "no cash at all" by clearing a field.
	d.Payment = domain.PaymentCash
	d.CODLimit = money.Taka(0)
	if _, err := domain.NewOrder(d); err != nil {
		t.Fatalf("a zero limit refused a cash order: %v", err)
	}
}

func TestNewOrderRefusesAnImpossibleDraft(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(*domain.Draft)
		want   error
	}{
		{"no customer", func(d *domain.Draft) { d.CustomerID = "" }, domain.ErrNoCustomer},
		{"no shop", func(d *domain.Draft) { d.MerchantID = "" }, domain.ErrNoMerchant},
		{"nothing in it", func(d *domain.Draft) { d.Lines = nil }, domain.ErrNoLines},
		{"nowhere to go", func(d *domain.Draft) { d.Destination.AddressID = "" }, domain.ErrNoAddress},
		{"no way to pay", func(d *domain.Draft) { d.Payment = "barter" }, domain.ErrUnknownPayment},
		{"a negative total", func(d *domain.Draft) { d.Charges.Total = money.Taka(-1) }, domain.ErrNegativeAmount},
		{"a negative subtotal", func(d *domain.Draft) { d.Charges.Subtotal = money.Taka(-1) }, domain.ErrNegativeAmount},
		// An event with no id is a row that collides with the next one, which
		// is a failure that only appears on the second order a process writes.
		{"no id for the placement event", func(d *domain.Draft) { d.EventID = "" }, domain.ErrNoEventID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := draft()
			tc.break_(&d)
			if _, err := domain.NewOrder(d); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// The order carries its own history, so no caller can change a status and
// forget to say when.
func TestMoveRecordsWhatHappened(t *testing.T) {
	order, err := domain.NewOrder(draft())
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	at := placedAt.Add(30 * time.Second)
	if err := order.Move(domain.StatusAccepted, domain.ActorMerchant, "MER-1", "", at); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if order.Status != domain.StatusAccepted || !order.UpdatedAt.Equal(at) {
		t.Fatalf("order = %+v", order)
	}
	if len(order.Events) != 2 || order.Events[1].ActorID != "MER-1" {
		t.Fatalf("events = %+v", order.Events)
	}
	when, accepted := order.AcceptedAt()
	if !accepted || !when.Equal(at) {
		t.Errorf("AcceptedAt = %v, %v", when, accepted)
	}

	// A refused move changes nothing at all — not the status, not the history.
	if err := order.Move(domain.StatusDelivered, domain.ActorMerchant, "MER-1", "", at); err == nil {
		t.Fatal("an illegal move was applied")
	}
	if order.Status != domain.StatusAccepted || len(order.Events) != 2 {
		t.Errorf("a refused move left a mark: %+v", order)
	}
}

func TestAcceptedAtOnAnOrderNobodyAccepted(t *testing.T) {
	order, err := domain.NewOrder(draft())
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	if _, accepted := order.AcceptedAt(); accepted {
		t.Error("a fresh order claimed to have been accepted")
	}
}

func TestLineTotals(t *testing.T) {
	line := domain.Line{
		UnitPrice: money.Taka(20000), Quantity: 3,
		Options: []domain.Option{
			{Price: money.Taka(5000)}, {Price: money.Taka(2500)},
		},
	}
	if got := line.Total(); got.Minor() != 82500 {
		t.Errorf("total = %d, want 82500", got.Minor())
	}
}
