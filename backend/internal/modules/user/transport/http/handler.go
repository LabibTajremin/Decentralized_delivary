// Package http exposes the user's own profile and address book.
//
// Every route here acts on the caller's own data. The user id comes from the
// verified token, never from the request: an endpoint that takes a user id from
// the client is an endpoint that will be asked for somebody else's home address.
package http

import (
	"net/http"
	"sort"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Guard wraps a handler in a role requirement. Passed in rather than imported,
// so the user module does not depend on identity's transport package.
type Guard func(http.Handler) http.Handler

// PrincipalOf reports the authenticated caller's user id.
//
// Supplied by the caller for the same reason as Guard: the identity of the
// principal is identity's business, and this module only needs the id.
type PrincipalOf func(*http.Request) (userID string, ok bool)

// Handler serves the profile and address endpoints.
type Handler struct {
	profiles  *application.ProfileUseCase
	addresses *application.AddressUseCase
	guard     Guard
	principal PrincipalOf
}

// NewHandler builds the handler.
//
// A nil guard or principal reader would publish every customer's address book,
// so both are refused at wiring time rather than becoming a hole nobody notices.
func NewHandler(
	profiles *application.ProfileUseCase,
	addresses *application.AddressUseCase,
	guard Guard,
	principal PrincipalOf,
) *Handler {
	if guard == nil || principal == nil {
		panic("user transport: a guard and a principal reader are required")
	}
	return &Handler{profiles: profiles, addresses: addresses, guard: guard, principal: principal}
}

func (h *Handler) routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/me":                         h.getProfile,
		"PATCH /v1/me":                       h.updateProfile,
		"GET /v1/me/addresses":               h.listAddresses,
		"POST /v1/me/addresses":              h.addAddress,
		"PUT /v1/me/addresses/{id}":          h.updateAddress,
		"DELETE /v1/me/addresses/{id}":       h.deleteAddress,
		"POST /v1/me/addresses/{id}/default": h.setDefaultAddress,
	}
}

// Register mounts the routes, every one behind the guard.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.routes() {
		mux.Handle(pattern, h.guard(handle))
	}
}

// Patterns returns the routes this module serves, sorted.
func Patterns() []string {
	routes := (&Handler{}).routes()
	out := make([]string, 0, len(routes))
	for pattern := range routes {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

// caller returns the authenticated user id.
func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := h.principal(r)
	if !ok || userID == "" {
		httpx.WriteError(w, errs.New(errs.KindUnauthorized, "not_authenticated", "Please sign in again."))
		return "", false
	}
	return userID, true
}

type profileResponse struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	Language    string `json:"language"`
}

// GET /v1/me
func (h *Handler) getProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	profile, err := h.profiles.Get(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, profileResponse{
		UserID:      profile.UserID,
		Name:        profile.Name,
		DisplayName: profile.DisplayName(),
		Email:       profile.Email,
		Language:    profile.Language.String(),
	})
}

// updateProfileBody uses pointers so "leave this alone" and "clear this" are
// different requests. Without that, a client updating only the name would have
// to send the email back, and one that forgot would silently erase it.
type updateProfileBody struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Language *string `json:"language"`
}

// PATCH /v1/me
func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body updateProfileBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}

	profile, err := h.profiles.Update(r.Context(), userID, application.UpdateRequest{
		Name: body.Name, Email: body.Email, Language: body.Language,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, profileResponse{
		UserID:      profile.UserID,
		Name:        profile.Name,
		DisplayName: profile.DisplayName(),
		Email:       profile.Email,
		Language:    profile.Language.String(),
	})
}

// addressResponse is the wire form of an address.
//
// single_line is composed by the server so every surface — the app, a receipt,
// the rider's screen — shows the same string (2.9).
type addressResponse struct {
	ID             string  `json:"id"`
	Label          string  `json:"label"`
	RecipientName  string  `json:"recipient_name"`
	RecipientPhone string  `json:"recipient_phone"`
	Line1          string  `json:"line1"`
	Line2          string  `json:"line2,omitempty"`
	Instructions   string  `json:"instructions,omitempty"`
	SingleLine     string  `json:"single_line"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
	AreaCode       string  `json:"area_code"`
	AreaName       string  `json:"area_name"`
	DistrictCode   string  `json:"district_code"`
	DivisionCode   string  `json:"division_code"`
	IsDefault      bool    `json:"is_default"`
}

// renderAddress converts a domain address to its wire form.
//
// It goes through the contract conversion so the address the app shows and the
// address a consuming module receives are assembled by the same code, and
// cannot drift apart.
func renderAddress(a domain.Address) addressResponse {
	c := application.ToContractAddress(a)
	return addressResponse{
		ID:             c.ID,
		Label:          c.Label,
		RecipientName:  c.RecipientName,
		RecipientPhone: c.RecipientPhone,
		Line1:          c.Line1,
		Line2:          c.Line2,
		Instructions:   c.Instructions,
		SingleLine:     c.SingleLine,
		Lat:            c.Lat,
		Lng:            c.Lng,
		AreaCode:       c.AreaCode,
		AreaName:       c.AreaName,
		DistrictCode:   c.DistrictCode,
		DivisionCode:   c.DivisionCode,
		IsDefault:      a.IsDefault,
	}
}

// GET /v1/me/addresses
func (h *Handler) listAddresses(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	addresses, err := h.addresses.List(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	out := make([]addressResponse, 0, len(addresses))
	for _, a := range addresses {
		out = append(out, renderAddress(a))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"addresses": out})
}

type addressBody struct {
	Label          string  `json:"label"`
	RecipientName  string  `json:"recipient_name"`
	RecipientPhone string  `json:"recipient_phone"`
	Line1          string  `json:"line1"`
	Line2          string  `json:"line2"`
	Instructions   string  `json:"instructions"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
	MakeDefault    bool    `json:"make_default"`
}

func (b addressBody) toRequest() application.AddressRequest {
	return application.AddressRequest{
		Label:          b.Label,
		RecipientName:  b.RecipientName,
		RecipientPhone: b.RecipientPhone,
		Line1:          b.Line1,
		Line2:          b.Line2,
		Instructions:   b.Instructions,
		Lat:            b.Lat,
		Lng:            b.Lng,
		MakeDefault:    b.MakeDefault,
	}
}

// POST /v1/me/addresses
func (h *Handler) addAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body addressBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	address, err := h.addresses.Add(r.Context(), userID, body.toRequest())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, renderAddress(address))
}

// PUT /v1/me/addresses/{id}
func (h *Handler) updateAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body addressBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	address, err := h.addresses.Update(r.Context(), userID, r.PathValue("id"), body.toRequest())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, renderAddress(address))
}

// DELETE /v1/me/addresses/{id}
func (h *Handler) deleteAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	if err := h.addresses.Delete(r.Context(), userID, r.PathValue("id")); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

// POST /v1/me/addresses/{id}/default
func (h *Handler) setDefaultAddress(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	address, err := h.addresses.SetDefault(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, renderAddress(address))
}
