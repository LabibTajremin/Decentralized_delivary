package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Errors returned when setting holiday mode.
var (
	// ErrHolidayInThePast means the shop would reopen before it closed.
	ErrHolidayInThePast = errors.New("that holiday ends in the past")
	// ErrHolidayTooLong means the break exceeds what we allow without review.
	ErrHolidayTooLong = errors.New("that holiday is too long")
)

// maxHolidayLength caps a self-serve break.
//
// Sixty days: long enough for Eid, a pilgrimage or a renovation, short enough
// that a shop abandoned without a word does not sit in the listings forever
// looking merely closed. A longer absence is a suspension, which is an admin's
// decision and leaves a record.
const maxHolidayLength = 60 * 24 * time.Hour

const maxHolidayReasonLength = 200

// Holiday is an owner closing their shop temporarily.
//
// Distinct from a suspension, which an admin imposes, and from being outside
// opening hours, which recurs. Holiday mode is the owner saying "not this week"
// — so it hides the shop from listings rather than showing it as closed, which
// is what a customer needs when the answer will not change by tomorrow.
type Holiday struct {
	Active bool
	// Until is when the shop reopens. Zero means indefinitely, which is
	// allowed: an owner dealing with a family emergency should not have to
	// predict the date before they can close.
	Until  time.Time
	Reason string
}

// NewHoliday starts a holiday.
func NewHoliday(until time.Time, reason string, now time.Time) (Holiday, error) {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > maxHolidayReasonLength {
		return Holiday{}, fmt.Errorf("%w: limit %d characters", ErrTooLong, maxHolidayReasonLength)
	}

	if !until.IsZero() {
		until = until.UTC()
		if !until.After(now) {
			return Holiday{}, fmt.Errorf("%w: %s", ErrHolidayInThePast, until.Format(time.RFC3339))
		}
		if until.Sub(now) > maxHolidayLength {
			return Holiday{}, fmt.Errorf("%w: limit %d days",
				ErrHolidayTooLong, int(maxHolidayLength/(24*time.Hour)))
		}
	}

	return Holiday{Active: true, Until: until, Reason: reason}, nil
}

// ActiveAt reports whether the shop is on holiday at an instant.
//
// A holiday with an end date expires on its own rather than needing a job to
// switch it off: a sweeper that fails leaves every shop that went away for Eid
// invisible, and nobody notices until the merchants call.
func (h Holiday) ActiveAt(now time.Time) bool {
	if !h.Active {
		return false
	}
	if h.Until.IsZero() {
		return true
	}
	return now.Before(h.Until)
}
