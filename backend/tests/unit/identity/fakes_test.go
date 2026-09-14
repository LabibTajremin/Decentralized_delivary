package identity

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"sync"
	"time"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
)

// In-memory stand-ins for every port. They are deliberately simple and
// deliberately faithful on the points that matter — TakeOTP is atomic here
// because it must be atomic in Redis, and Rotate implements the same three
// outcomes — so a test that passes against these is testing the rule, not the
// fake.

var errStore = errors.New("store unavailable")

type fakeOTPStore struct {
	mu       sync.Mutex
	hashes   map[string]string
	attempts map[string]int
	putErr   error
	peekErr  error
	// peekHash makes PeekOTP succeed for a code that TakeOTP will not find,
	// which is what the loser of a race to consume the same code sees.
	peekHash  string
	deleteErr error
	takeErr   error
	countErr  error
	incrErr   error
	clearErr  error
	deleted   []string
}

func newOTPStore() *fakeOTPStore {
	return &fakeOTPStore{hashes: map[string]string{}, attempts: map[string]int{}}
}

func (f *fakeOTPStore) PutOTP(_ context.Context, p domain.Phone, hash string, _ time.Duration) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hashes[p.String()] = hash
	return nil
}

func (f *fakeOTPStore) TakeOTP(_ context.Context, p domain.Phone) (string, bool, error) {
	if f.takeErr != nil {
		return "", false, f.takeErr
	}
	// Atomic, because it must be atomic in Redis: a read-then-delete lets two
	// submissions of one code both create a session.
	f.mu.Lock()
	defer f.mu.Unlock()
	hash, ok := f.hashes[p.String()]
	if ok {
		delete(f.hashes, p.String())
	}
	return hash, ok, nil
}

func (f *fakeOTPStore) PeekOTP(_ context.Context, p domain.Phone) (string, bool, error) {
	if f.peekErr != nil {
		return "", false, f.peekErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.peekHash != "" {
		return f.peekHash, true, nil
	}
	hash, ok := f.hashes[p.String()]
	return hash, ok, nil
}

func (f *fakeOTPStore) RecordFailedAttempt(_ context.Context, p domain.Phone, _ time.Duration) (int, error) {
	if f.incrErr != nil {
		return 0, f.incrErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts[p.String()]++
	return f.attempts[p.String()], nil
}

func (f *fakeOTPStore) FailedAttempts(_ context.Context, p domain.Phone) (int, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts[p.String()], nil
}

func (f *fakeOTPStore) ClearAttempts(_ context.Context, p domain.Phone) error {
	if f.clearErr != nil {
		return f.clearErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.attempts, p.String())
	return nil
}

func (f *fakeOTPStore) DeleteOTP(_ context.Context, p domain.Phone) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.hashes, p.String())
	f.deleted = append(f.deleted, p.String())
	return nil
}

// fakeSessionStore implements the same three rotation outcomes as the Lua
// script, including the spent-token index that distinguishes reuse from an
// unknown token.
type fakeSessionStore struct {
	mu       sync.Mutex
	sessions map[string]domain.Session
	current  map[string]string // refresh hash -> session id
	spent    map[string]string // rotated-away hash -> session id
	family   map[string][]string

	createErr error
	rotateErr error
	listErr   error
	revokeErr error
	readErr   error

	revoked     []string
	userRevoked []string
}

func newSessionStore() *fakeSessionStore {
	return &fakeSessionStore{
		sessions: map[string]domain.Session{},
		current:  map[string]string{},
		spent:    map[string]string{},
		family:   map[string][]string{},
	}
}

func (f *fakeSessionStore) CreateSession(_ context.Context, s domain.Session, hash string, _ time.Duration) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions[s.ID] = s
	f.current[hash] = s.ID
	f.family[s.ID] = append(f.family[s.ID], hash)
	return nil
}

func (f *fakeSessionStore) Rotate(_ context.Context, oldHash, newHash string, _ time.Duration, now time.Time) (ports.RotationOutcome, domain.Session, error) {
	if f.rotateErr != nil {
		return ports.RotationUnknownToken, domain.Session{}, f.rotateErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	sessionID, ok := f.current[oldHash]
	if !ok {
		if owner, spent := f.spent[oldHash]; spent {
			return ports.RotationReuse, f.sessions[owner], nil
		}
		return ports.RotationUnknownToken, domain.Session{}, nil
	}
	session, exists := f.sessions[sessionID]
	if !exists {
		return ports.RotationUnknownToken, domain.Session{}, nil
	}

	delete(f.current, oldHash)
	f.spent[oldHash] = sessionID
	f.current[newHash] = sessionID
	f.family[sessionID] = append(f.family[sessionID], newHash)

	session.LastSeenAt = now
	f.sessions[sessionID] = session
	return ports.RotationOK, session, nil
}

func (f *fakeSessionStore) Session(_ context.Context, id string) (domain.Session, error) {
	if f.readErr != nil {
		return domain.Session{}, f.readErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	if !ok {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	return s, nil
}

func (f *fakeSessionStore) UserSessions(_ context.Context, userID string) ([]domain.Session, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Session
	for _, s := range f.sessions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	// Oldest first with the id breaking ties, matching the real store, so
	// eviction has a defined victim even when two sessions share an instant.
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *fakeSessionStore) RevokeSession(_ context.Context, id string) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, hash := range f.family[id] {
		delete(f.current, hash)
		delete(f.spent, hash)
	}
	delete(f.family, id)
	delete(f.sessions, id)
	f.revoked = append(f.revoked, id)
	return nil
}

func (f *fakeSessionStore) RevokeUserSessions(ctx context.Context, userID string) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	sessions, err := f.UserSessions(ctx, userID)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if err := f.RevokeSession(ctx, s.ID); err != nil {
			return err
		}
	}
	f.userRevoked = append(f.userRevoked, userID)
	return nil
}

// fakeSigner produces predictable tokens so an assertion can name one.
type fakeSigner struct {
	signErr   error
	verifyErr error
	issued    []domain.Claims
	mu        sync.Mutex
}

func (f *fakeSigner) Sign(claims domain.Claims) (string, error) {
	if f.signErr != nil {
		return "", f.signErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.issued = append(f.issued, claims)
	return "access." + claims.UserID + "." + claims.SessionID, nil
}

func (f *fakeSigner) Verify(string) (domain.Claims, error) {
	if f.verifyErr != nil {
		return domain.Claims{}, f.verifyErr
	}
	return domain.Claims{}, nil
}

// fakeLimiter allows everything until told otherwise.
type fakeLimiter struct {
	deny     bool
	err      error
	seenKeys []string
	mu       sync.Mutex
}

func (f *fakeLimiter) Allow(_ context.Context, key string, _ int, window time.Duration) (bool, time.Duration, error) {
	if f.err != nil {
		return false, 0, f.err
	}
	f.mu.Lock()
	f.seenKeys = append(f.seenKeys, key)
	f.mu.Unlock()
	if f.deny {
		return false, window, nil
	}
	return true, 0, nil
}

// fakeSMS records what was sent.
type fakeSMS struct {
	err   error
	sent  []string
	codes []string
	mu    sync.Mutex
}

func (f *fakeSMS) SendOTP(_ context.Context, p domain.Phone, code string) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, p.String())
	f.codes = append(f.codes, code)
	return nil
}

// fakeDirectory hands out account ids.
type fakeDirectory struct {
	err      error
	known    map[string]string
	nextID   string
	ensureNo int
}

func newDirectory() *fakeDirectory {
	return &fakeDirectory{known: map[string]string{}, nextID: "usr_1"}
}

func (f *fakeDirectory) EnsureUser(_ context.Context, p domain.Phone, role domain.Role) (string, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	f.ensureNo++
	key := p.String() + ":" + role.String()
	if id, ok := f.known[key]; ok {
		return id, false, nil
	}
	f.known[key] = f.nextID
	return f.nextID, true, nil
}

// fakeConfig serves the registry defaults, or whatever a test overrides.
type fakeConfig struct {
	values map[string]int64
	err    error
}

func newConfig() *fakeConfig {
	return &fakeConfig{values: map[string]int64{
		cfgcontract.AuthOTPRequestsPerHour: 5,
		cfgcontract.AuthOTPVerifyAttempts:  5,
		cfgcontract.AuthOTPLockoutWindow:   900,
		cfgcontract.AuthOTPTTL:             300,
		cfgcontract.AuthAccessTokenTTL:     900,
		cfgcontract.AuthRefreshTokenTTL:    5_184_000,
		cfgcontract.AuthMaxSessions:        5,
	}}
}

func (f *fakeConfig) Settings(context.Context, cfgcontract.Placement) (cfgcontract.Settings, error) {
	if f.err != nil {
		return nil, f.err
	}
	return fakeSettings{values: f.values}, nil
}

type fakeSettings struct {
	values map[string]int64
}

func (s fakeSettings) Int(key string) (int64, error) {
	v, ok := s.values[key]
	if !ok {
		return 0, errors.New("unknown key " + key)
	}
	return v, nil
}

func (s fakeSettings) Bool(string) (bool, error)     { return false, errors.New("not a bool") }
func (s fakeSettings) Ratio(string) (float64, error) { return 0, errors.New("not a ratio") }

// Deterministic clock, ids and entropy.
type fixedClock struct{ at time.Time }

func (f fixedClock) Now() time.Time { return f.at }

type seqIDs struct {
	mu sync.Mutex
	n  int
}

func (s *seqIDs) New(prefix string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	return prefix + "_" + string(rune('a'+(s.n-1)%26))
}

// countingReader yields distinct bytes so generated tokens differ.
type countingReader struct {
	mu sync.Mutex
	n  byte
}

func (c *countingReader) Read(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range p {
		c.n++
		p[i] = c.n
	}
	return len(p), nil
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
