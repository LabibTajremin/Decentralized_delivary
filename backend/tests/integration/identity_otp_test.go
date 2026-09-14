package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	identityredis "github.com/rootlogic-lab/delivery/backend/internal/modules/identity/infrastructure/redis"
)

// uniquePhone gives each test its own number so tests sharing one Redis do not
// overwrite each other's codes.
func uniquePhone(t *testing.T) domain.Phone {
	t.Helper()
	// A valid Grameenphone number derived from the clock.
	suffix := fmt.Sprintf("%08d", time.Now().UnixNano()%100_000_000)
	p, err := domain.NewPhone("017" + suffix)
	if err != nil {
		t.Fatalf("NewPhone: %v", err)
	}
	return p
}

func TestTakingAnOTPIsSingleUse(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewOTPStore(redisClient(t))
	phone := uniquePhone(t)
	t.Cleanup(func() { _ = store.DeleteOTP(ctx, phone) })

	if err := store.PutOTP(ctx, phone, "the-hash", time.Minute); err != nil {
		t.Fatalf("PutOTP: %v", err)
	}

	hash, ok, err := store.TakeOTP(ctx, phone)
	if err != nil || !ok || hash != "the-hash" {
		t.Fatalf("first take = %q, %v, %v", hash, ok, err)
	}
	_, ok, err = store.TakeOTP(ctx, phone)
	if err != nil {
		t.Fatalf("second take: %v", err)
	}
	if ok {
		t.Error("the code was taken twice; two submissions could create two sessions from one code")
	}
}

// Peeking must not consume, so a mistyped digit does not destroy the code the
// user is still holding.
func TestPeekingDoesNotConsumeTheCode(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewOTPStore(redisClient(t))
	phone := uniquePhone(t)
	t.Cleanup(func() { _ = store.DeleteOTP(ctx, phone) })

	if err := store.PutOTP(ctx, phone, "the-hash", time.Minute); err != nil {
		t.Fatalf("PutOTP: %v", err)
	}
	for i := 0; i < 3; i++ {
		hash, ok, err := store.PeekOTP(ctx, phone)
		if err != nil || !ok || hash != "the-hash" {
			t.Fatalf("peek %d = %q, %v, %v", i, hash, ok, err)
		}
	}
}

func TestAMissingCodeIsReportedAsAbsentNotAsAnError(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewOTPStore(redisClient(t))
	phone := uniquePhone(t)

	if _, ok, err := store.PeekOTP(ctx, phone); err != nil || ok {
		t.Errorf("peek = %v, %v; want absent and no error", ok, err)
	}
	if _, ok, err := store.TakeOTP(ctx, phone); err != nil || ok {
		t.Errorf("take = %v, %v; want absent and no error", ok, err)
	}
}

// Storing a new code replaces the pending one: two live codes for one number
// doubles an attacker's chances for no user benefit.
func TestANewCodeReplacesThePendingOne(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewOTPStore(redisClient(t))
	phone := uniquePhone(t)
	t.Cleanup(func() { _ = store.DeleteOTP(ctx, phone) })

	if err := store.PutOTP(ctx, phone, "first", time.Minute); err != nil {
		t.Fatalf("PutOTP: %v", err)
	}
	if err := store.PutOTP(ctx, phone, "second", time.Minute); err != nil {
		t.Fatalf("PutOTP: %v", err)
	}

	hash, ok, err := store.PeekOTP(ctx, phone)
	if err != nil || !ok {
		t.Fatalf("PeekOTP: %v, %v", ok, err)
	}
	if hash != "second" {
		t.Errorf("stored hash = %q, want only the newest code to be live", hash)
	}
}

func TestTheCodeExpiresOnItsOwn(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewOTPStore(client)
	phone := uniquePhone(t)
	t.Cleanup(func() { _ = store.DeleteOTP(ctx, phone) })

	if err := store.PutOTP(ctx, phone, "the-hash", 30*time.Second); err != nil {
		t.Fatalf("PutOTP: %v", err)
	}
	ttl, err := client.TTL(ctx, "otp:"+phone.String()).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 || ttl > 30*time.Second {
		t.Errorf("TTL = %v, want a positive expiry no longer than requested", ttl)
	}
}

// TestTheLockoutWindowIsNotExtendedByFurtherGuesses is why the TTL is set only
// on the first increment. Refreshing it on every failure would let an attacker
// hold a number locked out indefinitely by guessing once a minute — turning a
// brute-force defence into a denial of service against the number's owner.
func TestTheLockoutWindowIsNotExtendedByFurtherGuesses(t *testing.T) {
	ctx := context.Background()
	client := redisClient(t)
	store := identityredis.NewOTPStore(redisClient(t))
	phone := uniquePhone(t)
	t.Cleanup(func() { _ = store.ClearAttempts(ctx, phone) })

	if _, err := store.RecordFailedAttempt(ctx, phone, 60*time.Second); err != nil {
		t.Fatalf("RecordFailedAttempt: %v", err)
	}
	first, err := client.TTL(ctx, "otp_attempts:"+phone.String()).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}

	// A later guess asking for a much longer window must not extend it.
	if _, err := store.RecordFailedAttempt(ctx, phone, time.Hour); err != nil {
		t.Fatalf("RecordFailedAttempt: %v", err)
	}
	second, err := client.TTL(ctx, "otp_attempts:"+phone.String()).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if second > first {
		t.Errorf("the lockout window grew from %v to %v; an attacker could hold a number locked out forever", first, second)
	}
}

func TestAttemptsCountUpAndClear(t *testing.T) {
	ctx := context.Background()
	store := identityredis.NewOTPStore(redisClient(t))
	phone := uniquePhone(t)
	t.Cleanup(func() { _ = store.ClearAttempts(ctx, phone) })

	if got, err := store.FailedAttempts(ctx, phone); err != nil || got != 0 {
		t.Fatalf("initial attempts = %d, %v", got, err)
	}
	for i := 1; i <= 3; i++ {
		got, err := store.RecordFailedAttempt(ctx, phone, time.Minute)
		if err != nil || got != i {
			t.Fatalf("attempt %d = %d, %v", i, got, err)
		}
	}
	if got, err := store.FailedAttempts(ctx, phone); err != nil || got != 3 {
		t.Errorf("attempts = %d, %v", got, err)
	}
	if err := store.ClearAttempts(ctx, phone); err != nil {
		t.Fatalf("ClearAttempts: %v", err)
	}
	if got, err := store.FailedAttempts(ctx, phone); err != nil || got != 0 {
		t.Errorf("after clearing = %d, %v", got, err)
	}
}

func TestTheRateLimiterCountsWithinItsWindow(t *testing.T) {
	ctx := context.Background()
	limiter := identityredis.NewRateLimiter(redisClient(t))
	client := redisClient(t)
	key := fmt.Sprintf("test:%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = client.Del(ctx, "rate:"+key) })

	for i := 1; i <= 3; i++ {
		allowed, _, err := limiter.Allow(ctx, key, 3, time.Minute)
		if err != nil {
			t.Fatalf("Allow %d: %v", i, err)
		}
		if !allowed {
			t.Fatalf("request %d of 3 was refused", i)
		}
	}

	allowed, retryAfter, err := limiter.Allow(ctx, key, 3, time.Minute)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if allowed {
		t.Error("the fourth request was allowed past a limit of 3")
	}
	if retryAfter <= 0 || retryAfter > time.Minute {
		t.Errorf("retry-after = %v, want a positive time within the window", retryAfter)
	}
}

// Separate keys are separate buckets: one number's abuse must not lock out
// another's.
func TestRateLimitKeysAreIndependent(t *testing.T) {
	ctx := context.Background()
	limiter := identityredis.NewRateLimiter(redisClient(t))
	client := redisClient(t)

	a := fmt.Sprintf("test:a:%d", time.Now().UnixNano())
	b := fmt.Sprintf("test:b:%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = client.Del(ctx, "rate:"+a, "rate:"+b) })

	for i := 0; i < 3; i++ {
		if _, _, err := limiter.Allow(ctx, a, 2, time.Minute); err != nil {
			t.Fatalf("Allow: %v", err)
		}
	}
	allowed, _, err := limiter.Allow(ctx, b, 2, time.Minute)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if !allowed {
		t.Error("exhausting one key's allowance blocked another")
	}
}
