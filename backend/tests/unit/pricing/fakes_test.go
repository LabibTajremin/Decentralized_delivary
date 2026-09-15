// Package pricing holds the unit tests for the pricing module.
//
// ALG-05 has one implementation in the system and this is where it is pinned.
// The band edges especially: a fee that is wrong by one band on either side of
// a kilometre is a fee somebody will notice on a receipt and nobody will be
// able to explain.
package pricing

import (
	"context"
	"errors"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/contract"
)

var errBoom = errors.New("boom")

// dhaka is the placement every price in these tests is worked out at.
var dhaka = contract.Placement{AreaCode: "DHA-DHK-DHN", DistrictCode: "DHA-DHK", DivisionCode: "DHA"}

// fakeSettings is a resolved configuration snapshot.
type fakeSettings struct {
	ints   map[string]int64
	ratios map[string]float64
	// failOn names a key every accessor refuses, for the "configuration is
	// broken" paths.
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

func (s fakeSettings) Bool(string) (bool, error) { return false, errBoom }

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
	seen     []cfgcontract.Placement
}

func (f *fakeConfig) Settings(_ context.Context, p cfgcontract.Placement) (cfgcontract.Settings, error) {
	f.seen = append(f.seen, p)
	if f.err != nil {
		return nil, f.err
	}
	return f.settings, nil
}

// appendixB is the default tariff from Appendix B: ৳40 base, ৳10 a kilometre,
// a 1.5× expansion multiplier, free delivery above ৳500.
func appendixB() fakeSettings {
	return fakeSettings{
		ints: map[string]int64{
			cfgcontract.PricingDeliveryBase:  4000,
			cfgcontract.PricingDeliveryPerKm: 1000,
			cfgcontract.PricingFreeDelivery:  50000,
		},
		ratios: map[string]float64{
			cfgcontract.PricingExpansionMult: 1.5,
		},
	}
}
