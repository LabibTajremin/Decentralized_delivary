// Package http exposes review and support to customers and support agents.
package http

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Guard wraps a handler in an access requirement.
type Guard func(http.Handler) http.Handler

// PrincipalOf reports the authenticated caller's user id.
type PrincipalOf func(*http.Request) (userID string, ok bool)

// Handler serves the review and support endpoints.
type Handler struct {
	submit      *application.SubmitReviewUseCase
	ratings     *application.RatingsUseCase
	listReviews *application.ListReviewsUseCase
	raiseTicket *application.RaiseTicketUseCase
	resolve     *application.ResolveTicketUseCase
	myTickets   *application.MyTicketsUseCase
	openTickets *application.OpenTicketsUseCase
	authed      Guard
	adminOnly   Guard
	principal   PrincipalOf
}

// NewHandler builds the handler. authed, adminOnly and principal must not be
// nil — a nil guard would silently publish the support queue, the same
// hazard every other module's transport handler is built to refuse.
func NewHandler(
	submit *application.SubmitReviewUseCase,
	ratings *application.RatingsUseCase,
	listReviews *application.ListReviewsUseCase,
	raiseTicket *application.RaiseTicketUseCase,
	resolve *application.ResolveTicketUseCase,
	myTickets *application.MyTicketsUseCase,
	openTickets *application.OpenTicketsUseCase,
	authed Guard,
	adminOnly Guard,
	principal PrincipalOf,
) *Handler {
	if authed == nil || adminOnly == nil || principal == nil {
		panic("review transport: an authenticated guard, an admin guard and a principal reader are required")
	}
	return &Handler{
		submit: submit, ratings: ratings, listReviews: listReviews,
		raiseTicket: raiseTicket, resolve: resolve, myTickets: myTickets, openTickets: openTickets,
		authed: authed, adminOnly: adminOnly, principal: principal,
	}
}

func (h *Handler) routes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"POST /v1/reviews":                       h.authedFn(h.submitReview),
		"GET /v1/reviews":                        h.authedFn(h.listReviewsFor),
		"GET /v1/ratings":                        h.authedFn(h.rating),
		"POST /v1/support/tickets":               h.authedFn(h.raiseTicketHandler),
		"GET /v1/me/support/tickets":             h.authedFn(h.myTicketsHandler),
		"GET /v1/admin/support/tickets":          h.adminOnlyFn(h.openTicketsHandler),
		"POST /v1/admin/support/tickets/resolve": h.adminOnlyFn(h.resolveTicketHandler),
	}
}

// authedFn and adminOnlyFn wrap one handler in its own guard, because the
// module mixes an authenticated read (ratings, a subject's reviews), an
// authenticated write (submitting a review, raising a ticket) and an
// admin-only write (resolving one) on one router.
func (h *Handler) authedFn(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { h.authed(fn).ServeHTTP(w, r) }
}

func (h *Handler) adminOnlyFn(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { h.adminOnly(fn).ServeHTTP(w, r) }
}

// Register mounts the review routes.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, handle := range h.routes() {
		mux.Handle(pattern, handle)
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

// ------------------------------------------------------------------- wire

type submitReviewRequest struct {
	OrderID   string `json:"order_id"`
	Subject   string `json:"subject"`
	SubjectID string `json:"subject_id"`
	Rating    int    `json:"rating"`
	Comment   string `json:"comment"`
}

type reviewResponse struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"order_id"`
	Subject   string    `json:"subject"`
	SubjectID string    `json:"subject_id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

func toReviewResponse(r domain.Review) reviewResponse {
	return reviewResponse{
		ID: r.ID, OrderID: r.OrderID, Subject: string(r.Subject), SubjectID: r.SubjectID,
		Rating: r.Rating, Comment: r.Comment, CreatedAt: r.CreatedAt,
	}
}

type ratingResponse struct {
	Subject   string  `json:"subject"`
	SubjectID string  `json:"subject_id"`
	Average   float64 `json:"average"`
	Count     int     `json:"count"`
}

func toRatingResponse(subject string, r domain.Rating) ratingResponse {
	return ratingResponse{Subject: subject, SubjectID: r.SubjectID, Average: r.Average, Count: r.Count}
}

type raiseTicketRequest struct {
	OrderID string `json:"order_id"`
	Subject string `json:"subject"`
}

type ticketResponse struct {
	ID         string     `json:"id"`
	OrderID    string     `json:"order_id"`
	Subject    string     `json:"subject"`
	Status     string     `json:"status"`
	Resolution string     `json:"resolution,omitempty"`
	Note       string     `json:"note,omitempty"`
	AgentID    string     `json:"agent_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

func toTicketResponse(t domain.Ticket) ticketResponse {
	out := ticketResponse{
		ID: t.ID, OrderID: t.OrderID, Subject: t.Subject, Status: string(t.Status),
		Resolution: string(t.Resolution), Note: t.Note, AgentID: t.AgentID, CreatedAt: t.CreatedAt,
	}
	if !t.ResolvedAt.IsZero() {
		out.ResolvedAt = &t.ResolvedAt
	}
	return out
}

type resolveTicketRequest struct {
	TicketID   string `json:"ticket_id"`
	Resolution string `json:"resolution"`
	Note       string `json:"note"`
}

// ---------------------------------------------------------------- handlers

// POST /v1/reviews
func (h *Handler) submitReview(w http.ResponseWriter, r *http.Request) {
	raterID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body submitReviewRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	review, err := h.submit.Execute(r.Context(), application.SubmitReviewRequest{
		OrderID: body.OrderID, RaterID: raterID,
		Subject: domain.Subject(body.Subject), SubjectID: body.SubjectID,
		Rating: body.Rating, Comment: body.Comment,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toReviewResponse(review))
}

// GET /v1/reviews?subject=&subject_id=&limit=
func (h *Handler) listReviewsFor(w http.ResponseWriter, r *http.Request) {
	subject, subjectID, ok := requiredSubject(w, r)
	if !ok {
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	reviews, err := h.listReviews.For(r.Context(), subject, subjectID, limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]reviewResponse, 0, len(reviews))
	for _, rv := range reviews {
		out = append(out, toReviewResponse(rv))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reviews": out})
}

// GET /v1/ratings?subject=&subject_id=
func (h *Handler) rating(w http.ResponseWriter, r *http.Request) {
	subject, subjectID, ok := requiredSubject(w, r)
	if !ok {
		return
	}
	rating, err := h.ratings.For(r.Context(), subject, subjectID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toRatingResponse(string(subject), rating))
}

// requiredSubject reads subject and subject_id, refusing the request before
// any use case is touched when either is missing — the same "check required
// parameters first" shape discovery's transport uses, so the two together
// stay the one place any route reads a missing required query parameter as
// 400 rather than passing an empty string on.
func requiredSubject(w http.ResponseWriter, r *http.Request) (domain.Subject, string, bool) {
	q := r.URL.Query()
	subject := q.Get("subject")
	subjectID := q.Get("subject_id")
	if subject == "" || subjectID == "" {
		httpx.WriteError(w, errs.New(errs.KindInvalid, "invalid_review_subject",
			"subject and subject_id are required."))
		return "", "", false
	}
	return domain.Subject(subject), subjectID, true
}

// POST /v1/support/tickets
func (h *Handler) raiseTicketHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body raiseTicketRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ticket, err := h.raiseTicket.Execute(r.Context(), body.OrderID, userID, body.Subject)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toTicketResponse(ticket))
}

// GET /v1/me/support/tickets
func (h *Handler) myTicketsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	tickets, err := h.myTickets.For(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]ticketResponse, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, toTicketResponse(t))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tickets": out})
}

// GET /v1/admin/support/tickets?limit=
func (h *Handler) openTicketsHandler(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	tickets, err := h.openTickets.Execute(r.Context(), limit)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]ticketResponse, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, toTicketResponse(t))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tickets": out})
}

// POST /v1/admin/support/tickets/resolve
func (h *Handler) resolveTicketHandler(w http.ResponseWriter, r *http.Request) {
	agentID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body resolveTicketRequest
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ticket, err := h.resolve.Execute(r.Context(), body.TicketID, agentID,
		domain.Resolution(body.Resolution), body.Note)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toTicketResponse(ticket))
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
