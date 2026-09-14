package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
)

// OTPStore keeps pending one-time codes and their failure counters.
type OTPStore struct {
	client Client
}

// NewOTPStore builds a store.
func NewOTPStore(client Client) *OTPStore { return &OTPStore{client: client} }

// otp:<phone>           str  hashed code, TTL = auth.otp_ttl
// otp_attempts:<phone>  str  failure counter, TTL = auth.otp_lockout_window
func otpKey(phone domain.Phone) string      { return "otp:" + phone.String() }
func attemptsKey(phone domain.Phone) string { return "otp_attempts:" + phone.String() }

// PutOTP stores a hashed code, replacing any pending one.
//
// Replacing rather than adding: two live codes for one number doubles an
// attacker's chances for no benefit to the user, who is only ever looking at
// the most recent message.
func (s *OTPStore) PutOTP(ctx context.Context, phone domain.Phone, hash string, ttl time.Duration) error {
	if err := s.client.Set(ctx, otpKey(phone), hash, ttl).Err(); err != nil {
		return fmt.Errorf("store otp: %w", err)
	}
	return nil
}

// TakeOTP atomically reads and deletes the pending hash.
//
// GETDEL, not GET-then-DEL. With a gap between them, two submissions of the
// same correct code both succeed and create two sessions from one code.
func (s *OTPStore) TakeOTP(ctx context.Context, phone domain.Phone) (string, bool, error) {
	hash, err := s.client.GetDel(ctx, otpKey(phone)).Result()
	if errors.Is(err, goredis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("take otp: %w", err)
	}
	return hash, true, nil
}

// PeekOTP reads the pending hash without consuming it.
//
// Used for the comparison, so a mistyped digit does not destroy the real code
// the user is still holding.
func (s *OTPStore) PeekOTP(ctx context.Context, phone domain.Phone) (string, bool, error) {
	hash, err := s.client.Get(ctx, otpKey(phone)).Result()
	if errors.Is(err, goredis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read otp: %w", err)
	}
	return hash, true, nil
}

// DeleteOTP removes a pending code.
func (s *OTPStore) DeleteOTP(ctx context.Context, phone domain.Phone) error {
	if err := s.client.Del(ctx, otpKey(phone)).Err(); err != nil {
		return fmt.Errorf("delete otp: %w", err)
	}
	return nil
}

// recordAttemptScript increments the counter and sets its window.
//
// One script rather than INCR followed by EXPIRE. As two commands, a process
// that died between them would leave a counter with no expiry — and a lockout
// counter that never expires is a permanent lockout of that phone number, which
// only a manual Redis edit could clear.
//
// The expiry is set on the first failure only. Refreshing it on every failure
// would let an attacker hold a number locked out indefinitely by guessing once
// a minute, turning a brute-force defence into a denial of service against the
// number's owner.
const recordAttemptScript = `
local count = redis.call('INCR', KEYS[1])
if count == 1 then
    redis.call('EXPIRE', KEYS[1], tonumber(ARGV[1]))
end
return count
`

// RecordFailedAttempt increments the counter and returns the new total.
func (s *OTPStore) RecordFailedAttempt(ctx context.Context, phone domain.Phone, window time.Duration) (int, error) {
	count, err := goredis.NewScript(recordAttemptScript).Run(ctx, s.client,
		[]string{attemptsKey(phone)},
		int64(window.Seconds()),
	).Int()
	if err != nil {
		return 0, fmt.Errorf("record attempt: %w", err)
	}
	return count, nil
}

// FailedAttempts returns the current failure count.
func (s *OTPStore) FailedAttempts(ctx context.Context, phone domain.Phone) (int, error) {
	count, err := s.client.Get(ctx, attemptsKey(phone)).Int()
	if errors.Is(err, goredis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read attempts: %w", err)
	}
	return count, nil
}

// ClearAttempts resets the counter after a successful verification.
func (s *OTPStore) ClearAttempts(ctx context.Context, phone domain.Phone) error {
	if err := s.client.Del(ctx, attemptsKey(phone)).Err(); err != nil {
		return fmt.Errorf("clear attempts: %w", err)
	}
	return nil
}
