package payment

import (
	"context"
	"testing"
)

// ascii reports whether a rendered label is plain English text. Bengali is the
// default everywhere (1.4), so a label with no non-ASCII rune in it is either
// English or a translation somebody forgot.
func ascii(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// Every payment status has a label in both languages — walked through the
// whole lifecycle a payment actually goes through, not by calling the label
// function directly, since a screen only ever sees it composed onto a view.
func TestEveryPaymentStatusLabelIsRenderedInBothLanguages(t *testing.T) {
	ctx := context.Background()
	for _, lang := range []string{"bn", "en"} {
		r, paymentID := startedCheckout(t, 50000)

		pending, err := r.reads.ForOrder(ctx, "ord_1", "usr_1", false, lang)
		if err != nil {
			t.Fatalf("ForOrder (pending): %v", err)
		}

		failedPayload, failedSig := sign(r.gateway.validSignature, webhookPayload{
			Reference: paymentID, Succeeded: false, Reason: "declined",
		})
		if err := r.webhook.Handle(ctx, failedPayload, failedSig); err != nil {
			t.Fatalf("Handle: %v", err)
		}
		failed, err := r.reads.ForOrder(ctx, "ord_1", "usr_1", false, lang)
		if err != nil {
			t.Fatalf("ForOrder (failed): %v", err)
		}

		// A fresh order and a fresh checkout for captured, then refunded.
		rc, capturedID := startedCheckout(t, 50000)
		capturedPayload, capturedSig := sign(rc.gateway.validSignature, webhookPayload{
			Reference: capturedID, GatewayRef: "gw_1", Succeeded: true, AmountMinor: 50000,
		})
		if err := rc.webhook.Handle(ctx, capturedPayload, capturedSig); err != nil {
			t.Fatalf("Handle: %v", err)
		}
		captured, err := rc.reads.ForOrder(ctx, "ord_1", "usr_1", false, lang)
		if err != nil {
			t.Fatalf("ForOrder (captured): %v", err)
		}

		if err := rc.refunds.Refund(ctx, "ord_1", "a customer complaint"); err != nil {
			t.Fatalf("Refund: %v", err)
		}
		refunded, err := rc.reads.ForOrder(ctx, "ord_1", "usr_1", false, lang)
		if err != nil {
			t.Fatalf("ForOrder (refunded): %v", err)
		}

		cases := []struct {
			name, status, label string
		}{
			{"pending", pending.Status, pending.StatusLabel},
			{"failed", failed.Status, failed.StatusLabel},
			{"captured", captured.Status, captured.StatusLabel},
			{"refunded", refunded.Status, refunded.StatusLabel},
		}
		for _, tc := range cases {
			if tc.label == "" {
				t.Fatalf("%s: no label at all", tc.name)
			}
			if ascii(tc.label) != (lang == "en") {
				t.Fatalf("%s in %s produced %q", tc.name, lang, tc.label)
			}
		}
	}
}

// The one collection label a screen ever renders: what a rider is holding, in
// both languages.
func TestTheHeldCollectionLabelIsRenderedInBothLanguages(t *testing.T) {
	ctx := context.Background()
	for _, lang := range []string{"bn", "en"} {
		r := newRig()
		seedHeld(t, r, "COL-1", "ord_1", "PTR-1", 30000)

		ledger, err := r.collections.MyLedger(ctx, "PTR-1", lang)
		if err != nil {
			t.Fatalf("MyLedger: %v", err)
		}
		if len(ledger.Held) != 1 || ledger.Held[0].StatusLabel == "" {
			t.Fatalf("ledger = %+v", ledger)
		}
		if ascii(ledger.Held[0].StatusLabel) != (lang == "en") {
			t.Fatalf("held label in %s produced %q", lang, ledger.Held[0].StatusLabel)
		}
	}
}
