package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	identityredis "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/redis"
)

// A session store that cannot reach Redis must fail loudly. The alternative —
// reporting "no session" — would silently sign every user out during an outage
// and, worse, report a valid token as unknown rather than as a temporary
// failure the client should retry.
//
// These drive that by pointing a real client at a closed port, which exercises
// the same error handling a real outage would rather than a stubbed one.

// deadClient returns a Redis client that cannot connect.
func deadClient(t *testing.T) *goredis.Client {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{
		// A port nothing listens on, with short timeouts so the suite does not
		// spend its life waiting for connections to fail.
		Addr:        "127.0.0.1:1",
		DialTimeout: 200 * time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestTheSessionStoreFailsLoudlyWhenRedisIsDown(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewSessionStore(deadClient(t))

	session := domain.Session{
		ID: "ses_down", UserID: "usr_down", Role: domain.RoleCustomer,
		CreatedAt: time.Now(), LastSeenAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	token, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	if err := store.CreateSession(ctx, session, token.Hash(), time.Hour); err == nil {
		t.Error("CreateSession succeeded against a dead Redis")
	}
	if _, _, err := store.Rotate(ctx, token.Hash(), token.Hash(), time.Hour, time.Now()); err == nil {
		t.Error("Rotate succeeded against a dead Redis")
	}
	if _, err := store.Session(ctx, "ses_down"); err == nil {
		t.Error("Session succeeded against a dead Redis")
	}
	if _, err := store.UserSessions(ctx, "usr_down"); err == nil {
		t.Error("UserSessions succeeded against a dead Redis")
	}
	if err := store.RevokeSession(ctx, "ses_down"); err == nil {
		t.Error("RevokeSession succeeded against a dead Redis")
	}
	if err := store.RevokeUserSessions(ctx, "usr_down"); err == nil {
		t.Error("RevokeUserSessions succeeded against a dead Redis")
	}
}

func TestTheOTPStoreFailsLoudlyWhenRedisIsDown(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewOTPStore(deadClient(t))
	phone := uniquePhone(t)

	if err := store.PutOTP(ctx, phone, "hash", time.Minute); err == nil {
		t.Error("PutOTP succeeded against a dead Redis")
	}
	if _, _, err := store.TakeOTP(ctx, phone); err == nil {
		t.Error("TakeOTP succeeded against a dead Redis")
	}
	if _, _, err := store.PeekOTP(ctx, phone); err == nil {
		t.Error("PeekOTP succeeded against a dead Redis")
	}
	if err := store.DeleteOTP(ctx, phone); err == nil {
		t.Error("DeleteOTP succeeded against a dead Redis")
	}
	if _, err := store.RecordFailedAttempt(ctx, phone, time.Minute); err == nil {
		t.Error("RecordFailedAttempt succeeded against a dead Redis")
	}
	if _, err := store.FailedAttempts(ctx, phone); err == nil {
		t.Error("FailedAttempts succeeded against a dead Redis")
	}
	if err := store.ClearAttempts(ctx, phone); err == nil {
		t.Error("ClearAttempts succeeded against a dead Redis")
	}
}

// A rate limiter that fails open is not a rate limiter. When the counter cannot
// be read the caller must be told, not waved through.
func TestTheRateLimiterFailsLoudlyWhenRedisIsDown(t *testing.T) {
	limiter := identityredis.NewRateLimiter(deadClient(t))

	allowed, _, err := limiter.Allow(context.Background(), "key", 5, time.Minute)
	if err == nil {
		t.Fatal("Allow succeeded against a dead Redis")
	}
	if allowed {
		t.Error("a request was allowed when the limiter could not reach Redis; the limit would not be enforced during an outage")
	}
}

// TestASessionWithACorruptTimestampStillLoads: a session whose stored time is
// unreadable is still a session. Only its display fields suffer, and failing
// the read would sign the user out over a cosmetic problem.
func TestASessionWithACorruptTimestampStillLoads(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	session := uniqueSession(t, "usr_corrupt_1")
	cleanupSession(t, store, session.ID)
	token, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if err := store.CreateSession(ctx, session, token.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := client.HSet(ctx, "session:"+session.ID, "created_at", "not a timestamp").Err(); err != nil {
		t.Fatalf("HSet: %v", err)
	}

	got, err := store.Session(ctx, session.ID)
	if err != nil {
		t.Fatalf("a corrupt timestamp failed the whole read: %v", err)
	}
	if got.UserID != session.UserID {
		t.Errorf("session = %+v", got)
	}
	if !got.CreatedAt.IsZero() {
		t.Errorf("created_at = %v, want the zero time for an unreadable value", got.CreatedAt)
	}
}

// A session whose role is not one of the four is refused rather than defaulted:
// defaulting would hand an unknown principal a working identity.
func TestASessionWithAnUnknownRoleIsRefused(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	session := uniqueSession(t, "usr_badrole_1")
	cleanupSession(t, store, session.ID)
	token, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if err := store.CreateSession(ctx, session, token.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := client.HSet(ctx, "session:"+session.ID, "role", "superuser").Err(); err != nil {
		t.Fatalf("HSet: %v", err)
	}

	if _, err := store.Session(ctx, session.ID); err == nil {
		t.Error("a session with an unrecognised role was loaded")
	}
	// And listing must surface it rather than quietly dropping the session.
	if _, err := store.UserSessions(ctx, session.UserID); err == nil {
		t.Error("listing sessions ignored an unreadable role")
	}
}

// A key of the wrong Redis type makes one specific command fail while the rest
// of the connection works. That reaches error branches a wholly-dead client
// cannot, because a dead client fails on the first call in every method.

func TestRevokingASessionSurfacesACorruptFamilyIndex(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	session := uniqueSession(t, "usr_badfamily_1")
	token, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if err := store.CreateSession(ctx, session, token.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Del(ctx, "refresh_family:"+session.ID, "session:"+session.ID,
			"user_sessions:"+session.UserID, "refresh:"+token.Hash())
	})

	// Replace the family set with a string: SMEMBERS then fails with WRONGTYPE.
	if err := client.Del(ctx, "refresh_family:"+session.ID).Err(); err != nil {
		t.Fatalf("Del: %v", err)
	}
	if err := client.Set(ctx, "refresh_family:"+session.ID, "not a set", time.Minute).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if err := store.RevokeSession(ctx, session.ID); err == nil {
		t.Error("RevokeSession reported success while it could not read the token family; " +
			"tokens would have been left usable")
	}
}

func TestListingSessionsSurfacesACorruptIndex(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	userID := fmt.Sprintf("usr_badindex_%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = client.Del(ctx, "user_sessions:"+userID) })
	if err := client.Set(ctx, "user_sessions:"+userID, "not a set", time.Minute).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if _, err := store.UserSessions(ctx, userID); err == nil {
		t.Error("UserSessions reported no sessions while it could not read the index")
	}
	if err := store.RevokeUserSessions(ctx, userID); err == nil {
		t.Error("RevokeUserSessions reported success while it could not read the index")
	}
}

// A session hash missing a timestamp must still load: only its display fields
// suffer, and failing would sign the user out over a cosmetic problem.
func TestASessionMissingATimestampStillLoads(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	session := uniqueSession(t, "usr_notime_1")
	cleanupSession(t, store, session.ID)
	token, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if err := store.CreateSession(ctx, session, token.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := client.HDel(ctx, "session:"+session.ID, "last_seen_at").Err(); err != nil {
		t.Fatalf("HDel: %v", err)
	}

	got, err := store.Session(ctx, session.ID)
	if err != nil {
		t.Fatalf("a missing timestamp failed the read: %v", err)
	}
	if !got.LastSeenAt.IsZero() {
		t.Errorf("last_seen_at = %v, want the zero time", got.LastSeenAt)
	}
}

// TestSessionsCreatedInTheSameInstantHaveADefinedOrder: without a tie-break,
// "the oldest session" is undefined and the device-limit rule would evict an
// arbitrary device.
func TestSessionsCreatedInTheSameInstantHaveADefinedOrder(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	userID := fmt.Sprintf("usr_tie_%d", time.Now().UnixNano())
	sameMoment := time.Now().UTC().Truncate(time.Millisecond)
	t.Cleanup(func() { _ = client.Del(ctx, "user_sessions:"+userID) })

	for _, suffix := range []string{"b", "a", "c"} {
		session := domain.Session{
			ID: "ses_tie_" + suffix + "_" + userID, UserID: userID, Role: domain.RoleCustomer,
			Device: suffix, CreatedAt: sameMoment, LastSeenAt: sameMoment,
			ExpiresAt: sameMoment.Add(time.Hour),
		}
		cleanupSession(t, store, session.ID)
		token, err := domain.GenerateRefreshToken(nil)
		if err != nil {
			t.Fatalf("GenerateRefreshToken: %v", err)
		}
		if err := store.CreateSession(ctx, session, token.Hash(), time.Hour); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}

	first, err := store.UserSessions(ctx, userID)
	if err != nil {
		t.Fatalf("UserSessions: %v", err)
	}
	second, err := store.UserSessions(ctx, userID)
	if err != nil {
		t.Fatalf("UserSessions: %v", err)
	}
	if len(first) != 3 {
		t.Fatalf("sessions = %d, want 3", len(first))
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Fatalf("the order changed between calls: %v then %v", first, second)
		}
		if i > 0 && first[i-1].ID > first[i].ID {
			t.Errorf("ties are not broken by id: %s before %s", first[i-1].ID, first[i].ID)
		}
	}
}

// TestARotationAgainstACorruptSessionFails.
//
// Replacing the session hash with a string makes the script's own HSET fail, so
// the rotation is refused inside Redis. An earlier version of this test claimed
// to exercise the "session could not be read afterwards" path; it did not,
// because the script fails first. The read-afterwards path is covered by
// TestRotationSurfacesAnUnreadableSessionAfterTheScriptSucceeds below, which
// drives it deterministically instead of relying on a race.
func TestARotationAgainstACorruptSessionFails(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	session := uniqueSession(t, "usr_unreadable_1")
	first, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if err := store.CreateSession(ctx, session, first.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Del(ctx, "session:"+session.ID, "refresh_family:"+session.ID,
			"user_sessions:"+session.UserID)
	})

	if err := client.Del(ctx, "session:"+session.ID).Err(); err != nil {
		t.Fatalf("Del: %v", err)
	}
	if err := client.Set(ctx, "session:"+session.ID, "not a hash", time.Minute).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

	second, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if _, _, err := store.Rotate(ctx, first.Hash(), second.Hash(), time.Hour, time.Now()); err == nil {
		t.Error("Rotate reported success against a corrupt session")
	}
}

// TestRotationSurfacesAnUnreadableSessionAfterTheScriptSucceeds.
//
// The script confirms the session exists and rotates, and the read that follows
// can still fail — the session can expire in between. Reporting success then
// would issue a token for a session nobody could describe, so the rotation is
// refused instead.
func TestRotationSurfacesAnUnreadableSessionAfterTheScriptSucceeds(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewSessionStoreWithScripter(
		redisClient(t),
		// The script reports success for a session that is not there.
		fakeScripter{reply: []any{"ok", "ses_vanished_between_the_two"}},
	)

	token, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	outcome, _, err := store.Rotate(ctx, token.Hash(), token.Hash(), time.Hour, time.Now())
	if err == nil {
		t.Errorf("Rotate reported outcome %v for a session it could not read", outcome)
	}
}

// A reuse whose session has already gone still reports the reuse, carrying the
// session id so the audit line names who was affected.
func TestReuseIsReportedEvenWhenTheSessionHasGone(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	session := uniqueSession(t, "usr_goneresue_1")
	first, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	second, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if err := store.CreateSession(ctx, session, first.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Del(ctx, "refresh_spent:"+first.Hash(), "refresh:"+second.Hash(),
			"refresh_family:"+session.ID, "user_sessions:"+session.UserID)
	})
	if _, _, err := store.Rotate(ctx, first.Hash(), second.Hash(), time.Hour, time.Now()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// The session expires between the rotation and the replay.
	if err := client.Del(ctx, "session:"+session.ID).Err(); err != nil {
		t.Fatalf("Del: %v", err)
	}

	third, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	outcome, got, err := store.Rotate(ctx, first.Hash(), third.Hash(), time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if outcome != ports.RotationReuse {
		t.Fatalf("outcome = %v, want RotationReuse", outcome)
	}
	if got.ID != session.ID {
		t.Errorf("session id = %q, want %q so the audit line names the session", got.ID, session.ID)
	}
}

func TestRevokingASessionSurfacesAnUnreadableSession(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	id := fmt.Sprintf("ses_badhash_%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = client.Del(ctx, "session:"+id) })
	if err := client.Set(ctx, "session:"+id, "not a hash", time.Minute).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if err := store.RevokeSession(ctx, id); err == nil {
		t.Error("RevokeSession reported success for a session it could not read")
	}
}

// A failure revoking one of a user's sessions must fail the whole operation.
// Reporting success while one device is still signed in defeats the point of
// "sign out everywhere".
func TestRevokingEverySessionFailsIfOneCannotBeRevoked(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	userID := fmt.Sprintf("usr_partial_%d", time.Now().UnixNano())
	session := uniqueSession(t, userID)
	token, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if err := store.CreateSession(ctx, session, token.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Del(ctx, "session:"+session.ID, "refresh_family:"+session.ID,
			"user_sessions:"+userID, "refresh:"+token.Hash())
	})

	// Corrupt the family index so revoking this session fails.
	if err := client.Del(ctx, "refresh_family:"+session.ID).Err(); err != nil {
		t.Fatalf("Del: %v", err)
	}
	if err := client.Set(ctx, "refresh_family:"+session.ID, "not a set", time.Minute).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if err := store.RevokeUserSessions(ctx, userID); err == nil {
		t.Error("RevokeUserSessions reported success while a session could not be revoked")
	}
}

// fakeScripter returns whatever reply a test asks for, so the handling of a
// malformed rotation reply is reachable. A real Redis running this package's
// own script cannot produce one — which is exactly why the handling needs
// asserting: it only ever runs when the script has been replaced or corrupted,
// and reading that as "rotation succeeded" would hand out a token for a session
// nobody verified.
type fakeScripter struct {
	reply any
	err   error
}

func (f fakeScripter) Eval(ctx context.Context, _ string, _ []string, _ ...any) *goredis.Cmd {
	return f.result(ctx)
}

func (f fakeScripter) EvalSha(ctx context.Context, _ string, _ []string, _ ...any) *goredis.Cmd {
	return f.result(ctx)
}

func (f fakeScripter) EvalRO(ctx context.Context, _ string, _ []string, _ ...any) *goredis.Cmd {
	return f.result(ctx)
}

func (f fakeScripter) EvalShaRO(ctx context.Context, _ string, _ []string, _ ...any) *goredis.Cmd {
	return f.result(ctx)
}

func (f fakeScripter) ScriptExists(ctx context.Context, _ ...string) *goredis.BoolSliceCmd {
	cmd := goredis.NewBoolSliceCmd(ctx)
	cmd.SetVal([]bool{true})
	return cmd
}

func (f fakeScripter) ScriptLoad(ctx context.Context, _ string) *goredis.StringCmd {
	cmd := goredis.NewStringCmd(ctx)
	cmd.SetVal("sha")
	return cmd
}

func (f fakeScripter) result(ctx context.Context) *goredis.Cmd {
	cmd := goredis.NewCmd(ctx)
	if f.err != nil {
		cmd.SetErr(f.err)
		return cmd
	}
	cmd.SetVal(f.reply)
	return cmd
}

func TestAMalformedRotationReplyIsRefused(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)

	for name, reply := range map[string]any{
		"not a list":              "ok",
		"empty":                   []any{},
		"success with no session": []any{"ok"},
	} {
		store := identityredis.NewSessionStoreWithScripter(client, fakeScripter{reply: reply})
		token, err := domain.GenerateRefreshToken(nil)
		if err != nil {
			t.Fatalf("GenerateRefreshToken: %v", err)
		}

		outcome, _, err := store.Rotate(ctx, token.Hash(), token.Hash(), time.Hour, time.Now())
		if err == nil {
			t.Errorf("%s: Rotate reported outcome %v with no error; a corrupted script must not read as success",
				name, outcome)
		}
	}
}

// TestRevokingASessionSurfacesAPipelineFailure: if the writes that actually
// delete the tokens fail, reporting success would leave a "revoked" session
// fully usable.
func TestRevokingASessionSurfacesAPipelineFailure(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	session := uniqueSession(t, fmt.Sprintf("usr_pipe_%d", time.Now().UnixNano()))
	token, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if err := store.CreateSession(ctx, session, token.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Del(ctx, "session:"+session.ID, "refresh_family:"+session.ID,
			"user_sessions:"+session.UserID, "refresh:"+token.Hash())
	})

	// Replace the user's session index with a string. The pipeline's SREM then
	// fails with WRONGTYPE while every other command in it would have worked.
	if err := client.Del(ctx, "user_sessions:"+session.UserID).Err(); err != nil {
		t.Fatalf("Del: %v", err)
	}
	if err := client.Set(ctx, "user_sessions:"+session.UserID, "not a set", time.Minute).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if err := store.RevokeSession(ctx, session.ID); err == nil {
		t.Error("RevokeSession reported success while its writes failed")
	}
}

// A script that will not run at all is distinct from one that returns
// something odd, and must also fail loudly rather than be read as "no such
// token" — which would sign a legitimate user out during a Redis incident.
func TestARotationScriptFailureIsRefused(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewSessionStoreWithScripter(
		redisClient(t),
		fakeScripter{err: errors.New("NOSCRIPT No matching script")},
	)

	token, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	outcome, _, err := store.Rotate(ctx, token.Hash(), token.Hash(), time.Hour, time.Now())
	if err == nil {
		t.Errorf("Rotate reported outcome %v with no error when the script could not run", outcome)
	}
}
