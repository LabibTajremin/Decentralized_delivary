package identity

import (
	"errors"
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
)

// A phone number is a credential here: it is the rate-limit key, the OTP key
// and the account identity. Two spellings of one number would be two rate-limit
// buckets, so an attacker could double their allowance by alternating format.

func TestEverySpellingOfANumberNormalisesToOne(t *testing.T) {
	spellings := []string{
		"01712345678",
		"+8801712345678",
		"8801712345678",
		"1712345678",
		"017-1234-5678",
		"+880 1712 345678",
		"  01712345678  ",
	}
	const want = "+8801712345678"

	for _, raw := range spellings {
		p, err := domain.NewPhone(raw)
		if err != nil {
			t.Errorf("NewPhone(%q): %v", raw, err)
			continue
		}
		if p.String() != want {
			t.Errorf("NewPhone(%q) = %q, want %q", raw, p.String(), want)
		}
	}
}

func TestEveryBangladeshiOperatorPrefixIsAccepted(t *testing.T) {
	// Grameenphone, Robi, Airtel, Banglalink, Teletalk.
	for _, prefix := range []string{"13", "14", "15", "16", "17", "18", "19"} {
		raw := "0" + prefix + "12345678"
		if _, err := domain.NewPhone(raw); err != nil {
			t.Errorf("NewPhone(%q): %v", raw, err)
		}
	}
}

func TestNonMobileNumbersAreRejected(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"0212345678",     // a Dhaka landline
		"01012345678",    // no such operator prefix
		"01112345678",    // nor this one
		"0171234567",     // too short
		"017123456789",   // too long
		"+9171234567890", // another country
		"not a number",
	}
	for _, raw := range cases {
		if _, err := domain.NewPhone(raw); err == nil {
			t.Errorf("NewPhone(%q) was accepted", raw)
		}
	}
}

func TestEmptyInputIsReportedAsEmpty(t *testing.T) {
	if _, err := domain.NewPhone("   "); !errors.Is(err, domain.ErrEmptyPhone) {
		t.Errorf("error = %v, want ErrEmptyPhone", err)
	}
}

func TestAWrongPrefixIsReportedAsInvalid(t *testing.T) {
	if _, err := domain.NewPhone("01012345678"); !errors.Is(err, domain.ErrInvalidPhone) {
		t.Errorf("error = %v, want ErrInvalidPhone", err)
	}
}

// A full phone number in a log line is personal data sitting where many people
// can read it, and an OTP screen showing the whole number confirms a guess.
func TestMaskingHidesTheMiddle(t *testing.T) {
	p, err := domain.NewPhone("01712345678")
	if err != nil {
		t.Fatalf("NewPhone: %v", err)
	}
	masked := p.Masked()

	if strings.Contains(masked, "12345") {
		t.Errorf("masked = %q, which still shows the middle digits", masked)
	}
	if !strings.HasPrefix(masked, "+88017") {
		t.Errorf("masked = %q, want the operator prefix kept so a user recognises it", masked)
	}
	if !strings.HasSuffix(masked, "678") {
		t.Errorf("masked = %q, want the last digits kept", masked)
	}
	if len(masked) != len(p.String()) {
		t.Errorf("masked = %q (%d chars), want the length of %q", masked, len(masked), p.String())
	}
}

func TestTheZeroPhoneIsRecognisable(t *testing.T) {
	var zero domain.Phone
	if !zero.IsZero() {
		t.Error("the zero Phone does not report itself as zero")
	}
	p, _ := domain.NewPhone("01712345678")
	if p.IsZero() {
		t.Error("a real phone reports itself as zero")
	}
	if got := zero.Masked(); got != "" {
		t.Errorf("zero masked = %q; masking must not panic on an empty value", got)
	}
}
