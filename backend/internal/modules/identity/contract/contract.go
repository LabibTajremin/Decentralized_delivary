// Package contract is the identity module's only public surface.
//
// Consumed by: every module that needs to know who is calling (Appendix A).
package contract

import "context"

// Role names what a caller is allowed to be.
type Role string

// The four roles, as primitives so a consumer never links identity's domain.
const (
	RoleCustomer Role = "customer"
	RolePartner  Role = "partner"
	RoleMerchant Role = "merchant"
	RoleAdmin    Role = "admin"
)

// Principal is an authenticated caller.
type Principal struct {
	UserID    string
	Role      Role
	SessionID string
}

// IsZero reports whether this is an unauthenticated caller.
func (p Principal) IsZero() bool { return p.UserID == "" }

// IdentityContract is the identity module's public interface.
//
// Deliberately small. Other modules need to know who is calling and to revoke
// access when something goes wrong; they do not need to issue tokens, and an
// interface that let them would put the ability to mint credentials in every
// module in the system.
type IdentityContract interface {
	// PrincipalFromToken verifies an access token and returns the caller.
	//
	// Signature-only, with no store lookup, so it stays cheap enough to call on
	// every request.
	PrincipalFromToken(ctx context.Context, accessToken string) (Principal, error)

	// RevokeUserSessions signs a user out everywhere. Used when an account is
	// suspended or a merchant is delisted, so losing permission takes effect at
	// once rather than when the last token happens to expire.
	RevokeUserSessions(ctx context.Context, userID string) error
}
