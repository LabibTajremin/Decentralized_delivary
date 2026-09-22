package payment

import (
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

var at = time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

func taka(minor int64) money.Money { return money.Taka(minor) }

// ------------------------------------------------------------------ Payment

func TestNewPaymentRefusesWhatItMust(t *testing.T) {
	cases := []struct {
		name                     string
		order, customer, gateway string
		amount                   money.Money
		want                     error
	}{
		{"no order", "", "usr_1", "manual", taka(1000), domain.ErrNoOrder},
		{"no customer", "ord_1", "", "manual", taka(1000), domain.ErrNoCustomer},
		{"no gateway", "ord_1", "usr_1", "", taka(1000), domain.ErrNoGateway},
		{"zero amount", "ord_1", "usr_1", "manual", taka(0), domain.ErrNotPositive},
		{"negative amount", "ord_1", "usr_1", "manual", money.Money{}, domain.ErrNotPositive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := domain.NewPayment("PAY-1", tc.order, tc.customer, tc.gateway, tc.amount, at); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	p, err := domain.NewPayment("PAY-1", "ord_1", "usr_1", "manual", taka(50000), at)
	if err != nil {
		t.Fatalf("NewPayment: %v", err)
	}
	if p.Status != domain.StatusPending || p.Reference != "PAY-1" {
		t.Fatalf("payment = %+v", p)
	}
}

func payment(t *testing.T) domain.Payment {
	t.Helper()
	p, err := domain.NewPayment("PAY-1", "ord_1", "usr_1", "manual", taka(50000), at)
	if err != nil {
		t.Fatalf("NewPayment: %v", err)
	}
	return p
}

func TestCapturingAPayment(t *testing.T) {
	p := payment(t)
	if err := p.Capture("gw_ref_1", taka(50000), at.Add(time.Minute)); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if p.Status != domain.StatusCaptured || p.GatewayRef != "gw_ref_1" {
		t.Fatalf("payment = %+v", p)
	}

	// A payment can be captured exactly once.
	if err := p.Capture("gw_ref_2", taka(50000), at); !errors.Is(err, domain.ErrNotPending) {
		t.Fatalf("err = %v, want ErrNotPending", err)
	}
}

// The gateway's word on the amount must agree with what was asked for. A
// gateway confirming a different sum is refused rather than trusted.
func TestCapturingTheWrongAmount(t *testing.T) {
	p := payment(t)
	if err := p.Capture("gw_ref_1", taka(40000), at); !errors.Is(err, domain.ErrAmountMismatch) {
		t.Fatalf("err = %v, want ErrAmountMismatch", err)
	}
	if p.Status != domain.StatusPending {
		t.Fatalf("a mismatched capture changed the status: %+v", p)
	}
}

func TestFailingAPayment(t *testing.T) {
	p := payment(t)
	if err := p.Fail("", at); !errors.Is(err, domain.ErrNoReason) {
		t.Fatalf("err = %v, want ErrNoReason", err)
	}
	if err := p.Fail("card declined", at); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if p.Status != domain.StatusFailed || p.FailureReason != "card declined" {
		t.Fatalf("payment = %+v", p)
	}
	if err := p.Fail("card declined again", at); !errors.Is(err, domain.ErrNotPending) {
		t.Fatalf("err = %v, want ErrNotPending", err)
	}
}

func TestRefundingAPayment(t *testing.T) {
	p := payment(t)
	if err := p.Refund("changed their mind", at); !errors.Is(err, domain.ErrNotCaptured) {
		t.Fatalf("refunding a pending payment: err = %v, want ErrNotCaptured", err)
	}

	if err := p.Capture("gw_ref_1", taka(50000), at); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if err := p.Refund("", at); !errors.Is(err, domain.ErrNoReason) {
		t.Fatalf("err = %v, want ErrNoReason", err)
	}
	if err := p.Refund("changed their mind", at.Add(time.Hour)); err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if p.Status != domain.StatusRefunded || p.RefundReason != "changed their mind" {
		t.Fatalf("payment = %+v", p)
	}
	if err := p.Refund("again", at); !errors.Is(err, domain.ErrNotCaptured) {
		t.Fatalf("a second refund: err = %v, want ErrNotCaptured", err)
	}
}

func TestStatusIsTerminal(t *testing.T) {
	cases := map[domain.Status]bool{
		domain.StatusPending:  false,
		domain.StatusCaptured: false,
		domain.StatusFailed:   true,
		domain.StatusRefunded: true,
	}
	for status, want := range cases {
		if got := status.IsTerminal(); got != want {
			t.Errorf("%s.IsTerminal() = %v, want %v", status, got, want)
		}
	}
}

// ------------------------------------------------------------------ Collection

func TestNewCollectionRefusesWhatItMust(t *testing.T) {
	cases := []struct {
		name           string
		order, partner string
		amount         money.Money
		want           error
	}{
		{"no order", "", "PTR-1", taka(1000), domain.ErrNoOrder},
		{"no partner", "ord_1", "", taka(1000), domain.ErrNoPartner},
		{"zero amount", "ord_1", "PTR-1", taka(0), domain.ErrNotPositive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := domain.NewCollection("COL-1", tc.order, tc.partner, tc.amount, at); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	c, err := domain.NewCollection("COL-1", "ord_1", "PTR-1", taka(50000), at)
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	if c.Status != domain.StatusHeld || c.CollectedAt != at {
		t.Fatalf("collection = %+v", c)
	}
}

func collection(t *testing.T) domain.Collection {
	t.Helper()
	c, err := domain.NewCollection("COL-1", "ord_1", "PTR-1", taka(50000), at)
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	return c
}

func TestRemittingACollection(t *testing.T) {
	c := collection(t)

	// Only the partner who collected it may remit it.
	if err := c.Remit("PTR-2", "REF-1", at); !errors.Is(err, domain.ErrWrongPartner) {
		t.Fatalf("err = %v, want ErrWrongPartner", err)
	}
	if err := c.Remit("PTR-1", "", at); !errors.Is(err, domain.ErrNoRemittanceRef) {
		t.Fatalf("err = %v, want ErrNoRemittanceRef", err)
	}
	if err := c.Remit("PTR-1", "REF-1", at.Add(time.Hour)); err != nil {
		t.Fatalf("Remit: %v", err)
	}
	if c.Status != domain.StatusRemitted || c.RemittanceRef != "REF-1" || c.RemittedAt.IsZero() {
		t.Fatalf("collection = %+v", c)
	}
	if err := c.Remit("PTR-1", "REF-2", at); !errors.Is(err, domain.ErrAlreadyRemitted) {
		t.Fatalf("a second remittance: err = %v, want ErrAlreadyRemitted", err)
	}
}

// Summarize is the ledger, recomputed from a partner's own collections rather
// than kept as a stored figure.
func TestSummarizingALedger(t *testing.T) {
	held1, _ := domain.NewCollection("COL-1", "ord_1", "PTR-1", taka(30000), at)
	held2, _ := domain.NewCollection("COL-2", "ord_2", "PTR-1", taka(20000), at)
	remitted, _ := domain.NewCollection("COL-3", "ord_3", "PTR-1", taka(15000), at)
	_ = remitted.Remit("PTR-1", "REF-1", at)

	ledger := domain.Summarize("PTR-1", []domain.Collection{held1, held2, remitted})
	if ledger.PartnerID != "PTR-1" {
		t.Errorf("partner id = %q", ledger.PartnerID)
	}
	if ledger.Outstanding.Minor() != 50000 {
		t.Errorf("outstanding = %d, want 50000", ledger.Outstanding.Minor())
	}
	if ledger.Remitted.Minor() != 15000 {
		t.Errorf("remitted = %d, want 15000", ledger.Remitted.Minor())
	}
	if len(ledger.Held) != 2 {
		t.Fatalf("held = %+v, want 2 entries", ledger.Held)
	}

	// An empty ledger is zero, not an error and not nil money.
	empty := domain.Summarize("PTR-2", nil)
	if !empty.Outstanding.IsZero() || !empty.Remitted.IsZero() || empty.Held != nil {
		t.Fatalf("empty ledger = %+v", empty)
	}
}
