// Package http exposes payment to customers, riders and operations.
//
// One route is unlike every other endpoint in this system: the webhook is
// reached by a gateway's own servers, never by a signed-in caller, so it is
// mounted with no identity guard at all and authenticates the payload's own
// signature instead (2.7 does not apply to a caller that is not a person).
package http

import (
	"io"
	"net/http"
	"sort"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/application"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// maxWebhookBytes caps a gateway callback the same way httpx.DecodeJSON caps
// every other request body — this route reads raw bytes itself because a
// webhook's signature is verified over the exact body sent, which strict JSON
// decoding does not preserve.
const maxWebhookBytes = 1 << 20

// Guard wraps a handler in an access requirement. Passed in rather than
// imported, so payment does not depend on identity's transport.
type Guard func(http.Handler) http.Handler

// PrincipalOf reports the authenticated caller's user id.
type PrincipalOf func(*http.Request) (userID string, ok bool)

// Handler serves the payment endpoints.
type Handler struct {
	checkout    *application.CheckoutUseCase
	webhook     *application.WebhookUseCase
	collections *application.CollectionUseCase
	refunds     *application.RefundUseCase
	reads       *application.ReadUseCase
	authed      Guard
	admin       Guard
	principal   PrincipalOf
	// devTools mounts the manual gateway's completion route. Never set in a
	// production deployment (cmd/api/main.go) — see WebhookUseCase.Simulate
	// for the second, independent check.
	devTools bool
}

// NewHandler builds the handler.
func NewHandler(
	checkout *application.CheckoutUseCase,
	webhook *application.WebhookUseCase,
	collections *application.CollectionUseCase,
	refunds *application.RefundUseCase,
	reads *application.ReadUseCase,
	authed, admin Guard,
	principal PrincipalOf,
	devTools bool,
) *Handler {
	if authed == nil || admin == nil || principal == nil {
		panic("payment transport: guards and a principal reader are required")
	}
	return &Handler{
		checkout: checkout, webhook: webhook, collections: collections,
		refunds: refunds, reads: reads,
		authed: authed, admin: admin, principal: principal, devTools: devTools,
	}
}

// customerRoutes are the app the person who is paying uses.
func (h *Handler) customerRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /v1/payments/checkout": h.startCheckout,
		"GET /v1/payments/{orderId}": h.myPayment,
		"GET /v1/partner/cod":        h.myLedger,
	}
}

// adminRoutes are platform operations.
func (h *Handler) adminRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/admin/payments/{orderId}":                  h.adminPayment,
		"POST /v1/admin/payments/{orderId}/refund":          h.refund,
		"GET /v1/admin/payments/cod/{partnerId}":            h.adminLedger,
		"POST /v1/admin/payments/cod/{partnerId}/reconcile": h.reconcile,
	}
}

// Register mounts the routes behind their guards.
//
// Two routes are outside Patterns() on purpose, and mounted here directly
// rather than through customerRoutes(): the webhook, because it is reached by
// a gateway's own servers and authenticated by its signature rather than a
// guard, and the manual gateway's dev-only completion route, which exists
// only outside production and is not part of the API this module documents —
// see WebhookUseCase.Simulate for why a route that vanished from wiring is not
// the only thing standing between it and production.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.customerRoutes() {
		mux.Handle(pattern, h.authed(handle))
	}
	for pattern, handle := range h.adminRoutes() {
		mux.Handle(pattern, h.admin(handle))
	}
	mux.HandleFunc("POST /v1/payments/manual/webhook", h.receiveWebhook)
	if h.devTools {
		mux.Handle(completeRoute, h.authed(http.HandlerFunc(h.simulate)))
	}
}

// completeRoute is the manual gateway's completion route.
//
// It used to be left out of Patterns() on the grounds that it was not part of
// the published API. That stopped being true when the customer app started
// calling it: a demo deployment tells the app it may complete a payment, and
// an endpoint a shipped client calls is part of the contract whether or not
// every deployment mounts it. It is documented, with a description saying it
// exists only outside production, and `demo_completion` on the checkout is
// how a client learns whether it is there.
const completeRoute = "POST /v1/payments/manual/complete"

// Patterns returns every route this module documents in the API contract,
// sorted.
func Patterns() []string {
	empty := &Handler{}
	out := make([]string, 0, 16)
	for pattern := range empty.customerRoutes() {
		out = append(out, pattern)
	}
	for pattern := range empty.adminRoutes() {
		out = append(out, pattern)
	}
	out = append(out, "POST /v1/payments/manual/webhook", completeRoute)
	sort.Strings(out)
	return out
}

// ------------------------------------------------------------------- wire

type moneyBody struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency"`
	Display  string `json:"display"`
}

func toMoneyBody(m application.MoneyView) moneyBody {
	return moneyBody{Minor: m.Minor, Currency: m.Currency, Display: m.Display}
}

type checkoutRequest struct {
	OrderID string `json:"order_id"`
}

type checkoutBody struct {
	PaymentID   string    `json:"payment_id"`
	OrderID     string    `json:"order_id"`
	Status      string    `json:"status"`
	StatusLabel string    `json:"status_label"`
	Amount      moneyBody `json:"amount"`
	RedirectURL string    `json:"redirect_url,omitempty"`
	// DemoCompletion tells a client it may complete this payment itself,
	// which is true only where the gateway is the stand-in *and* the route
	// that does it is mounted. Neither is ever so in production.
	DemoCompletion bool `json:"demo_completion,omitempty"`
}

// toCheckoutBody takes devTools rather than reading a package variable,
// because the two halves of "may a client complete this?" are known in two
// different layers: whether the gateway is the stand-in is the use case's
// answer, and whether the route exists is this one's.
func toCheckoutBody(v application.CheckoutView, devTools bool) checkoutBody {
	return checkoutBody{
		PaymentID: v.PaymentID, OrderID: v.OrderID,
		Status: v.Status, StatusLabel: v.StatusLabel,
		Amount: toMoneyBody(v.Amount), RedirectURL: v.RedirectURL,
		DemoCompletion: v.DemoCompletion && devTools,
	}
}

type paymentBody struct {
	ID          string    `json:"id"`
	OrderID     string    `json:"order_id"`
	Status      string    `json:"status"`
	StatusLabel string    `json:"status_label"`
	Amount      moneyBody `json:"amount"`
	Reason      string    `json:"reason,omitempty"`
}

func toPaymentBody(v application.PaymentView) paymentBody {
	return paymentBody{
		ID: v.ID, OrderID: v.OrderID,
		Status: v.Status, StatusLabel: v.StatusLabel,
		Amount: toMoneyBody(v.Amount), Reason: v.Reason,
	}
}

type collectionBody struct {
	ID            string    `json:"id"`
	OrderID       string    `json:"order_id"`
	Amount        moneyBody `json:"amount"`
	Status        string    `json:"status"`
	StatusLabel   string    `json:"status_label"`
	RemittanceRef string    `json:"remittance_ref,omitempty"`
}

type ledgerBody struct {
	PartnerID   string           `json:"partner_id"`
	Outstanding moneyBody        `json:"outstanding"`
	Remitted    moneyBody        `json:"remitted"`
	Held        []collectionBody `json:"held"`
}

func toLedgerBody(v application.LedgerView) ledgerBody {
	held := make([]collectionBody, 0, len(v.Held))
	for _, c := range v.Held {
		held = append(held, collectionBody{
			ID: c.ID, OrderID: c.OrderID, Amount: toMoneyBody(c.Amount),
			Status: c.Status, StatusLabel: c.StatusLabel, RemittanceRef: c.RemittanceRef,
		})
	}
	return ledgerBody{
		PartnerID: v.PartnerID, Outstanding: toMoneyBody(v.Outstanding),
		Remitted: toMoneyBody(v.Remitted), Held: held,
	}
}

type reconcileRequest struct {
	CollectionIDs []string `json:"collection_ids"`
	Reference     string   `json:"reference"`
}

type refundRequest struct {
	Reason string `json:"reason"`
}

type simulateRequest struct {
	PaymentID string `json:"payment_id"`
	Succeeded bool   `json:"succeeded"`
	Reason    string `json:"reason"`
}

// --------------------------------------------------------------- handlers

func (h *Handler) startCheckout(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body checkoutRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := h.checkout.Start(r.Context(), userID, body.OrderID, lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCheckoutBody(view, h.devTools))
}

func (h *Handler) myPayment(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	view, err := h.reads.ForOrder(r.Context(), r.PathValue("orderId"), userID, false, lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPaymentBody(view))
}

func (h *Handler) adminPayment(w http.ResponseWriter, r *http.Request) {
	view, err := h.reads.ForOrder(r.Context(), r.PathValue("orderId"), "", true, lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPaymentBody(view))
}

func (h *Handler) refund(w http.ResponseWriter, r *http.Request) {
	var body refundRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.refunds.Refund(r.Context(), r.PathValue("orderId"), body.Reason); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := h.reads.ForOrder(r.Context(), r.PathValue("orderId"), "", true, lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPaymentBody(view))
}

func (h *Handler) myLedger(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	view, err := h.collections.MyLedgerForUser(r.Context(), userID, lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toLedgerBody(view))
}

func (h *Handler) adminLedger(w http.ResponseWriter, r *http.Request) {
	view, err := h.collections.MyLedger(r.Context(), r.PathValue("partnerId"), lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toLedgerBody(view))
}

func (h *Handler) reconcile(w http.ResponseWriter, r *http.Request) {
	var body reconcileRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := h.collections.Reconcile(r.Context(), r.PathValue("partnerId"), body.CollectionIDs, body.Reference, lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toLedgerBody(view))
}

// receiveWebhook is the one route in this system a person never calls.
func (h *Handler) receiveWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, errs.Wrap(err, errs.KindInvalid, "request_too_large",
			"That request was too large."))
		return
	}
	signature := r.Header.Get("X-Webhook-Signature")
	if err := h.webhook.Handle(r.Context(), payload, signature); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

func (h *Handler) simulate(w http.ResponseWriter, r *http.Request) {
	customerID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body simulateRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.webhook.Simulate(r.Context(), body.PaymentID, customerID, body.Succeeded, body.Reason); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

// caller reads the verified user id, refusing when there is none.
func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := h.principal(r)
	if !ok || userID == "" {
		httpx.WriteError(w, errs.New(errs.KindUnauthorized, "unauthenticated",
			"Please sign in."))
		return "", false
	}
	return userID, true
}

// lang reads the requested language. Anything but "en" is Bengali (1.4).
func lang(r *http.Request) string { return r.URL.Query().Get("lang") }
