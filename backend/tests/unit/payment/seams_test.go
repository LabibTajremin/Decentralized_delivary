package payment

import (
	"context"
	"testing"

	ordercontract "github.com/rootlogic-lab/delivery/backend/internal/modules/order/contract"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/external/order"
)

// These exercise the one seam in this module that cannot be simplified away
// by structural typing (unlike payment/external/dispatch, dropped in favour of
// wiring dispatch's own Service in directly): payment's isolation rule (2.6)
// means what order hands across this boundary is copied into a type this
// module owns, never order's own contract.Order, so there is a real adapter
// here with real translation to get right.

// fakeOrderContract stands in for the order module's OrderContract.
type fakeOrderContract struct {
	orders map[string]ordercontract.Order

	orderErr      error
	markPaidErr   error
	markFailedErr error

	paidCalls   []string
	failedCalls []struct{ orderID, reason string }
}

func (f *fakeOrderContract) Order(_ context.Context, orderID string) (ordercontract.Order, error) {
	if f.orderErr != nil {
		return ordercontract.Order{}, f.orderErr
	}
	o, ok := f.orders[orderID]
	if !ok {
		return ordercontract.Order{}, errBoom
	}
	return o, nil
}

func (f *fakeOrderContract) MarkPaid(_ context.Context, orderID string) error {
	if f.markPaidErr != nil {
		return f.markPaidErr
	}
	f.paidCalls = append(f.paidCalls, orderID)
	return nil
}

func (f *fakeOrderContract) MarkPaymentFailed(_ context.Context, orderID, reason string) error {
	if f.markFailedErr != nil {
		return f.markFailedErr
	}
	f.failedCalls = append(f.failedCalls, struct{ orderID, reason string }{orderID, reason})
	return nil
}

func (f *fakeOrderContract) Advance(context.Context, string, string, string, string, string) error {
	return nil
}

var _ ordercontract.OrderContract = (*fakeOrderContract)(nil)

func TestTheOrderSeamCopiesFieldsRatherThanAliasing(t *testing.T) {
	ctx := context.Background()
	fake := &fakeOrderContract{orders: map[string]ordercontract.Order{
		"ord_1": {
			ID: "ord_1", CustomerID: "usr_1", Status: "pending_payment",
			PaymentMethod: "online",
			Total:         ordercontract.Money{Minor: 50000, Currency: "BDT", Display: "৳ 500"},
		},
	}}
	seam := orderx.New(fake)

	got, err := seam.Order(ctx, "ord_1")
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if got.ID != "ord_1" || got.CustomerID != "usr_1" || got.Status != "pending_payment" ||
		got.PaymentMethod != "online" || got.TotalMinor != 50000 || got.Currency != "BDT" {
		t.Fatalf("order = %+v", got)
	}

	if _, err := seam.Order(ctx, "ord_missing"); err == nil {
		t.Fatal("a missing order was not refused")
	}

	if err := seam.MarkPaid(ctx, "ord_1"); err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}
	if len(fake.paidCalls) != 1 || fake.paidCalls[0] != "ord_1" {
		t.Fatalf("paidCalls = %v", fake.paidCalls)
	}

	if err := seam.MarkPaymentFailed(ctx, "ord_1", "card declined"); err != nil {
		t.Fatalf("MarkPaymentFailed: %v", err)
	}
	if len(fake.failedCalls) != 1 || fake.failedCalls[0].reason != "card declined" {
		t.Fatalf("failedCalls = %+v", fake.failedCalls)
	}
}
