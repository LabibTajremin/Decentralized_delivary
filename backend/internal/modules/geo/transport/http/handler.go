// Package http exposes the geo module over HTTP.
//
// The handler depends on contract.GeoContract, not on geo's use cases: the same
// boundary another module goes through is the one transport goes through, so
// there is exactly one public surface to keep correct.
//
// Transport never imports infrastructure (05-architecture.md 2.2).
package http

import (
	"net/http"
	"sort"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Handler serves the geo endpoints.
type Handler struct {
	geo contract.GeoContract
}

// NewHandler builds the handler.
func NewHandler(geo contract.GeoContract) *Handler { return &Handler{geo: geo} }

// maxRadiusMetres caps a radius search at 25 km.
//
// It is a transport-level sanity bound, not the business radius: the real
// per-area limit comes from the config module (Appendix B) and the division
// ceiling (D3) is enforced in the query. This only stops a caller asking for a
// radius that would scan the country.
const maxRadiusMetres = 25_000

// maxLimit caps how many merchants one request may return.
const (
	defaultLimit = 20
	maxLimit     = 100
)

// routes maps mux patterns to handlers.
//
// Register mounts every entry and Patterns lists the keys, so the routes the
// server serves and the routes the OpenAPI drift check verifies come from one
// place. A second hand-maintained list would drift, and a drift check reading a
// drifted list proves nothing.
func (h *Handler) routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/geo/resolve":   h.resolveArea,
		"GET /v1/geo/merchants": h.merchantsWithinRadius,
		"GET /v1/geo/distance":  h.distance,
	}
}

// Register mounts the geo routes on a mux.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.routes() {
		mux.HandleFunc(pattern, handle)
	}
}

// Patterns returns the routes this module serves, sorted, as "METHOD /path".
//
// It needs no dependencies because it never invokes a handler — it only reads
// the keys — so the OpenAPI check can ask what the module serves without
// standing up a database.
func Patterns() []string {
	routes := (&Handler{}).routes()
	out := make([]string, 0, len(routes))
	for pattern := range routes {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

// areaResponse is the wire form of an administrative placement.
type areaResponse struct {
	AreaCode     string `json:"area_code"`
	AreaName     string `json:"area_name"`
	DistrictCode string `json:"district_code"`
	DivisionCode string `json:"division_code"`
	DivisionName string `json:"division_name"`
}

// GET /v1/geo/resolve?lat=&lng=
func (h *Handler) resolveArea(w http.ResponseWriter, r *http.Request) {
	point, err := pointFrom(r, "lat", "lng")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	area, err := h.geo.ResolveArea(r.Context(), point)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, areaResponse{
		AreaCode:     area.AreaCode,
		AreaName:     area.AreaName,
		DistrictCode: area.DistrictCode,
		DivisionCode: area.DivisionCode,
		DivisionName: area.DivisionName,
	})
}

// merchantResponse is one radius-search result.
//
// distance_m is the server's number. A client never recomputes it: two
// different distances for one journey is exactly what the thin-client rule
// forbids (2.9), and pricing bands off this same value.
type merchantResponse struct {
	MerchantID string  `json:"merchant_id"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	DistanceM  float64 `json:"distance_m"`
}

type merchantsResponse struct {
	Merchants []merchantResponse `json:"merchants"`
	Count     int                `json:"count"`
	RadiusM   float64            `json:"radius_m"`
}

// GET /v1/geo/merchants?lat=&lng=&radius_m=&limit=
func (h *Handler) merchantsWithinRadius(w http.ResponseWriter, r *http.Request) {
	point, err := pointFrom(r, "lat", "lng")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	radius, err := httpx.RequiredFloat(r, "radius_m")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if radius <= 0 || radius > maxRadiusMetres {
		httpx.WriteError(w, errs.New(errs.KindInvalid, "invalid_radius",
			"The search radius must be between 1 metre and 25 kilometres.").
			With("parameter", "radius_m"))
		return
	}
	limit, err := httpx.BoundedInt(r, "limit", defaultLimit, 1, maxLimit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	found, err := h.geo.MerchantsWithinRadius(r.Context(), point, radius, limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	// An empty result is an empty list, never null: a client that has to
	// special-case null before iterating is a client that will forget to.
	out := make([]merchantResponse, 0, len(found))
	for _, m := range found {
		out = append(out, merchantResponse{
			MerchantID: m.MerchantID,
			Lat:        m.Lat,
			Lng:        m.Lng,
			DistanceM:  m.DistanceM,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, merchantsResponse{
		Merchants: out,
		Count:     len(out),
		RadiusM:   radius,
	})
}

type distanceResponse struct {
	DistanceM float64 `json:"distance_m"`
}

// GET /v1/geo/distance?from_lat=&from_lng=&to_lat=&to_lng=
func (h *Handler) distance(w http.ResponseWriter, r *http.Request) {
	from, err := pointFrom(r, "from_lat", "from_lng")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	to, err := pointFrom(r, "to_lat", "to_lng")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	metres, err := h.geo.DistanceBetween(r.Context(), from, to)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, distanceResponse{DistanceM: metres})
}

// pointFrom reads a coordinate pair from the query string. Range validation
// stays in the domain — this only gets the numbers out.
func pointFrom(r *http.Request, latKey, lngKey string) (contract.Point, error) {
	lat, err := httpx.RequiredFloat(r, latKey)
	if err != nil {
		return contract.Point{}, err
	}
	lng, err := httpx.RequiredFloat(r, lngKey)
	if err != nil {
		return contract.Point{}, err
	}
	return contract.Point{Lat: lat, Lng: lng}, nil
}
