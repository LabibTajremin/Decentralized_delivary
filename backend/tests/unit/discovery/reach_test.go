package discovery

import (
	"context"
	"testing"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/geo"
	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// reachRig assembles the reachability use case and the service over it.
type reachRig struct {
	geo      *fakeGeo
	merchant *fakeMerchant
	config   *fakeConfig
	reach    *application.ReachUseCase
	service  *application.Service
}

func newReachRig(settings cfgcontract.Settings) *reachRig {
	g := &fakeGeo{area: dhaka()}
	m := &fakeMerchant{one: shop("m1", "Star Kabab", merchantcontract.TypeRestaurant, true)}
	c := &fakeConfig{settings: settings}
	uc := application.NewReachUseCase(g, m, c)
	return &reachRig{geo: g, merchant: m, config: c, reach: uc, service: application.NewService(uc)}
}

func defaultReachRig() *reachRig { return newReachRig(defaultSettings()) }

func TestReachInsideTheBaseRadius(t *testing.T) {
	r := defaultReachRig()
	r.geo.distance = 2100

	got, err := r.reach.Execute(context.Background(), dhanmondi, "m1", "en")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !got.Reachable || got.RequiredLevel != 0 || got.Expanded {
		t.Fatalf("a shop 2.1 km away needed an expansion: %+v", got)
	}
	if got.DistanceText != "2.1 km" || got.Reason != "" {
		t.Errorf("reach = %+v", got)
	}
}

func TestReachNeedsAnExpansion(t *testing.T) {
	cases := []struct {
		distance  float64
		wantLevel int
	}{
		{5000, 0},  // exactly the base radius is still base
		{5001, 1},  // a metre past it is one step
		{10000, 1}, // the whole of the first step
		{12500, 2},
		{25000, 4}, // the ceiling itself is reachable
	}
	for _, tc := range cases {
		r := defaultReachRig()
		r.geo.distance = tc.distance
		got, err := r.reach.Execute(context.Background(), dhanmondi, "m1", "en")
		if err != nil {
			t.Fatalf("Execute at %v m: %v", tc.distance, err)
		}
		if !got.Reachable {
			t.Fatalf("%v m was unreachable", tc.distance)
		}
		if got.RequiredLevel != tc.wantLevel {
			t.Errorf("%v m needs level %d, want %d", tc.distance, got.RequiredLevel, tc.wantLevel)
		}
		if got.Expanded != (tc.wantLevel > 0) {
			t.Errorf("%v m: Expanded = %v at level %d", tc.distance, got.Expanded, got.RequiredLevel)
		}
	}
}

// D3: another division is unreachable at every level, and that is a different
// answer from "too far". The distance is still reported — it is true — but no
// expansion is offered against it, because none would ever work.
func TestReachAcrossADivisionBoundaryIsNeverPossible(t *testing.T) {
	r := defaultReachRig()
	r.geo.distance = 900
	away := shop("m1", "Star Kabab", merchantcontract.TypeRestaurant, true)
	away.DivisionCode = "SYL"
	r.merchant.one = away

	got, err := r.reach.Execute(context.Background(), dhanmondi, "m1", "en")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Reachable || got.Reason != contract.ReasonOutsideDivision {
		t.Fatalf("a shop in another division was reachable: %+v", got)
	}
	// Config is never consulted: no radius could change the answer.
	if len(r.config.seen) != 0 {
		t.Errorf("the radius ladder was read for a cross-division shop: %+v", r.config.seen)
	}
}

func TestReachBeyondTheWidestRadius(t *testing.T) {
	r := defaultReachRig()
	r.geo.distance = 40000

	got, err := r.reach.Execute(context.Background(), dhanmondi, "m1", "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Reachable || got.Reason != contract.ReasonBeyondMaxRadius {
		t.Fatalf("a shop past the ceiling was reachable: %+v", got)
	}
	if got.DistanceText != "৪০.০ কিমি" {
		t.Errorf("distance text = %q, want Bengali by default", got.DistanceText)
	}
}

// The contract is what cart calls. It fixes the language rather than taking
// one, because the caller is a module and not a request handler.
func TestServiceImplementsTheContract(t *testing.T) {
	r := defaultReachRig()
	r.geo.distance = 1200

	var api contract.DiscoveryContract = r.service
	got, err := api.Reach(context.Background(), contract.Point{Lat: dhanmondi.Lat, Lng: dhanmondi.Lng}, "m1")
	if err != nil {
		t.Fatalf("Reach: %v", err)
	}
	if !got.Reachable || got.MerchantID != "m1" {
		t.Fatalf("reach = %+v", got)
	}
	if got.DistanceText != "১.২ কিমি" {
		t.Errorf("the contract did not compose Bengali: %q", got.DistanceText)
	}
}

func TestReachFailures(t *testing.T) {
	t.Run("no such shop", func(t *testing.T) {
		r := defaultReachRig()
		r.merchant.oneErr = errs.New(errs.KindNotFound, "merchant_not_found", "No such shop.")
		if _, err := r.reach.Execute(context.Background(), dhanmondi, "nope", ""); !errs.Is(err, errs.KindNotFound) {
			t.Fatalf("err = %v, want a not-found", err)
		}
	})

	t.Run("the point is outside every division", func(t *testing.T) {
		r := defaultReachRig()
		r.geo.areaErr = errBoom
		if _, err := r.reach.Execute(context.Background(), dhanmondi, "m1", ""); err == nil {
			t.Fatal("an unplaceable point was reported as reachable")
		}
	})

	t.Run("the distance cannot be measured", func(t *testing.T) {
		r := defaultReachRig()
		r.geo.distErr = errBoom
		if _, err := r.reach.Execute(context.Background(), dhanmondi, "m1", ""); err == nil {
			t.Fatal("an unmeasurable distance was reported as reachable")
		}
	})

	t.Run("configuration is unreachable", func(t *testing.T) {
		r := defaultReachRig()
		r.config.err = errBoom
		if _, err := r.reach.Execute(context.Background(), dhanmondi, "m1", ""); errs.CodeOf(err) != "discovery_config_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the ladder is impossible", func(t *testing.T) {
		settings := defaultSettings()
		settings.ints[cfgcontract.DiscoveryExpansionStep] = 0
		r := newReachRig(settings)
		if _, err := r.reach.Execute(context.Background(), dhanmondi, "m1", ""); errs.CodeOf(err) != "discovery_config_invalid" {
			t.Fatalf("err = %v", err)
		}
	})
}

// A merchant whose location geo has not indexed still has coordinates on its
// record, so the distance question is answerable; this pins that the use case
// reads them from the merchant rather than from the radius search it did not run.
func TestReachUsesTheMerchantsOwnCoordinates(t *testing.T) {
	r := defaultReachRig()
	r.geo.distance = 3000
	got, err := r.reach.Execute(context.Background(), geo.Point{Lat: 23.7, Lng: 90.4}, "m1", "en")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.DistanceM != 3000 {
		t.Errorf("distance = %v, want the measured 3000", got.DistanceM)
	}
}
