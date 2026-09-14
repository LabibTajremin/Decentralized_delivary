package merchant

import (
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
)

// window builds a window from two clock strings, failing the test if either is
// unreadable.
func window(t *testing.T, open, closes string) domain.Window {
	t.Helper()
	w, err := domain.ParseWindow(open + "-" + closes)
	if err != nil {
		t.Fatalf("ParseWindow(%s-%s): %v", open, closes, err)
	}
	return w
}

// ------------------------------------------------------------ time of day

func TestAClockTimeRoundTrips(t *testing.T) {
	cases := map[string]string{
		"00:00":  "00:00",
		"09:30":  "09:30",
		"23:59":  "23:59",
		"24:00":  "24:00",
		" 09:30": "09:30",
	}
	for in, want := range cases {
		parsed, err := domain.ParseTimeOfDay(in)
		if err != nil {
			t.Errorf("ParseTimeOfDay(%q): %v", in, err)
			continue
		}
		if parsed.String() != want {
			t.Errorf("ParseTimeOfDay(%q) = %q, want %q", in, parsed, want)
		}
	}
}

func TestUnreadableClockTimesAreRefused(t *testing.T) {
	for _, in := range []string{"", "9:30", "09:3", "0930", "09:30:00", "ab:cd", "09:cd", "25:00", "24:30", "09:60"} {
		if _, err := domain.ParseTimeOfDay(in); !errors.Is(err, domain.ErrInvalidTimeOfDay) {
			t.Errorf("ParseTimeOfDay(%q) error = %v, want ErrInvalidTimeOfDay", in, err)
		}
	}
}

func TestNewTimeOfDayRejectsImpossibleClocks(t *testing.T) {
	cases := [][2]int{{-1, 0}, {25, 0}, {9, -1}, {9, 60}, {24, 30}}
	for _, c := range cases {
		if _, err := domain.NewTimeOfDay(c[0], c[1]); !errors.Is(err, domain.ErrInvalidTimeOfDay) {
			t.Errorf("NewTimeOfDay(%d, %d) error = %v, want ErrInvalidTimeOfDay", c[0], c[1], err)
		}
	}
}

// ---------------------------------------------------------------- windows

// TestAWindowMayNotWrapPastMidnight: a wrapping window makes every "is it open"
// and "when does it close" answer ambiguous either side of midnight. A
// late-night shop enters two windows instead.
func TestAWindowMayNotWrapPastMidnight(t *testing.T) {
	late, err := domain.ParseTimeOfDay("22:00")
	if err != nil {
		t.Fatalf("ParseTimeOfDay: %v", err)
	}
	early, err := domain.ParseTimeOfDay("02:00")
	if err != nil {
		t.Fatalf("ParseTimeOfDay: %v", err)
	}
	if _, err := domain.NewWindow(late, early); !errors.Is(err, domain.ErrInvalidWindow) {
		t.Errorf("error = %v, want ErrInvalidWindow", err)
	}
}

func TestWindowsThatDoNotMakeSenseAreRefused(t *testing.T) {
	for _, in := range []string{"09:00-09:00", "09:00-08:00", "24:00-24:00", "09:00", "-09:00", "ab-cd", "09:00-ab"} {
		if _, err := domain.ParseWindow(in); err == nil {
			t.Errorf("ParseWindow(%q) was accepted", in)
		}
	}
}

// TestAWindowIsHalfOpen: a shop that closes at 22:00 is shut at 22:00, so two
// adjacent windows never both claim the same minute.
func TestAWindowIsHalfOpen(t *testing.T) {
	w := window(t, "09:00", "22:00")
	nine, _ := domain.NewTimeOfDay(9, 0)
	ten, _ := domain.NewTimeOfDay(22, 0)
	eight59, _ := domain.NewTimeOfDay(8, 59)

	if !w.Contains(nine) {
		t.Error("the opening minute is not inside the window")
	}
	if w.Contains(ten) {
		t.Error("the closing minute is inside the window")
	}
	if w.Contains(eight59) {
		t.Error("a minute before opening is inside the window")
	}
}

func TestAWindowRendersAsItWasWritten(t *testing.T) {
	if got := window(t, "09:00", "22:00").String(); got != "09:00-22:00" {
		t.Errorf("String() = %q", got)
	}
}

// ---------------------------------------------------------- weekly hours

func TestWindowsInADayAreSortedWhateverOrderTheyArrive(t *testing.T) {
	hours, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Monday: {window(t, "18:00", "23:00"), window(t, "07:00", "11:00")},
	})
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}
	windows := hours.Windows(time.Monday)
	if len(windows) != 2 || windows[0].String() != "07:00-11:00" {
		t.Errorf("windows = %v, want breakfast first", windows)
	}
}

func TestOverlappingWindowsAreRefused(t *testing.T) {
	_, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Monday: {window(t, "09:00", "14:00"), window(t, "13:00", "22:00")},
	})
	if !errors.Is(err, domain.ErrOverlappingWindows) {
		t.Errorf("error = %v, want ErrOverlappingWindows", err)
	}
}

// TestAdjacentWindowsAreNotOverlapping: 09:00-14:00 and 14:00-22:00 touch but
// do not collide, because a window is half-open.
func TestAdjacentWindowsAreNotOverlapping(t *testing.T) {
	if _, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Monday: {window(t, "09:00", "14:00"), window(t, "14:00", "22:00")},
	}); err != nil {
		t.Errorf("adjacent windows were refused: %v", err)
	}
}

func TestTooManyWindowsInOneDayAreRefused(t *testing.T) {
	_, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Monday: {
			window(t, "06:00", "08:00"), window(t, "09:00", "11:00"),
			window(t, "12:00", "14:00"), window(t, "15:00", "17:00"),
		},
	})
	if !errors.Is(err, domain.ErrTooManyWindows) {
		t.Errorf("error = %v, want ErrTooManyWindows", err)
	}
}

func TestADayOutsideTheWeekIsRefused(t *testing.T) {
	_, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Weekday(9): {window(t, "09:00", "22:00")},
	})
	if !errors.Is(err, domain.ErrInvalidWeekday) {
		t.Errorf("error = %v, want ErrInvalidWeekday", err)
	}
	_, err = domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Weekday(-1): {window(t, "09:00", "22:00")},
	})
	if !errors.Is(err, domain.ErrInvalidWeekday) {
		t.Errorf("error = %v, want ErrInvalidWeekday", err)
	}
}

// TestAWindowBuiltByHandIsStillValidated: Window is a plain struct, so a caller
// can assemble an impossible one. NewWeeklyHours re-checks rather than trusting
// it.
func TestAWindowBuiltByHandIsStillValidated(t *testing.T) {
	_, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Monday: {{Open: 600, Close: 100}},
	})
	if !errors.Is(err, domain.ErrInvalidWindow) {
		t.Errorf("error = %v, want ErrInvalidWindow", err)
	}
}

func TestWindowsForADayCannotBeEditedByItsCaller(t *testing.T) {
	hours := domain.DefaultHours()
	windows := hours.Windows(time.Monday)
	windows[0] = domain.Window{}

	if again := hours.Windows(time.Monday); again[0].String() == "00:00-00:00" {
		t.Error("editing the returned slice changed the schedule")
	}
}

func TestWindowsForADayOutsideTheWeekAreEmpty(t *testing.T) {
	if got := domain.DefaultHours().Windows(time.Weekday(9)); got != nil {
		t.Errorf("Windows(9) = %v, want nil", got)
	}
	if got := domain.DefaultHours().Windows(time.Weekday(-1)); got != nil {
		t.Errorf("Windows(-1) = %v, want nil", got)
	}
}

func TestDefaultHoursOpenEveryDay(t *testing.T) {
	hours := domain.DefaultHours()
	if hours.IsAlwaysClosed() {
		t.Fatal("the default schedule never opens")
	}
	for day := time.Sunday; day <= time.Saturday; day++ {
		if got := hours.Windows(day); len(got) != 1 || got[0].String() != "09:00-22:00" {
			t.Errorf("%s = %v, want one 09:00-22:00 window", day, got)
		}
	}
}

func TestAnEmptyScheduleIsAlwaysClosed(t *testing.T) {
	hours, err := domain.NewWeeklyHours(nil)
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}
	if !hours.IsAlwaysClosed() {
		t.Error("an empty schedule reports itself open at some point")
	}
}

// ---------------------------------------------------- opening in local time

// TestOpeningHoursAreBangladeshLocalTime: nine in the morning means nine in
// Dhaka, whatever timezone the instant arrives in.
func TestOpeningHoursAreBangladeshLocalTime(t *testing.T) {
	hours := domain.DefaultHours()

	// 04:00 UTC on a Monday is 10:00 in Dhaka, which is inside 09:00-22:00.
	utc := time.Date(2026, time.March, 2, 4, 0, 0, 0, time.UTC)
	if !hours.IsOpenAt(utc) {
		t.Error("10:00 Dhaka, expressed as 04:00 UTC, is reported closed")
	}

	// 04:00 UTC is 00:00 the next day in UTC+20 — but the shop's clock is the
	// only one that counts, and it still says 10:00.
	elsewhere := utc.In(time.FixedZone("+12", 12*60*60))
	if !hours.IsOpenAt(elsewhere) {
		t.Error("the same instant in another zone gave a different answer")
	}
}

// TestTheWeekdayIsTheLocalOne: an instant that is Sunday in UTC can be Monday
// in Dhaka, and the shop's Monday hours are the ones that apply.
func TestTheWeekdayIsTheLocalOne(t *testing.T) {
	hours, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Monday: {window(t, "00:00", "06:00")},
	})
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}

	// 20:00 UTC on Sunday 1 March 2026 is 02:00 Monday in Dhaka.
	instant := time.Date(2026, time.March, 1, 20, 0, 0, 0, time.UTC)
	if !hours.IsOpenAt(instant) {
		t.Error("02:00 Monday in Dhaka was matched against Sunday's hours")
	}
}

func TestClosingAfterNamesTheEndOfTheCurrentWindow(t *testing.T) {
	hours, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Monday: {window(t, "07:00", "11:00"), window(t, "18:00", "23:00")},
	})
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}

	closing, ok := hours.ClosingAfter(at(2026, time.March, 2, 8, 0))
	if !ok || closing.String() != "11:00" {
		t.Errorf("ClosingAfter(08:00) = %q, %v; want 11:00, true", closing, ok)
	}
	if _, ok := hours.ClosingAfter(at(2026, time.March, 2, 12, 0)); ok {
		t.Error("ClosingAfter reported a closing time while the shop was shut")
	}
}

func TestNextOpeningLooksAtTheRestOfTodayFirst(t *testing.T) {
	hours, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Monday:  {window(t, "07:00", "11:00"), window(t, "18:00", "23:00")},
		time.Tuesday: {window(t, "09:00", "22:00")},
	})
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}

	opening, ok := hours.NextOpening(at(2026, time.March, 2, 12, 0))
	if !ok || opening.String() != "18:00" {
		t.Errorf("NextOpening(Monday 12:00) = %q, %v; want 18:00, true", opening, ok)
	}

	// After the last window today, the answer is tomorrow's opening.
	opening, ok = hours.NextOpening(at(2026, time.March, 2, 23, 30))
	if !ok || opening.String() != "09:00" {
		t.Errorf("NextOpening(Monday 23:30) = %q, %v; want 09:00, true", opening, ok)
	}
}

// TestNextOpeningGivesUpAfterAWeek: a schedule that never opens has no answer,
// and looping to find one would hang the request instead of saying so.
func TestNextOpeningGivesUpAfterAWeek(t *testing.T) {
	hours, err := domain.NewWeeklyHours(nil)
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}
	if _, ok := hours.NextOpening(at(2026, time.March, 2, 12, 0)); ok {
		t.Error("a schedule that never opens named a next opening")
	}
}

// TestNextOpeningWrapsAroundTheWeek: on Saturday with only Sunday open, the
// answer is tomorrow, which is index 0 after the wrap.
func TestNextOpeningWrapsAroundTheWeek(t *testing.T) {
	hours, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Sunday: {window(t, "10:00", "20:00")},
	})
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}
	opening, ok := hours.NextOpening(at(2026, time.March, 7, 12, 0)) // a Saturday
	if !ok || opening.String() != "10:00" {
		t.Errorf("NextOpening(Saturday) = %q, %v; want 10:00, true", opening, ok)
	}
}

// -------------------------------------------------------------- storage form

func TestAScheduleSurvivesStorageUnchanged(t *testing.T) {
	original, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Monday: {window(t, "07:00", "11:00"), window(t, "18:00", "23:00")},
		time.Friday: {window(t, "15:00", "24:00")},
	})
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}

	decoded, err := domain.DecodeWeeklyHours(original.Encode())
	if err != nil {
		t.Fatalf("DecodeWeeklyHours: %v", err)
	}

	for day := time.Sunday; day <= time.Saturday; day++ {
		before, after := original.Windows(day), decoded.Windows(day)
		if len(before) != len(after) {
			t.Fatalf("%s: %d windows before, %d after", day, len(before), len(after))
		}
		for i := range before {
			if before[i] != after[i] {
				t.Errorf("%s window %d: %v before, %v after", day, i, before[i], after[i])
			}
		}
	}
}

// TestEncodingOmitsClosedDays keeps the stored row small and, more usefully,
// makes "closed" the absence of a key rather than an empty list that a decoder
// could read two ways.
func TestEncodingOmitsClosedDays(t *testing.T) {
	hours, err := domain.NewWeeklyHours(map[time.Weekday][]domain.Window{
		time.Monday: {window(t, "09:00", "22:00")},
	})
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}
	encoded := hours.Encode()
	if len(encoded) != 1 {
		t.Fatalf("encoded = %v, want one day", encoded)
	}
	if _, ok := encoded["1"]; !ok {
		t.Errorf("encoded = %v, want Monday under key \"1\"", encoded)
	}
}

// TestDecodingRecheckedEveryInvariant: a row written before a rule tightened
// must fail loudly rather than become an invalid schedule in memory.
func TestDecodingRechecksEveryInvariant(t *testing.T) {
	cases := map[string]struct {
		encoded map[string][]string
		want    error
	}{
		"unreadable day":    {map[string][]string{"monday": {"09:00-22:00"}}, domain.ErrInvalidWeekday},
		"day out of range":  {map[string][]string{"9": {"09:00-22:00"}}, domain.ErrInvalidWeekday},
		"negative day":      {map[string][]string{"-1": {"09:00-22:00"}}, domain.ErrInvalidWeekday},
		"unreadable window": {map[string][]string{"1": {"nine to five"}}, domain.ErrInvalidWindow},
		"overlapping":       {map[string][]string{"1": {"09:00-14:00", "13:00-22:00"}}, domain.ErrOverlappingWindows},
		"too many":          {map[string][]string{"1": {"06:00-08:00", "09:00-11:00", "12:00-14:00", "15:00-17:00"}}, domain.ErrTooManyWindows},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := domain.DecodeWeeklyHours(tc.encoded); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

// --------------------------------------------------------------- holidays

func TestAHolidayWithNoEndDateLastsUntilItIsEnded(t *testing.T) {
	now := at(2026, time.March, 2, 12, 0)
	holiday, err := domain.NewHoliday(time.Time{}, "Family emergency", now)
	if err != nil {
		t.Fatalf("NewHoliday: %v", err)
	}
	if !holiday.ActiveAt(now.AddDate(1, 0, 0)) {
		t.Error("an indefinite holiday expired on its own")
	}
}

// TestAHolidayExpiresWithoutAJob: a sweeper that fails would leave every shop
// that closed for Eid invisible, with nobody noticing until the merchants call.
func TestAHolidayExpiresWithoutAJob(t *testing.T) {
	now := at(2026, time.March, 2, 12, 0)
	until := at(2026, time.March, 10, 0, 0)

	holiday, err := domain.NewHoliday(until, "Eid", now)
	if err != nil {
		t.Fatalf("NewHoliday: %v", err)
	}
	if !holiday.ActiveAt(now) {
		t.Error("the holiday is not active on the day it starts")
	}
	if holiday.ActiveAt(until) {
		t.Error("the holiday is still active at its end instant")
	}
	if holiday.ActiveAt(until.Add(time.Hour)) {
		t.Error("the holiday outlived its end date")
	}
}

func TestAnInactiveHolidayIsNeverActive(t *testing.T) {
	if (domain.Holiday{}).ActiveAt(time.Now()) {
		t.Error("the zero holiday reports itself active")
	}
}

func TestHolidayDatesThatMakeNoSenseAreRefused(t *testing.T) {
	now := at(2026, time.March, 2, 12, 0)

	if _, err := domain.NewHoliday(now.Add(-time.Hour), "", now); !errors.Is(err, domain.ErrHolidayInThePast) {
		t.Errorf("past: error = %v, want ErrHolidayInThePast", err)
	}
	if _, err := domain.NewHoliday(now, "", now); !errors.Is(err, domain.ErrHolidayInThePast) {
		t.Errorf("now: error = %v, want ErrHolidayInThePast", err)
	}
	if _, err := domain.NewHoliday(now.AddDate(0, 0, 61), "", now); !errors.Is(err, domain.ErrHolidayTooLong) {
		t.Errorf("too long: error = %v, want ErrHolidayTooLong", err)
	}
}

func TestAHolidayReasonHasALimit(t *testing.T) {
	now := at(2026, time.March, 2, 12, 0)
	long := make([]rune, 201)
	for i := range long {
		long[i] = 'অ'
	}
	if _, err := domain.NewHoliday(time.Time{}, string(long), now); !errors.Is(err, domain.ErrTooLong) {
		t.Errorf("error = %v, want ErrTooLong", err)
	}
}

func TestAHolidayEndDateIsStoredInUTC(t *testing.T) {
	now := at(2026, time.March, 2, 12, 0)
	until := at(2026, time.March, 10, 9, 0)

	holiday, err := domain.NewHoliday(until, "Eid", now)
	if err != nil {
		t.Fatalf("NewHoliday: %v", err)
	}
	if holiday.Until.Location() != time.UTC {
		t.Errorf("until is in %v, want UTC", holiday.Until.Location())
	}
	if !holiday.Until.Equal(until) {
		t.Errorf("until = %v, want the same instant as %v", holiday.Until, until)
	}
}
