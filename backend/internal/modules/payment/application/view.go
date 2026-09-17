package application

import (
	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Everything a screen renders about a payment, already decided (2.9). Amounts
// cross the wire as the minor-unit integer and the rendered string together, so
// nothing on the other side of this module ever formats money.

// MoneyView is an amount as a screen shows it.
type MoneyView struct {
	Minor    int64
	Currency string
	Display  string
}

func moneyView(m money.Money) MoneyView {
	return MoneyView{Minor: m.Minor(), Currency: string(m.Currency()), Display: m.Display()}
}

// CheckoutView is what starting, or resuming, a checkout gets a customer.
type CheckoutView struct {
	PaymentID   string
	OrderID     string
	Status      string
	StatusLabel string
	Amount      MoneyView
	// RedirectURL is empty on a resumed checkout whose attempt never recorded
	// one, which a screen treats the same as "keep waiting".
	RedirectURL string
}

func (uc *CheckoutUseCase) viewOf(p domain.Payment, lang string) CheckoutView {
	return CheckoutView{
		PaymentID: p.ID, OrderID: p.OrderID,
		Status: string(p.Status), StatusLabel: statusLabel(p.Status, lang),
		Amount: moneyView(p.Amount),
	}
}

func checkoutViewOf(p domain.Payment, redirectURL, lang string) CheckoutView {
	return CheckoutView{
		PaymentID: p.ID, OrderID: p.OrderID,
		Status: string(p.Status), StatusLabel: statusLabel(p.Status, lang),
		Amount: moneyView(p.Amount), RedirectURL: redirectURL,
	}
}

// PaymentView is a payment as a screen reads it, in full.
type PaymentView struct {
	ID          string
	OrderID     string
	Status      string
	StatusLabel string
	Amount      MoneyView
	Reason      string
}

func paymentViewOf(p domain.Payment, lang string) PaymentView {
	return PaymentView{
		ID: p.ID, OrderID: p.OrderID,
		Status: string(p.Status), StatusLabel: statusLabel(p.Status, lang),
		Amount: moneyView(p.Amount), Reason: settledReason(p),
	}
}

// CollectionView is one COD collection.
type CollectionView struct {
	ID            string
	OrderID       string
	Amount        MoneyView
	Status        string
	StatusLabel   string
	RemittanceRef string
}

// collectionViewOf renders one held collection — the only status this ever
// sees, since ledgerViewOf only calls it over Ledger.Held. A remitted
// collection has no view today: the ledger reports its money in the Remitted
// total and nothing shows its row again, so there is no "remitted" label to
// compose here.
func collectionViewOf(c domain.Collection, lang string) CollectionView {
	return CollectionView{
		ID: c.ID, OrderID: c.OrderID, Amount: moneyView(c.Amount),
		Status: string(c.Status), StatusLabel: collectionStatusLabel(lang),
		RemittanceRef: c.RemittanceRef,
	}
}

// LedgerView is a partner's cash position.
type LedgerView struct {
	PartnerID   string
	Outstanding MoneyView
	Remitted    MoneyView
	Held        []CollectionView
}

func ledgerViewOf(l domain.Ledger, lang string) LedgerView {
	view := LedgerView{
		PartnerID:   l.PartnerID,
		Outstanding: moneyView(l.Outstanding),
		Remitted:    moneyView(l.Remitted),
	}
	for _, c := range l.Held {
		view.Held = append(view.Held, collectionViewOf(c, lang))
	}
	return view
}

// ------------------------------------------------------------------ words

func statusLabel(s domain.Status, lang string) string {
	bengali := lang != "en"
	switch s {
	case domain.StatusPending:
		if bengali {
			return "পরিশোধের অপেক্ষায়"
		}
		return "Waiting for payment"
	case domain.StatusCaptured:
		if bengali {
			return "পরিশোধ সম্পন্ন"
		}
		return "Paid"
	case domain.StatusFailed:
		if bengali {
			return "পরিশোধ ব্যর্থ হয়েছে"
		}
		return "Payment failed"
	default:
		if bengali {
			return "টাকা ফেরত দেওয়া হয়েছে"
		}
		return "Refunded"
	}
}

func collectionStatusLabel(lang string) string {
	if lang != "en" {
		return "আপনার কাছে আছে"
	}
	return "With you"
}
