// Package http exposes shop registration, day-to-day operation and the admin
// approval queue.
//
// Two audiences on one handler, split by guard. The owner routes act on the
// caller's own shop and take the owner id from the verified token, never from
// the request: an endpoint that accepted a merchant id from the client is an
// endpoint that will be asked to approve somebody else's shop. The admin routes
// take an id, and sit behind the admin role.
package http

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Guard wraps a handler in an access requirement. Passed in rather than
// imported, so the merchant module does not depend on identity's transport.
type Guard func(http.Handler) http.Handler

// PrincipalOf reports the authenticated caller's user id.
type PrincipalOf func(*http.Request) (userID string, ok bool)

// Handler serves the merchant endpoints.
type Handler struct {
	registration *application.RegistrationUseCase
	operations   *application.OperationsUseCase
	moderation   *application.ModerationUseCase
	clock        clock.Clock
	authed       Guard
	admin        Guard
	principal    PrincipalOf
}

// NewHandler builds the handler.
//
// A nil guard or principal reader would publish the approval queue to anyone
// who found the URL, so all three are refused at wiring time rather than
// becoming a hole nobody notices.
func NewHandler(
	registration *application.RegistrationUseCase,
	operations *application.OperationsUseCase,
	moderation *application.ModerationUseCase,
	c clock.Clock,
	authed Guard,
	admin Guard,
	principal PrincipalOf,
) *Handler {
	if authed == nil || admin == nil || principal == nil {
		panic("merchant transport: both guards and a principal reader are required")
	}
	return &Handler{
		registration: registration,
		operations:   operations,
		moderation:   moderation,
		clock:        c,
		authed:       authed,
		admin:        admin,
		principal:    principal,
	}
}

// ownerRoutes are the shop owner's own endpoints.
func (h *Handler) ownerRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /v1/merchants":                          h.register,
		"GET /v1/merchants/me":                        h.mine,
		"PATCH /v1/merchants/me":                      h.update,
		"DELETE /v1/merchants/me":                     h.withdraw,
		"PUT /v1/merchants/me/documents":              h.addDocument,
		"POST /v1/merchants/me/submit":                h.submit,
		"PUT /v1/merchants/me/hours":                  h.setHours,
		"PUT /v1/merchants/me/holiday":                h.startHoliday,
		"DELETE /v1/merchants/me/holiday":             h.endHoliday,
		"GET /v1/merchants/registration-requirements": h.requirements,
	}
}

// adminRoutes are the approval queue.
func (h *Handler) adminRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"GET /v1/admin/merchants":                 h.list,
		"GET /v1/admin/merchants/{id}":            h.get,
		"GET /v1/admin/merchants/{id}/history":    h.history,
		"POST /v1/admin/merchants/{id}/approve":   h.approve,
		"POST /v1/admin/merchants/{id}/reject":    h.reject,
		"POST /v1/admin/merchants/{id}/suspend":   h.suspend,
		"POST /v1/admin/merchants/{id}/reinstate": h.reinstate,
	}
}

// Register mounts the routes, each behind the guard its audience needs.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.ownerRoutes() {
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
	for pattern := range empty.ownerRoutes() {
		out = append(out, pattern)
	}
	for pattern := range empty.adminRoutes() {
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

// language returns the caller's preferred language for preformatted text.
//
// Bengali unless the client asks otherwise, because the audience is (1.4).
func language(r *http.Request) string {
	if r.URL.Query().Get("lang") == "en" {
		return "en"
	}
	return "bn"
}

type documentResponse struct {
	Kind       string `json:"kind"`
	Number     string `json:"number"`
	FileURL    string `json:"file_url"`
	UploadedAt string `json:"uploaded_at"`
}

// merchantResponse is the owner's and the admin's view of a shop.
//
// Richer than contract.Merchant, which is what other modules and ultimately
// customers see: this carries the documents, the review note and what is still
// missing, because the whole point of the owner's screen is to say what to do
// next.
type merchantResponse struct {
	ID          string  `json:"id"`
	OwnerUserID string  `json:"owner_user_id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	Phone       string  `json:"phone"`
	Email       string  `json:"email,omitempty"`
	LogoURL     string  `json:"logo_url,omitempty"`
	Line1       string  `json:"line1"`
	Line2       string  `json:"line2,omitempty"`
	SingleLine  string  `json:"single_line"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`

	AreaCode     string `json:"area_code"`
	AreaName     string `json:"area_name"`
	DistrictCode string `json:"district_code"`
	DivisionCode string `json:"division_code"`

	IsListed   bool   `json:"is_listed"`
	IsOpenNow  bool   `json:"is_open_now"`
	OpenStatus string `json:"open_status"`

	Hours   map[string][]string `json:"hours"`
	Holiday *holidayResponse    `json:"holiday,omitempty"`

	Documents         []documentResponse `json:"documents"`
	RequiredDocuments []string           `json:"required_documents"`
	MissingDocuments  []string           `json:"missing_documents"`
	CanSubmit         bool               `json:"can_submit"`
	NextStatuses      []string           `json:"next_statuses"`
	ReviewNote        string             `json:"review_note,omitempty"`
	CreatedAt         string             `json:"created_at"`
}

type holidayResponse struct {
	Until  string `json:"until,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// render converts a merchant to its wire form.
//
// It goes through the contract conversion for everything the contract carries,
// so the shop the app shows and the shop a consuming module receives are
// assembled by the same code and cannot drift apart.
func render(m domain.Merchant, now time.Time, lang string) merchantResponse {
	c := application.ToContract(m, now, lang)

	documents := make([]documentResponse, 0, len(m.Documents))
	for _, d := range m.Documents {
		documents = append(documents, documentResponse{
			Kind:       d.Kind.String(),
			Number:     d.Number,
			FileURL:    d.FileURL,
			UploadedAt: d.UploadedAt.UTC().Format(time.RFC3339),
		})
	}

	required := make([]string, 0, 3)
	for _, kind := range domain.RequiredDocuments(m.Type) {
		required = append(required, kind.String())
	}
	missing := make([]string, 0, len(required))
	for _, kind := range m.MissingDocuments() {
		missing = append(missing, kind.String())
	}
	next := make([]string, 0, 2)
	for _, status := range m.Status.NextStatuses() {
		next = append(next, status.String())
	}

	out := merchantResponse{
		ID: c.ID, OwnerUserID: c.OwnerUserID, Name: c.Name,
		Type: string(c.Type), Status: m.Status.String(),
		Phone: c.Phone, Email: m.Email, LogoURL: c.LogoURL,
		Line1: c.Line1, Line2: c.Line2, SingleLine: c.SingleLine,
		Lat: c.Lat, Lng: c.Lng,
		AreaCode: c.AreaCode, AreaName: c.AreaName,
		DistrictCode: c.DistrictCode, DivisionCode: c.DivisionCode,
		IsListed: c.IsListed, IsOpenNow: c.IsOpenNow, OpenStatus: c.OpenStatus,
		Hours:             m.Hours.Encode(),
		Documents:         documents,
		RequiredDocuments: required,
		MissingDocuments:  missing,
		CanSubmit:         m.ReadyForReview() == nil && m.Status.CanTransitionTo(domain.StatusPendingReview),
		NextStatuses:      next,
		ReviewNote:        m.ReviewNote,
		CreatedAt:         m.CreatedAt.UTC().Format(time.RFC3339),
	}

	if m.Holiday.ActiveAt(now) {
		holiday := holidayResponse{Reason: m.Holiday.Reason}
		if !m.Holiday.Until.IsZero() {
			holiday.Until = m.Holiday.Until.UTC().Format(time.RFC3339)
		}
		out.Holiday = &holiday
	}
	return out
}

// writeMerchant renders one merchant at a status code.
func (h *Handler) writeMerchant(w http.ResponseWriter, r *http.Request, status int, m domain.Merchant) {
	httpx.WriteJSON(w, status, render(m, h.clock.Now(), language(r)))
}

type detailsBody struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Phone   string  `json:"phone"`
	Email   string  `json:"email"`
	LogoURL string  `json:"logo_url"`
	Line1   string  `json:"line1"`
	Line2   string  `json:"line2"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

func (b detailsBody) toRequest() application.DetailsRequest {
	return application.DetailsRequest{
		Name: b.Name, Type: b.Type, Phone: b.Phone, Email: b.Email,
		LogoURL: b.LogoURL, Line1: b.Line1, Line2: b.Line2, Lat: b.Lat, Lng: b.Lng,
	}
}

// POST /v1/merchants
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body detailsBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	merchant, err := h.registration.Register(r.Context(), userID, body.toRequest())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.writeMerchant(w, r, http.StatusCreated, merchant)
}

// GET /v1/merchants/me
func (h *Handler) mine(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	merchant, err := h.registration.Mine(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.writeMerchant(w, r, http.StatusOK, merchant)
}

// PATCH /v1/merchants/me
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body detailsBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	merchant, err := h.registration.Update(r.Context(), userID, body.toRequest())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.writeMerchant(w, r, http.StatusOK, merchant)
}

// DELETE /v1/merchants/me
func (h *Handler) withdraw(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	if err := h.registration.Withdraw(r.Context(), userID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

type documentBody struct {
	Kind    string `json:"kind"`
	Number  string `json:"number"`
	FileURL string `json:"file_url"`
}

// PUT /v1/merchants/me/documents
func (h *Handler) addDocument(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body documentBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	merchant, err := h.registration.AddDocument(r.Context(), userID, application.DocumentRequest{
		Kind: body.Kind, Number: body.Number, FileURL: body.FileURL,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.writeMerchant(w, r, http.StatusOK, merchant)
}

// POST /v1/merchants/me/submit
func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	merchant, err := h.registration.SubmitForReview(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.writeMerchant(w, r, http.StatusOK, merchant)
}

type hoursBody struct {
	Days map[string][]string `json:"days"`
}

// PUT /v1/merchants/me/hours
func (h *Handler) setHours(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body hoursBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	merchant, err := h.operations.SetHours(r.Context(), userID, application.HoursRequest{Days: body.Days})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.writeMerchant(w, r, http.StatusOK, merchant)
}

type holidayBody struct {
	// Until is RFC 3339, or empty for indefinitely.
	Until  string `json:"until"`
	Reason string `json:"reason"`
}

// PUT /v1/merchants/me/holiday
func (h *Handler) startHoliday(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body holidayBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}

	var until time.Time
	if body.Until != "" {
		parsed, parseErr := time.Parse(time.RFC3339, body.Until)
		if parseErr != nil {
			httpx.WriteError(w, errs.Wrap(parseErr, errs.KindInvalid, "invalid_date",
				"Please give the reopening date as a full date and time."))
			return
		}
		until = parsed
	}

	merchant, err := h.operations.StartHoliday(r.Context(), userID,
		application.HolidayRequest{Until: until, Reason: body.Reason})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.writeMerchant(w, r, http.StatusOK, merchant)
}

// DELETE /v1/merchants/me/holiday
func (h *Handler) endHoliday(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	merchant, err := h.operations.EndHoliday(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.writeMerchant(w, r, http.StatusOK, merchant)
}

// requirementsResponse tells a client what to ask for, per shop type.
//
// Served rather than hard-coded in the app so the rule lives in one place: a
// Flutter build that decided for itself which licences a pharmacy needs would
// be a second copy of a legal requirement, updated on a different schedule
// (2.9).
type requirementsResponse struct {
	Types []typeRequirement `json:"types"`
}

type typeRequirement struct {
	Type              string   `json:"type"`
	RequiredDocuments []string `json:"required_documents"`
}

// GET /v1/merchants/registration-requirements
func (h *Handler) requirements(w http.ResponseWriter, _ *http.Request) {
	out := requirementsResponse{Types: make([]typeRequirement, 0, len(domain.AllTypes()))}
	for _, kind := range domain.AllTypes() {
		required := make([]string, 0, 3)
		for _, doc := range domain.RequiredDocuments(kind) {
			required = append(required, doc.String())
		}
		out.Types = append(out.Types, typeRequirement{Type: kind.String(), RequiredDocuments: required})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// GET /v1/admin/merchants
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	found, err := h.moderation.List(r.Context(), application.ListRequest{
		Status:       query.Get("status"),
		Type:         query.Get("type"),
		DivisionCode: query.Get("division"),
		Limit:        intParam(query.Get("limit")),
		Offset:       intParam(query.Get("offset")),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	now := h.clock.Now()
	lang := language(r)
	out := make([]merchantResponse, 0, len(found))
	for _, m := range found {
		out = append(out, render(m, now, lang))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"merchants": out})
}

// GET /v1/admin/merchants/{id}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	merchant, err := h.moderation.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.writeMerchant(w, r, http.StatusOK, merchant)
}

type statusEventResponse struct {
	ID          string `json:"id"`
	From        string `json:"from"`
	To          string `json:"to"`
	ActorUserID string `json:"actor_user_id"`
	Note        string `json:"note,omitempty"`
	At          string `json:"at"`
}

// GET /v1/admin/merchants/{id}/history
func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	events, err := h.moderation.History(r.Context(), r.PathValue("id"), intParam(r.URL.Query().Get("limit")))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]statusEventResponse, 0, len(events))
	for _, e := range events {
		out = append(out, statusEventResponse{
			ID: e.ID, From: e.From.String(), To: e.To.String(),
			ActorUserID: e.ActorUserID, Note: e.Note,
			At: e.At.UTC().Format(time.RFC3339),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"events": out})
}

type decisionBody struct {
	Note string `json:"note"`
}

// decision is one moderation action. The four differ only in which status they
// move to and whether a reason is required, both of which the use case owns, so
// the handler for each is the same handler with a different method.
type decision func(ctx context.Context, adminUserID, merchantID, note string) (domain.Merchant, error)

func (h *Handler) decide(w http.ResponseWriter, r *http.Request, act decision) {
	adminUserID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body decisionBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	merchant, err := act(r.Context(), adminUserID, r.PathValue("id"), body.Note)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.writeMerchant(w, r, http.StatusOK, merchant)
}

// POST /v1/admin/merchants/{id}/approve
func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, h.moderation.Approve)
}

// POST /v1/admin/merchants/{id}/reject
func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, h.moderation.Reject)
}

// POST /v1/admin/merchants/{id}/suspend
func (h *Handler) suspend(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, h.moderation.Suspend)
}

// POST /v1/admin/merchants/{id}/reinstate
func (h *Handler) reinstate(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, h.moderation.Reinstate)
}

// intParam reads a query parameter as a count, treating anything unreadable as
// absent. A bad limit falls back to the default page rather than refusing the
// request: the use case clamps it anyway, and a 400 for "limit=abc" is a
// support call about a page that will not load.
func intParam(raw string) int {
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}
