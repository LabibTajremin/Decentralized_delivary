// Package http exposes the customer's cart.
//
// Every route acts on the caller's own cart. The user id comes from the
// verified token and never from the request (2.7): an endpoint that took a
// cart id from the client is an endpoint that will be handed someone else's.
package http

import (
	"context"
	"net/http"
	"sort"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/application"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Guard wraps a handler in an access requirement. Passed in rather than
// imported, so the cart does not depend on identity's transport.
type Guard func(http.Handler) http.Handler

// PrincipalOf reports the authenticated caller's user id.
type PrincipalOf func(*http.Request) (userID string, ok bool)

// Handler serves the cart endpoints.
type Handler struct {
	carts     *application.CartUseCase
	authed    Guard
	principal PrincipalOf
}

// NewHandler builds the handler.
func NewHandler(carts *application.CartUseCase, authed Guard, principal PrincipalOf) *Handler {
	if authed == nil || principal == nil {
		panic("cart transport: a guard and a principal reader are required")
	}
	return &Handler{carts: carts, authed: authed, principal: principal}
}

func (h *Handler) routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/cart":                   h.current,
		"POST /v1/cart/items":            h.add,
		"POST /v1/cart/replace":          h.replace,
		"PUT /v1/cart/lines/{lineId}":    h.setQuantity,
		"DELETE /v1/cart/lines/{lineId}": h.remove,
		"PUT /v1/cart/address":           h.setAddress,
		"DELETE /v1/cart":                h.clear,
	}
}

// Register mounts the routes, all behind authentication.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.routes() {
		mux.Handle(pattern, h.authed(handle))
	}
}

// Patterns returns every route this module serves, sorted.
func Patterns() []string {
	empty := &Handler{}
	routes := empty.routes()
	out := make([]string, 0, len(routes))
	for pattern := range routes {
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
	GroupID  string    `json:"group_id"`
	OptionID string    `json:"option_id"`
	Name     string    `json:"name"`
	Price    moneyBody `json:"price"`
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
	Issue     string       `json:"issue,omitempty"`
	IssueText string       `json:"issue_text,omitempty"`
	Orderable bool         `json:"orderable"`
}

type cartBody struct {
	ID              string     `json:"id"`
	MerchantID      string     `json:"merchant_id"`
	MerchantName    string     `json:"merchant_name"`
	MerchantLogoURL string     `json:"merchant_logo_url,omitempty"`
	MerchantStatus  string     `json:"merchant_status,omitempty"`
	AddressID       string     `json:"address_id,omitempty"`
	Lines           []lineBody `json:"lines"`
	Count           int        `json:"count"`
	Subtotal        moneyBody  `json:"subtotal"`
	Orderable       bool       `json:"orderable"`
	Blocker         string     `json:"blocker,omitempty"`
	BlockerText     string     `json:"blocker_text,omitempty"`
}

type addRequest struct {
	MerchantID string `json:"merchant_id"`
	Kind       string `json:"kind"`
	TargetID   string `json:"target_id"`
	Quantity   int    `json:"quantity"`
	Choices    []struct {
		GroupID  string `json:"group_id"`
		OptionID string `json:"option_id"`
	} `json:"choices"`
	Note string `json:"note"`
}

type quantityRequest struct {
	Quantity int `json:"quantity"`
}

type addressRequest struct {
	AddressID string  `json:"address_id"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
}

// ---------------------------------------------------------------- handlers

// current returns the caller's cart, revalidated.
//
// An empty cart and no cart are the same answer to a client rendering a screen,
// so a customer with no cart gets a 200 with an empty body rather than a 404
// the app would have to special-case.
func (h *Handler) current(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	view, found, err := h.carts.Current(r.Context(), userID, lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if !found {
		httpx.WriteJSON(w, http.StatusOK, cartBody{Lines: []lineBody{}, Blocker: "empty"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toBody(view))
}

func (h *Handler) add(w http.ResponseWriter, r *http.Request) {
	h.mutateWithAdd(w, r, h.carts.Add)
}

func (h *Handler) replace(w http.ResponseWriter, r *http.Request) {
	h.mutateWithAdd(w, r, h.carts.Replace)
}

// mutateWithAdd is the shared body of add and replace, which differ only in
// what they do about a cart at another shop.
func (h *Handler) mutateWithAdd(
	w http.ResponseWriter, r *http.Request,
	run func(ctx context.Context, userID string, req application.AddRequest) (application.View, error),
) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body addRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	choices := make([]application.Choice, 0, len(body.Choices))
	for _, c := range body.Choices {
		choices = append(choices, application.Choice{GroupID: c.GroupID, OptionID: c.OptionID})
	}
	view, err := run(r.Context(), userID, application.AddRequest{
		MerchantID: body.MerchantID,
		Kind:       body.Kind,
		TargetID:   body.TargetID,
		Quantity:   body.Quantity,
		Choices:    choices,
		Note:       body.Note,
		Lang:       lang(r),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toBody(view))
}

func (h *Handler) setQuantity(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body quantityRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := h.carts.SetQuantity(r.Context(), userID, r.PathValue("lineId"), body.Quantity, lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toBody(view))
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	view, err := h.carts.Remove(r.Context(), userID, r.PathValue("lineId"), lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toBody(view))
}

func (h *Handler) setAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body addressRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := h.carts.SetAddress(r.Context(), userID, body.AddressID, body.Lat, body.Lng, lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toBody(view))
}

func (h *Handler) clear(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	if err := h.carts.Clear(r.Context(), userID); err != nil {
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
			"Please sign in to use your cart."))
		return "", false
	}
	return userID, true
}

// lang reads the requested language. Anything but "en" is Bengali (1.4).
func lang(r *http.Request) string { return r.URL.Query().Get("lang") }

func toBody(view application.View) cartBody {
	lines := make([]lineBody, 0, len(view.Lines))
	for _, l := range view.Lines {
		options := make([]optionBody, 0, len(l.Options))
		for _, o := range l.Options {
			options = append(options, optionBody{
				GroupID: o.GroupID, OptionID: o.OptionID, Name: o.Name, Price: toMoneyBody(o.Price),
			})
		}
		lines = append(lines, lineBody{
			ID: l.ID, Kind: l.Kind, TargetID: l.TargetID, Name: l.Name,
			Options: options, Quantity: l.Quantity, Note: l.Note,
			UnitPrice: toMoneyBody(l.UnitPrice), LineTotal: toMoneyBody(l.LineTotal),
			Issue: l.Issue, IssueText: l.IssueText, Orderable: l.Orderable,
		})
	}
	return cartBody{
		ID: view.ID, MerchantID: view.MerchantID,
		MerchantName: view.MerchantName, MerchantLogoURL: view.MerchantLogoURL,
		MerchantStatus: view.MerchantStatus, AddressID: view.AddressID,
		Lines: lines, Count: view.Count, Subtotal: toMoneyBody(view.Subtotal),
		Orderable: view.Orderable, Blocker: view.Blocker, BlockerText: view.BlockerText,
	}
}

func toMoneyBody(m application.Money) moneyBody {
	return moneyBody{Minor: m.Minor, Currency: m.Currency, Display: m.Display}
}
