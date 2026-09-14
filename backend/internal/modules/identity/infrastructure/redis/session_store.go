// Package redis implements the identity stores against Redis.
//
// Redis rather than Postgres for sessions and one-time codes because the access
// pattern is a key lookup with an expiry, and TTLs mean expiry is automatic:
// there is no sweeper job to write, no sweeper job to forget to run, and a
// revoked session cannot linger because a cleanup task fell over (ADR 0005).
package redis

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"sort"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
)

// Client is the Redis surface these stores need. *goredis.Client satisfies it.
type Client interface {
	goredis.Cmdable
}

// rotateScript performs the whole token swap in one atomic step.
//
//go:embed rotate.lua
var rotateScript string

// SessionStore keeps sessions and refresh tokens.
type SessionStore struct {
	client Client
	rotate *goredis.Script
	// scripter runs Lua. It is separate from client so a test can substitute
	// one that returns a malformed reply — the case that matters most and the
	// one a real Redis running our own script can never produce. Depending on
	// the narrow goredis.Scripter rather than the whole Cmdable is what makes
	// that practical.
	scripter goredis.Scripter
}

// NewSessionStore builds a store.
func NewSessionStore(client Client) *SessionStore {
	return &SessionStore{
		client:   client,
		rotate:   goredis.NewScript(rotateScript),
		scripter: client,
	}
}

// NewSessionStoreWithScripter builds a store whose Lua runs elsewhere.
//
// Exported for tests that need to drive the reply-handling paths; production
// wiring uses NewSessionStore.
func NewSessionStoreWithScripter(client Client, scripter goredis.Scripter) *SessionStore {
	return &SessionStore{
		client:   client,
		rotate:   goredis.NewScript(rotateScript),
		scripter: scripter,
	}
}

// Key layout. Documented here because the same shapes appear in the Lua script,
// and two places that must agree should at least be readable together.
//
//	session:<id>              hash  user_id, role, device, created_at, last_seen_at, expires_at
//	refresh:<hash>            str   session_id           — current token only, single use
//	refresh_family:<id>       set   every hash ever issued for this session
//	refresh_spent:<hash>      str   session id — a hash that has been rotated away
//	user_sessions:<user_id>   set   every live session id for the user
func sessionKey(id string) string       { return "session:" + id }
func refreshKey(hash string) string     { return "refresh:" + hash }
func familyKey(sessionID string) string { return "refresh_family:" + sessionID }
func spentKey(hash string) string       { return "refresh_spent:" + hash }
func userKey(userID string) string      { return "user_sessions:" + userID }

// CreateSession stores a session and its first refresh token.
//
// Everything is written in one pipeline so a session cannot exist without its
// token, or a token without a session to belong to.
func (s *SessionStore) CreateSession(ctx context.Context, session domain.Session, refreshHash string, ttl time.Duration) error {
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, sessionKey(session.ID), map[string]any{
		"user_id":      session.UserID,
		"role":         session.Role.String(),
		"device":       session.Device,
		"created_at":   session.CreatedAt.UTC().Format(time.RFC3339Nano),
		"last_seen_at": session.LastSeenAt.UTC().Format(time.RFC3339Nano),
		"expires_at":   session.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
	pipe.Expire(ctx, sessionKey(session.ID), ttl)

	pipe.Set(ctx, refreshKey(refreshHash), session.ID, ttl)

	pipe.SAdd(ctx, familyKey(session.ID), refreshHash)
	pipe.Expire(ctx, familyKey(session.ID), ttl)

	pipe.SAdd(ctx, userKey(session.UserID), session.ID)
	// The user's session set outlives any single session, so its TTL is
	// refreshed on every sign-in rather than being set once and forgotten.
	pipe.Expire(ctx, userKey(session.UserID), ttl)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// Rotate atomically swaps one refresh token hash for another.
//
// The whole decision — is this token current, was it already used, does its
// session still exist — happens inside one Lua script, so two refreshes racing
// from the same device produce one rotation rather than two valid tokens or a
// false theft alarm. Doing it as separate commands from Go would leave a window
// between the read and the delete in which both requests see a valid token.
func (s *SessionStore) Rotate(
	ctx context.Context,
	oldHash, newHash string,
	ttl time.Duration,
	now time.Time,
) (ports.RotationOutcome, domain.Session, error) {
	result, err := s.rotate.Run(ctx, s.scripter,
		[]string{refreshKey(oldHash)},
		oldHash, newHash, int64(ttl.Seconds()), now.UTC().Format(time.RFC3339Nano),
	).Result()
	if err != nil {
		return ports.RotationUnknownToken, domain.Session{}, fmt.Errorf("rotate refresh token: %w", err)
	}

	outcome, sessionID, err := ParseRotateReply(result)
	if err != nil {
		return ports.RotationUnknownToken, domain.Session{}, err
	}

	switch outcome {
	case ports.RotationOK:
		session, readErr := s.Session(ctx, sessionID)
		if readErr != nil {
			return ports.RotationUnknownToken, domain.Session{}, readErr
		}
		return ports.RotationOK, session, nil

	case ports.RotationReuse:
		// The session is returned even though it is about to be revoked: the
		// caller needs its id and user id to log who was affected. A session
		// that has already gone still yields its id, which is what the audit
		// line needs.
		session, readErr := s.Session(ctx, sessionID)
		if readErr != nil {
			session = domain.Session{ID: sessionID}
		}
		return ports.RotationReuse, session, nil

	default:
		return ports.RotationUnknownToken, domain.Session{}, nil
	}
}

// ParseRotateReply reads the rotation script's reply.
//
// Exported and pure so the malformed-reply handling is directly testable. It
// guards against a reply this package's own script cannot produce — which is
// exactly the situation where a silent misreading would be worst: a corrupted
// or replaced script must not be read as "rotation succeeded".
func ParseRotateReply(result any) (ports.RotationOutcome, string, error) {
	values, ok := result.([]any)
	if !ok || len(values) == 0 {
		return ports.RotationUnknownToken, "", fmt.Errorf("rotate refresh token: unexpected reply %T", result)
	}

	status, _ := values[0].(string)
	sessionID := ""
	if len(values) > 1 {
		sessionID, _ = values[1].(string)
	}

	switch status {
	case "ok":
		if sessionID == "" {
			return ports.RotationUnknownToken, "", errors.New("rotate refresh token: reply missing session")
		}
		return ports.RotationOK, sessionID, nil
	case "reuse":
		return ports.RotationReuse, sessionID, nil
	default:
		return ports.RotationUnknownToken, "", nil
	}
}

// Session reads one session.
func (s *SessionStore) Session(ctx context.Context, sessionID string) (domain.Session, error) {
	if sessionID == "" {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	values, err := s.client.HGetAll(ctx, sessionKey(sessionID)).Result()
	if err != nil {
		return domain.Session{}, fmt.Errorf("read session: %w", err)
	}
	if len(values) == 0 {
		return domain.Session{}, domain.ErrSessionNotFound
	}

	role, err := domain.ParseRole(values["role"])
	if err != nil {
		return domain.Session{}, fmt.Errorf("session %s: %w", sessionID, err)
	}
	return domain.Session{
		ID:         sessionID,
		UserID:     values["user_id"],
		Role:       role,
		Device:     values["device"],
		CreatedAt:  parseTime(values["created_at"]),
		LastSeenAt: parseTime(values["last_seen_at"]),
		ExpiresAt:  parseTime(values["expires_at"]),
	}, nil
}

// parseTime reads a stored timestamp, yielding the zero time for anything
// unreadable. A session with an unparseable timestamp is still a session; only
// its display fields suffer, and failing the read would sign the user out.
func parseTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

// UserSessions lists a user's live sessions, oldest first.
//
// Sorted so the eviction rule has a defined victim: "the oldest session" must
// mean the same thing on every call, or two sign-ins racing would evict
// different devices.
func (s *SessionStore) UserSessions(ctx context.Context, userID string) ([]domain.Session, error) {
	ids, err := s.client.SMembers(ctx, userKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	out := make([]domain.Session, 0, len(ids))
	var stale []string
	for _, id := range ids {
		session, readErr := s.Session(ctx, id)
		if errors.Is(readErr, domain.ErrSessionNotFound) {
			// The session's TTL has elapsed but the set still names it. Collect
			// it for removal rather than reporting a device the user no longer
			// has — and rather than leaving the set to grow without bound.
			stale = append(stale, id)
			continue
		}
		if readErr != nil {
			return nil, readErr
		}
		out = append(out, session)
	}
	if len(stale) > 0 {
		// Best effort: a failure here costs a little wasted space, and is not
		// worth failing the user's request over.
		_ = s.client.SRem(ctx, userKey(userID), toAny(stale)...).Err()
	}

	// Ordered by creation time, with the session id breaking ties.
	//
	// The tie-break is not cosmetic: two sign-ins in the same instant would
	// otherwise leave "the oldest session" undefined, and the eviction rule
	// would remove an arbitrary device. Session ids are time-ordered, so they
	// are a meaningful tie-break rather than an arbitrary one.
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func toAny(values []string) []any {
	out := make([]any, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

// RevokeSession deletes a session and every refresh token ever issued for it.
//
// The whole family goes, not just the current token. After a reuse detection
// the attacker may hold any token from the family's history, and deleting only
// the newest would leave the rest usable.
func (s *SessionStore) RevokeSession(ctx context.Context, sessionID string) error {
	session, err := s.Session(ctx, sessionID)
	if err != nil && !errors.Is(err, domain.ErrSessionNotFound) {
		return err
	}

	family, err := s.client.SMembers(ctx, familyKey(sessionID)).Result()
	if err != nil {
		return fmt.Errorf("read token family: %w", err)
	}

	pipe := s.client.TxPipeline()
	for _, hash := range family {
		pipe.Del(ctx, refreshKey(hash))
		// The spent-token index goes too. Without this, a client that signs
		// out and later presents its discarded token would be reported as a
		// theft rather than as a stale credential — and a reuse alarm that
		// fires on ordinary logouts is an alarm nobody will investigate.
		pipe.Del(ctx, spentKey(hash))
	}
	pipe.Del(ctx, familyKey(sessionID))
	pipe.Del(ctx, sessionKey(sessionID))
	if session.UserID != "" {
		pipe.SRem(ctx, userKey(session.UserID), sessionID)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// RevokeUserSessions deletes every session a user has.
func (s *SessionStore) RevokeUserSessions(ctx context.Context, userID string) error {
	ids, err := s.client.SMembers(ctx, userKey(userID)).Result()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}
	for _, id := range ids {
		if err := s.RevokeSession(ctx, id); err != nil {
			return err
		}
	}
	// The index needs no explicit delete: each RevokeSession removes its own id
	// from it, and Redis drops a set once its last member is gone. An extra DEL
	// here would be an error branch nothing could ever reach.
	return nil
}
