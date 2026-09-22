// Package manual is a stand-in payment gateway.
//
// A real gateway adapter belongs here alongside ManualGateway, behind the same
// ports.Gateway interface. It is not written yet because no provider has been
// chosen, and a speculative adapter for the wrong API is worse than none — the
// seam is what matters, and it exists (2.4). This is the same shape as
// identity's LogSender (infrastructure/sms): a working stand-in that refuses
// to run where it would matter.
package manual

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application/ports"
)

// ErrNotForProduction is returned when the manual gateway is constructed in a
// production deployment.
var ErrNotForProduction = errors.New("the manual gateway must not be used in production")

// ErrBadSignature means a webhook's signature did not verify.
var ErrBadSignature = errors.New("that webhook's signature does not verify")

// Name is what payments through this gateway are tagged with.
const Name = "manual"

// Gateway signs its own webhooks with a shared secret, standing in for the
// HMAC signing every real provider uses. It never moves money: Checkout
// returns a reference and nothing else, and completing a checkout means
// something else in this process — the transport layer's dev-only "complete"
// route — builds and signs the same payload a real gateway would send, and
// feeds it through the same webhook path a real one's callback would use. The
// webhook verification and the idempotency it protects are exercised for
// real; only the part no adapter can stand in for, an actual bank, is not
// there.
type Gateway struct {
	secret []byte
}

// New builds the manual gateway.
//
// Refuses to be constructed in production for the same reason LogSender does:
// a gateway that always succeeds is a gateway that has stopped meaning
// anything, and in production that silence is a customer who paid nothing for
// an order marked paid.
func New(secret string, production bool) (*Gateway, error) {
	if production {
		return nil, ErrNotForProduction
	}
	return &Gateway{secret: []byte(secret)}, nil
}

// Name identifies this gateway on every payment it touches.
func (g *Gateway) Name() string { return Name }

// Checkout starts collecting an amount. The manual gateway has no hosted page
// to redirect to, so RedirectURL is empty and GatewayRef echoes the reference
// it was given — a real adapter fills both from its own API's response.
func (g *Gateway) Checkout(_ context.Context, reference string, _ int64, _ string) (ports.CheckoutResult, error) {
	return ports.CheckoutResult{GatewayRef: reference}, nil
}

// webhookPayload is the wire shape of a manual-gateway callback.
type webhookPayload struct {
	Reference   string `json:"reference"`
	GatewayRef  string `json:"gateway_ref"`
	Succeeded   bool   `json:"succeeded"`
	Reason      string `json:"reason,omitempty"`
	AmountMinor int64  `json:"amount_minor"`
}

// Sign produces the signature this gateway's webhook expects for a payload —
// exported so the dev-only "complete" route can build one exactly as this
// gateway's real counterpart would, rather than reaching into this package's
// internals to fake it.
func Sign(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhook checks the signature and decodes the payload.
func (g *Gateway) VerifyWebhook(_ context.Context, payload []byte, signature string) (ports.WebhookEvent, error) {
	want := Sign(string(g.secret), payload)
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return ports.WebhookEvent{}, ErrBadSignature
	}
	var body webhookPayload
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		return ports.WebhookEvent{}, err
	}
	return ports.WebhookEvent{
		Reference: body.Reference, GatewayRef: body.GatewayRef,
		Succeeded: body.Succeeded, Reason: body.Reason,
		AmountMinor: body.AmountMinor,
	}, nil
}

// Refund gives money back. The manual gateway holds none, so it always
// succeeds — a real adapter calls its provider's own refund endpoint here.
func (g *Gateway) Refund(_ context.Context, gatewayRef string, _ int64) (ports.RefundResult, error) {
	return ports.RefundResult{RefundRef: gatewayRef}, nil
}
