package domain

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnknownRole means the role is not one of the four.
var ErrUnknownRole = errors.New("unknown role")

// Role is what a principal is allowed to be.
//
// Four roles, fixed. Every endpoint declares the role it requires explicitly;
// there are no implicitly-public endpoints (05-architecture.md 2.7), which is
// why there is no "none" or "any" member here — the absence of a requirement
// has to be written down, not inferred from a zero value.
type Role string

// The four roles.
const (
	RoleCustomer Role = "customer"
	RolePartner  Role = "partner"
	RoleMerchant Role = "merchant"
	RoleAdmin    Role = "admin"
)

// AllRoles lists every role in a fixed order.
func AllRoles() []Role {
	return []Role{RoleCustomer, RolePartner, RoleMerchant, RoleAdmin}
}

// ParseRole reads a role from a token claim or a request.
//
// An unrecognised role is an error, never a default. Defaulting an unknown role
// to customer would silently grant access to anyone who could put a typo in a
// claim.
func ParseRole(s string) (Role, error) {
	candidate := Role(strings.TrimSpace(strings.ToLower(s)))
	for _, r := range AllRoles() {
		if candidate == r {
			return r, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownRole, s)
}

// String returns the role as text.
func (r Role) String() string { return string(r) }

// IsValid reports whether the role is one of the four.
func (r Role) IsValid() bool {
	for _, known := range AllRoles() {
		if r == known {
			return true
		}
	}
	return false
}

// Principal is an authenticated caller: who they are, what they may do, and
// which session the claim came from.
//
// The session id travels with the identity so a use case can revoke exactly the
// session that made a request, rather than every session the user has.
type Principal struct {
	UserID    string
	Role      Role
	SessionID string
}

// IsZero reports whether this is an unauthenticated caller.
func (p Principal) IsZero() bool { return p.UserID == "" }

// Can reports whether this principal satisfies a role requirement.
//
// Admin is not a superset of the others. An admin calling a partner endpoint is
// almost always a bug — a mis-wired route or a confused client — and silently
// allowing it hides that. Where an admin genuinely needs to act for a partner,
// the endpoint says so by allowing both roles.
func (p Principal) Can(allowed ...Role) bool {
	for _, role := range allowed {
		if p.Role == role {
			return true
		}
	}
	return false
}
