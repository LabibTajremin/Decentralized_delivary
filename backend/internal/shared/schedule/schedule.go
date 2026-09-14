// Package schedule is the one representation of a recurring weekly timetable.
//
// Two modules already need one and more will: a shop's opening hours (P06), an
// item's availability (P07), a delivery partner's shift (P12). They differ in
// policy — how many windows a day is reasonable, whether an empty schedule is
// allowed — but not in what a window *is*, and a second copy of "parse HH:MM"
// is a second place for midnight to be handled differently.
//
// So the value object lives here and each module layers its own rules on top.
package schedule

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Errors returned when building a schedule.
var (
	// ErrInvalidTimeOfDay means a clock time is not readable.
	ErrInvalidTimeOfDay = errors.New("that time is not valid")
	// ErrInvalidWindow means a window does not make sense.
	ErrInvalidWindow = errors.New("that time range does not make sense")
	// ErrOverlappingWindows means two windows on one day collide.
	ErrOverlappingWindows = errors.New("those times overlap")
	// ErrInvalidWeekday means the day is not a day.
	ErrInvalidWeekday = errors.New("that day is not valid")
)

// Bangladesh is the one timezone this product operates in.
//
// A fixed offset rather than a named zone: the country has had no daylight
// saving since the 2009 experiment was abandoned, and a fixed zone means a
// timetable does not depend on whether the container image ships tzdata. If DST
// is ever adopted again, this is the single line to change.
var Bangladesh = time.FixedZone("+06", 6*60*60)

// EndOfDay is midnight expressed as a closing time, so something that runs until
// midnight can say so without wrapping into the next day.
const EndOfDay TimeOfDay = 24 * 60

// TimeOfDay is a clock time, as minutes from local midnight.
//
// Minutes rather than a time.Time: a recurring rule is not an instant, and
// storing an instant would tie the timetable to the date it was entered on.
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

// Window is a stretch of the day, half-open: [Open, Close).
type Window struct {
	Open  TimeOfDay
	Close TimeOfDay
}

// NewWindow validates a window.
//
// Closing must come after opening, so a window cannot wrap past midnight.
// Something running until 2am is entered as two windows — one ending at 24:00
// and one starting the next day at 00:00 — because a wrapping window makes every
// "is it on now" and "when does it end" answer ambiguous for the hour either
// side of midnight.
func NewWindow(open, closes TimeOfDay) (Window, error) {
	if open < 0 || open >= EndOfDay {
		return Window{}, fmt.Errorf("%w: starts at %s", ErrInvalidWindow, open)
	}
	if closes <= open || closes > EndOfDay {
		return Window{}, fmt.Errorf("%w: %s to %s", ErrInvalidWindow, open, closes)
	}
	return Window{Open: open, Close: closes}, nil
}

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

// Contains reports whether a clock time falls in the window.
func (w Window) Contains(t TimeOfDay) bool { return t >= w.Open && t < w.Close }

// String renders "09:00-22:00".
func (w Window) String() string { return w.Open.String() + "-" + w.Close.String() }

// Weekly is a timetable, week by week.
//
// The field is unexported and the type is copied by value, so a timetable handed
// to another package cannot be edited behind its owner's back.
type Weekly struct {
	days [7][]Window
}

// New builds a timetable. A day absent from the map is off.
//
// maxPerDay caps how many windows one day may hold; pass 0 for no cap. The
// limit is the caller's policy — three shifts is plenty for a restaurant,
// while an item's availability has no natural ceiling — so it is a parameter
// rather than a constant here.
func New(days map[time.Weekday][]Window, maxPerDay int) (Weekly, error) {
	var w Weekly
	for day, windows := range days {
		if day < time.Sunday || day > time.Saturday {
			return Weekly{}, fmt.Errorf("%w: %d", ErrInvalidWeekday, int(day))
		}
		if maxPerDay > 0 && len(windows) > maxPerDay {
			return Weekly{}, fmt.Errorf("%w: %s has %d, limit %d",
				ErrTooManyWindows, day, len(windows), maxPerDay)
		}

		sorted := make([]Window, len(windows))
		copy(sorted, windows)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Open < sorted[j].Open })

		for i, window := range sorted {
			if _, err := NewWindow(window.Open, window.Close); err != nil {
				return Weekly{}, err
			}
			if i > 0 && window.Open < sorted[i-1].Close {
				return Weekly{}, fmt.Errorf("%w: %s on %s overlaps %s",
					ErrOverlappingWindows, window, day, sorted[i-1])
			}
		}
		w.days[int(day)] = sorted
	}
	return w, nil
}

// ErrTooManyWindows means a day holds more windows than the caller allows.
var ErrTooManyWindows = errors.New("too many time ranges in one day")

// EveryDay builds a timetable with the same window on all seven days.
func EveryDay(window Window) Weekly {
	var w Weekly
	for day := range w.days {
		w.days[day] = []Window{window}
	}
	return w
}

// Windows returns the windows for a day, earliest first.
func (w Weekly) Windows(day time.Weekday) []Window {
	if day < time.Sunday || day > time.Saturday {
		return nil
	}
	out := make([]Window, len(w.days[int(day)]))
	copy(out, w.days[int(day)])
	return out
}

// IsEmpty reports whether the timetable never opens.
func (w Weekly) IsEmpty() bool {
	for _, windows := range w.days {
		if len(windows) > 0 {
			return false
		}
	}
	return true
}

// Local converts an instant to the local day and clock time.
func Local(t time.Time) (time.Weekday, TimeOfDay) {
	in := t.In(Bangladesh)
	return in.Weekday(), TimeOfDay(in.Hour()*60 + in.Minute())
}

// Covers reports whether an instant falls inside a window.
func (w Weekly) Covers(t time.Time) bool {
	day, now := Local(t)
	for _, window := range w.days[int(day)] {
		if window.Contains(now) {
			return true
		}
	}
	return false
}

// EndsAfter returns when the window covering an instant closes.
func (w Weekly) EndsAfter(t time.Time) (TimeOfDay, bool) {
	day, now := Local(t)
	for _, window := range w.days[int(day)] {
		if window.Contains(now) {
			return window.Close, true
		}
	}
	return 0, false
}

// NextStart returns the next time a window opens, searching a week ahead.
//
// A week and no further: a timetable that never opens has no answer, and looping
// to find one would hang the request rather than say so.
func (w Weekly) NextStart(t time.Time) (TimeOfDay, bool) {
	day, now := Local(t)
	for ahead := 0; ahead < 7; ahead++ {
		index := (int(day) + ahead) % 7
		for _, window := range w.days[index] {
			if ahead > 0 || window.Open > now {
				return window.Open, true
			}
		}
	}
	return 0, false
}

// Encode renders the timetable for storage, keyed by weekday number.
//
// A storage form the value object owns, rather than JSON tags on the type: a
// timetable has an invariant — sorted, non-overlapping — and letting a decoder
// build one field by field would let it build an invalid one.
func (w Weekly) Encode() map[string][]string {
	out := make(map[string][]string, 7)
	for day, windows := range w.days {
		if len(windows) == 0 {
			continue
		}
		rendered := make([]string, 0, len(windows))
		for _, window := range windows {
			rendered = append(rendered, window.String())
		}
		out[strconv.Itoa(day)] = rendered
	}
	return out
}

// Decode reads the stored form back, re-checking every invariant.
func Decode(encoded map[string][]string, maxPerDay int) (Weekly, error) {
	days := make(map[time.Weekday][]Window, len(encoded))
	for key, rendered := range encoded {
		day, err := strconv.Atoi(key)
		if err != nil || day < 0 || day > 6 {
			return Weekly{}, fmt.Errorf("%w: %q", ErrInvalidWeekday, key)
		}
		windows := make([]Window, 0, len(rendered))
		for _, text := range rendered {
			window, parseErr := ParseWindow(text)
			if parseErr != nil {
				return Weekly{}, parseErr
			}
			windows = append(windows, window)
		}
		days[time.Weekday(day)] = windows
	}
	return New(days, maxPerDay)
}
