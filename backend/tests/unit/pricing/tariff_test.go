package pricing

import (
	"context"
	"testing"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	pricingapp "github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// tariffOf resolves a tariff from a settings snapshot.
func tariffOf(t *testing.T, settings cfgcontract.Settings) contract.Tariff {
	t.Helper()
	tariff, err := pricingapp.NewService(&fakeConfig{settings: settings}).Tariff(context.Background(), dhaka)
	if err != nil {
		t.Fatalf("Tariff: %v", err)
	}
	return tariff
}

func defaultTariff(t *testing.T) contract.Tariff {
	t.Helper()
	return tariffOf(t, appendixB())
}

// The phase's first acceptance criterion: ALG-05's banding is exact at the band
// edges. Any part of a kilometre is a kilometre, and 1000 m is one band while
// 1001 m is two.
func TestBandingIsExactAtTheEdges(t *testing.T) {
	tariff := defaultTariff(t)
	cases := []struct {
		distanceM float64
		wantMinor int64
	}{
		{0, 4000}, // ৳40 — the base fee covers the pickup
		{1, 5000}, // any part of a kilometre is a kilometre
		{999, 5000},
		{1000, 5000}, // exactly one band
		{1000.0001, 6000},
		{1001, 6000}, // one metre past it is two
		{1999, 6000},
		{2000, 6000},
		{2001, 7000},
		{25000, 29000}, // the widest the ladder reaches: ৳40 + 25 × ৳10
	}
	for _, tc := range cases {
		got, err := tariff.DeliveryFee(contract.QuoteRequest{DistanceM: tc.distanceM})
		if err != nil {
			t.Fatalf("DeliveryFee(%v): %v", tc.distanceM, err)
		}
		if got.Amount.Minor != tc.wantMinor {
			t.Errorf("%v m = %d poisha, want %d", tc.distanceM, got.Amount.Minor, tc.wantMinor)
		}
		if got.Amount.Currency != "BDT" || got.Amount.Display == "" {
			t.Errorf("%v m: money crossed with no rendered string: %+v", tc.distanceM, got.Amount)
		}
		if got.Expanded || got.Surcharge.Minor != 0 {
			t.Errorf("%v m: a base-radius fee carried a surcharge: %+v", tc.distanceM, got)
		}
	}
}

// The second acceptance criterion: D2's surcharge applies when the radius was
// extended, and the surcharge is reported as its own number.
func TestTheExpansionSurchargeApplies(t *testing.T) {
	tariff := defaultTariff(t)

	base, err := tariff.DeliveryFee(contract.QuoteRequest{DistanceM: 4800})
	if err != nil {
		t.Fatalf("DeliveryFee: %v", err)
	}
	expanded, err := tariff.DeliveryFee(contract.QuoteRequest{DistanceM: 4800, ExpansionLevel: 1})
	if err != nil {
		t.Fatalf("DeliveryFee: %v", err)
	}

	// ৳40 + 5 × ৳10 = ৳90, then × 1.5 = ৳135.
	if base.Amount.Minor != 9000 || expanded.Amount.Minor != 13500 {
		t.Fatalf("base = %d, expanded = %d", base.Amount.Minor, expanded.Amount.Minor)
	}
	if !expanded.Expanded || expanded.Surcharge.Minor != 4500 {
		t.Errorf("surcharge = %+v, want ৳45 flagged", expanded)
	}

	// Every level above zero is expanded. The multiplier is applied once, not
	// once per rung: a customer four steps out pays for the distance, which the
	// per-kilometre rate already charges them for.
	for _, level := range []int{1, 2, 3, 4} {
		got, feeErr := tariff.DeliveryFee(contract.QuoteRequest{DistanceM: 4800, ExpansionLevel: level})
		if feeErr != nil {
			t.Fatalf("level %d: %v", level, feeErr)
		}
		if got.Amount.Minor != 13500 {
			t.Errorf("level %d = %d, want the same 13500 for the same distance", level, got.Amount.Minor)
		}
	}
}

// An odd amount times 1.5 lands on a half-poisha. Rounding to nearest rather
// than truncating keeps the platform from quietly taking the difference.
func TestTheMultiplierRoundsRatherThanTruncates(t *testing.T) {
	settings := appendixB()
	settings.ints[cfgcontract.PricingDeliveryBase] = 4001
	settings.ints[cfgcontract.PricingDeliveryPerKm] = 0

	got, err := tariffOf(t, settings).DeliveryFee(contract.QuoteRequest{DistanceM: 100, ExpansionLevel: 1})
	if err != nil {
		t.Fatalf("DeliveryFee: %v", err)
	}
	if got.Amount.Minor != 6002 { // 4001 × 1.5 = 6001.5
		t.Errorf("minor = %d, want 6002", got.Amount.Minor)
	}
}

func TestNegativeDistanceIsRefused(t *testing.T) {
	tariff := defaultTariff(t)
	if _, err := tariff.DeliveryFee(contract.QuoteRequest{DistanceM: -1}); errs.CodeOf(err) != "invalid_quote_request" {
		t.Fatalf("fee: err = %v", err)
	}
	if _, err := tariff.Quote(contract.QuoteRequest{DistanceM: -1}); errs.CodeOf(err) != "invalid_quote_request" {
		t.Fatalf("quote: err = %v", err)
	}
	if _, err := tariff.Quote(contract.QuoteRequest{SubtotalMinor: -1}); errs.CodeOf(err) != "invalid_quote_request" {
		t.Fatalf("negative subtotal: err = %v", err)
	}
	if _, err := tariff.DeliveryFee(contract.QuoteRequest{DistanceM: -1, ExpansionLevel: 1}); err == nil {
		t.Fatal("an expanded negative distance was priced")
	}
}

func TestTariffResolutionFailures(t *testing.T) {
	t.Run("configuration is unreachable", func(t *testing.T) {
		_, err := pricingapp.NewService(&fakeConfig{err: errBoom}).Tariff(context.Background(), dhaka)
		if errs.CodeOf(err) != "pricing_config_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	for _, key := range []string{
		cfgcontract.PricingDeliveryBase,
		cfgcontract.PricingDeliveryPerKm,
		cfgcontract.PricingExpansionMult,
		cfgcontract.PricingFreeDelivery,
	} {
		t.Run("missing "+key, func(t *testing.T) {
			settings := appendixB()
			settings.failOn = key
			_, err := pricingapp.NewService(&fakeConfig{settings: settings}).Tariff(context.Background(), dhaka)
			if errs.CodeOf(err) != "pricing_config_unavailable" {
				t.Fatalf("err = %v", err)
			}
		})
	}

	// A multiplier below one would make a longer, dearer journey cost the
	// customer less — the opposite of the rule the expansion ladder exists to
	// enforce. Refused rather than clamped, because a tuner that wrote it is a
	// tuner nobody should trust silently.
	t.Run("a multiplier below one", func(t *testing.T) {
		settings := appendixB()
		settings.ratios[cfgcontract.PricingExpansionMult] = 0.5
		_, err := pricingapp.NewService(&fakeConfig{settings: settings}).Tariff(context.Background(), dhaka)
		if errs.CodeOf(err) != "pricing_config_invalid" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a negative base fee", func(t *testing.T) {
		settings := appendixB()
		settings.ints[cfgcontract.PricingDeliveryBase] = -1
		_, err := pricingapp.NewService(&fakeConfig{settings: settings}).Tariff(context.Background(), dhaka)
		if errs.CodeOf(err) != "pricing_config_invalid" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a negative rate", func(t *testing.T) {
		settings := appendixB()
		settings.ints[cfgcontract.PricingDeliveryPerKm] = -1
		_, err := pricingapp.NewService(&fakeConfig{settings: settings}).Tariff(context.Background(), dhaka)
		if errs.CodeOf(err) != "pricing_config_invalid" {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTheTariffIsResolvedForThePlaceItIsAskedAbout(t *testing.T) {
	config := &fakeConfig{settings: appendixB()}
	if _, err := pricingapp.NewService(config).Tariff(context.Background(), dhaka); err != nil {
		t.Fatalf("Tariff: %v", err)
	}
	if len(config.seen) != 1 || config.seen[0].AreaCode != "DHA-DHK-DHN" ||
		config.seen[0].DivisionCode != "DHA" {
		t.Fatalf("config was asked about %+v", config.seen)
	}
}
