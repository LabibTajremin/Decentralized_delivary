package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// LogoutUseCase revokes sessions.
type LogoutUseCase struct {
	sessions ports.SessionStore
	logger   *slog.Logger
}

// NewLogoutUseCase wires the use case.
func NewLogoutUseCase(sessions ports.SessionStore, logger *slog.Logger) *LogoutUseCase {
	return &LogoutUseCase{sessions: sessions, logger: logger}
}

// Execute revokes the session the caller is using.
//
// The access token is not revoked and cannot be: it is stateless, which is the
// whole reason the common request path costs no Redis read. Its fifteen-minute
// lifetime is the bound on that, and it is why the ceiling on
// auth.access_token_ttl is an hour rather than a day.
func (uc *LogoutUseCase) Execute(ctx context.Context, principal domain.Principal) error {
	if principal.IsZero() || principal.SessionID == "" {
		return errs.New(errs.KindUnauthorized, "not_authenticated", "Please sign in again.")
	}

	// A caller may only revoke a session that is theirs. Without this check a
	// valid token for any account could sign out any other account, given a
	// session id.
	session, err := uc.sessions.Session(ctx, principal.SessionID)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			// Already gone. Logout is idempotent: reporting an error would make
			// a client retry something that has already succeeded.
			return nil
		}
		return errs.Wrap(err, errs.KindUnavailable, "session_store_unavailable",
			"We could not sign you out just now. Please try again.")
	}
	if session.UserID != principal.UserID {
		return errs.New(errs.KindForbidden, "not_your_session",
			"You cannot sign out of that session.")
	}

	if err := uc.sessions.RevokeSession(ctx, principal.SessionID); err != nil {
		return errs.Wrap(err, errs.KindUnavailable, "session_store_unavailable",
			"We could not sign you out just now. Please try again.")
	}
	uc.logger.Info("logout", "user_id", principal.UserID, "session_id", principal.SessionID)
	return nil
}

// ExecuteAll revokes every session the user has, on every device.
//
// This is the control someone reaches for when they think their account is
// compromised, so it must not depend on knowing which device was stolen.
func (uc *LogoutUseCase) ExecuteAll(ctx context.Context, principal domain.Principal) error {
	if principal.IsZero() {
		return errs.New(errs.KindUnauthorized, "not_authenticated", "Please sign in again.")
	}
	if err := uc.sessions.RevokeUserSessions(ctx, principal.UserID); err != nil {
		return errs.Wrap(err, errs.KindUnavailable, "session_store_unavailable",
			"We could not sign you out just now. Please try again.")
	}
	uc.logger.Info("logout all devices", "user_id", principal.UserID)
	return nil
}

// ListSessionsUseCase shows a user their active devices.
type ListSessionsUseCase struct {
	sessions ports.SessionStore
}

// NewListSessionsUseCase wires the use case.
func NewListSessionsUseCase(sessions ports.SessionStore) *ListSessionsUseCase {
	return &ListSessionsUseCase{sessions: sessions}
}

// DeviceSession is one entry in the active-devices list.
//
// It carries no token and no hash. The list exists so a user can recognise and
// revoke a device, which needs a label and a time, not a credential.
type DeviceSession struct {
	SessionID  string
	Device     string
	CreatedAt  string
	LastSeenAt string
	// Current marks the session making this request, so the UI can label it
	// rather than the user guessing which line is the phone in their hand.
	Current bool
}

// Execute lists the caller's sessions.
func (uc *ListSessionsUseCase) Execute(ctx context.Context, principal domain.Principal) ([]DeviceSession, error) {
	if principal.IsZero() {
		return nil, errs.New(errs.KindUnauthorized, "not_authenticated", "Please sign in again.")
	}
	sessions, err := uc.sessions.UserSessions(ctx, principal.UserID)
	if err != nil {
		return nil, errs.Wrap(err, errs.KindUnavailable, "session_store_unavailable",
			"We could not load your devices just now. Please try again.")
	}

	out := make([]DeviceSession, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, DeviceSession{
			SessionID:  s.ID,
			Device:     s.Device,
			CreatedAt:  s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			LastSeenAt: s.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Current:    s.ID == principal.SessionID,
		})
	}
	return out, nil
}
