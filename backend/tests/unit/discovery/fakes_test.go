// Package discovery holds the unit tests for the discovery module.
//
// Discovery owns no storage, so every fake here stands in for another module's
// contract rather than a database. That is the point of the module: what it
// does is decide, and what it decides can be tested without a Postgres.
package discovery

import (
	"context"
	"errors"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/geo"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/merchant"
	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
)

// errBoom is the generic downstream failure. Every fake can be told to return
// it, because the question these tests keep asking is "what does the customer
// see when the thing underneath breaks".
var errBoom = errors.New("boom")

// fakeGeo stands in for the geo module.
type fakeGeo struct {
	area     geo.Area
	areaErr  error
	nearby   []geo.NearbyMerchant
	nearErr  error
	counts   []int
	countAt  int
	countErr error
	distance float64
	distErr  error

	// radii records every radius the use case searched at, which is how the
	// expansion tests check that ALG-02 walked the ladder rather than jumping.
	radii   []float64
	counted []float64
}

func (f *fakeGeo) ResolveDivision(_ context.Context, _ geo.Point) (geo.Area, error) {
	return f.area, f.areaErr
}

func (f *fakeGeo) MerchantsWithinRadius(_ context.Context, _ geo.Point, radiusM float64, _ int) ([]geo.NearbyMerchant, error) {
	f.radii = append(f.radii, radiusM)
	return f.nearby, f.nearErr
}

// CountMerchantsWithinRadius replays a scripted sequence, one entry per step of
// the expansion ladder. A sequence rather than a single number because the
// whole behaviour under test is what happens as the radius grows.
func (f *fakeGeo) CountMerchantsWithinRadius(_ context.Context, _ geo.Point, radiusM float64) (int, error) {
	f.counted = append(f.counted, radiusM)
	if f.countErr != nil {
		return 0, f.countErr
	}
	if f.countAt < len(f.counts) {
		n := f.counts[f.countAt]
		f.countAt++
		return n, nil
	}
	return 0, nil
}

func (f *fakeGeo) DistanceBetween(_ context.Context, _, _ geo.Point) (float64, error) {
	return f.distance, f.distErr
}

// fakeMerchant stands in for the merchant module.
type fakeMerchant struct {
	listed    []merchant.Merchant
	listedErr error
	one       merchant.Merchant
	oneErr    error

	// asked records the ids handed to Listed, so a test can prove discovery
	// passed the radius result through rather than querying on its own.
	asked []string
}

func (f *fakeMerchant) Listed(_ context.Context, ids []string) ([]merchant.Merchant, error) {
	f.asked = append(f.asked, ids...)
	return f.listed, f.listedErr
}

func (f *fakeMerchant) Merchant(_ context.Context, _ string) (merchant.Merchant, error) {
	return f.one, f.oneErr
}

// fakeSettings is a resolved configuration snapshot.
type fakeSettings struct {
	ints   map[string]int64
	bools  map[string]bool
	ratios map[string]float64
	// failOn names a key every accessor refuses, for the "configuration is
	// broken" paths. One field rather than three so a test names the key it
	// cares about and does not have to know which accessor reads it.
	failOn string
}

func (s fakeSettings) Int(key string) (int64, error) {
	if key == s.failOn {
		return 0, errBoom
	}
	v, ok := s.ints[key]
	if !ok {
		return 0, errBoom
	}
	return v, nil
}

func (s fakeSettings) Bool(key string) (bool, error) {
	if key == s.failOn {
		return false, errBoom
	}
	v, ok := s.bools[key]
	if !ok {
		return false, errBoom
	}
	return v, nil
}

func (s fakeSettings) Ratio(key string) (float64, error) {
	if key == s.failOn {
		return 0, errBoom
	}
	v, ok := s.ratios[key]
	if !ok {
		return 0, errBoom
	}
	return v, nil
}

// fakeConfig stands in for the config module.
type fakeConfig struct {
	settings cfgcontract.Settings
	err      error
	// seen records the placement config was asked about, which is how the
	// tests check that area → district → division actually reached config.
	seen []cfgcontract.Placement
}

func (f *fakeConfig) Settings(_ context.Context, p cfgcontract.Placement) (cfgcontract.Settings, error) {
	f.seen = append(f.seen, p)
	if f.err != nil {
		return nil, f.err
	}
	return f.settings, nil
}

// fakeQuoter stands in for the delivery fee.
type fakeQuoter struct {
	err error
	// requests records what it was asked, so a test can assert that an expanded
	// search quoted at the expanded level (D2).
	requests []ports.DeliveryQuoteRequest
}

func (f *fakeQuoter) QuoteDelivery(_ context.Context, _ ports.Placement, req ports.DeliveryQuoteRequest) (ports.DeliveryQuote, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return ports.DeliveryQuote{}, f.err
	}
	minor := int64(4000) + int64(req.DistanceM/1000)*1000
	if req.ExpansionLevel > 0 {
		minor = minor * 3 / 2
	}
	return ports.DeliveryQuote{
		Minor: minor, Currency: "BDT", Display: "fee", Expanded: req.ExpansionLevel > 0,
	}, nil
}

// defaultSettings is Appendix B's table: 5 km base, 5 km steps, four of them,
// five merchants as the threshold, no auto-expand, ceiling on.
func defaultSettings() fakeSettings {
	return fakeSettings{
		ints: map[string]int64{
			cfgcontract.DiscoveryBaseRadius:    5000,
			cfgcontract.DiscoveryExpansionStep: 5000,
			cfgcontract.DiscoveryMaxExpansions: 4,
			cfgcontract.DiscoveryMinMerchants:  5,
			cfgcontract.PricingDeliveryBase:    4000,
			cfgcontract.PricingDeliveryPerKm:   1000,
		},
		bools: map[string]bool{
			cfgcontract.DiscoveryAutoExpand:   false,
			cfgcontract.DiscoveryDivisionCeil: true,
		},
		ratios: map[string]float64{
			cfgcontract.PricingExpansionMult: 1.5,
		},
	}
}

// shop builds a listed merchant at a distance.
func shop(id, name string, kind merchantcontract.Type, open bool) merchant.Merchant {
	return merchant.Merchant{
		ID: id, Name: name, Type: kind,
		IsListed: true, IsOpenNow: open,
		OpenStatus: "status", AreaName: "Dhanmondi", DivisionCode: "DHA",
		Lat: 23.75, Lng: 90.38,
	}
}
