package config

import (
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
)

// dhanmondi is a fully-specified placement.
var dhanmondi = domain.Placement{AreaCode: "DHK-DHM", DistrictCode: "DHK", DivisionCode: "DHA"}

func mustDistance(t *testing.T, metres int64) domain.Value {
	t.Helper()
	v, err := domain.Distance(metres)
	if err != nil {
		t.Fatalf("Distance(%d): %v", metres, err)
	}
	return v
}

func mustMoney(t *testing.T, minor int64) domain.Value {
	t.Helper()
	v, err := domain.Money(minor)
	if err != nil {
		t.Fatalf("Money(%d): %v", minor, err)
	}
	return v
}

func override(t *testing.T, key domain.Key, level domain.Level, code string, v domain.Value) domain.Override {
	t.Helper()
	scope, err := domain.NewScope(level, code)
	if err != nil {
		t.Fatalf("NewScope: %v", err)
	}
	return domain.Override{Key: key, Scope: scope, Value: v}
}

// TestResolutionOrderIsAreaDistrictDivisionGlobal is the Appendix B rule. Every
// fee and every radius in the country depends on it being exactly this order.
func TestResolutionOrderIsAreaDistrictDivisionGlobal(t *testing.T) {
	all := []domain.Override{
		override(t, domain.DiscoveryBaseRadius, domain.LevelGlobal, "", mustDistance(t, 1000)),
		override(t, domain.DiscoveryBaseRadius, domain.LevelDivision, "DHA", mustDistance(t, 2000)),
		override(t, domain.DiscoveryBaseRadius, domain.LevelDistrict, "DHK", mustDistance(t, 3000)),
		override(t, domain.DiscoveryBaseRadius, domain.LevelArea, "DHK-DHM", mustDistance(t, 4000)),
	}

	// Peel the most specific scope off one at a time; each removal must fall
	// through to exactly the next level.
	wants := []struct {
		using  []domain.Override
		metres int64
		source string
	}{
		{all, 4000, "area:DHK-DHM"},
		{all[:3], 3000, "district:DHK"},
		{all[:2], 2000, "division:DHA"},
		{all[:1], 1000, "global"},
		{nil, 5000, "global"}, // the registry default
	}
	for _, want := range wants {
		resolved := domain.Resolve(dhanmondi, want.using)
		got, err := resolved.Int(domain.DiscoveryBaseRadius)
		if err != nil {
			t.Fatalf("Int: %v", err)
		}
		if got != want.metres {
			t.Errorf("with %d overrides: radius = %d, want %d", len(want.using), got, want.metres)
		}
		source, err := resolved.Source(domain.DiscoveryBaseRadius)
		if err != nil {
			t.Fatalf("Source: %v", err)
		}
		if source.String() != want.source {
			t.Errorf("with %d overrides: source = %s, want %s", len(want.using), source, want.source)
		}
	}
}

// TestOverrideOrderDoesNotMatter: the precedence is by scope, not by the order
// rows come back from the database.
func TestOverrideOrderDoesNotMatter(t *testing.T) {
	forwards := []domain.Override{
		override(t, domain.DiscoveryBaseRadius, domain.LevelGlobal, "", mustDistance(t, 1000)),
		override(t, domain.DiscoveryBaseRadius, domain.LevelArea, "DHK-DHM", mustDistance(t, 4000)),
	}
	backwards := []domain.Override{forwards[1], forwards[0]}

	for _, overrides := range [][]domain.Override{forwards, backwards} {
		got, err := domain.Resolve(dhanmondi, overrides).Int(domain.DiscoveryBaseRadius)
		if err != nil {
			t.Fatalf("Int: %v", err)
		}
		if got != 4000 {
			t.Errorf("radius = %d, want the area override to win regardless of row order", got)
		}
	}
}

// TestAnotherAreasOverrideIsIgnored: two areas must not read each other's
// settings.
func TestAnotherAreasOverrideIsIgnored(t *testing.T) {
	overrides := []domain.Override{
		override(t, domain.DiscoveryBaseRadius, domain.LevelArea, "DHK-GUL", mustDistance(t, 20000)),
	}
	got, err := domain.Resolve(dhanmondi, overrides).Int(domain.DiscoveryBaseRadius)
	if err != nil {
		t.Fatalf("Int: %v", err)
	}
	if got != 5000 {
		t.Errorf("radius = %d, want the default; Gulshan's override must not apply in Dhanmondi", got)
	}
}

// TestDivisionCeilingIgnoresEveryOverride is the hard invariant. Resolve
// enforces it as well as the write path, so a row inserted straight into the
// database — by a migration, a console, or a bug — still cannot turn it off.
func TestDivisionCeilingIgnoresEveryOverride(t *testing.T) {
	for _, level := range []domain.Level{domain.LevelArea, domain.LevelDistrict, domain.LevelDivision, domain.LevelGlobal} {
		code := ""
		switch level {
		case domain.LevelArea:
			code = "DHK-DHM"
		case domain.LevelDistrict:
			code = "DHK"
		case domain.LevelDivision:
			code = "DHA"
		case domain.LevelGlobal:
		}
		scope, err := domain.NewScope(level, code)
		if err != nil {
			t.Fatalf("NewScope: %v", err)
		}
		overrides := []domain.Override{{
			Key:   domain.DiscoveryDivisionCeil,
			Scope: scope,
			Value: domain.Bool(false),
		}}

		on, err := domain.Resolve(dhanmondi, overrides).Bool(domain.DiscoveryDivisionCeil)
		if err != nil {
			t.Fatalf("Bool: %v", err)
		}
		if !on {
			t.Errorf("a %s override switched off the division ceiling", level)
		}
	}
}

// A key removed from the registry leaves rows behind. One stale row must not
// stop the system loading its configuration.
func TestUnknownKeysInStoredOverridesAreIgnored(t *testing.T) {
	overrides := []domain.Override{
		{Key: "pricing.removed_in_2025", Scope: domain.GlobalScope, Value: mustMoney(t, 999)},
		override(t, domain.PricingDeliveryBase, domain.LevelArea, "DHK-DHM", mustMoney(t, 6000)),
	}
	got, err := domain.Resolve(dhanmondi, overrides).Int(domain.PricingDeliveryBase)
	if err != nil {
		t.Fatalf("Int: %v", err)
	}
	if got != 6000 {
		t.Errorf("delivery base = %d, want the valid override still applied", got)
	}
}

// A stored value whose type no longer matches the definition is skipped rather
// than coerced: silently reading a money amount as a distance is worse than
// falling back to the default.
func TestOverridesOfTheWrongTypeAreIgnored(t *testing.T) {
	overrides := []domain.Override{{
		Key:   domain.PricingDeliveryBase,
		Scope: domain.GlobalScope,
		Value: mustDistance(t, 12345),
	}}
	got, err := domain.Resolve(dhanmondi, overrides).Int(domain.PricingDeliveryBase)
	if err != nil {
		t.Fatalf("Int: %v", err)
	}
	if got != 4000 {
		t.Errorf("delivery base = %d, want the default when the stored type is wrong", got)
	}
}

// TestResolveAlwaysProducesEveryKey is what lets consumers read a value without
// checking whether it exists.
func TestResolveAlwaysProducesEveryKey(t *testing.T) {
	resolved := domain.Resolve(domain.Placement{}, nil)
	got := resolved.Keys()
	if len(got) != len(domain.AllKeys()) {
		t.Fatalf("snapshot has %d keys, registry has %d", len(got), len(domain.AllKeys()))
	}
	for _, key := range domain.AllKeys() {
		if _, err := resolved.Value(key); err != nil {
			t.Errorf("%s is missing from the snapshot: %v", key, err)
		}
	}
}

// A partial placement still resolves. A coordinate that placed only to a
// division should get division and global values, not an error.
func TestAPartialPlacementStillResolves(t *testing.T) {
	divisionOnly := domain.Placement{DivisionCode: "SYL"}
	overrides := []domain.Override{
		override(t, domain.DiscoveryBaseRadius, domain.LevelDivision, "SYL", mustDistance(t, 9000)),
	}
	got, err := domain.Resolve(divisionOnly, overrides).Int(domain.DiscoveryBaseRadius)
	if err != nil {
		t.Fatalf("Int: %v", err)
	}
	if got != 9000 {
		t.Errorf("radius = %d, want the division override", got)
	}
}

func TestEmptyPlacementFallsBackToGlobal(t *testing.T) {
	chain := domain.Placement{}.Chain()
	if len(chain) != 1 || chain[0] != domain.GlobalScope {
		t.Errorf("chain = %v, want just the global scope", chain)
	}
}

func TestChainIsMostSpecificFirst(t *testing.T) {
	chain := dhanmondi.Chain()
	want := []string{"area:DHK-DHM", "district:DHK", "division:DHA", "global"}
	if len(chain) != len(want) {
		t.Fatalf("chain = %v, want %v", chain, want)
	}
	for i := range want {
		if chain[i].String() != want[i] {
			t.Errorf("chain[%d] = %s, want %s", i, chain[i], want[i])
		}
	}
}

// Whitespace-only codes are treated as absent rather than producing a scope
// that can never match anything.
func TestBlankPlacementCodesAreTreatedAsAbsent(t *testing.T) {
	chain := domain.Placement{AreaCode: "  ", DistrictCode: "DHK"}.Chain()
	want := []string{"district:DHK", "global"}
	if len(chain) != len(want) {
		t.Fatalf("chain = %v, want %v", chain, want)
	}
}

func TestResolvedRejectsAnUnknownKey(t *testing.T) {
	resolved := domain.Resolve(dhanmondi, nil)
	if _, err := resolved.Value("nope"); !errors.Is(err, domain.ErrUnknownKey) {
		t.Errorf("Value: %v, want ErrUnknownKey", err)
	}
	if _, err := resolved.Source("nope"); !errors.Is(err, domain.ErrUnknownKey) {
		t.Errorf("Source: %v, want ErrUnknownKey", err)
	}
	if _, err := resolved.Int("nope"); !errors.Is(err, domain.ErrUnknownKey) {
		t.Errorf("Int: %v, want ErrUnknownKey", err)
	}
	if _, err := resolved.Bool("nope"); !errors.Is(err, domain.ErrUnknownKey) {
		t.Errorf("Bool: %v, want ErrUnknownKey", err)
	}
	if _, err := resolved.Ratio("nope"); !errors.Is(err, domain.ErrUnknownKey) {
		t.Errorf("Ratio: %v, want ErrUnknownKey", err)
	}
}

// Reading a value as the wrong type is an error, not a zero. A silent zero for
// a delivery fee read as a radius is a bug that ships.
func TestReadingAValueAsTheWrongTypeFails(t *testing.T) {
	resolved := domain.Resolve(dhanmondi, nil)
	if _, err := resolved.Int(domain.DiscoveryAutoExpand); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("Int on a bool = %v, want ErrKindMismatch", err)
	}
	if _, err := resolved.Bool(domain.PricingDeliveryBase); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("Bool on money = %v, want ErrKindMismatch", err)
	}
	if _, err := resolved.Ratio(domain.PricingDeliveryBase); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("Ratio on money = %v, want ErrKindMismatch", err)
	}
}

func TestResolvedReportsItsPlacement(t *testing.T) {
	if got := domain.Resolve(dhanmondi, nil).Placement(); got != dhanmondi {
		t.Errorf("Placement = %+v, want %+v", got, dhanmondi)
	}
}
