// Package http exposes notification to the person receiving it.
package http

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/application"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Guard wraps a handler in an access requirement.
type Guard func(http.Handler) http.Handler

// PrincipalOf reports the authenticated caller's user id.
type PrincipalOf func(*http.Request) (userID string, ok bool)

// Handler serves the notification endpoints.
type Handler struct {
	register  *application.RegisterDeviceUseCase
	reads     *application.ReadUseCase
	authed    Guard
	principal PrincipalOf
}

// NewHandler builds the handler.
func NewHandler(register *application.RegisterDeviceUseCase, reads *application.ReadUseCase, authed Guard, principal PrincipalOf) *Handler {
	if authed == nil || principal == nil {
		panic("notification transport: a guard and a principal reader are required")
	}
	return &Handler{register: register, reads: reads, authed: authed, principal: principal}
}

func (h *Handler) routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /v1/me/device":       h.registerDevice,
		"GET /v1/me/notifications": h.myNotifications,
	}
}

// Register mounts the routes behind their guard.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.routes() {
		mux.Handle(pattern, h.authed(handle))
	}
}

// Patterns returns every route this module documents in the API contract.
func Patterns() []string {
	empty := &Handler{}
	out := make([]string, 0, 4)
	for pattern := range empty.routes() {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

// ------------------------------------------------------------------- wire

type deviceBody struct {
	Platform string `json:"platform"`
	Token    string `json:"token"`
}

type notificationBody struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Channel   string    `json:"channel"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func toBody(v application.View) notificationBody {
	return notificationBody{
		ID: v.ID, Title: v.Title, Body: v.Body,
		Channel: v.Channel, Status: v.Status, CreatedAt: v.CreatedAt,
	}
}

// ---------------------------------------------------------------- handlers

func (h *Handler) registerDevice(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body deviceBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.register.Register(r.Context(), userID, body.Platform, body.Token); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

func (h *Handler) myNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	views, err := h.reads.ForUser(r.Context(), userID, limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]notificationBody, 0, len(views))
	for _, v := range views {
		out = append(out, toBody(v))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// caller reads the signed-in user id, writing the unauthenticated response
// itself when there is none.
func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := h.principal(r)
	if !ok || userID == "" {
		httpx.WriteError(w, errs.New(errs.KindUnauthorized, "unauthenticated",
			"Please sign in."))
		return "", false
	}
	return userID, true
}
