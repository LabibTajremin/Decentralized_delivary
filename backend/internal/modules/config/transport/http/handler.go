// Package http exposes the config module over HTTP.
//
// These are admin endpoints. Authentication and role checks arrive in P04; until
// then the routes exist and are documented, and the P04 middleware wraps them
// without any change here — which is the point of keeping transport thin.
package http

import (
	"net/http"
	"sort"
	"strings"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Handler serves the config endpoints.
type Handler struct {
	resolve *application.ResolveUseCase
	set     *application.SetOverrideUseCase
	reset   *application.ClearOverrideUseCase
}

// NewHandler builds the handler.
func NewHandler(resolve *application.ResolveUseCase, set *application.SetOverrideUseCase, reset *application.ClearOverrideUseCase) *Handler {
	return &Handler{resolve: resolve, set: set, reset: reset}
}

// routes maps mux patterns to handlers. Register mounts them and Patterns
// reports them, so the router and the OpenAPI check read one list.
func (h *Handler) routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/config/definitions": h.definitions,
		"GET /v1/config/effective":   h.effective,
		"PUT /v1/config/overrides":   h.setOverride,
		"DELETE /v1/config/override": h.clearOverride,
	}
}

// Register mounts the config routes.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.routes() {
		mux.HandleFunc(pattern, handle)
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

// definitionResponse describes one variable to an admin screen.
//
// Bounds and the auto-tunable flag are included because an admin editing a
// value needs to know what is allowed before submitting, not after being
// rejected. Immutable is included so the UI can show the division ceiling as a
// visible, locked fact rather than hiding it.
type definitionResponse struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Unit        string `json:"unit"`
	Default     string `json:"default"`
	Minimum     string `json:"minimum,omitempty"`
	Maximum     string `json:"maximum,omitempty"`
	AutoTunable bool   `json:"auto_tunable"`
	Immutable   bool   `json:"immutable"`
	Purpose     string `json:"purpose"`
}

// GET /v1/config/definitions
func (h *Handler) definitions(w http.ResponseWriter, _ *http.Request) {
	defs := domain.AllDefinitions()
	out := make([]definitionResponse, 0, len(defs))
	for _, d := range defs {
		item := definitionResponse{
			Key:         string(d.Key),
			Type:        d.Kind.String(),
			Unit:        d.Kind.Unit(),
			Default:     d.Default.String(),
			AutoTunable: d.AutoTunable,
			Immutable:   d.Immutable,
			Purpose:     d.Purpose,
		}
		// Bools carry no meaningful bounds; sending "0" and "0" would look like
		// a range that forbids true.
		if d.Kind != domain.KindBool {
			item.Minimum, item.Maximum = d.Min.String(), d.Max.String()
		}
		out = append(out, item)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"definitions": out})
}

// effectiveValue is one resolved setting with the scope it came from.
type effectiveValue struct {
	Key    string `json:"key"`
	Type   string `json:"type"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

// GET /v1/config/effective?area=&district=&division=
func (h *Handler) effective(w http.ResponseWriter, r *http.Request) {
	placement := domain.Placement{
		AreaCode:     r.URL.Query().Get("area"),
		DistrictCode: r.URL.Query().Get("district"),
		DivisionCode: r.URL.Query().Get("division"),
	}
	resolved, err := h.resolve.Execute(r.Context(), placement)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	entries := resolved.Entries()
	out := make([]effectiveValue, 0, len(entries))
	for _, e := range entries {
		out = append(out, effectiveValue{
			Key:    string(e.Key),
			Type:   e.Value.Kind().String(),
			Value:  e.Value.String(),
			Source: e.Source.String(),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"values": out})
}

// setOverrideRequest is an admin change.
//
// The value is a string rather than a JSON number on purpose: money is minor
// units and must never round-trip through a float, and a single textual form
// means the server parses each kind exactly once, the same way the database
// stores it.
type setOverrideRequest struct {
	Key     string `json:"key"`
	Level   string `json:"level"`
	Code    string `json:"code"`
	Value   string `json:"value"`
	Pinned  bool   `json:"pinned"`
	Reason  string `json:"reason"`
	ActorID string `json:"actor_id"`
}

type changeResponse struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Scope    string `json:"scope"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
	Actor    string `json:"actor"`
	Reason   string `json:"reason"`
	ChangedA string `json:"changed_at"`
}

func toChangeResponse(c domain.Change) changeResponse {
	actor := c.Actor.Kind
	if c.Actor.ID != "" {
		actor += ":" + c.Actor.ID
	}
	return changeResponse{
		ID:       c.ID,
		Key:      string(c.Key),
		Scope:    c.Scope.String(),
		OldValue: c.OldValue.String(),
		NewValue: c.NewValue.String(),
		Actor:    actor,
		Reason:   c.Reason,
		ChangedA: c.At.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// PUT /v1/config/overrides
func (h *Handler) setOverride(w http.ResponseWriter, r *http.Request) {
	var req setOverrideRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}

	scope, err := parseScope(req.Level, req.Code)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	def, err := domain.Lookup(domain.Key(strings.TrimSpace(req.Key)))
	if err != nil {
		httpx.WriteError(w, errs.Wrap(err, errs.KindNotFound, "unknown_config_key",
			"That setting does not exist.").With("key", req.Key))
		return
	}
	value, err := domain.Parse(def.Kind, req.Value)
	if err != nil {
		httpx.WriteError(w, errs.Wrap(err, errs.KindInvalid, "invalid_config_value",
			"That value could not be read for this setting.").
			With("key", req.Key).
			With("expected", def.Kind.String()))
		return
	}

	change, err := h.set.Execute(r.Context(), application.SetRequest{
		Key:    def.Key,
		Scope:  scope,
		Value:  value,
		Pinned: req.Pinned,
		// Until P04 supplies an authenticated identity, the actor comes from
		// the request. The endpoint is admin-only and will be behind the RBAC
		// middleware; the use case already treats the actor as authoritative,
		// so P04 changes where this value comes from and nothing else.
		Actor:  domain.AdminActor(strings.TrimSpace(req.ActorID)),
		Reason: req.Reason,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toChangeResponse(change))
}

// clearOverrideRequest resets one setting at one scope.
type clearOverrideRequest struct {
	Key     string `json:"key"`
	Level   string `json:"level"`
	Code    string `json:"code"`
	Reason  string `json:"reason"`
	ActorID string `json:"actor_id"`
}

// DELETE /v1/config/override
func (h *Handler) clearOverride(w http.ResponseWriter, r *http.Request) {
	var req clearOverrideRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	scope, err := parseScope(req.Level, req.Code)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	change, err := h.reset.Execute(r.Context(), domain.Key(strings.TrimSpace(req.Key)), scope,
		domain.AdminActor(strings.TrimSpace(req.ActorID)), req.Reason)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toChangeResponse(change))
}

// parseScope turns the wire form into a validated scope.
func parseScope(level, code string) (domain.Scope, error) {
	parsed, err := domain.ParseLevel(level)
	if err != nil {
		return domain.Scope{}, errs.Wrap(err, errs.KindInvalid, "invalid_scope_level",
			"Scope must be area, district, division or global.").
			With("level", level)
	}
	scope, err := domain.NewScope(parsed, code)
	if err != nil {
		return domain.Scope{}, errs.Wrap(err, errs.KindInvalid, "invalid_scope",
			"That scope is not valid.").
			With("level", level).
			With("code", code)
	}
	return scope, nil
}
