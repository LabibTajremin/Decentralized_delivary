// Package http exposes discovery to the customer app.
//
// Public, deliberately. Browsing is what the app does before anyone signs in —
// a customer who has to create an account to find out whether anything delivers
// to their village is a customer who does not create an account. The delivery
// point comes from the query rather than a saved address for the same reason.
package http

import (
	"net/http"
	"sort"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/geo"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
)

// Handler serves the discovery endpoints.
type Handler struct {
	search *application.SearchUseCase
	reach  *application.ReachUseCase
}

// NewHandler builds the handler.
func NewHandler(search *application.SearchUseCase, reach *application.ReachUseCase) *Handler {
	return &Handler{search: search, reach: reach}
}

// routes are the customer's discovery endpoints.
func (h *Handler) routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/discovery/merchants":              h.searchMerchants,
		"GET /v1/discovery/merchants/{merchantId}": h.merchantReach,
	}
}

// Register mounts the routes.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.routes() {
		mux.Handle(pattern, handle)
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

// merchantCardBody is one shop on the customer's list.
type merchantCardBody struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	LogoURL    string      `json:"logo_url,omitempty"`
	Phone      string      `json:"phone,omitempty"`
	AreaName   string      `json:"area_name,omitempty"`
	Lat        float64     `json:"lat"`
	Lng        float64     `json:"lng"`
	DistanceM  float64     `json:"distance_m"`
	Distance   string      `json:"distance"`
	IsOpenNow  bool        `json:"is_open_now"`
	OpenStatus string      `json:"open_status"`
	Delivery   deliveryFee `json:"delivery"`
}

// deliveryFee is a fee in both forms: the integer to compute with and the
// string to render (2.9).
type deliveryFee struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency"`
	Display  string `json:"display"`
	Expanded bool   `json:"expanded"`
}

// expansionBody is the whole expansion decision, already made.
type expansionBody struct {
	Level       int     `json:"level"`
	RadiusM     float64 `json:"radius_m"`
	Radius      string  `json:"radius"`
	Stage       string  `json:"stage"`
	CanExpand   bool    `json:"can_expand"`
	Offered     bool    `json:"offered"`
	AtCeiling   bool    `json:"at_ceiling"`
	NextLevel   int     `json:"next_level,omitempty"`
	NextRadiusM float64 `json:"next_radius_m,omitempty"`
	NextRadius  string  `json:"next_radius,omitempty"`
}

// searchBody is the search response.
type searchBody struct {
	Division  string             `json:"division"`
	AreaName  string             `json:"area_name,omitempty"`
	Notice    string             `json:"notice"`
	Total     int                `json:"total"`
	Expansion expansionBody      `json:"expansion"`
	Merchants []merchantCardBody `json:"merchants"`
}

// reachBody is the single-merchant reachability response.
type reachBody struct {
	MerchantID    string  `json:"merchant_id"`
	DistanceM     float64 `json:"distance_m"`
	Distance      string  `json:"distance"`
	Reachable     bool    `json:"reachable"`
	RequiredLevel int     `json:"required_level"`
	Expanded      bool    `json:"expanded"`
	Reason        string  `json:"reason,omitempty"`
}

// searchMerchants answers "what can I order from, here".
func (h *Handler) searchMerchants(w http.ResponseWriter, r *http.Request) {
	point, ok := pointOf(w, r)
	if !ok {
		return
	}
	level, err := httpx.OptionalInt(r, "level", 0)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	limit, err := httpx.BoundedInt(r, "limit", 20, 1, 50)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	offset, err := httpx.OptionalInt(r, "offset", 0)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	result, err := h.search.Execute(r.Context(), application.Query{
		Point:  point,
		Level:  level,
		Type:   r.URL.Query().Get("type"),
		Text:   r.URL.Query().Get("q"),
		Lang:   r.URL.Query().Get("lang"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toSearchBody(result))
}

// merchantReach answers whether one shop is reachable from a point.
func (h *Handler) merchantReach(w http.ResponseWriter, r *http.Request) {
	point, ok := pointOf(w, r)
	if !ok {
		return
	}
	reach, err := h.reach.Execute(r.Context(), point, r.PathValue("merchantId"), r.URL.Query().Get("lang"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, reachBody{
		MerchantID:    reach.MerchantID,
		DistanceM:     reach.DistanceM,
		Distance:      reach.DistanceText,
		Reachable:     reach.Reachable,
		RequiredLevel: reach.RequiredLevel,
		Expanded:      reach.Expanded,
		Reason:        reach.Reason,
	})
}

// pointOf reads the delivery point, writing the error itself so both handlers
// read as a straight line.
func pointOf(w http.ResponseWriter, r *http.Request) (geo.Point, bool) {
	lat, err := httpx.RequiredFloat(r, "lat")
	if err != nil {
		httpx.WriteError(w, err)
		return geo.Point{}, false
	}
	lng, err := httpx.RequiredFloat(r, "lng")
	if err != nil {
		httpx.WriteError(w, err)
		return geo.Point{}, false
	}
	return geo.Point{Lat: lat, Lng: lng}, true
}

// toSearchBody maps the use-case result onto the wire shape.
func toSearchBody(result application.Result) searchBody {
	merchants := make([]merchantCardBody, 0, len(result.Merchants))
	for _, m := range result.Merchants {
		merchants = append(merchants, merchantCardBody{
			ID: m.ID, Name: m.Name, Type: m.Type, LogoURL: m.LogoURL,
			Phone: m.Phone, AreaName: m.AreaName, Lat: m.Lat, Lng: m.Lng,
			DistanceM: m.DistanceM, Distance: m.Distance,
			IsOpenNow: m.IsOpenNow, OpenStatus: m.OpenStatus,
			Delivery: deliveryFee{
				Minor:    m.Delivery.Minor,
				Currency: m.Delivery.Currency,
				Display:  m.Delivery.Display,
				Expanded: m.Delivery.Expanded,
			},
		})
	}
	return searchBody{
		Division:  result.Placement.DivisionName,
		AreaName:  result.Placement.AreaName,
		Notice:    result.Notice,
		Total:     result.Total,
		Expansion: toExpansionBody(result),
		Merchants: merchants,
	}
}

func toExpansionBody(result application.Result) expansionBody {
	e := result.Expansion
	return expansionBody{
		Level:       e.Level,
		RadiusM:     e.RadiusM,
		Radius:      result.RadiusText,
		Stage:       string(e.Stage),
		CanExpand:   e.CanExpand,
		Offered:     e.Offered,
		AtCeiling:   e.AtCeiling,
		NextLevel:   e.NextLevel,
		NextRadiusM: e.NextRadiusM,
		NextRadius:  result.NextRadiusText,
	}
}
