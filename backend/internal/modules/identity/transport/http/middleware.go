// Package http exposes authentication over HTTP and guards every other route.
package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/platform/httpx"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

type ctxKey int

const principalKey ctxKey = iota

// PrincipalFrom returns the authenticated caller, if any.
func PrincipalFrom(ctx context.Context) (domain.Principal, bool) {
	p, ok := ctx.Value(principalKey).(domain.Principal)
	return p, ok && !p.IsZero()
}

// WithPrincipal attaches a principal to a context. Exported so use cases and
// background work can carry an identity, and so tests can build one directly.
func WithPrincipal(ctx context.Context, p domain.Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// Authenticator verifies access tokens and enforces roles.
type Authenticator struct {
	signer ports.TokenSigner
}

// NewAuthenticator builds the middleware factory.
func NewAuthenticator(signer ports.TokenSigner) *Authenticator {
	return &Authenticator{signer: signer}
}

// Require returns middleware that admits only the listed roles.
//
// The roles are always named explicitly; there is no "any authenticated user"
// shorthand and no variant that admits everyone. Rule 2.7 requires every
// endpoint to declare what it needs, and a default that admits more than
// intended is the failure mode worth designing out.
//
// Admin is not implicitly allowed everywhere. An admin token reaching a partner
// endpoint is nearly always a mis-wired route or a confused client, and
// silently permitting it hides the bug. Where an admin genuinely acts for
// another role, the route lists both.
func (a *Authenticator) Require(roles ...domain.Role) httpx.Middleware {
	if len(roles) == 0 {
		// A programming error, caught at wiring time rather than at 3am: a
		// Require() with no roles would otherwise admit nobody, and look like
		// a broken endpoint rather than a broken guard.
		panic("identity: Require needs at least one role")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := a.authenticate(r)
			if err != nil {
				httpx.WriteError(w, err)
				return
			}
			if !principal.Can(roles...) {
				// The response does not say which role was needed. Telling an
				// attacker that an endpoint wants "admin" maps the system for
				// them.
				httpx.WriteError(w, errs.New(errs.KindForbidden, "insufficient_role",
					"You do not have access to this."))
				return
			}
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
		})
	}
}

// Authenticated returns middleware that requires any valid token, for endpoints
// that serve every role identically — signing out, listing your own devices.
//
// It is written as "every role" rather than "skip the role check", so adding a
// fifth role forces a decision here instead of silently widening access.
func (a *Authenticator) Authenticated() httpx.Middleware {
	return a.Require(domain.AllRoles()...)
}

// authenticate extracts and verifies the bearer token.
func (a *Authenticator) authenticate(r *http.Request) (domain.Principal, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return domain.Principal{}, errs.New(errs.KindUnauthorized, "missing_token",
			"Please sign in.")
	}

	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return domain.Principal{}, errs.New(errs.KindUnauthorized, "malformed_authorization",
			"Please sign in.")
	}

	claims, err := a.signer.Verify(strings.TrimSpace(token))
	if err != nil {
		// An expired token gets its own code, because it is the one case the
		// client is expected to handle silently: it triggers exactly one
		// refresh and a retry, which is what makes auto-login invisible.
		// Everything else means "sign in again".
		if errors.Is(err, domain.ErrAccessTokenExpired) {
			return domain.Principal{}, errs.Wrap(err, errs.KindUnauthorized, "token_expired",
				"Your session needs refreshing.")
		}
		return domain.Principal{}, errs.Wrap(err, errs.KindUnauthorized, "invalid_token",
			"Please sign in again.")
	}

	return claims.Principal(), nil
}
