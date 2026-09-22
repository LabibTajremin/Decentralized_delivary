package identity

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
)

func phone(t *testing.T, raw string) domain.Phone {
	t.Helper()
	p, err := domain.NewPhone(raw)
	if err != nil {
		t.Fatalf("NewPhone(%q): %v", raw, err)
	}
	return p
}

func TestGeneratedCodesAreSixDigits(t *testing.T) {
	for i := 0; i < 200; i++ {
		code, err := domain.GenerateOTP(nil)
		if err != nil {
			t.Fatalf("GenerateOTP: %v", err)
		}
		s := code.String()
		if len(s) != domain.OTPLength {
			t.Fatalf("code = %q, want %d digits", s, domain.OTPLength)
		}
		for _, r := range s {
			if r < '0' || r > '9' {
				t.Fatalf("code = %q contains a non-digit", s)
			}
		}
	}
}

// TestGenerationIsUnbiased is why the generator uses rejection sampling. Taking
// a random byte mod 10 makes 0-5 more likely than 6-9, and a code generator
// with a measurable bias is one worth attacking.
func TestGenerationIsUnbiased(t *testing.T) {
	counts := make(map[rune]int)
	const samples = 4000
	for i := 0; i < samples; i++ {
		code, err := domain.GenerateOTP(nil)
		if err != nil {
			t.Fatalf("GenerateOTP: %v", err)
		}
		for _, r := range code.String() {
			counts[r]++
		}
	}

	expected := float64(samples*domain.OTPLength) / 10
	for digit := '0'; digit <= '9'; digit++ {
		got := float64(counts[digit])
		// A generous band: this looks for gross bias, not a specific
		// distribution. A tight bound would make the test itself flaky.
		if got < expected*0.85 || got > expected*1.15 {
			t.Errorf("digit %c appeared %.0f times, expected about %.0f", digit, got, expected)
		}
	}
}

// The rejection loop must discard out-of-range bytes rather than fold them in.
func TestGenerationDiscardsOutOfRangeBytes(t *testing.T) {
	stream := bytes.NewReader([]byte{250, 251, 252, 253, 254, 255, 1, 2, 3, 4, 5, 6})
	code, err := domain.GenerateOTP(stream)
	if err != nil {
		t.Fatalf("GenerateOTP: %v", err)
	}
	if code.String() != "123456" {
		t.Errorf("code = %q, want the rejected bytes skipped", code.String())
	}
}

func TestGenerationSurfacesAnEntropyFailure(t *testing.T) {
	if _, err := domain.GenerateOTP(bytes.NewReader(nil)); err == nil {
		t.Error("GenerateOTP must fail when entropy runs out rather than return a short code")
	}
}

func TestParseRejectsAnythingButSixDigits(t *testing.T) {
	for _, raw := range []string{"", "12345", "1234567", "12345a", "abcdef", "12 345"} {
		if _, err := domain.ParseOTP(raw); !errors.Is(err, domain.ErrInvalidOTPFormat) {
			t.Errorf("ParseOTP(%q) = %v, want ErrInvalidOTPFormat", raw, err)
		}
	}
	if _, err := domain.ParseOTP("  123456  "); err != nil {
		t.Errorf("ParseOTP with surrounding space: %v", err)
	}
}

// Without the phone salt, six-digit codes have a million possible hashes, so
// one precomputed table reverses every OTP in the store at once.
func TestTheHashIsSaltedWithThePhone(t *testing.T) {
	code, err := domain.ParseOTP("123456")
	if err != nil {
		t.Fatalf("ParseOTP: %v", err)
	}
	a := code.Hash(phone(t, "01712345678"))
	b := code.Hash(phone(t, "01812345678"))

	if a == b {
		t.Error("the same code hashes identically for two different numbers")
	}
	if strings.Contains(a, "123456") {
		t.Errorf("the hash %q contains the plaintext code", a)
	}
	if len(a) != 64 {
		t.Errorf("hash = %q, want 64 hex characters of SHA-256", a)
	}
}

func TestHashingIsDeterministic(t *testing.T) {
	code, _ := domain.ParseOTP("123456")
	p := phone(t, "01712345678")
	if code.Hash(p) != code.Hash(p) {
		t.Error("hashing the same code twice gave different results")
	}
}

func TestMatchesAcceptsTheRightCodeAndRejectsOthers(t *testing.T) {
	p := phone(t, "01712345678")
	issued, _ := domain.ParseOTP("123456")
	stored := issued.Hash(p)

	if !issued.Matches(p, stored) {
		t.Error("the issued code does not match its own hash")
	}
	wrong, _ := domain.ParseOTP("654321")
	if wrong.Matches(p, stored) {
		t.Error("a different code matched")
	}
	// The salt is what stops a code harvested for one number being replayed
	// against another.
	if issued.Matches(phone(t, "01812345678"), stored) {
		t.Error("a code matched against a different phone number")
	}
	if issued.Matches(p, "") {
		t.Error("an empty stored hash matched")
	}
}
