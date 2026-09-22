// Package http exposes dispatch to delivery partners and to operations.
//
// Every partner route acts on the caller's own partner record. The partner id
// comes from the verified token by way of the partner lookup, never from the
// request (2.7): an endpoint that took a partner id from the client is an
// endpoint that will be handed somebody else's shift.
package http

import (
	"net/http"
	"sort"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/dispatch/application"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Guard wraps a handler in an access requirement. Passed in rather than
// imported, so dispatch does not depend on identity's transport.
type Guard func(http.Handler) http.Handler

// PrincipalOf reports the authenticated caller's user id.
type PrincipalOf func(*http.Request) (userID string, ok bool)

// Handler serves the dispatch endpoints.
type Handler struct {
	partners  *application.PartnerUseCase
	offers    *application.OfferUseCase
	authed    Guard
	admin     Guard
	principal PrincipalOf
}

// NewHandler builds the handler.
func NewHandler(
	partners *application.PartnerUseCase,
	offers *application.OfferUseCase,
	authed Guard,
	admin Guard,
	principal PrincipalOf,
) *Handler {
	if authed == nil || admin == nil || principal == nil {
		panic("dispatch transport: guards and a principal reader are required")
	}
	return &Handler{partners: partners, offers: offers, authed: authed, admin: admin, principal: principal}
}

// partnerRoutes are the rider's app.
func (h *Handler) partnerRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /v1/partner":                      h.register,
		"GET /v1/partner":                       h.me,
		"PUT /v1/partner/availability":          h.setAvailability,
		"PUT /v1/partner/preference":            h.setPreference,
		"PUT /v1/partner/location":              h.reportLocation,
		"GET /v1/partner/feed":                  h.feed,
		"GET /v1/partner/jobs":                  h.myJobs,
		"POST /v1/partner/jobs/{jobId}/accept":  h.accept,
		"POST /v1/partner/jobs/{jobId}/decline": h.decline,
		"POST /v1/partner/jobs/{jobId}/collect": h.collect,
		"POST /v1/partner/jobs/{jobId}/deliver": h.deliver,
		"POST /v1/partner/jobs/{jobId}/fail":    h.fail,
	}
}

// adminRoutes are platform operations.
func (h *Handler) adminRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /v1/admin/dispatch/sweep": h.sweep,
	}
}

// Register mounts the routes behind their guards.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.partnerRoutes() {
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
	for pattern := range empty.partnerRoutes() {
		out = append(out, pattern)
	}
	for pattern := range empty.adminRoutes() {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

// ------------------------------------------------------------------- wire

type partnerBody struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Phone             string  `json:"phone"`
	Vehicle           string  `json:"vehicle,omitempty"`
	Availability      string  `json:"availability"`
	AvailabilityLabel string  `json:"availability_label"`
	Preference        string  `json:"preference"`
	PreferenceLabel   string  `json:"preference_label"`
	Lat               float64 `json:"lat"`
	Lng               float64 `json:"lng"`
	Carrying          int     `json:"carrying"`
	AcceptancePercent int     `json:"acceptance_percent"`
}

type placeBody struct {
	Name       string  `json:"name"`
	Phone      string  `json:"phone,omitempty"`
	SingleLine string  `json:"single_line,omitempty"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
}

type jobBody struct {
	ID          string    `json:"id"`
	OrderID     string    `json:"order_id"`
	Code        string    `json:"code"`
	Status      string    `json:"status"`
	StatusLabel string    `json:"status_label"`
	Live        bool      `json:"live"`
	Band        string    `json:"band"`
	BandLabel   string    `json:"band_label"`
	DistanceM   float64   `json:"distance_m"`
	Distance    string    `json:"distance"`
	ToPickupM   float64   `json:"to_pickup_m,omitempty"`
	ToPickup    string    `json:"to_pickup,omitempty"`
	Pickup      placeBody `json:"pickup"`
	Destination placeBody `json:"destination"`
	Reason      string    `json:"reason,omitempty"`
	SecondsLeft int       `json:"seconds_left,omitempty"`
}

type feedBody struct {
	Partner partnerBody `json:"partner"`
	Jobs    []jobBody   `json:"jobs"`
	RadiusM float64     `json:"radius_m"`
	Reason  string      `json:"reason,omitempty"`
	Notice  string      `json:"notice,omitempty"`
}

type jobListBody struct {
	Jobs  []jobBody `json:"jobs"`
	Total int       `json:"total"`
}

type registerRequest struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Vehicle string `json:"vehicle"`
}

type availabilityRequest struct {
	Availability string `json:"availability"`
}

type preferenceRequest struct {
	Preference string `json:"preference"`
}

type locationRequest struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

// --------------------------------------------------------------- handlers

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body registerRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := h.partners.Register(r.Context(), userID, lang(r), application.RegisterRequest{
		Name: body.Name, Phone: body.Phone, Vehicle: body.Vehicle,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPartnerBody(view))
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	h.partnerAction(w, r, func(userID string) (application.PartnerView, error) {
		return h.partners.Me(r.Context(), userID, lang(r))
	})
}

func (h *Handler) setAvailability(w http.ResponseWriter, r *http.Request) {
	var body availabilityRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.partnerAction(w, r, func(userID string) (application.PartnerView, error) {
		return h.partners.SetAvailability(r.Context(), userID, body.Availability, lang(r))
	})
}

func (h *Handler) setPreference(w http.ResponseWriter, r *http.Request) {
	var body preferenceRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.partnerAction(w, r, func(userID string) (application.PartnerView, error) {
		return h.partners.SetPreference(r.Context(), userID, body.Preference, lang(r))
	})
}

func (h *Handler) reportLocation(w http.ResponseWriter, r *http.Request) {
	var body locationRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.partnerAction(w, r, func(userID string) (application.PartnerView, error) {
		return h.partners.ReportLocation(r.Context(), userID, body.Lat, body.Lng, lang(r))
	})
}

// partnerAction is the shared tail of every endpoint that returns a partner.
func (h *Handler) partnerAction(w http.ResponseWriter, r *http.Request, run func(string) (application.PartnerView, error)) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	view, err := run(userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPartnerBody(view))
}

func (h *Handler) feed(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	view, err := h.partners.Feed(r.Context(), userID, lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	jobs := make([]jobBody, 0, len(view.Jobs))
	for _, job := range view.Jobs {
		jobs = append(jobs, toJobBody(job))
	}
	httpx.WriteJSON(w, http.StatusOK, feedBody{
		Partner: toPartnerBody(view.Partner), Jobs: jobs,
		RadiusM: view.RadiusM, Reason: view.Reason, Notice: view.Notice,
	})
}

func (h *Handler) myJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
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
	list, err := h.partners.MyJobs(r.Context(), userID,
		r.URL.Query().Get("live") == "true", limit, offset, lang(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	jobs := make([]jobBody, 0, len(list.Jobs))
	for _, job := range list.Jobs {
		jobs = append(jobs, toJobBody(job))
	}
	httpx.WriteJSON(w, http.StatusOK, jobListBody{Jobs: jobs, Total: list.Total})
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	h.jobAction(w, r, func(userID, jobID string) (application.JobView, error) {
		return h.partners.Accept(r.Context(), userID, jobID, lang(r))
	})
}

func (h *Handler) decline(w http.ResponseWriter, r *http.Request) {
	h.jobAction(w, r, func(userID, jobID string) (application.JobView, error) {
		return h.partners.Decline(r.Context(), userID, jobID, lang(r))
	})
}

func (h *Handler) collect(w http.ResponseWriter, r *http.Request) {
	h.jobAction(w, r, func(userID, jobID string) (application.JobView, error) {
		return h.partners.Collect(r.Context(), userID, jobID, lang(r))
	})
}

func (h *Handler) deliver(w http.ResponseWriter, r *http.Request) {
	h.jobAction(w, r, func(userID, jobID string) (application.JobView, error) {
		return h.partners.Deliver(r.Context(), userID, jobID, lang(r))
	})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request) {
	var body reasonRequest
	if r.ContentLength > 0 {
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	h.jobAction(w, r, func(userID, jobID string) (application.JobView, error) {
		return h.partners.Fail(r.Context(), userID, jobID, body.Reason, lang(r))
	})
}

// jobAction is the shared tail of every endpoint that moves a job.
func (h *Handler) jobAction(w http.ResponseWriter, r *http.Request, run func(userID, jobID string) (application.JobView, error)) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	view, err := run(userID, r.PathValue("jobId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toJobBody(view))
}

// sweep runs the dispatch heartbeat: expire dead offers, re-offer waiting jobs.
//
// An endpoint rather than a background goroutine, so the sweep runs where
// somebody can see it: an operator can trigger it, a cron can call it, and it
// is safe to run twice. A goroutine inside the API would run once per replica
// and be invisible when it stopped.
func (h *Handler) sweep(w http.ResponseWriter, r *http.Request) {
	limit, err := httpx.BoundedInt(r, "limit", 50, 1, 200)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	result, err := h.offers.Sweep(r.Context(), limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]int{
		"expired": result.Expired,
		"offered": result.Offered,
	})
}

// caller reads the verified user id, refusing when there is none.
func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := h.principal(r)
	if !ok || userID == "" {
		httpx.WriteError(w, errs.New(errs.KindUnauthorized, "unauthenticated",
			"Please sign in to take deliveries."))
		return "", false
	}
	return userID, true
}

// lang reads the requested language. Anything but "en" is Bengali (1.4).
func lang(r *http.Request) string { return r.URL.Query().Get("lang") }

// ----------------------------------------------------------------- mapping

func toPartnerBody(view application.PartnerView) partnerBody {
	return partnerBody{
		ID: view.ID, Name: view.Name, Phone: view.Phone, Vehicle: view.Vehicle,
		Availability: view.Availability, AvailabilityLabel: view.AvailabilityLabel,
		Preference: view.Preference, PreferenceLabel: view.PreferenceLabel,
		Lat: view.Lat, Lng: view.Lng, Carrying: view.Carrying,
		AcceptancePercent: view.AcceptancePercent,
	}
}

func toJobBody(view application.JobView) jobBody {
	return jobBody{
		ID: view.ID, OrderID: view.OrderID, Code: view.Code,
		Status: view.Status, StatusLabel: view.StatusLabel, Live: view.Live,
		Band: view.Band, BandLabel: view.BandLabel,
		DistanceM: view.DistanceM, Distance: view.Distance,
		ToPickupM: view.ToPickupM, ToPickup: view.ToPickup,
		Pickup:      toPlaceBody(view.Pickup),
		Destination: toPlaceBody(view.Destination),
		Reason:      view.Reason, SecondsLeft: view.SecondsLeft,
	}
}

func toPlaceBody(p application.PlaceView) placeBody {
	return placeBody{
		Name: p.Name, Phone: p.Phone, SingleLine: p.SingleLine, Lat: p.Lat, Lng: p.Lng,
	}
}
