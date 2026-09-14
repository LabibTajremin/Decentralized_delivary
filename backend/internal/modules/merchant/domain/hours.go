package domain

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Errors returned when building opening hours.
var (
	// ErrInvalidTimeOfDay means a clock time is not readable.
	ErrInvalidTimeOfDay = errors.New("that time is not valid")
	// ErrInvalidWindow means an opening window does not make sense.
	ErrInvalidWindow = errors.New("that opening time does not make sense")
	// ErrOverlappingWindows means two windows on one day collide.
	ErrOverlappingWindows = errors.New("those opening times overlap")
	// ErrInvalidWeekday means the day is not a day.
	ErrInvalidWeekday = errors.New("that day is not valid")
	// ErrTooManyWindows means a day has more opening windows than we allow.
	ErrTooManyWindows = errors.New("too many opening times in one day")
)

// bangladesh is the one timezone this product operates in.
//
// A fixed offset rather than a named zone: Bangladesh has had no daylight
// saving since the 2009 experiment was abandoned, and a fixed zone means
// opening hours do not depend on whether the container image happens to ship
// tzdata. If the country ever adopts DST again this is the single line to
// change.
var bangladesh = time.FixedZone("+06", 6*60*60)

// maxWindowsPerDay caps the split shifts in one day. Three covers a restaurant
// that opens for breakfast, lunch and dinner; more is a data-entry accident.
const maxWindowsPerDay = 3

// endOfDay is midnight expressed as a closing time, so a shop that trades until
// midnight can say so without wrapping into the next day.
const endOfDay TimeOfDay = 24 * 60

// TimeOfDay is a clock time, as minutes from local midnight.
//
// Minutes rather than a time.Time: an opening hour is a recurring rule, not an
// instant, and storing an instant would tie the schedule to the date it was
// entered on.
type TimeOfDay int

// NewTimeOfDay builds a clock time.
func NewTimeOfDay(hour, minute int) (TimeOfDay, error) {
	if hour < 0 || hour > 24 || minute < 0 || minute > 59 || (hour == 24 && minute != 0) {
		return 0, fmt.Errorf("%w: %02d:%02d", ErrInvalidTimeOfDay, hour, minute)
	}
	return TimeOfDay(hour*60 + minute), nil
}

// ParseTimeOfDay reads "HH:MM".
func ParseTimeOfDay(s string) (TimeOfDay, error) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidTimeOfDay, s)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidTimeOfDay, s)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidTimeOfDay, s)
	}
	return NewTimeOfDay(hour, minute)
}

// String renders "HH:MM".
func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", int(t)/60, int(t)%60)
}

// Window is a stretch of the day a shop is open, half-open: [Open, Close).
type Window struct {
	Open  TimeOfDay
	Close TimeOfDay
}

// NewWindow validates an opening window.
//
// Closing must come after opening, so a shop cannot declare hours that wrap past
// midnight. A restaurant trading until 2am enters two windows — one ending at
// 24:00 and one starting the next day at 00:00 — because a wrapping window makes
// every "is it open" and "when does it close" answer ambiguous for the hour
// either side of midnight.
func NewWindow(open, closes TimeOfDay) (Window, error) {
	if open < 0 || open >= endOfDay {
		return Window{}, fmt.Errorf("%w: opens at %s", ErrInvalidWindow, open)
	}
	if closes <= open || closes > endOfDay {
		return Window{}, fmt.Errorf("%w: %s to %s", ErrInvalidWindow, open, closes)
	}
	return Window{Open: open, Close: closes}, nil
}

// Contains reports whether a clock time falls in the window.
func (w Window) Contains(t TimeOfDay) bool { return t >= w.Open && t < w.Close }

// String renders "09:00-22:00".
func (w Window) String() string { return w.Open.String() + "-" + w.Close.String() }

// ParseWindow reads "09:00-22:00".
func ParseWindow(s string) (Window, error) {
	opens, closes, found := strings.Cut(strings.TrimSpace(s), "-")
	if !found {
		return Window{}, fmt.Errorf("%w: %q", ErrInvalidWindow, s)
	}
	from, err := ParseTimeOfDay(opens)
	if err != nil {
		return Window{}, err
	}
	to, err := ParseTimeOfDay(closes)
	if err != nil {
		return Window{}, err
	}
	return NewWindow(from, to)
}

// WeeklyHours is when a shop is open, week by week.
//
// The field is unexported and the type is copied by value, so a schedule handed
// to another package cannot be edited behind the owner's back.
type WeeklyHours struct {
	days [7][]Window
}

// NewWeeklyHours builds a schedule. A day absent from the map is closed.
func NewWeeklyHours(schedule map[time.Weekday][]Window) (WeeklyHours, error) {
	var h WeeklyHours
	for day, windows := range schedule {
		if day < time.Sunday || day > time.Saturday {
			return WeeklyHours{}, fmt.Errorf("%w: %d", ErrInvalidWeekday, int(day))
		}
		if len(windows) > maxWindowsPerDay {
			return WeeklyHours{}, fmt.Errorf("%w: %s has %d, limit %d",
				ErrTooManyWindows, day, len(windows), maxWindowsPerDay)
		}

		sorted := make([]Window, len(windows))
		copy(sorted, windows)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Open < sorted[j].Open })

		for i, w := range sorted {
			if _, err := NewWindow(w.Open, w.Close); err != nil {
				return WeeklyHours{}, err
			}
			if i > 0 && w.Open < sorted[i-1].Close {
				return WeeklyHours{}, fmt.Errorf("%w: %s on %s overlaps %s",
					ErrOverlappingWindows, w, day, sorted[i-1])
			}
		}
		h.days[int(day)] = sorted
	}
	return h, nil
}

// DefaultHours is what a new shop gets: nine to ten, every day.
//
// A working default rather than an empty schedule, because a shop registered
// with no hours is a shop that is approved and still invisible, and its owner
// has no way to tell why.
func DefaultHours() WeeklyHours {
	opens, _ := NewTimeOfDay(9, 0)
	closes, _ := NewTimeOfDay(22, 0)
	window := Window{Open: opens, Close: closes}

	var h WeeklyHours
	for day := range h.days {
		h.days[day] = []Window{window}
	}
	return h
}

// Windows returns the opening windows for a day, earliest first.
func (h WeeklyHours) Windows(day time.Weekday) []Window {
	if day < time.Sunday || day > time.Saturday {
		return nil
	}
	out := make([]Window, len(h.days[int(day)]))
	copy(out, h.days[int(day)])
	return out
}

// IsAlwaysClosed reports whether the schedule never opens.
func (h WeeklyHours) IsAlwaysClosed() bool {
	for _, windows := range h.days {
		if len(windows) > 0 {
			return false
		}
	}
	return true
}

// local converts an instant to the local day and clock time.
func local(t time.Time) (time.Weekday, TimeOfDay) {
	in := t.In(bangladesh)
	return in.Weekday(), TimeOfDay(in.Hour()*60 + in.Minute())
}

// IsOpenAt reports whether the shop is open at an instant.
func (h WeeklyHours) IsOpenAt(t time.Time) bool {
	day, now := local(t)
	for _, w := range h.days[int(day)] {
		if w.Contains(now) {
			return true
		}
	}
	return false
}

// ClosingAfter returns when the current opening window ends.
func (h WeeklyHours) ClosingAfter(t time.Time) (TimeOfDay, bool) {
	day, now := local(t)
	for _, w := range h.days[int(day)] {
		if w.Contains(now) {
			return w.Close, true
		}
	}
	return 0, false
}

// NextOpening returns the next time the shop opens, searching a week ahead.
//
// A week and no further: a schedule that never opens has no answer, and looping
// forever looking for one would hang the request rather than say so.
func (h WeeklyHours) NextOpening(t time.Time) (TimeOfDay, bool) {
	day, now := local(t)
	for ahead := 0; ahead < 7; ahead++ {
		index := (int(day) + ahead) % 7
		for _, w := range h.days[index] {
			if ahead > 0 || w.Open > now {
				return w.Open, true
			}
		}
	}
	return 0, false
}

// Encode renders the schedule for storage, keyed by weekday number.
//
// A storage form the domain owns, rather than JSON tags on the type: the
// schedule is a value object with an invariant (sorted, non-overlapping), and
// letting a decoder build one field by field would let it build an invalid one.
func (h WeeklyHours) Encode() map[string][]string {
	out := make(map[string][]string, 7)
	for day, windows := range h.days {
		if len(windows) == 0 {
			continue
		}
		rendered := make([]string, 0, len(windows))
		for _, w := range windows {
			rendered = append(rendered, w.String())
		}
		out[strconv.Itoa(day)] = rendered
	}
	return out
}

// DecodeWeeklyHours reads the stored form back, re-checking every invariant.
func DecodeWeeklyHours(encoded map[string][]string) (WeeklyHours, error) {
	schedule := make(map[time.Weekday][]Window, len(encoded))
	for key, rendered := range encoded {
		day, err := strconv.Atoi(key)
		if err != nil || day < 0 || day > 6 {
			return WeeklyHours{}, fmt.Errorf("%w: %q", ErrInvalidWeekday, key)
		}
		windows := make([]Window, 0, len(rendered))
		for _, text := range rendered {
			w, parseErr := ParseWindow(text)
			if parseErr != nil {
				return WeeklyHours{}, parseErr
			}
			windows = append(windows, w)
		}
		schedule[time.Weekday(day)] = windows
	}
	return NewWeeklyHours(schedule)
}
