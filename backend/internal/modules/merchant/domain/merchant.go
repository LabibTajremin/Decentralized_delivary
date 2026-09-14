// Package domain holds the merchant model: a shop, who owns it, where it is,
// and whether the public may see it.
//
// The two rules that shape everything here are D1 and the approval workflow. D1
// says a merchant may register from anywhere in Bangladesh, so nothing in
// registration asks where they are relative to anyone else. Approval then gates
// visibility, so a merchant that exists and a merchant customers can see are
// deliberately two different things — see IsListed.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Errors returned when building or changing a merchant.
var (
	// ErrEmptyOwner means no account was named as the owner.
	ErrEmptyOwner = errors.New("an owner account is required")
	// ErrEmptyName means the shop has no name.
	ErrEmptyName = errors.New("a shop name is required")
	// ErrNameTooLong means the shop name exceeds the limit.
	ErrNameTooLong = errors.New("shop name is too long")
	// ErrUnknownType means the merchant type is not one we support.
	ErrUnknownType = errors.New("unknown merchant type")
	// ErrUnknownStatus means the status is not one we recognise.
	ErrUnknownStatus = errors.New("unknown merchant status")
	// ErrInvalidPhone means the contact number is not usable.
	ErrInvalidPhone = errors.New("that phone number does not look valid")
	// ErrInvalidEmail means the address is not usable.
	ErrInvalidEmail = errors.New("that email address does not look valid")
	// ErrEmptyAddressLine means the street address is missing.
	ErrEmptyAddressLine = errors.New("a street address is required")
	// ErrTooLong means a free-text field exceeds its limit.
	ErrTooLong = errors.New("that text is too long")
	// ErrInvalidPin means the map pin is not a usable coordinate.
	ErrInvalidPin = errors.New("that map location is not valid")
	// ErrMerchantNotFound means no such merchant exists.
	ErrMerchantNotFound = errors.New("merchant not found")
	// ErrAlreadyRegistered means this owner already has a shop.
	ErrAlreadyRegistered = errors.New("this account already has a shop")
	// ErrInvalidTransition means the requested status change is not allowed.
	ErrInvalidTransition = errors.New("that status change is not allowed")
	// ErrInvalidLogoURL means the logo location is not usable.
	ErrInvalidLogoURL = errors.New("that logo address is not valid")
)

const (
	maxShopNameLength    = 120
	maxAddressLineLength = 200
	maxNoteLength        = 500
	maxEmailLength       = 254
)

// Type is what kind of shop this is.
//
// The three types differ in what the law requires of them and in how their
// catalogue is shaped (P07), which is why the type is fixed at registration and
// not a free-text category.
type Type string

// The supported merchant types.
const (
	TypeRestaurant Type = "restaurant"
	TypeGrocery    Type = "grocery"
	TypePharmacy   Type = "pharmacy"
)

// AllTypes lists the supported types in a fixed order.
func AllTypes() []Type { return []Type{TypeRestaurant, TypeGrocery, TypePharmacy} }

// ParseType reads a merchant type.
func ParseType(s string) (Type, error) {
	candidate := Type(strings.TrimSpace(strings.ToLower(s)))
	for _, known := range AllTypes() {
		if candidate == known {
			return known, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownType, s)
}

// String returns the type code.
func (t Type) String() string { return string(t) }

// Status is where a shop sits in the approval workflow.
type Status string

// The statuses a merchant moves through.
const (
	// StatusDraft is a registration in progress: the shop exists, nobody but
	// its owner can see it, and details can still be edited freely.
	StatusDraft Status = "draft"
	// StatusPendingReview means the owner has submitted and an admin has not
	// yet decided. Details are frozen so a reviewer is not deciding on a
	// version that changed underneath them.
	StatusPendingReview Status = "pending_review"
	// StatusApproved means customers can see it.
	StatusApproved Status = "approved"
	// StatusRejected means an admin refused it, with a reason. Not terminal:
	// the usual cause is a missing or unreadable document, and the owner can
	// fix it and resubmit.
	StatusRejected Status = "rejected"
	// StatusSuspended means an approved shop has been pulled from view. Kept
	// distinct from rejected so a reinstatement does not need a fresh review.
	StatusSuspended Status = "suspended"
)

// AllStatuses lists the statuses in workflow order.
func AllStatuses() []Status {
	return []Status{StatusDraft, StatusPendingReview, StatusApproved, StatusRejected, StatusSuspended}
}

// ParseStatus reads a status.
func ParseStatus(s string) (Status, error) {
	candidate := Status(strings.TrimSpace(strings.ToLower(s)))
	for _, known := range AllStatuses() {
		if candidate == known {
			return known, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownStatus, s)
}

// String returns the status code.
func (s Status) String() string { return string(s) }

// transitions is the whole approval workflow, written once.
//
// A table rather than a chain of ifs spread across the use cases: the set of
// legal moves is a product decision, and the only way to keep it reviewable is
// to keep it in one place where an admin route cannot quietly invent a new one.
var transitions = map[Status][]Status{
	StatusDraft:         {StatusPendingReview},
	StatusPendingReview: {StatusApproved, StatusRejected},
	StatusRejected:      {StatusPendingReview},
	StatusApproved:      {StatusSuspended},
	StatusSuspended:     {StatusApproved},
}

// CanTransitionTo reports whether a status change is allowed.
func (s Status) CanTransitionTo(next Status) bool {
	for _, allowed := range transitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// NextStatuses lists what this status may become, in a fixed order.
func (s Status) NextStatuses() []Status {
	out := make([]Status, len(transitions[s]))
	copy(out, transitions[s])
	return out
}

// Merchant is a shop.
type Merchant struct {
	ID          string
	OwnerUserID string
	Name        string
	Type        Type
	Status      Status
	Phone       string
	Email       string
	LogoURL     string
	Line1       string
	Line2       string
	Pin         Pin
	Placement   Placement
	Documents   []Document
	Hours       WeeklyHours
	Holiday     Holiday
	// ReviewNote is the admin's last word: the reason for a rejection or a
	// suspension. Shown to the owner, because "your shop is not visible" with
	// no reason is a support ticket we have to answer by hand.
	ReviewNote string
	CreatedAt  time.Time
}

// Details is everything an owner tells us about their shop.
//
// Grouped into one struct because registration and editing validate exactly the
// same things, and two copies of the rules is two places for them to drift.
type Details struct {
	Name    string
	Type    Type
	Phone   string
	Email   string
	LogoURL string
	Line1   string
	Line2   string
	Pin     Pin
}

// NewMerchant validates details into a new shop.
//
// It starts in draft with default opening hours, not approved and not visible.
// D1 is honoured by what is *not* here: nothing asks where the shop is relative
// to a customer, a division quota or an existing merchant. Anywhere in
// Bangladesh registers; whether anyone sees it is the approval workflow's
// business and the radius search's (D3).
func NewMerchant(id, ownerUserID string, d Details, now time.Time) (Merchant, error) {
	if strings.TrimSpace(ownerUserID) == "" {
		return Merchant{}, ErrEmptyOwner
	}

	m := Merchant{
		ID:          id,
		OwnerUserID: ownerUserID,
		Status:      StatusDraft,
		Hours:       DefaultHours(),
		CreatedAt:   now,
	}
	return m.WithDetails(d)
}

// WithDetails returns a copy carrying validated details.
func (m Merchant) WithDetails(d Details) (Merchant, error) {
	name := strings.TrimSpace(d.Name)
	if name == "" {
		return Merchant{}, ErrEmptyName
	}
	if utf8.RuneCountInString(name) > maxShopNameLength {
		return Merchant{}, fmt.Errorf("%w: limit %d characters", ErrNameTooLong, maxShopNameLength)
	}

	kind, err := ParseType(d.Type.String())
	if err != nil {
		return Merchant{}, err
	}

	phone, err := NormalisePhone(d.Phone)
	if err != nil {
		return Merchant{}, err
	}

	email := strings.TrimSpace(strings.ToLower(d.Email))
	if email != "" {
		if len(email) > maxEmailLength || !plausibleEmail(email) {
			return Merchant{}, fmt.Errorf("%w: %q", ErrInvalidEmail, d.Email)
		}
	}

	logo, err := normaliseLogoURL(d.LogoURL)
	if err != nil {
		return Merchant{}, err
	}

	line1 := strings.TrimSpace(d.Line1)
	if line1 == "" {
		return Merchant{}, ErrEmptyAddressLine
	}
	line2 := strings.TrimSpace(d.Line2)
	for _, line := range []string{line1, line2} {
		if utf8.RuneCountInString(line) > maxAddressLineLength {
			return Merchant{}, fmt.Errorf("%w: limit %d characters", ErrTooLong, maxAddressLineLength)
		}
	}

	if d.Pin == (Pin{}) {
		return Merchant{}, fmt.Errorf("%w: a map location is required", ErrInvalidPin)
	}

	m.Name = name
	m.Type = kind
	m.Phone = phone
	m.Email = email
	m.LogoURL = logo
	m.Line1 = line1
	m.Line2 = line2
	m.Pin = d.Pin
	return m, nil
}

// WithPlacement returns a copy located in a service area.
func (m Merchant) WithPlacement(p Placement) Merchant {
	m.Placement = p
	return m
}

// WithStatus returns a copy in the new status, refusing an illegal move.
//
// The note is kept whatever the move, including the empty note an approval
// carries: an approval that left a previous rejection reason in place would
// show the owner a live shop alongside the text explaining why it was refused.
func (m Merchant) WithStatus(next Status, note string) (Merchant, error) {
	if !m.Status.CanTransitionTo(next) {
		return Merchant{}, fmt.Errorf("%w: %s to %s", ErrInvalidTransition, m.Status, next)
	}
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > maxNoteLength {
		return Merchant{}, fmt.Errorf("%w: limit %d characters", ErrTooLong, maxNoteLength)
	}
	m.Status = next
	m.ReviewNote = note
	return m, nil
}

// WithHours returns a copy with a new opening schedule.
func (m Merchant) WithHours(h WeeklyHours) Merchant {
	m.Hours = h
	return m
}

// WithHoliday returns a copy with holiday mode set.
func (m Merchant) WithHoliday(h Holiday) Merchant {
	m.Holiday = h
	return m
}

// IsListed reports whether customers may see this shop at all.
//
// This is the acceptance criterion for the phase, and it is one expression
// rather than a check repeated at every call site: a shop is listed when an
// admin has approved it and its owner has not closed it for a holiday. Being
// shut for the evening is not the same thing — see IsOpenAt — because a
// customer scrolling at midnight should still find the restaurant and see when
// it opens.
func (m Merchant) IsListed(now time.Time) bool {
	return m.Status == StatusApproved && !m.Holiday.ActiveAt(now)
}

// IsOpenAt reports whether the shop is taking orders right now.
func (m Merchant) IsOpenAt(now time.Time) bool {
	return m.IsListed(now) && m.Hours.IsOpenAt(now)
}

// OpenStatus is a preformatted line about whether the shop is taking orders.
//
// The server composes it rather than sending the parts, for the same reason as
// every other display string in this system (2.9): a client that assembles
// "Opens at 9:00 AM" itself will assemble it differently from the next client,
// and the customer comparing two screens sees two answers. It is Bengali-first
// because the audience is (1.4).
func (m Merchant) OpenStatus(now time.Time, lang string) string {
	bengali := lang != "en"
	switch {
	case m.Status != StatusApproved:
		if bengali {
			return "এখন বন্ধ"
		}
		return "Not available"
	case m.Holiday.ActiveAt(now):
		if bengali {
			return "ছুটিতে আছে"
		}
		return "On holiday"
	default:
		// ClosingAfter answers "is it open" and "until when" together. Asking
		// IsOpenAt first and then for the closing time would leave a branch for
		// "open but with no closing time", which the schedule cannot produce —
		// both read the same window list with the same test.
		if closing, ok := m.Hours.ClosingAfter(now); ok {
			if bengali {
				return "খোলা আছে — বন্ধ হবে " + closing.String()
			}
			return "Open until " + closing.String()
		}
		if opening, ok := m.Hours.NextOpening(now); ok {
			if bengali {
				return "বন্ধ — খুলবে " + opening.String()
			}
			return "Opens at " + opening.String()
		}
		if bengali {
			return "বন্ধ"
		}
		return "Closed"
	}
}

// SingleLine renders the shop address for a receipt or a rider's screen.
func (m Merchant) SingleLine() string {
	parts := make([]string, 0, 3)
	parts = append(parts, m.Line1)
	if m.Line2 != "" {
		parts = append(parts, m.Line2)
	}
	if m.Placement.AreaName != "" {
		parts = append(parts, m.Placement.AreaName)
	}
	return strings.Join(parts, ", ")
}

// NormalisePhone puts a Bangladeshi number into one stored form.
//
// The merchant module validates the number itself rather than borrowing
// identity's parser: the two are different numbers for different purposes — a
// shop's public line is not the owner's login — and reaching into another
// module's domain to save thirty lines is exactly the coupling 2.5 forbids.
func NormalisePhone(raw string) (string, error) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)

	switch {
	case strings.HasPrefix(digits, "880") && len(digits) == 13:
		digits = "0" + digits[3:]
	case strings.HasPrefix(digits, "1") && len(digits) == 10:
		digits = "0" + digits
	}

	// 01 followed by an operator digit 3-9 and eight more: the whole of the
	// Bangladeshi mobile range, and nothing else.
	if len(digits) != 11 || !strings.HasPrefix(digits, "01") || digits[2] < '3' || digits[2] > '9' {
		return "", fmt.Errorf("%w: %q", ErrInvalidPhone, raw)
	}
	return "+880" + digits[1:], nil
}

// normaliseLogoURL accepts a path we serve or an https URL, and nothing else.
//
// http:// is refused rather than upgraded: a logo loaded over plain HTTP turns
// every merchant page into mixed content, and silently rewriting a URL an owner
// typed hides the problem instead of fixing it.
func normaliseLogoURL(raw string) (string, error) {
	logo := strings.TrimSpace(raw)
	switch {
	case logo == "":
		return "", nil
	case strings.HasPrefix(logo, "//"):
		// Protocol-relative, so it inherits the page's scheme. Refused rather
		// than accepted as "a path we serve": it is not one, and the scheme it
		// would inherit is not ours to promise.
		return "", fmt.Errorf("%w: %q", ErrInvalidLogoURL, raw)
	case strings.HasPrefix(logo, "/"):
		return logo, nil
	case strings.HasPrefix(logo, "https://") && len(logo) > len("https://"):
		return logo, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidLogoURL, raw)
	}
}

// plausibleEmail checks the shape of an address, not its existence.
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
	return !strings.ContainsAny(email, " \t\r\n\"<>,;")
}
