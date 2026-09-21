package config

import (
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
)

func baseRadiusDef(t *testing.T) domain.Definition {
	t.Helper()
	def, err := domain.Lookup(domain.DiscoveryBaseRadius)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	return def
}

// ALG-09 widens a radius when the area is thin on merchants relative to its
// own discovery threshold.
func TestTuneWidensWhenMerchantsAreThin(t *testing.T) {
	def := baseRadiusDef(t)
	current := mustDistance(t, 5000)

	next, reason, changed := domain.Tune(def, current, 10, domain.TuningSignal{MerchantsNearby: 3})
	if !changed {
		t.Fatal("expected a change")
	}
	got, _ := next.Int()
	if got <= 5000 {
		t.Errorf("radius = %d, want it widened past 5000", got)
	}
	if reason == "" {
		t.Error("no reason given for the change")
	}
}

// ALG-09 widens when deliveries are failing more than it accepts, even with
// plenty of merchants nearby.
func TestTuneWidensWhenDeliveriesAreFailing(t *testing.T) {
	def := baseRadiusDef(t)
	current := mustDistance(t, 5000)

	next, reason, changed := domain.Tune(def, current, 5, domain.TuningSignal{
		MerchantsNearby: 20, OrderFailureRate: 0.30,
	})
	if !changed {
		t.Fatal("expected a change")
	}
	got, _ := next.Int()
	if got <= 5000 {
		t.Errorf("radius = %d, want it widened", got)
	}
	if reason == "" {
		t.Error("no reason given")
	}
}

// Both pressures at once still produce exactly one widen, not a double
// step, and the reason names both.
func TestTuneWidensOnceWhenBothPressuresArePresent(t *testing.T) {
	def := baseRadiusDef(t)
	current := mustDistance(t, 5000)

	next, reason, changed := domain.Tune(def, current, 10, domain.TuningSignal{
		MerchantsNearby: 3, OrderFailureRate: 0.30,
	})
	if !changed {
		t.Fatal("expected a change")
	}
	got, _ := next.Int()
	if got != 5500 {
		t.Errorf("radius = %d, want exactly one 10%% step to 5500", got)
	}
	if reason == "" {
		t.Error("no reason given")
	}
}

// ALG-09 narrows only when density is comfortably above the threshold and
// delivery success is fine.
func TestTuneNarrowsWhenOversupplied(t *testing.T) {
	def := baseRadiusDef(t)
	current := mustDistance(t, 10000)

	next, reason, changed := domain.Tune(def, current, 5, domain.TuningSignal{
		MerchantsNearby: 20, OrderFailureRate: 0.02,
	})
	if !changed {
		t.Fatal("expected a change")
	}
	got, _ := next.Int()
	if got >= 10000 {
		t.Errorf("radius = %d, want it narrowed below 10000", got)
	}
	if reason == "" {
		t.Error("no reason given")
	}
}

// Comfortably served and not oversupplied: nothing moves. A stable area
// must not keep shrinking pass after pass.
func TestTuneLeavesAComfortableAreaAlone(t *testing.T) {
	def := baseRadiusDef(t)
	current := mustDistance(t, 5000)

	next, reason, changed := domain.Tune(def, current, 5, domain.TuningSignal{
		MerchantsNearby: 6, OrderFailureRate: 0.05,
	})
	if changed {
		t.Errorf("changed = true, next = %v, reason = %q, want no change", next, reason)
	}
	if !next.Equal(current) {
		t.Errorf("next = %v, want it to echo current", next)
	}
}

// A widen already at the definition's maximum has nowhere to go — reported
// as no change, not an error.
func TestTuneClampsAtTheMaximum(t *testing.T) {
	def := baseRadiusDef(t)
	maxMetres, err := def.Max.Int()
	if err != nil {
		t.Fatalf("Max.Int: %v", err)
	}
	current := mustDistance(t, maxMetres)

	_, _, changed := domain.Tune(def, current, 100, domain.TuningSignal{MerchantsNearby: 1})
	if changed {
		t.Error("a value already at the maximum was reported as changed")
	}
}

// A narrow already at the definition's minimum has nowhere to go either.
func TestTuneClampsAtTheMinimum(t *testing.T) {
	def := baseRadiusDef(t)
	minMetres, err := def.Min.Int()
	if err != nil {
		t.Fatalf("Min.Int: %v", err)
	}
	current := mustDistance(t, minMetres)

	_, _, changed := domain.Tune(def, current, 1, domain.TuningSignal{MerchantsNearby: 1000})
	if changed {
		t.Error("a value already at the minimum was reported as changed")
	}
}

// A radius small enough that 10% rounds to nothing still has to move by
// something, or it would never budge.
func TestTuneMovesByAtLeastOneMetreOnATinyRadius(t *testing.T) {
	def := domain.Definition{Kind: domain.KindDistance, Min: mustDistance(t, 0), Max: mustDistance(t, 100)}
	current := mustDistance(t, 5)

	next, _, changed := domain.Tune(def, current, 10, domain.TuningSignal{MerchantsNearby: 0})
	if !changed {
		t.Fatal("expected a change")
	}
	got, _ := next.Int()
	if got <= 5 {
		t.Errorf("radius = %d, want it to have moved at all", got)
	}
}

// A minMerchants of zero (no threshold configured) never reads as "thin" or
// "oversupplied" — there is nothing to compare density against.
func TestTuneWithNoMerchantThresholdOnlyReactsToFailures(t *testing.T) {
	def := baseRadiusDef(t)
	current := mustDistance(t, 5000)

	_, _, changed := domain.Tune(def, current, 0, domain.TuningSignal{MerchantsNearby: 0, OrderFailureRate: 0.02})
	if changed {
		t.Error("a zero merchant threshold should not itself trigger a change")
	}

	next, _, changed := domain.Tune(def, current, 0, domain.TuningSignal{OrderFailureRate: 0.50})
	if !changed {
		t.Fatal("expected a change from the failure rate alone")
	}
	got, _ := next.Int()
	if got <= 5000 {
		t.Errorf("radius = %d, want it widened", got)
	}
}
