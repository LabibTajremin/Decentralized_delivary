package domain

import (
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/schedule"
)

// Opening hours are a shared/schedule timetable with the merchant module's own
// policy on top.
//
// The value object — what a window is, how midnight is handled, how a timetable
// is stored — lives in the kernel because an item's availability (P07) and a
// partner's shift (P12) need exactly the same thing, and a second copy of
// "parse HH:MM" is a second place for midnight to be handled differently. What
// stays here is the part that is about shops: how many shifts a day is
// plausible, and that a shop must open at some point.

// Errors returned when building opening hours. They alias the kernel's so a
// caller can keep matching on domain errors, and so the wording an owner sees
// is decided here rather than in a shared package that knows nothing about
// shops.
var (
	// ErrInvalidTimeOfDay means a clock time is not readable.
	ErrInvalidTimeOfDay = schedule.ErrInvalidTimeOfDay
	// ErrInvalidWindow means an opening window does not make sense.
	ErrInvalidWindow = schedule.ErrInvalidWindow
	// ErrOverlappingWindows means two windows on one day collide.
	ErrOverlappingWindows = schedule.ErrOverlappingWindows
	// ErrInvalidWeekday means the day is not a day.
	ErrInvalidWeekday = schedule.ErrInvalidWeekday
	// ErrTooManyWindows means a day has more opening windows than we allow.
	ErrTooManyWindows = schedule.ErrTooManyWindows
)

// TimeOfDay is a clock time in the shop's local day.
type TimeOfDay = schedule.TimeOfDay

// Window is a stretch of the day a shop is open.
type Window = schedule.Window

// NewTimeOfDay builds a clock time.
func NewTimeOfDay(hour, minute int) (TimeOfDay, error) { return schedule.NewTimeOfDay(hour, minute) }

// ParseTimeOfDay reads "HH:MM".
func ParseTimeOfDay(s string) (TimeOfDay, error) { return schedule.ParseTimeOfDay(s) }

// NewWindow validates an opening window.
func NewWindow(open, closes TimeOfDay) (Window, error) { return schedule.NewWindow(open, closes) }

// ParseWindow reads "09:00-22:00".
func ParseWindow(s string) (Window, error) { return schedule.ParseWindow(s) }

// maxWindowsPerDay caps the split shifts in one day. Three covers a restaurant
// that opens for breakfast, lunch and dinner; more is a data-entry accident.
const maxWindowsPerDay = 3

// WeeklyHours is when a shop is open, week by week.
type WeeklyHours struct {
	week schedule.Weekly
}

// NewWeeklyHours builds a schedule. A day absent from the map is closed.
func NewWeeklyHours(days map[time.Weekday][]Window) (WeeklyHours, error) {
	week, err := schedule.New(days, maxWindowsPerDay)
	if err != nil {
		return WeeklyHours{}, err
	}
	return WeeklyHours{week: week}, nil
}

// DefaultHours is what a new shop gets: nine to ten, every day.
//
// A working default rather than an empty schedule, because a shop registered
// with no hours is a shop that is approved and still invisible, and its owner
// has no way to tell why.
func DefaultHours() WeeklyHours {
	opens, _ := schedule.NewTimeOfDay(9, 0)
	closes, _ := schedule.NewTimeOfDay(22, 0)
	return WeeklyHours{week: schedule.EveryDay(Window{Open: opens, Close: closes})}
}

// Windows returns the opening windows for a day, earliest first.
func (h WeeklyHours) Windows(day time.Weekday) []Window { return h.week.Windows(day) }

// IsAlwaysClosed reports whether the schedule never opens.
func (h WeeklyHours) IsAlwaysClosed() bool { return h.week.IsEmpty() }

// IsOpenAt reports whether the shop is open at an instant.
func (h WeeklyHours) IsOpenAt(t time.Time) bool { return h.week.Covers(t) }

// ClosingAfter returns when the current opening window ends.
func (h WeeklyHours) ClosingAfter(t time.Time) (TimeOfDay, bool) { return h.week.EndsAfter(t) }

// NextOpening returns the next time the shop opens, searching a week ahead.
func (h WeeklyHours) NextOpening(t time.Time) (TimeOfDay, bool) { return h.week.NextStart(t) }

// Encode renders the schedule for storage, keyed by weekday number.
func (h WeeklyHours) Encode() map[string][]string { return h.week.Encode() }

// DecodeWeeklyHours reads the stored form back, re-checking every invariant.
//
// A row written before a rule tightened must fail loudly here rather than become
// an invalid schedule in memory.
func DecodeWeeklyHours(encoded map[string][]string) (WeeklyHours, error) {
	week, err := schedule.Decode(encoded, maxWindowsPerDay)
	if err != nil {
		return WeeklyHours{}, err
	}
	return WeeklyHours{week: week}, nil
}
