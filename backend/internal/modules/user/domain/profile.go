// Package domain holds the user model: who someone is to the product, as
// distinct from how they prove it.
//
// Identity (P04) owns authentication and keeps only (id, phone, role). Everything
// a person tells us about themselves lives here, so the two can be separated
// later without untangling a shared table — and so a breach of one does not
// hand over the other.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Errors returned when building a profile.
var (
	// ErrEmptyUserID means no account was named.
	ErrEmptyUserID = errors.New("user id is required")
	// ErrNameTooLong means the display name exceeds the limit.
	ErrNameTooLong = errors.New("name is too long")
	// ErrInvalidEmail means the address is not usable.
	ErrInvalidEmail = errors.New("that email address does not look valid")
	// ErrUnsupportedLanguage means the language is not one we serve.
	ErrUnsupportedLanguage = errors.New("unsupported language")
	// ErrProfileNotFound means the user has never saved one.
	ErrProfileNotFound = errors.New("profile not found")
)

// maxNameLength caps a display name.
//
// 80 runes, counted as runes rather than bytes: a Bengali name is three bytes
// per character, and a byte limit would cut it to a third of the length an
// English name gets.
const maxNameLength = 80

// Language is a supported interface language.
type Language string

// The supported languages. Bengali is first because most customers read it, and
// the default falls out of that rather than out of habit.
const (
	LanguageBengali Language = "bn"
	LanguageEnglish Language = "en"
)

// DefaultLanguage is what a new profile gets.
const DefaultLanguage = LanguageBengali

// AllLanguages lists the supported languages in a fixed order.
func AllLanguages() []Language { return []Language{LanguageBengali, LanguageEnglish} }

// ParseLanguage reads a language code.
func ParseLanguage(s string) (Language, error) {
	candidate := Language(strings.TrimSpace(strings.ToLower(s)))
	if candidate == "" {
		return DefaultLanguage, nil
	}
	for _, known := range AllLanguages() {
		if candidate == known {
			return known, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnsupportedLanguage, s)
}

// String returns the language code.
func (l Language) String() string { return string(l) }

// Profile is what a person has told us about themselves.
//
// Sparse on purpose. A delivery service needs a name to put on a parcel and a
// language to speak; everything else is data we would have to protect, explain
// and eventually delete.
type Profile struct {
	UserID string
	// Name may be empty: a customer can order without giving one, and demanding
	// it at sign-up costs more sign-ups than the name is worth.
	Name string
	// Email is optional and used only for receipts.
	Email    string
	Language Language
}

// NewProfile validates and builds a profile.
func NewProfile(userID, name, email string, language Language) (Profile, error) {
	if strings.TrimSpace(userID) == "" {
		return Profile{}, ErrEmptyUserID
	}

	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > maxNameLength {
		return Profile{}, fmt.Errorf("%w: %d characters, limit %d",
			ErrNameTooLong, utf8.RuneCountInString(name), maxNameLength)
	}

	email = strings.TrimSpace(strings.ToLower(email))
	if email != "" && !plausibleEmail(email) {
		return Profile{}, fmt.Errorf("%w: %q", ErrInvalidEmail, email)
	}

	if language == "" {
		language = DefaultLanguage
	}
	parsed, err := ParseLanguage(language.String())
	if err != nil {
		return Profile{}, err
	}

	return Profile{UserID: userID, Name: name, Email: email, Language: parsed}, nil
}

// plausibleEmail checks the shape of an address, not its existence.
//
// Deliberately loose. The only way to know an address works is to send to it,
// and a strict pattern rejects valid addresses — which for a receipt field
// means turning away a customer over an apostrophe. Anything obviously
// unusable is caught; the rest is decided by whether the receipt arrives.
func plausibleEmail(email string) bool {
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return false
	}
	if strings.Count(email, "@") != 1 {
		return false
	}
	domain := email[at+1:]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	if strings.ContainsAny(email, " \t\r\n\"<>,;") {
		return false
	}
	return true
}

// DisplayName returns something safe to greet the user with.
//
// The server decides this rather than the client: an app that falls back to
// "there" in English and another that falls back to a blank line is two
// products. It is also why the fallback is a phrase and not an empty string.
func (p Profile) DisplayName() string {
	if p.Name != "" {
		return p.Name
	}
	if p.Language == LanguageBengali {
		return "গ্রাহক"
	}
	return "Customer"
}
