package discovery

import (
	"context"
	"testing"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/infrastructure/fees"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// dhakaPlacement is the placement every quote in these tests is asked about.
var dhakaPlacement = ports.Placement{AreaCode: "DHA-DHK-DHN", DistrictCode: "DHA-DHK", DivisionCode: "DHA"}

func quoterWith(settings cfgcontract.Settings) (*fees.Quoter, *fakeConfig) {
	c := &fakeConfig{settings: settings}
	return fees.NewQuoter(c), c
}

// ALG-05 on the Appendix B defaults: ৳40 base plus ৳10 a kilometre, banded up.
func TestQuoteDeliveryBandsByKilometre(t *testing.T) {
	q, _ := quoterWith(defaultSettings())
	cases := []struct {
		distanceM float64
		wantMinor int64
		wantText  string
	}{
		{0, 4000, "৳ 40"},
		{1, 5000, "৳ 50"}, // any part of a kilometre is a kilometre
		{1000, 5000, "৳ 50"},
		{1001, 6000, "৳ 60"},
		{4800, 9000, "৳ 90"},
	}
	for _, tc := range cases {
		got, err := q.QuoteDelivery(context.Background(), dhakaPlacement, ports.DeliveryQuoteRequest{DistanceM: tc.distanceM})
		if err != nil {
			t.Fatalf("QuoteDelivery(%v): %v", tc.distanceM, err)
		}
		if got.Minor != tc.wantMinor || got.Display != tc.wantText {
			t.Errorf("%v m = %d (%q), want %d (%q)", tc.distanceM, got.Minor, got.Display, tc.wantMinor, tc.wantText)
		}
		if got.Currency != "BDT" || got.Expanded {
			t.Errorf("%v m: %+v", tc.distanceM, got)
		}
	}
}

// D2: the surcharge, and the flag that lets the app say why.
func TestQuoteDeliveryAppliesTheExpansionMultiplier(t *testing.T) {
	q, _ := quoterWith(defaultSettings())
	base, err := q.QuoteDelivery(context.Background(), dhakaPlacement, ports.DeliveryQuoteRequest{DistanceM: 4800})
	if err != nil {
		t.Fatalf("QuoteDelivery: %v", err)
	}
	expanded, err := q.QuoteDelivery(context.Background(), dhakaPlacement,
		ports.DeliveryQuoteRequest{DistanceM: 4800, ExpansionLevel: 1})
	if err != nil {
		t.Fatalf("QuoteDelivery: %v", err)
	}
	if expanded.Minor != base.Minor*3/2 {
		t.Fatalf("expanded = %d, want 1.5× %d", expanded.Minor, base.Minor)
	}
	if !expanded.Expanded {
		t.Error("the surcharge was applied without being flagged")
	}
}

// An odd amount times 1.5 lands on a half-paisa. Rounding to nearest rather
// than truncating keeps the platform from quietly taking the difference.
func TestQuoteDeliveryRoundsRatherThanTruncates(t *testing.T) {
	settings := defaultSettings()
	settings.ints[cfgcontract.PricingDeliveryBase] = 4001
	settings.ints[cfgcontract.PricingDeliveryPerKm] = 0
	q, _ := quoterWith(settings)

	got, err := q.QuoteDelivery(context.Background(), dhakaPlacement,
		ports.DeliveryQuoteRequest{DistanceM: 100, ExpansionLevel: 1})
	if err != nil {
		t.Fatalf("QuoteDelivery: %v", err)
	}
	if got.Minor != 6002 { // 4001 × 1.5 = 6001.5
		t.Errorf("minor = %d, want 6002", got.Minor)
	}
}

func TestQuoteDeliveryPassesThePlacementThrough(t *testing.T) {
	q, c := quoterWith(defaultSettings())
	if _, err := q.QuoteDelivery(context.Background(), dhakaPlacement, ports.DeliveryQuoteRequest{}); err != nil {
		t.Fatalf("QuoteDelivery: %v", err)
	}
	if len(c.seen) != 1 || c.seen[0].AreaCode != "DHA-DHK-DHN" || c.seen[0].DivisionCode != "DHA" {
		t.Errorf("config was asked about %+v", c.seen)
	}
}

func TestQuoteDeliveryFailures(t *testing.T) {
	t.Run("configuration is unreachable", func(t *testing.T) {
		c := &fakeConfig{err: errBoom}
		if _, err := fees.NewQuoter(c).QuoteDelivery(context.Background(), dhakaPlacement, ports.DeliveryQuoteRequest{}); err == nil {
			t.Fatal("a fee was quoted with no configuration")
		}
	})

	for _, key := range []string{
		cfgcontract.PricingDeliveryBase,
		cfgcontract.PricingDeliveryPerKm,
		cfgcontract.PricingExpansionMult,
	} {
		t.Run("missing "+key, func(t *testing.T) {
			settings := defaultSettings()
			settings.failOn = key
			q, _ := quoterWith(settings)
			if _, err := q.QuoteDelivery(context.Background(), dhakaPlacement, ports.DeliveryQuoteRequest{}); err == nil {
				t.Fatalf("a fee was quoted without %s", key)
			}
		})
	}

	t.Run("a negative distance", func(t *testing.T) {
		q, _ := quoterWith(defaultSettings())
		_, err := q.QuoteDelivery(context.Background(), dhakaPlacement, ports.DeliveryQuoteRequest{DistanceM: -1})
		if errs.CodeOf(err) != "invalid_distance" {
			t.Fatalf("err = %v, want invalid_distance", err)
		}
	})

	// A configuration that produces a negative fee is refused rather than
	// charged: a delivery the platform pays the customer for is a bug, and
	// silently clamping it to zero would hide the misconfiguration.
	t.Run("configuration that prices below zero", func(t *testing.T) {
		settings := defaultSettings()
		settings.ints[cfgcontract.PricingDeliveryBase] = -10000
		q, _ := quoterWith(settings)
		_, err := q.QuoteDelivery(context.Background(), dhakaPlacement, ports.DeliveryQuoteRequest{DistanceM: 100})
		if errs.CodeOf(err) != "invalid_delivery_fee" {
			t.Fatalf("err = %v, want invalid_delivery_fee", err)
		}
	})
}
