package config

import (
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
)

// The registry is the system's rulebook. These tests check it against
// Appendix B directly, because a wrong default here is a wrong fee charged to
// every customer in the country until somebody notices.

// appendixB is the table from docs/build/appendix-b-config.md, transcribed
// independently of the registry so the two have to agree.
var appendixB = []struct {
	key         domain.Key
	kind        domain.Kind
	defaultText string // canonical stored form
	autoTuned   bool
	immutable   bool
}{
	{domain.DiscoveryBaseRadius, domain.KindDistance, "5000", true, false},
	{domain.DiscoveryExpansionStep, domain.KindDistance, "5000", true, false},
	{domain.DiscoveryMaxExpansions, domain.KindCount, "4", false, false},
	{domain.DiscoveryMinMerchants, domain.KindCount, "5", false, false},
	{domain.DiscoveryAutoExpand, domain.KindBool, "false", false, false},
	{domain.DiscoveryDivisionCeil, domain.KindBool, "true", false, true},
	{domain.PricingDeliveryBase, domain.KindMoney, "4000", true, false},
	{domain.PricingDeliveryPerKm, domain.KindMoney, "1000", true, false},
	{domain.PricingExpansionMult, domain.KindRatio, "1.5", true, false},
	{domain.PricingFreeDelivery, domain.KindMoney, "50000", true, false},
	{domain.DispatchShortDistance, domain.KindDistance, "5000", true, false},
	{domain.DispatchLongDistance, domain.KindDistance, "25000", true, false},
	{domain.DispatchPartnerRadius, domain.KindDistance, "7000", true, false},
	{domain.DispatchAssignTimeout, domain.KindDuration, "30", false, false},
	{domain.DispatchMaxConcurrent, domain.KindCount, "3", false, false},
	{domain.OrderCancellationWindow, domain.KindDuration, "120", false, false},
	{domain.OrderCODLimit, domain.KindMoney, "500000", true, false},
}

func TestRegistryMatchesAppendixB(t *testing.T) {
	for _, want := range appendixB {
		def, err := domain.Lookup(want.key)
		if err != nil {
			t.Errorf("%s is missing from the registry", want.key)
			continue
		}
		if def.Kind != want.kind {
			t.Errorf("%s kind = %s, want %s", want.key, def.Kind, want.kind)
		}
		if got := def.Default.String(); got != want.defaultText {
			t.Errorf("%s default = %s, want %s", want.key, got, want.defaultText)
		}
		if def.AutoTunable != want.autoTuned {
			t.Errorf("%s auto-tunable = %v, want %v", want.key, def.AutoTunable, want.autoTuned)
		}
		if def.Immutable != want.immutable {
			t.Errorf("%s immutable = %v, want %v", want.key, def.Immutable, want.immutable)
		}
	}
}

// TestRegistryHasNothingAppendixBDoesNot catches the other direction: a key
// added to the code but not to the specification.
func TestRegistryHasNothingAppendixBDoesNot(t *testing.T) {
	documented := make(map[domain.Key]bool, len(appendixB))
	for _, d := range appendixB {
		documented[d.key] = true
	}
	for _, key := range domain.AllKeys() {
		if !documented[key] {
			t.Errorf("%s is in the registry but not in Appendix B", key)
		}
	}
	if len(domain.AllKeys()) != len(appendixB) {
		t.Errorf("registry has %d keys, Appendix B has %d", len(domain.AllKeys()), len(appendixB))
	}
}

// TestExactlyOneVariableIsImmutable: the division ceiling is the one hard
// invariant of the system. A second immutable variable is either a mistake or a
// decision that deserves its own discussion, so it fails here either way.
func TestExactlyOneVariableIsImmutable(t *testing.T) {
	var immutable []domain.Key
	for _, def := range domain.AllDefinitions() {
		if def.Immutable {
			immutable = append(immutable, def.Key)
		}
	}
	if len(immutable) != 1 || immutable[0] != domain.DiscoveryDivisionCeil {
		t.Errorf("immutable keys = %v, want exactly [%s]", immutable, domain.DiscoveryDivisionCeil)
	}
}

// TestEveryTunableVariableHasBounds: an auto-tuner with no upper bound on a
// delivery fee is one bad input away from quoting a fee nobody would pay.
func TestEveryTunableVariableHasBounds(t *testing.T) {
	for _, def := range domain.AllDefinitions() {
		if !def.AutoTunable || def.Kind == domain.KindBool {
			continue
		}
		order, err := def.Min.Compare(def.Max)
		if err != nil {
			t.Errorf("%s bounds are not comparable: %v", def.Key, err)
			continue
		}
		if order >= 0 {
			t.Errorf("%s has min %s >= max %s, so no value is valid", def.Key, def.Min, def.Max)
		}
		if err := def.InBounds(def.Default); err != nil {
			t.Errorf("%s default %s is outside its own bounds: %v", def.Key, def.Default, err)
		}
	}
}

// TestEveryDefinitionHasAPurpose: the admin UI lists these, and an unexplained
// switch is one nobody dares touch.
func TestEveryDefinitionHasAPurpose(t *testing.T) {
	for _, def := range domain.AllDefinitions() {
		if def.Purpose == "" {
			t.Errorf("%s has no stated purpose", def.Key)
		}
	}
}

// TestExpansionMultiplierCannotInvertPricing: a multiplier below 1 would make a
// longer delivery cheaper than a shorter one, which inverts D2.
func TestExpansionMultiplierCannotInvertPricing(t *testing.T) {
	def, err := domain.Lookup(domain.PricingExpansionMult)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	minimum, err := def.Min.Ratio()
	if err != nil {
		t.Fatalf("min ratio: %v", err)
	}
	if minimum < 1 {
		t.Errorf("minimum expansion multiplier = %v; below 1 a longer trip costs less", minimum)
	}
}

func TestLookupRejectsAnUnknownKey(t *testing.T) {
	if _, err := domain.Lookup("discovery.base_radius_typo"); !errors.Is(err, domain.ErrUnknownKey) {
		t.Errorf("error = %v, want ErrUnknownKey", err)
	}
}

func TestAllKeysIsSortedAndStable(t *testing.T) {
	first := domain.AllKeys()
	second := domain.AllKeys()
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("AllKeys is not stable: %v then %v", first, second)
		}
		if i > 0 && first[i-1] > first[i] {
			t.Errorf("AllKeys is not sorted: %s before %s", first[i-1], first[i])
		}
	}
}

func TestAllDefinitionsMatchesAllKeys(t *testing.T) {
	keys := domain.AllKeys()
	defs := domain.AllDefinitions()
	if len(keys) != len(defs) {
		t.Fatalf("%d keys but %d definitions", len(keys), len(defs))
	}
	for i := range keys {
		if defs[i].Key != keys[i] {
			t.Errorf("definition %d is %s, want %s", i, defs[i].Key, keys[i])
		}
	}
}

func TestInBoundsAcceptsTheEdges(t *testing.T) {
	def, _ := domain.Lookup(domain.PricingDeliveryBase)
	for _, v := range []domain.Value{def.Min, def.Max, def.Default} {
		if err := def.InBounds(v); err != nil {
			t.Errorf("InBounds(%s) = %v, want the edge accepted", v, err)
		}
	}
}

func TestInBoundsRejectsOutsideTheRange(t *testing.T) {
	def, _ := domain.Lookup(domain.PricingDeliveryBase)
	tooLow, _ := domain.Money(1)
	tooHigh, _ := domain.Money(10_000_000)
	for _, v := range []domain.Value{tooLow, tooHigh} {
		if err := def.InBounds(v); !errors.Is(err, domain.ErrOutOfBounds) {
			t.Errorf("InBounds(%s) = %v, want ErrOutOfBounds", v, err)
		}
	}
}

func TestInBoundsRejectsTheWrongKind(t *testing.T) {
	def, _ := domain.Lookup(domain.PricingDeliveryBase)
	wrong, _ := domain.Distance(5000)
	if err := def.InBounds(wrong); !errors.Is(err, domain.ErrKindMismatch) {
		t.Errorf("error = %v, want ErrKindMismatch", err)
	}
}

// A bool has no order, so its bounds are not checked — the kind check has
// already established it is one of two allowed values.
func TestInBoundsAcceptsEitherBool(t *testing.T) {
	def, _ := domain.Lookup(domain.DiscoveryAutoExpand)
	for _, b := range []bool{true, false} {
		if err := def.InBounds(domain.Bool(b)); err != nil {
			t.Errorf("InBounds(%v) = %v", b, err)
		}
	}
}

// TestNoDefinitionIsNegative closes the loop opened by building the table with
// the package-internal constructors, which skip the non-negative checks the
// public ones apply. A negative delivery fee would credit every customer.
func TestNoDefinitionIsNegative(t *testing.T) {
	for _, def := range domain.AllDefinitions() {
		if def.Kind == domain.KindBool {
			continue
		}
		for label, v := range map[string]domain.Value{
			"default": def.Default, "minimum": def.Min, "maximum": def.Max,
		} {
			if def.Kind == domain.KindRatio {
				f, err := v.Ratio()
				if err != nil || f < 0 {
					t.Errorf("%s %s = %s, %v", def.Key, label, v, err)
				}
				continue
			}
			n, err := v.Int()
			if err != nil || n < 0 {
				t.Errorf("%s %s = %s, %v", def.Key, label, v, err)
			}
		}
	}
}

// TestEveryBoundSharesItsDefinitionsKind: InBounds compares Min and Max
// directly on the assumption that they are the same kind as the value. That
// assumption is the table's job to keep, so it is checked here.
func TestEveryBoundSharesItsDefinitionsKind(t *testing.T) {
	for _, def := range domain.AllDefinitions() {
		if def.Kind == domain.KindBool {
			continue
		}
		if def.Min.Kind() != def.Kind {
			t.Errorf("%s minimum is %s, want %s", def.Key, def.Min.Kind(), def.Kind)
		}
		if def.Max.Kind() != def.Kind {
			t.Errorf("%s maximum is %s, want %s", def.Key, def.Max.Kind(), def.Kind)
		}
	}
}

// TestNonTunableVariablesAlsoHaveUsableBounds: an admin is bounded too, so a
// variable the tuner cannot touch still needs a sane range.
func TestNonTunableVariablesAlsoHaveUsableBounds(t *testing.T) {
	for _, def := range domain.AllDefinitions() {
		if def.Kind == domain.KindBool {
			continue
		}
		if err := def.InBounds(def.Default); err != nil {
			t.Errorf("%s default is outside its own bounds: %v", def.Key, err)
		}
	}
}
