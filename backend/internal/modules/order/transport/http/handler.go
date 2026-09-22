// Package http exposes orders to the three people who have one.
//
// Three surfaces, one handler. The customer's routes act on their own orders;
// the shop's routes act on its own queue and check ownership against the
// merchant record; the admin's routes see everything. Which set a caller
// reaches is decided by the guard they came through, and *which orders* they
// reach is re-checked in the use case — an endpoint that trusted the guard
// alone would be one refactor away from a shop reading another shop's queue.
package http

import (
	"net/http"
	"sort"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Guard wraps a handler in an access requirement. Passed in rather than
// imported, so the order does not depend on identity's transport.
type Guard func(http.Handler) http.Handler

// PrincipalOf reports the authenticated caller's user id.
type PrincipalOf func(*http.Request) (userID string, ok bool)

// MerchantsOf reports which shops a caller owns.
//
// Passed in rather than read here, because "which shops are yours" is the
// merchant module's answer and order reaching for it directly would be the
// cross-module import 2.5 forbids.
type MerchantsOf func(r *http.Request, userID string) ([]string, error)

// Handler serves the order endpoints.
type Handler struct {
	place       *application.PlaceUseCase
	transitions *application.TransitionUseCase
	reads       *application.ReadUseCase
	authed      Guard
	admin       Guard
	principal   PrincipalOf
	merchants   MerchantsOf
}

// NewHandler builds the handler.
func NewHandler(
	place *application.PlaceUseCase,
	transitions *application.TransitionUseCase,
	reads *application.ReadUseCase,
	authed Guard,
	admin Guard,
	principal PrincipalOf,
	merchants MerchantsOf,
) *Handler {
	if authed == nil || admin == nil || principal == nil || merchants == nil {
		panic("order transport: guards, a principal reader and a merchant lookup are required")
	}
	return &Handler{
		place: place, transitions: transitions, reads: reads,
		authed: authed, admin: admin, principal: principal, merchants: merchants,
	}
}

// customerRoutes are what the person who placed the order calls.
func (h *Handler) customerRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /v1/orders":                       h.placeOrder,
		"GET /v1/orders":                        h.myOrders,
		"GET /v1/orders/{orderId}":              h.myOrder,
		"GET /v1/orders/{orderId}/cancellation": h.cancellation,
		"POST /v1/orders/{orderId}/cancel":      h.cancel,
	}
}

// merchantRoutes are the shop's queue.
func (h *Handler) merchantRoutes() map[string]http.HandlerFunc {
	const base = "/v1/merchants/{merchantId}/orders"
	return map[string]http.HandlerFunc{
		"GET " + base:                           h.shopOrders,
		"GET " + base + "/{orderId}":            h.shopOrder,
		"POST " + base + "/{orderId}/accept":    h.accept,
		"POST " + base + "/{orderId}/reject":    h.reject,
		"POST " + base + "/{orderId}/preparing": h.preparing,
		"POST " + base + "/{orderId}/ready":     h.ready,
	}
}

// adminRoutes are platform operations.
func (h *Handler) adminRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /v1/admin/orders/{orderId}/transition": h.adminTransition,
	}
}

// Register mounts the routes behind their guards.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.customerRoutes() {
		mux.Handle(pattern, h.authed(handle))
	}
	for pattern, handle := range h.merchantRoutes() {
		mux.Handle(pattern, h.authed(handle))
	}
	for pattern, handle := range h.adminRoutes() {
		mux.Handle(pattern, h.admin(handle))
	}
}

// Patterns returns every route this module serves, sorted.
func Patterns() []string {
	empty := &Handler{}
	out := make([]string, 0, 16)
	for pattern := range empty.customerRoutes() {
		out = append(out, pattern)
	}
	for pattern := range empty.merchantRoutes() {
		out = append(out, pattern)
	}
	for pattern := range empty.adminRoutes() {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

// ------------------------------------------------------------------- wire

type moneyBody struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency"`
	Display  string `json:"display"`
}

type optionBody struct {
	Name  string    `json:"name"`
	Price moneyBody `json:"price"`
}

type lineBody struct {
	ID        string       `json:"id"`
	Kind      string       `json:"kind"`
	TargetID  string       `json:"target_id"`
	Name      string       `json:"name"`
	Options   []optionBody `json:"options"`
	Quantity  int          `json:"quantity"`
	Note      string       `json:"note,omitempty"`
	UnitPrice moneyBody    `json:"unit_price"`
	LineTotal moneyBody    `json:"line_total"`
}

type receiptRowBody struct {
	Key    string    `json:"key"`
	Label  string    `json:"label"`
	Amount moneyBody `json:"amount"`
}

type placeBody struct {
	Name       string  `json:"name"`
	Phone      string  `json:"phone,omitempty"`
	SingleLine string  `json:"single_line,omitempty"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
}

type eventBody struct {
	Status string    `json:"status"`
	Label  string    `json:"label"`
	Actor  string    `json:"actor"`
	Reason string    `json:"reason,omitempty"`
	At     time.Time `json:"at"`
}

type cancelBody struct {
	Allowed     bool      `json:"allowed"`
	Reason      string    `json:"reason,omitempty"`
	Text        string    `json:"text,omitempty"`
	SecondsLeft int       `json:"seconds_left"`
	FreeUntil   time.Time `json:"free_until"`
}

type orderBody struct {
	ID          string           `json:"id"`
	Code        string           `json:"code"`
	MerchantID  string           `json:"merchant_id"`
	PartnerID   string           `json:"partner_id,omitempty"`
	Status      string           `json:"status"`
	StatusLabel string           `json:"status_label"`
	Live        bool             `json:"live"`
	Payment     string           `json:"payment_method"`
	Lines       []lineBody       `json:"lines"`
	Count       int              `json:"count"`
	Subtotal    moneyBody        `json:"subtotal"`
	Delivery    moneyBody        `json:"delivery"`
	Total       moneyBody        `json:"total"`
	Receipt     []receiptRowBody `json:"receipt"`
	Expanded    bool             `json:"expanded"`
	DistanceM   float64          `json:"distance_m"`
	Pickup      placeBody        `json:"pickup"`
	Destination placeBody        `json:"destination"`
	Events      []eventBody      `json:"events"`
	NextActions []string         `json:"next_actions"`
	Cancel      cancelBody       `json:"cancel"`
	PlacedAt    time.Time        `json:"placed_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type listBody struct {
	Orders []orderBody `json:"orders"`
	Total  int         `json:"total"`
}

type placeRequest struct {
	AddressID      string `json:"address_id"`
	PaymentMethod  string `json:"payment_method"`
	IdempotencyKey string `json:"idempotency_key"`
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

type transitionRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// ---------------------------------------------------------------- handlers

// placeOrder is POST /v1/orders.
func (h *Handler) placeOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body placeRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	// The header is the conventional place for it; the body is accepted too,
	// because a generated client may find one easier than the other.
	key := body.IdempotencyKey
	if header := r.Header.Get("Idempotency-Key"); header != "" {
		key = header
	}

	view, err := h.place.Execute(r.Context(), userID, application.PlaceRequest{
		AddressID: body.AddressID, Payment: body.PaymentMethod,
		IdempotencyKey: key, Lang: lang(r),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toBody(view))
}

func (h *Handler) myOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	query, ok := listQuery(w, r)
	if !ok {
		return
	}
	list, err := h.reads.OfCustomer(r.Context(), userID, query)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toListBody(list))
}

func (h *Handler) myOrder(w http.ResponseWriter, r *http.Request) {
	caller, ok := h.customer(w, r)
	if !ok {
		return
	}
	view, err := h.reads.One(r.Context(), r.PathValue("orderId"), caller)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toBody(view))
}

func (h *Handler) cancellation(w http.ResponseWriter, r *http.Request) {
	caller, ok := h.customer(w, r)
	if !ok {
		return
	}
	cancel, err := h.transitions.CancelStatus(r.Context(), r.PathValue("orderId"), caller)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCancelBody(cancel))
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	caller, ok := h.customer(w, r)
	if !ok {
		return
	}
	var body reasonRequest
	if r.ContentLength > 0 {
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	h.transition(w, r, domain.StatusCancelled, body.Reason, caller)
}

func (h *Handler) shopOrders(w http.ResponseWriter, r *http.Request) {
	// The ownership check is the point of resolving the caller here; the
	// listing itself is scoped by the merchant id in the path.
	if _, ok := h.merchant(w, r); !ok {
		return
	}
	query, ok := listQuery(w, r)
	if !ok {
		return
	}
	list, err := h.reads.OfMerchant(r.Context(), r.PathValue("merchantId"), query)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toListBody(list))
}

func (h *Handler) shopOrder(w http.ResponseWriter, r *http.Request) {
	caller, ok := h.merchant(w, r)
	if !ok {
		return
	}
	view, err := h.reads.One(r.Context(), r.PathValue("orderId"), caller)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toBody(view))
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	h.shopTransition(w, r, domain.StatusAccepted, false)
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	h.shopTransition(w, r, domain.StatusRejected, true)
}

func (h *Handler) preparing(w http.ResponseWriter, r *http.Request) {
	h.shopTransition(w, r, domain.StatusPreparing, false)
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	h.shopTransition(w, r, domain.StatusReady, false)
}

// shopTransition is the shared body of the four shop actions.
func (h *Handler) shopTransition(w http.ResponseWriter, r *http.Request, to domain.Status, needsReason bool) {
	caller, ok := h.merchant(w, r)
	if !ok {
		return
	}
	var body reasonRequest
	if r.ContentLength > 0 {
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	if needsReason && body.Reason == "" {
		httpx.WriteError(w, errs.New(errs.KindInvalid, "reason_required",
			"Please say why you cannot take this order."))
		return
	}
	h.transition(w, r, to, body.Reason, caller)
}

func (h *Handler) adminTransition(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body transitionRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	status := domain.Status(body.Status)
	if !status.Valid() {
		httpx.WriteError(w, errs.New(errs.KindInvalid, "unknown_status",
			"That is not a state an order can be in."))
		return
	}
	h.transition(w, r, status, body.Reason, application.Caller{
		Actor: domain.ActorAdmin, ID: userID, Lang: lang(r),
	})
}

// transition is the shared tail of every state change.
func (h *Handler) transition(w http.ResponseWriter, r *http.Request, to domain.Status, reason string, caller application.Caller) {
	view, err := h.transitions.Execute(r.Context(), r.PathValue("orderId"), to, reason, caller)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toBody(view))
}

// ----------------------------------------------------------------- callers

func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := h.principal(r)
	if !ok || userID == "" {
		httpx.WriteError(w, errs.New(errs.KindUnauthorized, "unauthenticated",
			"Please sign in to see your orders."))
		return "", false
	}
	return userID, true
}

func (h *Handler) customer(w http.ResponseWriter, r *http.Request) (application.Caller, bool) {
	userID, ok := h.caller(w, r)
	if !ok {
		return application.Caller{}, false
	}
	return application.Caller{Actor: domain.ActorCustomer, ID: userID, Lang: lang(r)}, true
}

// merchant resolves the caller as an owner of the shop in the path.
//
// The ownership check is here *and* in the use case. Here so the request fails
// early with the right shape; there because the use case is also reached by the
// contract, and a check that only exists at the edge is a check the next
// endpoint forgets.
func (h *Handler) merchant(w http.ResponseWriter, r *http.Request) (application.Caller, bool) {
	userID, ok := h.caller(w, r)
	if !ok {
		return application.Caller{}, false
	}
	owned, err := h.merchants(r, userID)
	if err != nil {
		httpx.WriteError(w, err)
		return application.Caller{}, false
	}
	merchantID := r.PathValue("merchantId")
	for _, id := range owned {
		if id == merchantID {
			return application.Caller{
				Actor: domain.ActorMerchant, ID: userID,
				MerchantIDs: owned, Lang: lang(r),
			}, true
		}
	}
	// Not found rather than forbidden: "that shop exists but is not yours"
	// tells a prober which ids are real.
	httpx.WriteError(w, errs.New(errs.KindNotFound, "merchant_not_found",
		"We could not find that shop."))
	return application.Caller{}, false
}

func listQuery(w http.ResponseWriter, r *http.Request) (application.ListQuery, bool) {
	limit, err := httpx.BoundedInt(r, "limit", 20, 1, 50)
	if err != nil {
		httpx.WriteError(w, err)
		return application.ListQuery{}, false
	}
	offset, err := httpx.OptionalInt(r, "offset", 0)
	if err != nil {
		httpx.WriteError(w, err)
		return application.ListQuery{}, false
	}
	return application.ListQuery{
		LiveOnly: r.URL.Query().Get("live") == "true",
		Limit:    limit, Offset: offset, Lang: lang(r),
	}, true
}

// lang reads the requested language. Anything but "en" is Bengali (1.4).
func lang(r *http.Request) string { return r.URL.Query().Get("lang") }

// ------------------------------------------------------------------ mapping

func toBody(view application.View) orderBody {
	lines := make([]lineBody, 0, len(view.Lines))
	for _, l := range view.Lines {
		options := make([]optionBody, 0, len(l.Options))
		for _, o := range l.Options {
			options = append(options, optionBody{Name: o.Name, Price: toMoneyBody(o.Price)})
		}
		lines = append(lines, lineBody{
			ID: l.ID, Kind: l.Kind, TargetID: l.TargetID, Name: l.Name,
			Options: options, Quantity: l.Quantity, Note: l.Note,
			UnitPrice: toMoneyBody(l.UnitPrice), LineTotal: toMoneyBody(l.LineTotal),
		})
	}
	receipt := make([]receiptRowBody, 0, len(view.Receipt))
	for _, row := range view.Receipt {
		receipt = append(receipt, receiptRowBody{
			Key: row.Key, Label: row.Label, Amount: toMoneyBody(row.Amount),
		})
	}
	events := make([]eventBody, 0, len(view.Events))
	for _, e := range view.Events {
		events = append(events, eventBody{
			Status: e.Status, Label: e.Label, Actor: e.Actor, Reason: e.Reason, At: e.At,
		})
	}
	return orderBody{
		ID: view.ID, Code: view.Code,
		MerchantID: view.MerchantID, PartnerID: view.PartnerID,
		Status: view.Status, StatusLabel: view.StatusLabel,
		Live: view.Live, Payment: view.Payment,
		Lines: lines, Count: view.Count,
		Subtotal: toMoneyBody(view.Subtotal),
		Delivery: toMoneyBody(view.Delivery),
		Total:    toMoneyBody(view.Total),
		Receipt:  receipt,
		Expanded: view.Expanded, DistanceM: view.DistanceM,
		Pickup:      toPlaceBody(view.Pickup),
		Destination: toPlaceBody(view.Destination),
		Events:      events, NextActions: view.NextActions,
		Cancel:   toCancelBody(view.Cancel),
		PlacedAt: view.PlacedAt, UpdatedAt: view.UpdatedAt,
	}
}

func toListBody(list application.List) listBody {
	orders := make([]orderBody, 0, len(list.Orders))
	for _, o := range list.Orders {
		orders = append(orders, toBody(o))
	}
	return listBody{Orders: orders, Total: list.Total}
}

func toMoneyBody(m application.Money) moneyBody {
	return moneyBody{Minor: m.Minor, Currency: m.Currency, Display: m.Display}
}

func toPlaceBody(p application.PlaceView) placeBody {
	return placeBody{
		Name: p.Name, Phone: p.Phone, SingleLine: p.SingleLine, Lat: p.Lat, Lng: p.Lng,
	}
}

func toCancelBody(c application.CancelView) cancelBody {
	return cancelBody{
		Allowed: c.Allowed, Reason: c.Reason, Text: c.Text,
		SecondsLeft: c.SecondsLeft, FreeUntil: c.FreeUntil,
	}
}
