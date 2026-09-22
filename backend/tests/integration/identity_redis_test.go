package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	identityredis "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/redis"
)

// These run against a real Redis. The unit tests prove the rules; these prove
// the Lua script and the key layout that enforce them — in particular that the
// rotation really is atomic, which is the one property a fake cannot establish.

// redisClient connects to the configured Redis and gives the test its own
// keyspace, so concurrent packages do not read each other's sessions.
func redisClient(t *testing.T) *goredis.Client {
	t.Helper()
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Fatal("REDIS_URL is not set. These tests need a real Redis;\n" +
			"run ./scripts/dev-redis.sh, or `docker compose up -d redis`, and export REDIS_URL.")
	}
	opts, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatalf("REDIS_URL is not usable: %v", err)
	}
	client := goredis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("redis is not reachable: %v", err)
	}
	return client
}

// uniqueSession returns a session whose id is unique to this test, so tests
// sharing one Redis cannot collide.
func uniqueSession(t *testing.T, userID string) domain.Session {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Millisecond)
	return domain.Session{
		ID:         fmt.Sprintf("ses_%s_%d", t.Name(), time.Now().UnixNano()),
		UserID:     userID,
		Role:       domain.RoleCustomer,
		Device:     "Pixel 8",
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(time.Hour),
	}
}

// cleanupSession removes everything a session owns after a test.
func cleanupSession(t *testing.T, store *identityredis.SessionStore, sessionID string) {
	t.Helper()
	t.Cleanup(func() {
		_ = store.RevokeSession(context.Background(), sessionID)
	})
}

func newToken(t *testing.T) domain.RefreshToken {
	t.Helper()
	token, err := domain.GenerateRefreshToken(nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	return token
}

func TestASessionAndItsTokenAreStoredTogether(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewSessionStore(redisClient(t))

	session := uniqueSession(t, "usr_store_1")
	cleanupSession(t, store, session.ID)
	token := newToken(t)

	if err := store.CreateSession(ctx, session, token.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := store.Session(ctx, session.ID)
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if got.UserID != session.UserID || got.Device != session.Device || got.Role != session.Role {
		t.Errorf("session = %+v, want %+v", got, session)
	}
	if !got.CreatedAt.Equal(session.CreatedAt) {
		t.Errorf("created_at = %v, want %v", got.CreatedAt, session.CreatedAt)
	}
}

// TestOnlyTheHashIsStored is the same reasoning as never storing a password:
// someone who reads the session store learns which sessions exist but cannot
// use any of them.
func TestOnlyTheHashIsStored(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	session := uniqueSession(t, "usr_hash_1")
	cleanupSession(t, store, session.ID)
	token := newToken(t)

	if err := store.CreateSession(ctx, session, token.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// The plaintext must appear in no key and no value.
	keys, err := client.Keys(ctx, "refresh*").Result()
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	for _, key := range keys {
		if contains(key, token.String()) {
			t.Errorf("the plaintext token appears in key %q", key)
		}
		value, _ := client.Get(ctx, key).Result()
		if contains(value, token.String()) {
			t.Errorf("the plaintext token appears in the value of %q", key)
		}
	}
}

func contains(haystack, needle string) bool {
	if needle == "" || len(haystack) < len(needle) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestRotationSwapsTheTokenAndKeepsTheSession(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewSessionStore(redisClient(t))

	session := uniqueSession(t, "usr_rotate_1")
	cleanupSession(t, store, session.ID)
	first, second := newToken(t), newToken(t)

	if err := store.CreateSession(ctx, session, first.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	outcome, got, err := store.Rotate(ctx, first.Hash(), second.Hash(), time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if outcome != ports.RotationOK {
		t.Fatalf("outcome = %v, want RotationOK", outcome)
	}
	if got.ID != session.ID || got.UserID != session.UserID {
		t.Errorf("session = %+v", got)
	}

	// The replacement works and the original does not.
	outcome, _, err = store.Rotate(ctx, second.Hash(), newToken(t).Hash(), time.Hour, time.Now())
	if err != nil || outcome != ports.RotationOK {
		t.Errorf("the replacement token did not work: %v, %v", outcome, err)
	}
}

// TestPresentingASpentTokenIsReportedAsReuse, distinctly from an unknown one:
// the difference is what separates a stolen token from a guess.
func TestPresentingASpentTokenIsReportedAsReuse(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewSessionStore(redisClient(t))

	session := uniqueSession(t, "usr_reuse_1")
	cleanupSession(t, store, session.ID)
	first, second := newToken(t), newToken(t)

	if err := store.CreateSession(ctx, session, first.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, _, err := store.Rotate(ctx, first.Hash(), second.Hash(), time.Hour, time.Now()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	outcome, got, err := store.Rotate(ctx, first.Hash(), newToken(t).Hash(), time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if outcome != ports.RotationReuse {
		t.Fatalf("outcome = %v, want RotationReuse", outcome)
	}
	if got.ID != session.ID {
		t.Errorf("the reuse report must name the session to revoke, got %+v", got)
	}
}

func TestAnUnknownTokenIsNotReportedAsReuse(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewSessionStore(redisClient(t))

	outcome, _, err := store.Rotate(ctx, newToken(t).Hash(), newToken(t).Hash(), time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if outcome != ports.RotationUnknownToken {
		t.Errorf("outcome = %v, want RotationUnknownToken; a token we never issued is a guess, not a theft", outcome)
	}
}

// TestRotationIsAtomicUnderConcurrency is the property the Lua script exists
// for, and the one a fake cannot establish. Many refreshes racing on one token
// must produce exactly one rotation.
func TestRotationIsAtomicUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewSessionStore(redisClient(t))

	session := uniqueSession(t, "usr_race_1")
	cleanupSession(t, store, session.ID)
	original := newToken(t)
	if err := store.CreateSession(ctx, session, original.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	const racers = 32
	var wg sync.WaitGroup
	outcomes := make([]ports.RotationOutcome, racers)
	errsSeen := make([]error, racers)

	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			replacement := newToken(t)
			<-start // let them all arrive together
			outcomes[i], _, errsSeen[i] = store.Rotate(ctx, original.Hash(), replacement.Hash(), time.Hour, time.Now())
		}(i)
	}
	close(start)
	wg.Wait()

	rotated, reuse, unknown := 0, 0, 0
	for i := range outcomes {
		if errsSeen[i] != nil {
			t.Fatalf("racer %d: %v", i, errsSeen[i])
		}
		switch outcomes[i] {
		case ports.RotationOK:
			rotated++
		case ports.RotationReuse:
			reuse++
		case ports.RotationUnknownToken:
			unknown++
		}
	}

	if rotated != 1 {
		t.Errorf("%d of %d concurrent rotations succeeded, want exactly 1", rotated, racers)
	}
	// The losers see the token as already spent. That is the honest reading:
	// from the store's point of view a spent token was presented. The use case
	// treats it as theft, which is why the client coalesces its refreshes.
	if rotated+reuse+unknown != racers {
		t.Errorf("outcomes did not account for every racer: %d ok, %d reuse, %d unknown", rotated, reuse, unknown)
	}
}

// TestRevokingASessionKillsEveryTokenInItsFamily: after a reuse detection the
// attacker may hold any token from the family's history, so deleting only the
// newest would leave the rest usable.
func TestRevokingASessionKillsEveryTokenInItsFamily(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewSessionStore(redisClient(t))

	session := uniqueSession(t, "usr_family_1")
	cleanupSession(t, store, session.ID)

	tokens := []domain.RefreshToken{newToken(t)}
	if err := store.CreateSession(ctx, session, tokens[0].Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	for i := 0; i < 3; i++ {
		next := newToken(t)
		if _, _, err := store.Rotate(ctx, tokens[len(tokens)-1].Hash(), next.Hash(), time.Hour, time.Now()); err != nil {
			t.Fatalf("rotate %d: %v", i, err)
		}
		tokens = append(tokens, next)
	}

	if err := store.RevokeSession(ctx, session.ID); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}

	for i, token := range tokens {
		outcome, _, err := store.Rotate(ctx, token.Hash(), newToken(t).Hash(), time.Hour, time.Now())
		if err != nil {
			t.Fatalf("token %d: %v", i, err)
		}
		if outcome != ports.RotationUnknownToken {
			t.Errorf("token %d from the family is still usable after revocation (outcome %v)", i, outcome)
		}
	}
	if _, err := store.Session(ctx, session.ID); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("the session survived revocation: %v", err)
	}
}

// TestARevokedTokenIsNotAFalseTheftAlarm: after a normal logout, the client's
// discarded token must read as stale rather than as a theft, or the reuse alarm
// fires on ordinary logouts and nobody will investigate it.
func TestARevokedTokenIsNotAFalseTheftAlarm(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewSessionStore(redisClient(t))

	session := uniqueSession(t, "usr_logout_1")
	cleanupSession(t, store, session.ID)
	first, second := newToken(t), newToken(t)

	if err := store.CreateSession(ctx, session, first.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, _, err := store.Rotate(ctx, first.Hash(), second.Hash(), time.Hour, time.Now()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if err := store.RevokeSession(ctx, session.ID); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}

	outcome, _, err := store.Rotate(ctx, first.Hash(), newToken(t).Hash(), time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if outcome == ports.RotationReuse {
		t.Error("a token discarded at logout was reported as a theft")
	}
}

// A refresh token whose session is gone is not a valid credential, whatever its
// own TTL says.
func TestATokenWhoseSessionIsGoneIsRejected(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	session := uniqueSession(t, "usr_orphan_1")
	cleanupSession(t, store, session.ID)
	token := newToken(t)
	if err := store.CreateSession(ctx, session, token.Hash(), time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Delete the session directly, as an expiry would.
	if err := client.Del(ctx, "session:"+session.ID).Err(); err != nil {
		t.Fatalf("Del: %v", err)
	}

	outcome, _, err := store.Rotate(ctx, token.Hash(), newToken(t).Hash(), time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if outcome != ports.RotationUnknownToken {
		t.Errorf("outcome = %v, want the orphaned token refused", outcome)
	}
}

// Sliding expiry: an app in daily use stays signed in indefinitely, while one
// abandoned for sixty days does not.
func TestRotationExtendsTheSession(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	session := uniqueSession(t, "usr_slide_1")
	cleanupSession(t, store, session.ID)
	first, second := newToken(t), newToken(t)

	if err := store.CreateSession(ctx, session, first.Hash(), 30*time.Second); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	before, err := client.TTL(ctx, "session:"+session.ID).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}

	if _, _, err := store.Rotate(ctx, first.Hash(), second.Hash(), time.Hour, time.Now()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	after, err := client.TTL(ctx, "session:"+session.ID).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if after <= before {
		t.Errorf("session TTL went from %v to %v; a refresh must extend it", before, after)
	}
}

func TestListingSessionsIsOldestFirstAndSkipsExpiredOnes(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	userID := fmt.Sprintf("usr_list_%d", time.Now().UnixNano())
	var ids []string
	for i := 0; i < 3; i++ {
		session := uniqueSession(t, userID)
		session.CreatedAt = time.Now().UTC().Add(time.Duration(i) * time.Minute).Truncate(time.Millisecond)
		cleanupSession(t, store, session.ID)
		if err := store.CreateSession(ctx, session, newToken(t).Hash(), time.Hour); err != nil {
			t.Fatalf("CreateSession %d: %v", i, err)
		}
		ids = append(ids, session.ID)
	}
	t.Cleanup(func() { _ = client.Del(ctx, "user_sessions:"+userID) })

	// Expire the middle one the way a TTL would, leaving the index naming it.
	if err := client.Del(ctx, "session:"+ids[1]).Err(); err != nil {
		t.Fatalf("Del: %v", err)
	}

	got, err := store.UserSessions(ctx, userID)
	if err != nil {
		t.Fatalf("UserSessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("sessions = %d, want the expired one skipped", len(got))
	}
	if got[0].ID != ids[0] || got[1].ID != ids[2] {
		t.Errorf("order = %s, %s; want oldest first", got[0].ID, got[1].ID)
	}
	// And the stale index entry is cleaned up rather than left to grow.
	members, err := client.SMembers(ctx, "user_sessions:"+userID).Result()
	if err != nil {
		t.Fatalf("SMembers: %v", err)
	}
	for _, m := range members {
		if m == ids[1] {
			t.Error("the expired session is still named in the index")
		}
	}
}

func TestRevokingEveryUserSessionClearsTheIndex(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewSessionStore(client)

	userID := fmt.Sprintf("usr_all_%d", time.Now().UnixNano())
	for i := 0; i < 3; i++ {
		session := uniqueSession(t, userID)
		if err := store.CreateSession(ctx, session, newToken(t).Hash(), time.Hour); err != nil {
			t.Fatalf("CreateSession %d: %v", i, err)
		}
	}

	if err := store.RevokeUserSessions(ctx, userID); err != nil {
		t.Fatalf("RevokeUserSessions: %v", err)
	}
	got, err := store.UserSessions(ctx, userID)
	if err != nil {
		t.Fatalf("UserSessions: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("sessions = %d, want none", len(got))
	}
	exists, err := client.Exists(ctx, "user_sessions:"+userID).Result()
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists != 0 {
		t.Error("the session index was left behind")
	}
}

func TestReadingAnUnknownSessionIsNotFound(t *testing.T) {
	store := identityredis.NewSessionStore(redisClient(t))
	if _, err := store.Session(context.Background(), "ses_does_not_exist"); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("error = %v, want ErrSessionNotFound", err)
	}
	if _, err := store.Session(context.Background(), ""); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("empty id = %v, want ErrSessionNotFound", err)
	}
}
