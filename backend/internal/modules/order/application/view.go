package application

import (
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Everything an order screen renders, already decided. The status has a
// sentence, the receipt has its rows, the cancel button knows whether it works
// and for how long, and the list of things this caller may do next came from
// the state machine rather than from a copy of it in the app (2.9).

// Money is an amount in both forms.
type Money struct {
	Minor    int64
	Currency string
	Display  string
}

func toMoney(m money.Money) Money {
	return Money{Minor: m.Minor(), Currency: string(m.Currency()), Display: m.Display()}
}

// OptionView is one choice on a line.
type OptionView struct {
	Name  string
	Price Money
}

// LineView is one thing that was bought.
type LineView struct {
	ID        string
	Kind      string
	TargetID  string
	Name      string
	Options   []OptionView
	Quantity  int
	Note      string
	UnitPrice Money
	LineTotal Money
}

// ReceiptRow is one line of the frozen receipt.
type ReceiptRow struct {
	Key    string
	Label  string
	Amount Money
}

// PlaceView is one end of the delivery, as the screen shows it.
type PlaceView struct {
	Name       string
	Phone      string
	SingleLine string
	Lat        float64
	Lng        float64
}

// EventView is one entry in the order's history.
type EventView struct {
	Status string
	Label  string
	Actor  string
	Reason string
	At     time.Time
}

// CancelView is whether the customer may still call the order off.
type CancelView struct {
	Allowed bool
	// Reason names the refusal, and Text says it in words.
	Reason string
	Text   string
	// SecondsLeft drives a countdown. The server's clock decides, because the
	// phone's is wrong often enough to matter.
	SecondsLeft int
	FreeUntil   time.Time
}

// View is a whole order, ready to render.
type View struct {
	ID   string
	Code string

	Status      string
	StatusLabel string
	Live        bool
	Payment     string

	Lines []LineView
	Count int

	Subtotal Money
	Delivery Money
	Total    Money
	Receipt  []ReceiptRow

	Expanded  bool
	DistanceM float64

	Pickup      PlaceView
	Destination PlaceView

	Events []EventView
	// NextActions is what the caller may do now, straight from the state
	// machine. A client with its own copy of the transition table would offer a
	// shop an Accept button on an order somebody already cancelled.
	NextActions []string
	Cancel      CancelView

	PlacedAt  time.Time
	UpdatedAt time.Time
}

// viewOf composes the screen for a customer.
func viewOf(o domain.Order, cancel CancelView, lang string) View {
	return viewFor(o, domain.ActorCustomer, cancel, lang)
}

// viewFor composes the screen for whoever is looking, which decides what
// NextActions says.
func viewFor(o domain.Order, actor domain.Actor, cancel CancelView, lang string) View {
	lines := make([]LineView, 0, len(o.Lines))
	for _, l := range o.Lines {
		options := make([]OptionView, 0, len(l.Options))
		for _, opt := range l.Options {
			options = append(options, OptionView{Name: opt.Name, Price: toMoney(opt.Price)})
		}
		lines = append(lines, LineView{
			ID: l.ID, Kind: l.Kind, TargetID: l.TargetID, Name: l.Name,
			Options: options, Quantity: l.Quantity, Note: l.Note,
			UnitPrice: toMoney(l.UnitPrice), LineTotal: toMoney(l.Total()),
		})
	}

	events := make([]EventView, 0, len(o.Events))
	for _, e := range o.Events {
		events = append(events, EventView{
			Status: string(e.Status), Label: statusLabel(e.Status, lang),
			Actor: string(e.Actor), Reason: e.Reason, At: e.At,
		})
	}

	next := domain.NextStatuses(o.Status, actor)
	actions := make([]string, 0, len(next))
	for _, s := range next {
		actions = append(actions, string(s))
	}

	return View{
		ID: o.ID, Code: o.Code,
		Status: string(o.Status), StatusLabel: statusLabel(o.Status, lang),
		Live: o.Status.IsLive(), Payment: string(o.Payment),
		Lines: lines, Count: o.Count(),
		Subtotal: toMoney(o.Charges.Subtotal),
		Delivery: toMoney(o.Charges.Delivery),
		Total:    toMoney(o.Charges.Total),
		Receipt:  receiptOf(o.Charges, lang),
		Expanded: o.Charges.Expanded, DistanceM: o.Charges.DistanceM,
		Pickup: PlaceView{
			Name: o.Pickup.Name, Phone: o.Pickup.Phone,
			SingleLine: o.Pickup.SingleLine, Lat: o.Pickup.Lat, Lng: o.Pickup.Lng,
		},
		Destination: PlaceView{
			Name: o.Destination.Recipient, Phone: o.Destination.Phone,
			SingleLine: o.Destination.SingleLine,
			Lat:        o.Destination.Lat, Lng: o.Destination.Lng,
		},
		Events: events, NextActions: actions, Cancel: cancel,
		PlacedAt: o.PlacedAt, UpdatedAt: o.UpdatedAt,
	}
}

// receiptOf rebuilds the receipt from the frozen charges.
//
// Rebuilt from what was written down rather than re-quoted: a receipt that
// changed when an admin retuned the area's delivery rate would be a receipt
// nobody could reconcile.
func receiptOf(c domain.Charges, lang string) []ReceiptRow {
	rows := []ReceiptRow{
		{Key: "subtotal", Label: receiptLabel("subtotal", lang), Amount: toMoney(c.Subtotal)},
		{Key: "delivery", Label: receiptLabel("delivery", lang), Amount: toMoney(c.Delivery)},
	}
	if c.Expanded && !c.ExpansionSurcharge.IsZero() {
		rows = append(rows, ReceiptRow{
			Key: "expansion_surcharge", Label: receiptLabel("expansion_surcharge", lang),
			Amount: toMoney(c.ExpansionSurcharge),
		})
	}
	if c.FreeDelivery {
		rows = append(rows, ReceiptRow{
			Key: "free_delivery", Label: receiptLabel("free_delivery", lang),
			Amount: toMoney(money.Taka(0)),
		})
	}
	return append(rows, ReceiptRow{
		Key: "total", Label: receiptLabel("total", lang), Amount: toMoney(c.Total),
	})
}

func receiptLabel(key, lang string) string {
	bengali := lang != "en"
	switch key {
	case "subtotal":
		if bengali {
			return "পণ্যের মোট"
		}
		return "Items"
	case "delivery":
		if bengali {
			return "ডেলিভারি চার্জ"
		}
		return "Delivery"
	case "expansion_surcharge":
		if bengali {
			return "দূরত্বের জন্য অতিরিক্ত"
		}
		return "Extra distance charge"
	case "free_delivery":
		if bengali {
			return "ফ্রি ডেলিভারি"
		}
		return "Free delivery"
	default:
		if bengali {
			return "সর্বমোট"
		}
		return "Total"
	}
}

// statusLabel is where the order is, in words. Bengali-first (1.4).
//
// Written from the customer's point of view rather than the system's: "রান্না
// হচ্ছে" is what is happening to their food, where "preparing" is what a column
// says.
func statusLabel(s domain.Status, lang string) string {
	bengali := lang != "en"
	switch s {
	case domain.StatusPendingPayment:
		if bengali {
			return "পেমেন্টের অপেক্ষায়"
		}
		return "Waiting for payment"
	case domain.StatusPlaced:
		if bengali {
			return "দোকানে পাঠানো হয়েছে"
		}
		return "Sent to the shop"
	case domain.StatusAccepted:
		if bengali {
			return "দোকান অর্ডার নিয়েছে"
		}
		return "The shop has accepted"
	case domain.StatusPreparing:
		if bengali {
			return "তৈরি হচ্ছে"
		}
		return "Being prepared"
	case domain.StatusReady:
		if bengali {
			return "রাইডারের অপেক্ষায়"
		}
		return "Waiting for a rider"
	case domain.StatusPickedUp:
		if bengali {
			return "পথে আছে"
		}
		return "On the way"
	case domain.StatusDelivered:
		if bengali {
			return "পৌঁছে দেওয়া হয়েছে"
		}
		return "Delivered"
	case domain.StatusCancelled:
		if bengali {
			return "বাতিল করা হয়েছে"
		}
		return "Cancelled"
	case domain.StatusRejected:
		if bengali {
			return "দোকান নিতে পারেনি"
		}
		return "The shop could not take it"
	default:
		if bengali {
			return "পৌঁছে দেওয়া যায়নি"
		}
		return "Could not be delivered"
	}
}

// cancelViewOf turns the domain's decision into the screen's version of it.
func cancelViewOf(d domain.CancelDecision) CancelView {
	return CancelView{
		Allowed: d.Allowed, Reason: d.Reason,
		SecondsLeft: d.SecondsLeft, FreeUntil: d.FreeUntil,
	}
}

// withCancelText fills in the sentence, which needs the language the decision
// does not carry.
func withCancelText(v CancelView, lang string) CancelView {
	v.Text = cancelText(v.Reason, lang)
	return v
}

// cancelText says why the customer cannot cancel, in words.
func cancelText(reason, lang string) string {
	bengali := lang != "en"
	switch reason {
	case domain.ReasonAlreadyFinished:
		if bengali {
			return "এই অর্ডার শেষ হয়ে গেছে।"
		}
		return "That order has already finished."
	case domain.ReasonTooLate:
		if bengali {
			return "বাতিল করার সময় পেরিয়ে গেছে।"
		}
		return "The time to cancel has passed."
	case domain.ReasonUnderway:
		if bengali {
			return "দোকান অর্ডার তৈরি শুরু করেছে, তাই এখন বাতিল করা যাবে না।"
		}
		return "The shop has started preparing this, so it can no longer be cancelled."
	default:
		return ""
	}
}
