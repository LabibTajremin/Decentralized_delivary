package payment

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/infrastructure/gateway/manual"
)

// TestTheManualGatewayRefusesProduction is the guard that matters. A gateway
// that always succeeds is a gateway that has stopped meaning anything, and in
// production that silence is a customer who paid nothing for an order marked
// paid.
func TestTheManualGatewayRefusesProduction(t *testing.T) {
	gw, err := manual.New("a-secret", true)
	if !errors.Is(err, manual.ErrNotForProduction) {
		t.Fatalf("error = %v, want ErrNotForProduction", err)
	}
	if gw != nil {
		t.Error("a gateway was returned for production")
	}
}

func TestTheManualGatewayNamesItself(t *testing.T) {
	gw, err := manual.New("a-secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if gw.Name() != "manual" {
		t.Errorf("Name() = %q", gw.Name())
	}
}

func TestTheManualGatewayChecksOut(t *testing.T) {
	gw, err := manual.New("a-secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := gw.Checkout(context.Background(), "PAY-1", 50000, "BDT")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	// No real gateway to redirect to: the reference is echoed back as the
	// gateway's own, and it is up to the demo completion route to finish it.
	if result.GatewayRef != "PAY-1" {
		t.Errorf("GatewayRef = %q, want the reference echoed back", result.GatewayRef)
	}
}

func TestTheManualGatewayVerifiesItsOwnSignature(t *testing.T) {
	gw, err := manual.New("a-secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	payload, _ := json.Marshal(map[string]any{
		"reference": "PAY-1", "gateway_ref": "PAY-1",
		"succeeded": true, "amount_minor": 50000,
	})
	signature := manual.Sign("a-secret", payload)

	event, err := gw.VerifyWebhook(context.Background(), payload, signature)
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if event.Reference != "PAY-1" || !event.Succeeded || event.AmountMinor != 50000 {
		t.Fatalf("event = %+v", event)
	}

	// A different secret produces a different signature, and a tampered
	// payload does not verify against a signature computed for another body —
	// forging either half alone is not enough.
	if _, err := gw.VerifyWebhook(context.Background(), payload, manual.Sign("a-different-secret", payload)); !errors.Is(err, manual.ErrBadSignature) {
		t.Fatalf("err = %v, want ErrBadSignature", err)
	}
	tampered, _ := json.Marshal(map[string]any{
		"reference": "PAY-1", "gateway_ref": "PAY-1",
		"succeeded": true, "amount_minor": 99999999,
	})
	if _, err := gw.VerifyWebhook(context.Background(), tampered, signature); !errors.Is(err, manual.ErrBadSignature) {
		t.Fatalf("a tampered payload verified against the original signature: err = %v", err)
	}
}

func TestTheManualGatewayRefusesAMalformedPayload(t *testing.T) {
	gw, err := manual.New("a-secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	payload := []byte(`{"reference": "PAY-1", "unknown_field": true}`)
	signature := manual.Sign("a-secret", payload)
	if _, err := gw.VerifyWebhook(context.Background(), payload, signature); err == nil {
		t.Fatal("an unknown field in a verified payload was silently accepted")
	}
}

func TestTheManualGatewayRefunds(t *testing.T) {
	gw, err := manual.New("a-secret", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := gw.Refund(context.Background(), "gw_ref_1", 50000)
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if result.RefundRef != "gw_ref_1" {
		t.Errorf("RefundRef = %q", result.RefundRef)
	}
}
