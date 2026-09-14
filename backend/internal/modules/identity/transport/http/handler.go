package http

import (
	"net/http"
	"sort"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/external/config"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// Handler serves the authentication endpoints.
type Handler struct {
	requestOTP *application.RequestOTPUseCase
	verifyOTP  *application.VerifyOTPUseCase
	refresh    *application.RefreshSessionUseCase
	logout     *application.LogoutUseCase
	sessions   *application.ListSessionsUseCase
	auth       *Authenticator
}

// NewHandler builds the handler.
func NewHandler(
	requestOTP *application.RequestOTPUseCase,
	verifyOTP *application.VerifyOTPUseCase,
	refresh *application.RefreshSessionUseCase,
	logout *application.LogoutUseCase,
	sessions *application.ListSessionsUseCase,
	auth *Authenticator,
) *Handler {
	return &Handler{
		requestOTP: requestOTP, verifyOTP: verifyOTP, refresh: refresh,
		logout: logout, sessions: sessions, auth: auth,
	}
}

// routes maps patterns to handlers, with the role each requires.
//
// The role is part of the route table rather than a check inside each handler,
// so "what does this endpoint require" is answerable by reading one list — and
// a new route with no entry here does not quietly default to public.
type route struct {
	handle http.HandlerFunc
	// public marks an endpoint that must work before a token exists. There are
	// exactly three, and each is rate-limited; everything else names its roles.
	public bool
	roles  []domain.Role
}

func (h *Handler) routes() map[string]route {
	return map[string]route{
		// Sign-in cannot require a token, by definition. These are the only
		// unauthenticated endpoints in the system (2.7), and the fact is
		// written down here rather than inferred from missing middleware.
		"POST /v1/auth/otp/request": {handle: h.requestOTPHandler, public: true},
		"POST /v1/auth/otp/verify":  {handle: h.verifyOTPHandler, public: true},
		"POST /v1/auth/refresh":     {handle: h.refreshHandler, public: true},

		"POST /v1/auth/logout":     {handle: h.logoutHandler, roles: domain.AllRoles()},
		"POST /v1/auth/logout-all": {handle: h.logoutAllHandler, roles: domain.AllRoles()},
		"GET /v1/auth/sessions":    {handle: h.sessionsHandler, roles: domain.AllRoles()},
	}
}

// Register mounts the auth routes, wrapping each in the guard it declares.
func (h *Handler) Register(mux *http.ServeMux) {
	for pattern, r := range h.routes() {
		handler := http.Handler(r.handle)
		if !r.public {
			handler = h.auth.Require(r.roles...)(handler)
		}
		mux.Handle(pattern, handler)
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

// placementFrom reads the optional area hint.
//
// Auth limits resolve per area like every other business rule, but someone
// signing in has not chosen an address yet — so an absent placement is normal
// and resolves to the global values.
func placementFrom(r *http.Request) cfgcontract.Placement {
	q := r.URL.Query()
	return cfgcontract.Placement{
		AreaCode:     q.Get("area"),
		DistrictCode: q.Get("district"),
		DivisionCode: q.Get("division"),
	}
}

type requestOTPBody struct {
	Phone string `json:"phone"`
}

type requestOTPResponse struct {
	// The number is echoed masked so the verification screen can show which
	// number was used without displaying it in full.
	Phone string `json:"phone"`
	// ExpiresIn and ResendAfter are the server's numbers. The client counts
	// down with them rather than holding its own idea of the TTL (2.9).
	ExpiresIn   int64 `json:"expires_in"`
	ResendAfter int64 `json:"resend_after"`
}

// POST /v1/auth/otp/request
func (h *Handler) requestOTPHandler(w http.ResponseWriter, r *http.Request) {
	var body requestOTPBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	result, err := h.requestOTP.Execute(r.Context(), body.Phone, placementFrom(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, requestOTPResponse{
		Phone:       result.MaskedPhone,
		ExpiresIn:   result.ExpiresIn,
		ResendAfter: result.ResendAfter,
	})
}

type verifyOTPBody struct {
	Phone  string `json:"phone"`
	Code   string `json:"code"`
	Role   string `json:"role"`
	Device string `json:"device"`
}

// tokenResponse is what a client stores.
//
// expires_in is included so the client knows when to expect a 401 without
// decoding the JWT. It never parses the access token: that would be a business
// rule in the app (2.9), and a client that decides for itself when a token is
// valid is a client that disagrees with the server.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Role         string `json:"role"`
	UserID       string `json:"user_id"`
	SessionID    string `json:"session_id"`
	NewUser      bool   `json:"new_user,omitempty"`
}

func toTokenResponse(pair domain.TokenPair, newUser bool) tokenResponse {
	return tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken.String(),
		TokenType:    "Bearer",
		ExpiresIn:    pair.ExpiresIn,
		Role:         pair.Role.String(),
		UserID:       pair.UserID,
		SessionID:    pair.SessionID,
		NewUser:      newUser,
	}
}

// POST /v1/auth/otp/verify
func (h *Handler) verifyOTPHandler(w http.ResponseWriter, r *http.Request) {
	var body verifyOTPBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	role, err := domain.ParseRole(body.Role)
	if err != nil {
		httpx.WriteError(w, errs.Wrap(err, errs.KindInvalid, "invalid_role",
			"That sign-in type is not recognised."))
		return
	}

	result, err := h.verifyOTP.Execute(r.Context(), application.VerifyRequest{
		Phone:     body.Phone,
		Code:      body.Code,
		Role:      role,
		Device:    body.Device,
		Placement: placementFrom(r),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toTokenResponse(result.Tokens, result.NewUser))
}

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
}

// POST /v1/auth/refresh
//
// The auto-login endpoint. The app calls it on start and on any 401; a valid
// token means the user never sees a login screen.
func (h *Handler) refreshHandler(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	pair, err := h.refresh.Execute(r.Context(), body.RefreshToken, placementFrom(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toTokenResponse(pair, false))
}

// POST /v1/auth/logout
//
// The principal is passed straight through rather than checked here first. The
// guard has already established one, and the use case refuses a zero principal
// anyway — so a check in between would be a branch that can only run if both
// of those are wrong at once, which no test can arrange and no reviewer can
// verify.
func (h *Handler) logoutHandler(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	if err := h.logout.Execute(r.Context(), principal); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

// POST /v1/auth/logout-all
func (h *Handler) logoutAllHandler(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	if err := h.logout.ExecuteAll(r.Context(), principal); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

type deviceResponse struct {
	SessionID  string `json:"session_id"`
	Device     string `json:"device"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
	Current    bool   `json:"current"`
}

// GET /v1/auth/sessions
func (h *Handler) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFrom(r.Context())
	sessions, err := h.sessions.Execute(r.Context(), principal)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]deviceResponse, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, deviceResponse(s))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"sessions": out})
}
