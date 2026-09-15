package shared

import (
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/schedule"
)

// The weekly timetable is shared by a shop's opening hours (P06), an item's
// availability (P07) and a partner's shift (P12). These tests are the reason
// those three can share it: they pin the behaviour at midnight, across
// timezones, and through storage, which is where three separate copies would
// have drifted apart.

// dhaka builds an instant in Bangladesh local time, so a test that says
// "Monday at nine" means what a shop owner would mean by it.
func dhaka(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, schedule.Bangladesh)
}

func window(t *testing.T, text string) schedule.Window {
	t.Helper()
	w, err := schedule.ParseWindow(text)
	if err != nil {
		t.Fatalf("ParseWindow(%q): %v", text, err)
	}
	return w
}

// ---------------------------------------------------------- time of day

func TestAClockTimeRoundTrips(t *testing.T) {
	for _, text := range []string{"00:00", "09:30", "23:59", "24:00"} {
		parsed, err := schedule.ParseTimeOfDay(text)
		if err != nil {
			t.Errorf("ParseTimeOfDay(%q): %v", text, err)
			continue
		}
		if parsed.String() != text {
			t.Errorf("ParseTimeOfDay(%q).String() = %q", text, parsed)
		}
	}

	if parsed, err := schedule.ParseTimeOfDay("  09:30 "); err != nil || parsed.String() != "09:30" {
		t.Errorf("a padded time = %q, %v", parsed, err)
	}
}

func TestUnreadableClockTimesAreRefused(t *testing.T) {
	for _, text := range []string{"", "9:30", "09:3", "0930", "09:30:00", "ab:cd", "09:cd", "25:00", "24:30", "09:60"} {
		if _, err := schedule.ParseTimeOfDay(text); !errors.Is(err, schedule.ErrInvalidTimeOfDay) {
			t.Errorf("ParseTimeOfDay(%q) error = %v, want ErrInvalidTimeOfDay", text, err)
		}
	}
}

func TestNewTimeOfDayRejectsImpossibleClocks(t *testing.T) {
	for _, c := range [][2]int{{-1, 0}, {25, 0}, {9, -1}, {9, 60}, {24, 30}} {
		if _, err := schedule.NewTimeOfDay(c[0], c[1]); !errors.Is(err, schedule.ErrInvalidTimeOfDay) {
			t.Errorf("NewTimeOfDay(%d, %d) error = %v", c[0], c[1], err)
		}
	}
	if got, err := schedule.NewTimeOfDay(24, 0); err != nil || got != schedule.EndOfDay {
		t.Errorf("NewTimeOfDay(24, 0) = %v, %v; want EndOfDay", got, err)
	}
}

// ------------------------------------------------------------- windows

// TestAWindowMayNotWrapPastMidnight. A wrapping window makes every "is it on"
// and "when does it end" answer ambiguous for the hour either side of midnight;
// something running late is entered as two windows instead.
func TestAWindowMayNotWrapPastMidnight(t *testing.T) {
	late, _ := schedule.ParseTimeOfDay("22:00")
	early, _ := schedule.ParseTimeOfDay("02:00")

	if _, err := schedule.NewWindow(late, early); !errors.Is(err, schedule.ErrInvalidWindow) {
		t.Errorf("error = %v, want ErrInvalidWindow", err)
	}
}

func TestWindowsThatDoNotMakeSenseAreRefused(t *testing.T) {
	for _, text := range []string{"09:00-09:00", "09:00-08:00", "24:00-24:00", "09:00", "-09:00", "ab-cd", "09:00-ab"} {
		if _, err := schedule.ParseWindow(text); err == nil {
			t.Errorf("ParseWindow(%q) was accepted", text)
		}
	}
	// A window starting at the very end of the day has nowhere to run to.
	if _, err := schedule.NewWindow(schedule.EndOfDay, schedule.EndOfDay); !errors.Is(err, schedule.ErrInvalidWindow) {
		t.Errorf("error = %v, want ErrInvalidWindow", err)
	}
	if _, err := schedule.NewWindow(-1, 100); !errors.Is(err, schedule.ErrInvalidWindow) {
		t.Errorf("a negative start: error = %v", err)
	}
}

// TestAWindowIsHalfOpen so two adjacent windows never both claim one minute.
func TestAWindowIsHalfOpen(t *testing.T) {
	w := window(t, "09:00-22:00")
	nine, _ := schedule.NewTimeOfDay(9, 0)
	ten, _ := schedule.NewTimeOfDay(22, 0)
	before, _ := schedule.NewTimeOfDay(8, 59)

	if !w.Contains(nine) {
		t.Error("the opening minute is not inside the window")
	}
	if w.Contains(ten) {
		t.Error("the closing minute is inside the window")
	}
	if w.Contains(before) {
		t.Error("a minute before opening is inside the window")
	}
	if w.String() != "09:00-22:00" {
		t.Errorf("String() = %q", w)
	}
}

// -------------------------------------------------------- weekly shape

func TestWindowsInADayAreSortedWhateverOrderTheyArrive(t *testing.T) {
	week, err := schedule.New(map[time.Weekday][]schedule.Window{
		time.Monday: {window(t, "18:00-23:00"), window(t, "07:00-11:00")},
	}, 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := week.Windows(time.Monday); len(got) != 2 || got[0].String() != "07:00-11:00" {
		t.Errorf("windows = %v, want the earlier one first", got)
	}
}

func TestOverlappingWindowsAreRefusedAndAdjacentOnesAreNot(t *testing.T) {
	_, err := schedule.New(map[time.Weekday][]schedule.Window{
		time.Monday: {window(t, "09:00-14:00"), window(t, "13:00-22:00")},
	}, 0)
	if !errors.Is(err, schedule.ErrOverlappingWindows) {
		t.Errorf("overlapping: error = %v", err)
	}

	if _, err := schedule.New(map[time.Weekday][]schedule.Window{
		time.Monday: {window(t, "09:00-14:00"), window(t, "14:00-22:00")},
	}, 0); err != nil {
		t.Errorf("adjacent windows were refused: %v", err)
	}
}

// TestTheWindowCapIsTheCallersPolicy: a shop allows three shifts a day, an
// item's availability has no natural ceiling, so the limit is a parameter
// rather than a constant in the kernel.
func TestTheWindowCapIsTheCallersPolicy(t *testing.T) {
	four := []schedule.Window{
		window(t, "06:00-08:00"), window(t, "09:00-11:00"),
		window(t, "12:00-14:00"), window(t, "15:00-17:00"),
	}

	if _, err := schedule.New(map[time.Weekday][]schedule.Window{time.Monday: four}, 3); !errors.Is(err, schedule.ErrTooManyWindows) {
		t.Errorf("with a cap of 3: error = %v, want ErrTooManyWindows", err)
	}
	if _, err := schedule.New(map[time.Weekday][]schedule.Window{time.Monday: four}, 0); err != nil {
		t.Errorf("with no cap: %v", err)
	}
}

func TestADayOutsideTheWeekIsRefused(t *testing.T) {
	for _, day := range []time.Weekday{time.Weekday(9), time.Weekday(-1)} {
		_, err := schedule.New(map[time.Weekday][]schedule.Window{day: {window(t, "09:00-22:00")}}, 0)
		if !errors.Is(err, schedule.ErrInvalidWeekday) {
			t.Errorf("day %d: error = %v", int(day), err)
		}
	}
}

// TestAWindowBuiltByHandIsStillValidated: Window is a plain struct, so a caller
// can assemble an impossible one. New re-checks rather than trusting it.
func TestAWindowBuiltByHandIsStillValidated(t *testing.T) {
	_, err := schedule.New(map[time.Weekday][]schedule.Window{
		time.Monday: {{Open: 600, Close: 100}},
	}, 0)
	if !errors.Is(err, schedule.ErrInvalidWindow) {
		t.Errorf("error = %v, want ErrInvalidWindow", err)
	}
}

func TestWindowsCannotBeEditedByTheirCaller(t *testing.T) {
	week := schedule.EveryDay(window(t, "09:00-22:00"))
	got := week.Windows(time.Monday)
	got[0] = schedule.Window{}

	if again := week.Windows(time.Monday); again[0].String() != "09:00-22:00" {
		t.Error("editing the returned slice changed the timetable")
	}
	if week.Windows(time.Weekday(9)) != nil {
		t.Error("a day outside the week returned windows")
	}
}

func TestEveryDayAndEmptiness(t *testing.T) {
	week := schedule.EveryDay(window(t, "09:00-22:00"))
	if week.IsEmpty() {
		t.Error("EveryDay produced an empty timetable")
	}
	for day := time.Sunday; day <= time.Saturday; day++ {
		if len(week.Windows(day)) != 1 {
			t.Errorf("%s has %d windows", day, len(week.Windows(day)))
		}
	}

	empty, err := schedule.New(nil, 0)
	if err != nil {
		t.Fatalf("New(nil): %v", err)
	}
	if !empty.IsEmpty() {
		t.Error("an empty timetable reports itself non-empty")
	}
}

// --------------------------------------------------------- local time

// TestTheTimetableIsReadInBangladeshLocalTime: nine in the morning means nine
// in Dhaka, whatever timezone the instant arrives in.
func TestTheTimetableIsReadInBangladeshLocalTime(t *testing.T) {
	week := schedule.EveryDay(window(t, "09:00-22:00"))

	// 04:00 UTC on a Monday is 10:00 in Dhaka.
	utc := time.Date(2026, time.March, 2, 4, 0, 0, 0, time.UTC)
	if !week.Covers(utc) {
		t.Error("10:00 Dhaka, expressed as 04:00 UTC, reads as closed")
	}
	if !week.Covers(utc.In(time.FixedZone("+12", 12*60*60))) {
		t.Error("the same instant in another zone gave a different answer")
	}
}

// TestTheWeekdayIsTheLocalOne: an instant that is Sunday in UTC can be Monday
// in Dhaka, and Monday's windows are the ones that apply.
func TestTheWeekdayIsTheLocalOne(t *testing.T) {
	week, err := schedule.New(map[time.Weekday][]schedule.Window{
		time.Monday: {window(t, "00:00-06:00")},
	}, 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// 20:00 UTC Sunday 1 March 2026 is 02:00 Monday in Dhaka.
	if !week.Covers(time.Date(2026, time.March, 1, 20, 0, 0, 0, time.UTC)) {
		t.Error("02:00 Monday in Dhaka was matched against Sunday's windows")
	}

	day, clock := schedule.Local(time.Date(2026, time.March, 1, 20, 0, 0, 0, time.UTC))
	if day != time.Monday || clock.String() != "02:00" {
		t.Errorf("Local() = %s %s, want Monday 02:00", day, clock)
	}
}

func TestEndsAfterNamesTheEndOfTheCurrentWindow(t *testing.T) {
	week, err := schedule.New(map[time.Weekday][]schedule.Window{
		time.Monday: {window(t, "07:00-11:00"), window(t, "18:00-23:00")},
	}, 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ends, ok := week.EndsAfter(dhaka(2026, time.March, 2, 8, 0))
	if !ok || ends.String() != "11:00" {
		t.Errorf("EndsAfter(08:00) = %q, %v", ends, ok)
	}
	if _, ok := week.EndsAfter(dhaka(2026, time.March, 2, 12, 0)); ok {
		t.Error("EndsAfter reported an end while nothing was running")
	}
}

func TestNextStartLooksAtTheRestOfTodayFirst(t *testing.T) {
	week, err := schedule.New(map[time.Weekday][]schedule.Window{
		time.Monday:  {window(t, "07:00-11:00"), window(t, "18:00-23:00")},
		time.Tuesday: {window(t, "09:00-22:00")},
	}, 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	start, ok := week.NextStart(dhaka(2026, time.March, 2, 12, 0))
	if !ok || start.String() != "18:00" {
		t.Errorf("NextStart(Monday 12:00) = %q, %v", start, ok)
	}

	start, ok = week.NextStart(dhaka(2026, time.March, 2, 23, 30))
	if !ok || start.String() != "09:00" {
		t.Errorf("NextStart(Monday 23:30) = %q, %v", start, ok)
	}
}

// TestNextStartGivesUpAfterAWeek: a timetable that never opens has no answer,
// and looping to find one would hang the request rather than say so.
func TestNextStartGivesUpAfterAWeek(t *testing.T) {
	empty, err := schedule.New(nil, 0)
	if err != nil {
		t.Fatalf("New(nil): %v", err)
	}
	if _, ok := empty.NextStart(dhaka(2026, time.March, 2, 12, 0)); ok {
		t.Error("an empty timetable named a next start")
	}
}

func TestNextStartWrapsAroundTheWeek(t *testing.T) {
	week, err := schedule.New(map[time.Weekday][]schedule.Window{
		time.Sunday: {window(t, "10:00-20:00")},
	}, 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Saturday, with only Sunday open: the answer is tomorrow, after the wrap.
	start, ok := week.NextStart(dhaka(2026, time.March, 7, 12, 0))
	if !ok || start.String() != "10:00" {
		t.Errorf("NextStart(Saturday) = %q, %v", start, ok)
	}
}

// ----------------------------------------------------------- storage

func TestATimetableSurvivesStorageUnchanged(t *testing.T) {
	original, err := schedule.New(map[time.Weekday][]schedule.Window{
		time.Monday: {window(t, "07:00-11:00"), window(t, "18:00-23:00")},
		time.Friday: {window(t, "15:00-24:00")},
	}, 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	decoded, err := schedule.Decode(original.Encode(), 0)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for day := time.Sunday; day <= time.Saturday; day++ {
		before, after := original.Windows(day), decoded.Windows(day)
		if len(before) != len(after) {
			t.Fatalf("%s: %d windows before, %d after", day, len(before), len(after))
		}
		for i := range before {
			if before[i] != after[i] {
				t.Errorf("%s window %d: %v then %v", day, i, before[i], after[i])
			}
		}
	}
}

// TestEncodingOmitsEmptyDays keeps "off" the absence of a key rather than an
// empty list a decoder could read two ways.
func TestEncodingOmitsEmptyDays(t *testing.T) {
	week, err := schedule.New(map[time.Weekday][]schedule.Window{
		time.Monday: {window(t, "09:00-22:00")},
	}, 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	encoded := week.Encode()
	if len(encoded) != 1 {
		t.Fatalf("encoded = %v, want one day", encoded)
	}
	if _, ok := encoded["1"]; !ok {
		t.Errorf("encoded = %v, want Monday under key \"1\"", encoded)
	}
}

// TestDecodingRechecksEveryInvariant: a row written before a rule tightened
// must fail loudly rather than become an invalid timetable in memory.
func TestDecodingRechecksEveryInvariant(t *testing.T) {
	cases := map[string]struct {
		encoded  map[string][]string
		maxDaily int
		want     error
	}{
		"unreadable day":    {map[string][]string{"monday": {"09:00-22:00"}}, 0, schedule.ErrInvalidWeekday},
		"day out of range":  {map[string][]string{"9": {"09:00-22:00"}}, 0, schedule.ErrInvalidWeekday},
		"negative day":      {map[string][]string{"-1": {"09:00-22:00"}}, 0, schedule.ErrInvalidWeekday},
		"unreadable window": {map[string][]string{"1": {"nine to five"}}, 0, schedule.ErrInvalidWindow},
		"overlapping":       {map[string][]string{"1": {"09:00-14:00", "13:00-22:00"}}, 0, schedule.ErrOverlappingWindows},
		"past the cap":      {map[string][]string{"1": {"06:00-08:00", "09:00-11:00", "12:00-14:00"}}, 2, schedule.ErrTooManyWindows},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := schedule.Decode(tc.encoded, tc.maxDaily); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDecodingNothingGivesAnEmptyTimetable(t *testing.T) {
	week, err := schedule.Decode(nil, 0)
	if err != nil {
		t.Fatalf("Decode(nil): %v", err)
	}
	if !week.IsEmpty() {
		t.Error("decoding nothing produced a non-empty timetable")
	}
}

// TestCoversSaysNoWhenNothingMatches walks past a day's windows without
// finding one, which is the branch a test that only ever asks inside a window
// never reaches.
func TestCoversSaysNoWhenNothingMatches(t *testing.T) {
	week, err := schedule.New(map[time.Weekday][]schedule.Window{
		time.Monday: {window(t, "07:00-11:00"), window(t, "18:00-23:00")},
	}, 0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Monday lunchtime: after the first window, before the second.
	if week.Covers(dhaka(2026, time.March, 2, 14, 0)) {
		t.Error("the gap between two windows reads as covered")
	}
	// Tuesday, which has no windows at all.
	if week.Covers(dhaka(2026, time.March, 3, 9, 0)) {
		t.Error("a day with no windows reads as covered")
	}
}
