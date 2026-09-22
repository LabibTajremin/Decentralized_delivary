// Package http exposes tracking to the party watching a delivery.
package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Guard wraps a handler in an access requirement.
type Guard func(http.Handler) http.Handler

// PrincipalOf reports the authenticated caller's user id.
type PrincipalOf func(*http.Request) (userID string, ok bool)

// Handler serves the tracking endpoint.
type Handler struct {
	snapshot *application.SnapshotUseCase
	// interval is how often a live stream polls for a change. A field rather
	// than a constant so production and tests run it at different speeds
	// without the handler knowing which it is.
	interval  time.Duration
	authed    Guard
	principal PrincipalOf
}

// NewHandler builds the handler.
func NewHandler(snapshot *application.SnapshotUseCase, interval time.Duration, authed Guard, principal PrincipalOf) *Handler {
	if authed == nil || principal == nil {
		panic("tracking transport: a guard and a principal reader are required")
	}
	if interval <= 0 {
		interval = 3 * time.Second
	}
	return &Handler{snapshot: snapshot, interval: interval, authed: authed, principal: principal}
}

func (h *Handler) routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/track/{orderId}": h.stream,
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

// stream serves a delivery's status, and its rider's position while one is
// carrying it, as Server-Sent Events.
//
// The first frame answers the ownership question the same way a JSON
// endpoint would — a caller who may not watch this order gets a 404 before
// anything is upgraded to a stream, not a stream that opens and then
// refuses. After that, one frame per change until the order reaches a
// terminal state or the client goes away.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	orderID := r.PathValue("orderId")
	lang := r.URL.Query().Get("lang")

	snap, err := h.snapshot.For(r.Context(), orderID, userID, lang)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	flusher, canFlush := w.(http.Flusher)
	if !canFlush {
		httpx.WriteError(w, errs.New(errs.KindUnavailable, "streaming_unavailable",
			"We could not open a live connection just now. Please try again."))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	writeEvent(w, snap)
	flusher.Flush()
	if !snap.Live {
		return
	}

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			next, err := h.snapshot.For(ctx, orderID, userID, lang)
			if err != nil {
				return
			}
			if next.Changed(snap) {
				writeEvent(w, next)
				flusher.Flush()
			}
			snap = next
			if !snap.Live {
				return
			}
		}
	}
}

// writeEvent marshals unconditionally: snapshotBody is built from primitives
// only, so json.Marshal on it cannot fail — there is no error branch to
// handle here.
func writeEvent(w http.ResponseWriter, snap domain.Snapshot) {
	body, _ := json.Marshal(snapshotBody{
		OrderID: snap.OrderID, Status: snap.Status, StatusLabel: snap.StatusLabel, Live: snap.Live,
		Partner: partnerBodyOf(snap.Partner),
	})
	// A write failing here means the client has already gone — the stream
	// loop's own ctx.Done() case is what notices that and stops it; there is
	// nothing further to do with the error at the point it happens.
	_, _ = fmt.Fprintf(w, "data: %s\n\n", body)
}

type snapshotBody struct {
	OrderID     string       `json:"order_id"`
	Status      string       `json:"status"`
	StatusLabel string       `json:"status_label"`
	Live        bool         `json:"live"`
	Partner     *partnerBody `json:"partner,omitempty"`
}

type partnerBody struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Phone   string  `json:"phone"`
	Vehicle string  `json:"vehicle"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

func partnerBodyOf(p *domain.Partner) *partnerBody {
	if p == nil {
		return nil
	}
	return &partnerBody{ID: p.ID, Name: p.Name, Phone: p.Phone, Vehicle: p.Vehicle, Lat: p.Lat, Lng: p.Lng}
}

// caller reads the signed-in user id, writing the unauthenticated response
// itself when there is none.
func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := h.principal(r)
	if !ok || userID == "" {
		httpx.WriteError(w, errs.New(errs.KindUnauthorized, "unauthenticated",
			"Please sign in to track your order."))
		return "", false
	}
	return userID, true
}
