package domain

import "github.com/rootlogic-lab/delivery/backend/internal/shared/money"

// A quote is a receipt before the fact. Every row on it is composed here, with
// a key for the client to branch on and an amount already worked out, because
// a client that assembled a receipt would assemble it differently from the one
// the customer gets afterwards (2.9).

// RowKey names one line of the receipt.
type RowKey string

// The rows a quote can carry, in the order they are shown.
const (
	// RowSubtotal is what the goods cost.
	RowSubtotal RowKey = "subtotal"
	// RowDelivery is the delivery fee as charged.
	RowDelivery RowKey = "delivery"
	// RowExpansionSurcharge is how much of the delivery fee is the D2
	// surcharge. Shown separately because a customer who widened their search
	// should see what that decision cost, not just a bigger number.
	RowExpansionSurcharge RowKey = "expansion_surcharge"
	// RowFreeDelivery is the delivery fee waived, as a negative amount.
	RowFreeDelivery RowKey = "free_delivery"
	// RowTotal is what will be charged.
	RowTotal RowKey = "total"
)

// Row is one line of the receipt.
type Row struct {
	Key    RowKey
	Amount money.Money
}

// Quote is the whole price of an order.
type Quote struct {
	Subtotal money.Money
	// Delivery is what the customer pays to have it brought, after any waiver.
	Delivery money.Money
	// DeliveryBeforeWaiver is what it would have cost. Equal to Delivery unless
	// free delivery applied.
	DeliveryBeforeWaiver money.Money
	// ExpansionSurcharge is the part of DeliveryBeforeWaiver that exists only
	// because the customer widened the radius (D2). Zero at the base radius.
	ExpansionSurcharge money.Money
	// FreeDelivery is whether the waiver applied.
	FreeDelivery bool
	// FreeDeliveryAt is the order value that earns the waiver, or zero when the
	// promotion is off here. Carried so a client can say "৳120 more for free
	// delivery" without holding the figure.
	FreeDeliveryAt money.Money
	// AwayFromFreeDelivery is how much more the customer would have to spend,
	// or zero when they already qualify or the promotion is off or an expanded
	// search has forfeited it.
	AwayFromFreeDelivery money.Money
	Total                money.Money

	DistanceM float64
	Expanded  bool

	// Rows is the receipt, already ordered.
	Rows []Row
}

// Price works out what an order comes to. O(1).
func (t Tariff) Price(subtotal money.Money, distanceM float64, expanded bool) (Quote, error) {
	if subtotal.IsNegative() {
		return Quote{}, ErrNegativeSubtotal
	}
	full, surcharge, err := t.DeliveryFee(distanceM, expanded)
	if err != nil {
		return Quote{}, err
	}

	free := t.FreeDeliveryApplies(subtotal, expanded)
	charged := full
	if free {
		charged = money.Taka(0)
	}

	quote := Quote{
		Subtotal:             subtotal,
		Delivery:             charged,
		DeliveryBeforeWaiver: full,
		ExpansionSurcharge:   surcharge,
		FreeDelivery:         free,
		FreeDeliveryAt:       t.freeAbove,
		AwayFromFreeDelivery: t.awayFromFree(subtotal, expanded),
		Total:                money.Taka(subtotal.Minor() + charged.Minor()),
		DistanceM:            distanceM,
		Expanded:             expanded,
	}
	quote.Rows = quote.rows()
	return quote, nil
}

// awayFromFree is how much more would earn the waiver.
func (t Tariff) awayFromFree(subtotal money.Money, expanded bool) money.Money {
	if expanded || t.freeAbove.IsZero() {
		return money.Taka(0)
	}
	short := t.freeAbove.Minor() - subtotal.Minor()
	if short <= 0 {
		return money.Taka(0)
	}
	return money.Taka(short)
}

// rows builds the receipt.
//
// Only the rows that say something appear. A receipt with a zero "expansion
// surcharge" line on every local order trains people to stop reading it.
func (q Quote) rows() []Row {
	rows := []Row{
		{Key: RowSubtotal, Amount: q.Subtotal},
		{Key: RowDelivery, Amount: q.Delivery},
	}
	if q.Expanded && !q.ExpansionSurcharge.IsZero() {
		rows = append(rows, Row{Key: RowExpansionSurcharge, Amount: q.ExpansionSurcharge})
	}
	if q.FreeDelivery {
		rows = append(rows, Row{Key: RowFreeDelivery, Amount: money.Taka(-q.DeliveryBeforeWaiver.Minor())})
	}
	return append(rows, Row{Key: RowTotal, Amount: q.Total})
}
