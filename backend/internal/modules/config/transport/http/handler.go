// Package http exposes the config module over HTTP.
//
// These are admin endpoints, and every one of them is behind the admin role.
// The read endpoints are protected too, not only the writes: the effective
// configuration for an area states its COD limit, its fee bands and its
// discovery radius, which together describe how to price around the system.
package http

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Guard wraps a handler in a role requirement.
//
// It is the identity module's Require, passed in rather than imported: config
// must not depend on identity's transport package, and a function value is the
// smallest seam that keeps the dependency pointing the right way.
type Guard func(http.Handler) http.Handler

// PrincipalOf reports the authenticated caller's user id.
type PrincipalOf func(*http.Request) (userID string, ok bool)

// Handler serves the config endpoints.
type Handler struct {
	resolve   *application.ResolveUseCase
	set       *application.SetOverrideUseCase
	reset     *application.ClearOverrideUseCase
	changes   *application.ListChangesUseCase
	tune      *application.AutoTuneUseCase
	adminOnly Guard
	principal PrincipalOf
}

// NewHandler builds the handler.
//
// adminOnly and principal must not be nil. A nil guard would silently
// publish the whole configuration surface, so both are refused at wiring
// time rather than becoming a hole nobody notices.
func NewHandler(
	resolve *application.ResolveUseCase,
	set *application.SetOverrideUseCase,
	reset *application.ClearOverrideUseCase,
	changes *application.ListChangesUseCase,
	tune *application.AutoTuneUseCase,
	adminOnly Guard,
	principal PrincipalOf,
) *Handler {
	if adminOnly == nil || principal == nil {
		panic("config transport: an admin guard and a principal reader are required")
	}
	return &Handler{
		resolve: resolve, set: set, reset: reset, changes: changes, tune: tune,
		adminOnly: adminOnly, principal: principal,
	}
}

// routes maps mux patterns to handlers. Register mounts them and Patterns
// reports them, so the router and the OpenAPI check read one list.
func (h *Handler) routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/config/definitions":     h.definitions,
		"GET /v1/config/effective":       h.effective,
		"PUT /v1/config/overrides":       h.setOverride,
		"DELETE /v1/config/override":     h.clearOverride,
		"GET /v1/admin/config/changes":   h.listChanges,
		"POST /v1/admin/config/autotune": h.autotune,
	}
}

// Register mounts the config routes, every one behind the admin guard.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.routes() {
		mux.Handle(pattern, h.adminOnly(handle))
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
	Key    string `json:"key"`
	Level  string `json:"level"`
	Code   string `json:"code"`
	Value  string `json:"value"`
	Pinned bool   `json:"pinned"`
	Reason string `json:"reason"`
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
	adminID, ok := h.caller(w, r)
	if !ok {
		return
	}
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
		Actor:  domain.AdminActor(adminID),
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
	Key    string `json:"key"`
	Level  string `json:"level"`
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

// DELETE /v1/config/override
func (h *Handler) clearOverride(w http.ResponseWriter, r *http.Request) {
	adminID, ok := h.caller(w, r)
	if !ok {
		return
	}
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
		domain.AdminActor(adminID), req.Reason)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toChangeResponse(change))
}

// listChangesRequest narrows GET /v1/admin/config/changes.
//
// GET /v1/admin/config/changes?key=&level=&code=&limit=
func (h *Handler) listChanges(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := ports.ChangeFilter{}
	if key := strings.TrimSpace(q.Get("key")); key != "" {
		filter.Key = domain.Key(key)
	}
	if level := strings.TrimSpace(q.Get("level")); level != "" {
		scope, err := parseScope(level, q.Get("code"))
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		filter.Scope = &scope
	}
	if limit := q.Get("limit"); limit != "" {
		if n, err := strconv.Atoi(limit); err == nil {
			filter.Limit = n
		}
	}

	changes, err := h.changes.Execute(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]changeResponse, 0, len(changes))
	for _, c := range changes {
		out = append(out, toChangeResponse(c))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"changes": out})
}

// autotuneRequest is one ALG-09 pass over an area.
type autotuneRequest struct {
	Area             string  `json:"area"`
	District         string  `json:"district"`
	Division         string  `json:"division"`
	MerchantsNearby  int     `json:"merchants_nearby"`
	OrderFailureRate float64 `json:"order_failure_rate"`
}

type tuneOutcomeResponse struct {
	Key     string          `json:"key"`
	Applied bool            `json:"applied"`
	Skipped string          `json:"skipped,omitempty"`
	Change  *changeResponse `json:"change,omitempty"`
}

// POST /v1/admin/config/autotune
//
// ALG-09. The signal an operator or a future scheduled job supplies here —
// merchant density and the order failure rate — is the telemetry this phase
// has: what the algorithm does with it, not how it is gathered, is what
// Appendix B specifies.
func (h *Handler) autotune(w http.ResponseWriter, r *http.Request) {
	var req autotuneRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}

	outcomes, err := h.tune.Execute(r.Context(), domain.Placement{
		AreaCode: req.Area, DistrictCode: req.District, DivisionCode: req.Division,
	}, domain.TuningSignal{
		MerchantsNearby: req.MerchantsNearby, OrderFailureRate: req.OrderFailureRate,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]tuneOutcomeResponse, 0, len(outcomes))
	for _, o := range outcomes {
		item := tuneOutcomeResponse{Key: string(o.Key), Applied: o.Applied, Skipped: o.Skipped}
		if o.Applied {
			cr := toChangeResponse(o.Change)
			item.Change = &cr
		}
		out = append(out, item)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"results": out})
}

// caller reads the signed-in admin's user id, writing the unauthenticated
// response itself when there is none.
func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := h.principal(r)
	if !ok || userID == "" {
		httpx.WriteError(w, errs.New(errs.KindUnauthorized, "unauthenticated",
			"Please sign in."))
		return "", false
	}
	return userID, true
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
