package payment

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// startedCheckout is a rig with an open checkout already running, and the
// payment id it minted.
func startedCheckout(t *testing.T, totalMinor int64) (*rig, string) {
	t.Helper()
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = pendingOrder("ord_1", "usr_1", totalMinor)
	view, err := r.checkout.Start(ctx, "usr_1", "ord_1", "en")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return r, view.PaymentID
}

func TestAWebhookCapturesThePayment(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)

	payload, sig := sign(r.gateway.validSignature, webhookPayload{
		Reference: paymentID, GatewayRef: "gw_txn_1", Succeeded: true, AmountMinor: 50000,
	})
	if err := r.webhook.Handle(ctx, payload, sig); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	stored := r.repo.payments[paymentID]
	if stored.Status != domain.StatusCaptured || stored.GatewayRef != "gw_txn_1" {
		t.Fatalf("payment = %+v", stored)
	}
	if len(r.order.paidCalls) != 1 || r.order.paidCalls[0] != "ord_1" {
		t.Fatalf("MarkPaid calls = %v", r.order.paidCalls)
	}
}

func TestAWebhookFailsThePayment(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)

	payload, sig := sign(r.gateway.validSignature, webhookPayload{
		Reference: paymentID, Succeeded: false, Reason: "insufficient funds",
	})
	if err := r.webhook.Handle(ctx, payload, sig); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	stored := r.repo.payments[paymentID]
	if stored.Status != domain.StatusFailed || stored.FailureReason != "insufficient funds" {
		t.Fatalf("payment = %+v", stored)
	}
	if len(r.order.failedCalls) != 1 || r.order.failedCalls[0].reason != "insufficient funds" {
		t.Fatalf("MarkPaymentFailed calls = %+v", r.order.failedCalls)
	}
}

// A failed event with no reason is refused before it touches the payment —
// a rider or a customer is owed something to go on.
func TestAFailedWebhookNeedsAReason(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)

	payload, sig := sign(r.gateway.validSignature, webhookPayload{Reference: paymentID, Succeeded: false})
	if err := r.webhook.Handle(ctx, payload, sig); errs.CodeOf(err) != "reason_required" {
		t.Fatalf("err = %v", err)
	}
	if r.repo.payments[paymentID].Status != domain.StatusPending {
		t.Fatalf("a rejected webhook changed the payment: %+v", r.repo.payments[paymentID])
	}
}

// The one thing a webhook must never do twice: capture, save and notify order
// exactly once, however many times the same event arrives.
func TestAReplayedWebhookIsIdempotent(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)

	payload, sig := sign(r.gateway.validSignature, webhookPayload{
		Reference: paymentID, GatewayRef: "gw_txn_1", Succeeded: true, AmountMinor: 50000,
	})
	for i := 0; i < 3; i++ {
		if err := r.webhook.Handle(ctx, payload, sig); err != nil {
			t.Fatalf("Handle (attempt %d): %v", i+1, err)
		}
	}
	if len(r.order.paidCalls) != 1 {
		t.Fatalf("MarkPaid was called %d times, want 1", len(r.order.paidCalls))
	}
}

// A late "failed" arriving after a successful capture does not undo it — the
// good news, once recorded, stands.
func TestALateFailureAfterCaptureDoesNothing(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)

	captured, sig := sign(r.gateway.validSignature, webhookPayload{
		Reference: paymentID, GatewayRef: "gw_txn_1", Succeeded: true, AmountMinor: 50000,
	})
	if err := r.webhook.Handle(ctx, captured, sig); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	late, sig2 := sign(r.gateway.validSignature, webhookPayload{
		Reference: paymentID, Succeeded: false, Reason: "timed out",
	})
	if err := r.webhook.Handle(ctx, late, sig2); err != nil {
		t.Fatalf("Handle (late): %v", err)
	}
	if r.repo.payments[paymentID].Status != domain.StatusCaptured {
		t.Fatalf("a late failure overturned a capture: %+v", r.repo.payments[paymentID])
	}
	if len(r.order.failedCalls) != 0 {
		t.Fatalf("MarkPaymentFailed was called after the order was already paid: %+v", r.order.failedCalls)
	}
}

// The gateway's confirmed amount must match what was asked for, or the
// payment is not captured.
func TestAWebhookWithTheWrongAmountIsRefused(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)

	payload, sig := sign(r.gateway.validSignature, webhookPayload{
		Reference: paymentID, GatewayRef: "gw_txn_1", Succeeded: true, AmountMinor: 40000,
	})
	if err := r.webhook.Handle(ctx, payload, sig); errs.CodeOf(err) != "amount_mismatch" {
		t.Fatalf("err = %v", err)
	}
	if r.repo.payments[paymentID].Status != domain.StatusPending {
		t.Fatalf("a mismatched webhook changed the payment: %+v", r.repo.payments[paymentID])
	}
}

func TestAWebhookThatDoesNotVerifyIsRefused(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)

	payload, _ := sign(r.gateway.validSignature, webhookPayload{
		Reference: paymentID, Succeeded: true, AmountMinor: 50000,
	})
	if err := r.webhook.Handle(ctx, payload, "wrong-signature"); errs.CodeOf(err) != "invalid_webhook" {
		t.Fatalf("err = %v", err)
	}
}

func TestAWebhookForAnUnknownReferenceIsRefused(t *testing.T) {
	ctx := context.Background()
	r := newRig()

	payload, sig := sign(r.gateway.validSignature, webhookPayload{
		Reference: "PAY-NOBODY", Succeeded: true, AmountMinor: 50000,
	})
	if err := r.webhook.Handle(ctx, payload, sig); errs.CodeOf(err) != "payment_not_found" {
		t.Fatalf("err = %v", err)
	}
}

func TestWebhookStorageFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("the lookup fails", func(t *testing.T) {
		r, paymentID := startedCheckout(t, 50000)
		r.repo.byReferenceErr = errBoom
		payload, sig := sign(r.gateway.validSignature, webhookPayload{
			Reference: paymentID, Succeeded: true, AmountMinor: 50000,
		})
		if err := r.webhook.Handle(ctx, payload, sig); errs.CodeOf(err) != "payment_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the save fails for a reason other than a lost race", func(t *testing.T) {
		r, paymentID := startedCheckout(t, 50000)
		r.repo.savePaymentErr = errBoom
		payload, sig := sign(r.gateway.validSignature, webhookPayload{
			Reference: paymentID, Succeeded: true, AmountMinor: 50000,
		})
		if err := r.webhook.Handle(ctx, payload, sig); errs.CodeOf(err) != "payment_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	// A replayed event that finds the payment already settled never reaches
	// Save at all — Capture or Fail refuses first, and that refusal is itself
	// the idempotent answer.
	t.Run("a replay after settlement never reaches Save", func(t *testing.T) {
		r, paymentID := startedCheckout(t, 50000)
		stored := r.repo.payments[paymentID]
		stored.Status = domain.StatusCaptured
		r.repo.payments[paymentID] = stored
		payload, sig := sign(r.gateway.validSignature, webhookPayload{
			Reference: paymentID, Succeeded: false, Reason: "late",
		})
		if err := r.webhook.Handle(ctx, payload, sig); err != nil {
			t.Fatalf("Handle: %v", err)
		}
	})

	// Two webhooks read the same pending payment before either wrote — the
	// second one's own compare-and-set is what catches the race, and its
	// outcome stands rather than erroring.
	t.Run("the save itself loses a genuine race", func(t *testing.T) {
		r, paymentID := startedCheckout(t, 50000)
		r.repo.savePaymentErr = errs.New(errs.KindConflict, "payment_moved", "That payment has already moved on.")
		payload, sig := sign(r.gateway.validSignature, webhookPayload{
			Reference: paymentID, GatewayRef: "gw_txn_1", Succeeded: true, AmountMinor: 50000,
		})
		if err := r.webhook.Handle(ctx, payload, sig); err != nil {
			t.Fatalf("Handle: %v", err)
		}
	})

	t.Run("order cannot be told", func(t *testing.T) {
		r, paymentID := startedCheckout(t, 50000)
		r.order.markPaidErr = errBoom
		payload, sig := sign(r.gateway.validSignature, webhookPayload{
			Reference: paymentID, Succeeded: true, AmountMinor: 50000,
		})
		if err := r.webhook.Handle(ctx, payload, sig); err == nil {
			t.Fatal("a webhook that could not tell order succeeded silently")
		}
		// The payment itself is still captured — only the notification failed.
		if r.repo.payments[paymentID].Status != domain.StatusCaptured {
			t.Fatalf("payment = %+v", r.repo.payments[paymentID])
		}
	})
}

// ------------------------------------------------------------------ Simulate

func TestSimulateAppliesLikeARealWebhook(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)

	if err := r.webhook.Simulate(ctx, paymentID, "usr_1", true, ""); err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	if r.repo.payments[paymentID].Status != domain.StatusCaptured {
		t.Fatalf("payment = %+v", r.repo.payments[paymentID])
	}
	if len(r.order.paidCalls) != 1 {
		t.Fatalf("MarkPaid calls = %v", r.order.paidCalls)
	}
}

func TestSimulateFailing(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)

	if err := r.webhook.Simulate(ctx, paymentID, "usr_1", false, "declined by demo"); err != nil {
		t.Fatalf("Simulate: %v", err)
	}
	if r.repo.payments[paymentID].Status != domain.StatusFailed {
		t.Fatalf("payment = %+v", r.repo.payments[paymentID])
	}
}

// Simulate refuses to run against anything but the manual gateway — the
// second, independent guard alongside never mounting its route in production.
func TestSimulateRefusesAnyGatewayButManual(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)
	r.gateway.name = "sslcommerz"

	if err := r.webhook.Simulate(ctx, paymentID, "usr_1", true, ""); errs.CodeOf(err) != "dev_only" {
		t.Fatalf("err = %v", err)
	}
	if r.repo.payments[paymentID].Status != domain.StatusPending {
		t.Fatalf("payment = %+v", r.repo.payments[paymentID])
	}
}

func TestSimulateOnAMissingPayment(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.webhook.Simulate(ctx, "PAY-MISSING", "usr_1", true, ""); errs.CodeOf(err) != "payment_not_found" {
		t.Fatalf("err = %v", err)
	}
}

// A demo lets a customer settle their own payment. It does not let them settle
// somebody else's, and the refusal is a not-found rather than a forbidden —
// the same answer an id that never existed gets, so a list of ids cannot be
// used to discover which ones are real.
func TestSimulateRefusesAnotherCustomersPayment(t *testing.T) {
	ctx := context.Background()
	r, paymentID := startedCheckout(t, 50000)

	if err := r.webhook.Simulate(ctx, paymentID, "usr_2", true, ""); errs.CodeOf(err) != "payment_not_found" {
		t.Fatalf("err = %v", err)
	}
	if r.repo.payments[paymentID].Status != domain.StatusPending {
		t.Fatalf("a stranger moved the payment: %+v", r.repo.payments[paymentID])
	}
}
