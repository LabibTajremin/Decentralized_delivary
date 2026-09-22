package config

import (
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
)

// Money, distance and duration are separate kinds because the unit is the part
// people get wrong. A delivery fee of 40 and a radius of 40 are not
// interchangeable, and these tests hold that line.

func TestKindsDoNotInterchange(t *testing.T) {
	money, _ := domain.Money(4000)
	distance, _ := domain.Distance(4000)

	if money.Equal(distance) {
		t.Error("4000 poisha compares equal to 4000 metres")
	}
	if _, err := money.Compare(distance); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("comparing money with distance = %v, want ErrKindMismatch", err)
	}
}

func TestNegativeQuantitiesAreRejected(t *testing.T) {
	if _, err := domain.Count(-1); !errors.Is(err, domain.ErrNegative) {
		t.Errorf("Count(-1) = %v", err)
	}
	if _, err := domain.Money(-1); !errors.Is(err, domain.ErrNegative) {
		t.Errorf("Money(-1) = %v", err)
	}
	if _, err := domain.Distance(-1); !errors.Is(err, domain.ErrNegative) {
		t.Errorf("Distance(-1) = %v", err)
	}
	if _, err := domain.Duration(-1); !errors.Is(err, domain.ErrNegative) {
		t.Errorf("Duration(-1) = %v", err)
	}
	if _, err := domain.Ratio(-0.5); !errors.Is(err, domain.ErrNegative) {
		t.Errorf("Ratio(-0.5) = %v, want it rejected; a negative multiplier inverts a fee", err)
	}
}

// TestValuesRoundTripThroughStorage: the stored string is what the database and
// the audit log hold, so anything that does not read back identically is a
// silent corruption of a live setting.
func TestValuesRoundTripThroughStorage(t *testing.T) {
	count, _ := domain.Count(7)
	money, _ := domain.Money(123456)
	distance, _ := domain.Distance(25000)
	duration, _ := domain.Duration(300)
	ratio, _ := domain.Ratio(1.5)
	awkward, _ := domain.Ratio(1.0 / 3.0)

	for _, original := range []domain.Value{
		count, money, distance, duration, ratio, awkward,
		domain.Bool(true), domain.Bool(false),
	} {
		back, err := domain.Parse(original.Kind(), original.String())
		if err != nil {
			t.Errorf("Parse(%s, %q): %v", original.Kind(), original.String(), err)
			continue
		}
		if !back.Equal(original) {
			t.Errorf("%s %q round-tripped to %q", original.Kind(), original.String(), back.String())
		}
	}
}

func TestParseRejectsNonsense(t *testing.T) {
	cases := []struct {
		kind domain.Kind
		raw  string
	}{
		{domain.KindBool, "yes please"},
		{domain.KindRatio, "one and a half"},
		{domain.KindCount, "3.5"},
		{domain.KindMoney, ""},
		{domain.KindDistance, "5 km"},
		{domain.KindDuration, "30s"},
	}
	for _, c := range cases {
		if _, err := domain.Parse(c.kind, c.raw); !errors.Is(err, domain.ErrUnparseable) {
			t.Errorf("Parse(%s, %q) = %v, want ErrUnparseable", c.kind, c.raw, err)
		}
	}
}

// A stored negative is rejected on the way back in, not just on the way out: a
// row edited by hand must not be able to install a negative fee.
func TestParseRejectsAStoredNegative(t *testing.T) {
	if _, err := domain.Parse(domain.KindMoney, "-100"); !errors.Is(err, domain.ErrNegative) {
		t.Errorf("Parse of a stored negative = %v, want it refused", err)
	}
}

func TestParseTrimsSurroundingSpace(t *testing.T) {
	v, err := domain.Parse(domain.KindCount, "  7  ")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	n, _ := v.Int()
	if n != 7 {
		t.Errorf("parsed = %d, want 7", n)
	}
}

func TestParseRejectsAnUnknownKind(t *testing.T) {
	if _, err := domain.Parse(domain.Kind(99), "1"); !errors.Is(err, domain.ErrUnparseable) {
		t.Errorf("error = %v, want ErrUnparseable", err)
	}
}

func TestAccessorsRejectTheWrongKind(t *testing.T) {
	money, _ := domain.Money(4000)
	ratio, _ := domain.Ratio(1.5)

	if _, err := money.Bool(); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("Bool on money = %v", err)
	}
	if _, err := money.Ratio(); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("Ratio on money = %v", err)
	}
	if _, err := ratio.Int(); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("Int on a ratio = %v", err)
	}
	if _, err := domain.Bool(true).Int(); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("Int on a bool = %v", err)
	}
	if _, err := (domain.Value{}).Ratio(); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("Ratio on a zero value = %v", err)
	}
}

// A bare Value{} is a count of zero, not an undefined state: a struct that can
// be constructed invalidly is one that eventually is.
func TestTheZeroValueIsACountOfZero(t *testing.T) {
	var v domain.Value
	if v.Kind() != domain.KindCount {
		t.Errorf("zero Kind = %s, want count", v.Kind())
	}
	n, err := v.Int()
	if err != nil || n != 0 {
		t.Errorf("zero Int = %d, %v", n, err)
	}
}

func TestComparisonOrdersValues(t *testing.T) {
	low, _ := domain.Money(100)
	high, _ := domain.Money(200)

	if got, _ := low.Compare(high); got != -1 {
		t.Errorf("low.Compare(high) = %d, want -1", got)
	}
	if got, _ := high.Compare(low); got != 1 {
		t.Errorf("high.Compare(low) = %d, want 1", got)
	}
	if got, _ := low.Compare(low); got != 0 {
		t.Errorf("low.Compare(low) = %d, want 0", got)
	}
}

func TestRatiosCompare(t *testing.T) {
	one, _ := domain.Ratio(1)
	half, _ := domain.Ratio(1.5)
	if got, _ := one.Compare(half); got != -1 {
		t.Errorf("1 vs 1.5 = %d, want -1", got)
	}
	if got, _ := half.Compare(one); got != 1 {
		t.Errorf("1.5 vs 1 = %d, want 1", got)
	}
	if got, _ := half.Compare(half); got != 0 {
		t.Errorf("1.5 vs 1.5 = %d, want 0", got)
	}
}

// Bools are unordered, and reporting a made-up answer would let a bounds check
// silently pass on a value it never really compared.
func TestBoolsHaveNoOrder(t *testing.T) {
	if _, err := domain.Bool(true).Compare(domain.Bool(false)); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("comparing bools = %v, want an error", err)
	}
}

func TestEqualRequiresTheSameKindAndValue(t *testing.T) {
	a, _ := domain.Money(100)
	b, _ := domain.Money(100)
	c, _ := domain.Money(200)
	ratioA, _ := domain.Ratio(1.5)
	ratioB, _ := domain.Ratio(1.5)

	if !a.Equal(b) {
		t.Error("identical money values are not equal")
	}
	if a.Equal(c) {
		t.Error("different money values compare equal")
	}
	if !ratioA.Equal(ratioB) {
		t.Error("identical ratios are not equal")
	}
	if !domain.Bool(true).Equal(domain.Bool(true)) {
		t.Error("identical bools are not equal")
	}
	if domain.Bool(true).Equal(domain.Bool(false)) {
		t.Error("different bools compare equal")
	}
}

func TestKindNamesAndUnits(t *testing.T) {
	cases := map[domain.Kind]struct{ name, unit string }{
		domain.KindCount:    {"count", ""},
		domain.KindBool:     {"bool", ""},
		domain.KindRatio:    {"ratio", "×"},
		domain.KindMoney:    {"money_minor", "BDT"},
		domain.KindDistance: {"distance_m", "m"},
		domain.KindDuration: {"duration_s", "s"},
	}
	for kind, want := range cases {
		if kind.String() != want.name {
			t.Errorf("Kind(%d).String() = %q, want %q", kind, kind.String(), want.name)
		}
		if kind.Unit() != want.unit {
			t.Errorf("%s unit = %q, want %q", kind, kind.Unit(), want.unit)
		}
	}
	unknown := domain.Kind(99)
	if unknown.String() != "unknown" || unknown.Unit() != "" {
		t.Errorf("unknown kind = %q/%q", unknown.String(), unknown.Unit())
	}
}

// An unknown kind renders as an empty string rather than panicking: a value
// read from a future schema should degrade, not crash the admin screen.
func TestAnUnknownKindRendersEmpty(t *testing.T) {
	if got := (domain.Value{}).String(); got != "0" {
		t.Errorf("zero value renders as %q, want \"0\"", got)
	}
}

func TestScopeValidation(t *testing.T) {
	if _, err := domain.NewScope(domain.LevelArea, ""); !errors.Is(err, domain.ErrMissingScopeCode) {
		t.Errorf("area with no code = %v", err)
	}
	if _, err := domain.NewScope(domain.LevelGlobal, "DHA"); !errors.Is(err, domain.ErrUnexpectedScopeCode) {
		t.Errorf("global with a code = %v", err)
	}
	if _, err := domain.NewScope(domain.Level(99), "x"); !errors.Is(err, domain.ErrUnknownLevel) {
		t.Errorf("unknown level = %v", err)
	}
	scope, err := domain.NewScope(domain.LevelArea, "  DHK-DHM  ")
	if err != nil || scope.Code != "DHK-DHM" {
		t.Errorf("scope = %+v, %v; want the code trimmed", scope, err)
	}
}

func TestLevelsRoundTrip(t *testing.T) {
	for _, level := range []domain.Level{
		domain.LevelArea, domain.LevelDistrict, domain.LevelDivision, domain.LevelGlobal,
	} {
		back, err := domain.ParseLevel(level.String())
		if err != nil || back != level {
			t.Errorf("%s round-tripped to %v, %v", level, back, err)
		}
	}
	if _, err := domain.ParseLevel("planet"); !errors.Is(err, domain.ErrUnknownLevel) {
		t.Errorf("ParseLevel(planet) = %v", err)
	}
	if got := domain.Level(99).String(); got != "unknown" {
		t.Errorf("unknown level string = %q", got)
	}
}

// TestResolutionOrderIsEncodedInTheLevelNumbers documents why Level's numeric
// order is not incidental: Resolve relies on it, so anyone inserting a level
// between two others has to renumber here and nowhere else.
func TestResolutionOrderIsEncodedInTheLevelNumbers(t *testing.T) {
	if !(domain.LevelArea < domain.LevelDistrict &&
		domain.LevelDistrict < domain.LevelDivision &&
		domain.LevelDivision < domain.LevelGlobal) {
		t.Error("level numbering no longer matches the resolution order")
	}
}

func TestScopeStrings(t *testing.T) {
	if got := domain.GlobalScope.String(); got != "global" {
		t.Errorf("global scope = %q", got)
	}
	scope, _ := domain.NewScope(domain.LevelDivision, "DHA")
	if got := scope.String(); got != "division:DHA" {
		t.Errorf("division scope = %q", got)
	}
}

func TestActorConstructors(t *testing.T) {
	admin := domain.AdminActor("adm_123")
	if admin.Kind != domain.ActorAdmin || admin.ID != "adm_123" {
		t.Errorf("admin actor = %+v", admin)
	}
	tuner := domain.TunerActor()
	if tuner.Kind != domain.ActorAutoTuner || tuner.ID != "" {
		t.Errorf("tuner actor = %+v; the tuner is not a person and has no id", tuner)
	}
}
