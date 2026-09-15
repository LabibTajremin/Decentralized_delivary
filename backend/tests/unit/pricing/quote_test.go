package pricing

import (
	"testing"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/contract"
)

// rowsOf indexes a receipt by key so an assertion can name the row it means.
func rowsOf(t *testing.T, quote contract.Quote) map[string]contract.Row {
	t.Helper()
	out := map[string]contract.Row{}
	for _, row := range quote.Rows {
		if _, seen := out[row.Key]; seen {
			t.Fatalf("the receipt has two %q rows", row.Key)
		}
		out[row.Key] = row
	}
	return out
}

func TestAPlainQuote(t *testing.T) {
	// ৳300 of food, 2.4 km away: ৳40 + 3 × ৳10 = ৳70 delivery, ৳370 total.
	got, err := defaultTariff(t).Quote(contract.QuoteRequest{
		SubtotalMinor: 30000, DistanceM: 2400, Lang: "en",
	})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if got.Delivery.Minor != 7000 || got.Total.Minor != 37000 {
		t.Fatalf("quote = %+v", got)
	}
	if got.FreeDelivery || got.Expanded || got.ExpansionSurcharge.Minor != 0 {
		t.Errorf("quote = %+v", got)
	}
	// ৳200 short of the ৳500 threshold, and the notice says so — without the
	// client ever holding the threshold (2.9).
	if got.AwayFromFreeDelivery.Minor != 20000 {
		t.Errorf("away from free = %d, want 20000", got.AwayFromFreeDelivery.Minor)
	}
	if got.Notice != "৳ 200 more for free delivery." {
		t.Errorf("notice = %q", got.Notice)
	}

	rows := rowsOf(t, got)
	if len(rows) != 3 {
		t.Fatalf("receipt = %+v, want subtotal, delivery, total", got.Rows)
	}
	if rows["subtotal"].Amount.Minor != 30000 || rows["total"].Amount.Minor != 37000 {
		t.Errorf("rows = %+v", got.Rows)
	}
	if rows["delivery"].Label != "Delivery" {
		t.Errorf("label = %q", rows["delivery"].Label)
	}
	// The order matters: a receipt reads top to bottom.
	if got.Rows[0].Key != "subtotal" || got.Rows[len(got.Rows)-1].Key != "total" {
		t.Errorf("receipt order = %+v", got.Rows)
	}
}

func TestQuoteIsBengaliByDefault(t *testing.T) {
	got, err := defaultTariff(t).Quote(contract.QuoteRequest{SubtotalMinor: 30000, DistanceM: 2400})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	rows := rowsOf(t, got)
	if rows["subtotal"].Label != "পণ্যের মোট" || rows["delivery"].Label != "ডেলিভারি চার্জ" ||
		rows["total"].Label != "সর্বমোট" {
		t.Fatalf("labels = %+v", got.Rows)
	}
	if got.Notice != "আর ৳ 200 কিনলে ডেলিভারি ফ্রি।" {
		t.Errorf("notice = %q", got.Notice)
	}
}

func TestAnExpandedQuoteShowsWhatTheExpansionCost(t *testing.T) {
	got, err := defaultTariff(t).Quote(contract.QuoteRequest{
		SubtotalMinor: 20000, DistanceM: 8000, ExpansionLevel: 1, Lang: "en",
	})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	// ৳40 + 8 × ৳10 = ৳120, × 1.5 = ৳180, of which ৳60 is the surcharge.
	if got.Delivery.Minor != 18000 || got.ExpansionSurcharge.Minor != 6000 {
		t.Fatalf("quote = %+v", got)
	}
	if !got.Expanded || got.Total.Minor != 38000 {
		t.Errorf("quote = %+v", got)
	}

	rows := rowsOf(t, got)
	surcharge, ok := rows["expansion_surcharge"]
	if !ok || surcharge.Amount.Minor != 6000 {
		t.Fatalf("receipt = %+v", got.Rows)
	}
	if surcharge.Label != "Extra distance charge" {
		t.Errorf("label = %q", surcharge.Label)
	}
	if got.Notice != "Delivery costs ৳ 60 more because this shop is outside your usual area." {
		t.Errorf("notice = %q", got.Notice)
	}
}

// Free delivery, and the decision that it does not survive an expansion.
func TestFreeDelivery(t *testing.T) {
	tariff := defaultTariff(t)

	local, err := tariff.Quote(contract.QuoteRequest{SubtotalMinor: 60000, DistanceM: 2400, Lang: "en"})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if !local.FreeDelivery || local.Delivery.Minor != 0 || local.Total.Minor != 60000 {
		t.Fatalf("quote = %+v", local)
	}
	// What it would have cost is still reported, so the receipt can show the
	// waiver rather than a silently missing line.
	if local.DeliveryBeforeWaiver.Minor != 7000 {
		t.Errorf("before waiver = %d, want 7000", local.DeliveryBeforeWaiver.Minor)
	}
	rows := rowsOf(t, local)
	waiver, ok := rows["free_delivery"]
	if !ok || waiver.Amount.Minor != -7000 || waiver.Label != "Free delivery" {
		t.Fatalf("receipt = %+v", local.Rows)
	}
	if local.Notice != "Delivery is free on this order." || local.AwayFromFreeDelivery.Minor != 0 {
		t.Errorf("quote = %+v", local)
	}

	// Exactly at the threshold qualifies. "Above ৳500" reads as "৳500 or more"
	// to everybody who is not writing the comparison.
	atThreshold, err := tariff.Quote(contract.QuoteRequest{SubtotalMinor: 50000, DistanceM: 2400})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if !atThreshold.FreeDelivery {
		t.Error("an order at exactly the threshold did not earn free delivery")
	}

	// Expanding forfeits it. Otherwise a ৳500 order could be carried across the
	// division for nothing, and D2 — "when the radius is extended, the delivery
	// charge increases" — would stop being true above the threshold.
	expanded, err := tariff.Quote(contract.QuoteRequest{
		SubtotalMinor: 60000, DistanceM: 2400, ExpansionLevel: 1, Lang: "en",
	})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if expanded.FreeDelivery || expanded.Delivery.Minor == 0 {
		t.Fatalf("free delivery survived an expansion: %+v", expanded)
	}
	if expanded.AwayFromFreeDelivery.Minor != 0 {
		t.Errorf("an expanded quote still offered a path to free delivery: %+v", expanded)
	}
	if _, offered := rowsOf(t, expanded)["free_delivery"]; offered {
		t.Error("an expanded receipt carried a free-delivery row")
	}
}

// A threshold of zero switches the promotion off rather than making every
// delivery free. Nobody configures "free delivery on orders over nothing", and
// reading it that way would give away every delivery in an area the moment
// somebody cleared the field.
func TestAZeroThresholdMeansNoPromotion(t *testing.T) {
	settings := appendixB()
	settings.ints[cfgcontract.PricingFreeDelivery] = 0

	got, err := tariffOf(t, settings).Quote(contract.QuoteRequest{
		SubtotalMinor: 100000, DistanceM: 1000, Lang: "en",
	})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if got.FreeDelivery || got.Delivery.Minor != 5000 {
		t.Fatalf("a zero threshold gave away a delivery: %+v", got)
	}
	if got.AwayFromFreeDelivery.Minor != 0 || got.Notice != "" {
		t.Errorf("a switched-off promotion still spoke: %+v", got)
	}
}

// An empty cart is still a valid question — a client may ask what delivery
// would cost before anything is in it.
func TestAZeroSubtotalIsPriced(t *testing.T) {
	got, err := defaultTariff(t).Quote(contract.QuoteRequest{DistanceM: 1000, Lang: "en"})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if got.Total.Minor != 5000 || got.FreeDelivery {
		t.Fatalf("quote = %+v", got)
	}
	if got.AwayFromFreeDelivery.Minor != 50000 {
		t.Errorf("away from free = %d, want the whole threshold", got.AwayFromFreeDelivery.Minor)
	}
}

// The two sentences an expanded quote can carry, in Bengali. Pinned here
// because the audience reads Bengali (1.4) and because a change to the wording
// should be a deliberate act.
func TestAnExpandedQuoteSpeaksBengali(t *testing.T) {
	got, err := defaultTariff(t).Quote(contract.QuoteRequest{
		SubtotalMinor: 60000, DistanceM: 8000, ExpansionLevel: 1,
	})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	rows := rowsOf(t, got)
	if rows["expansion_surcharge"].Label != "দূরত্বের জন্য অতিরিক্ত" {
		t.Errorf("label = %q", rows["expansion_surcharge"].Label)
	}
	if got.Notice != "দূরের দোকান বেছে নেওয়ায় ডেলিভারি চার্জ ৳ 60 বেশি।" {
		t.Errorf("notice = %q", got.Notice)
	}
}

func TestFreeDeliverySpeaksBengali(t *testing.T) {
	got, err := defaultTariff(t).Quote(contract.QuoteRequest{SubtotalMinor: 60000, DistanceM: 2400})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if rowsOf(t, got)["free_delivery"].Label != "ফ্রি ডেলিভারি" {
		t.Errorf("label = %q", rowsOf(t, got)["free_delivery"].Label)
	}
	if got.Notice != "এই অর্ডারে ডেলিভারি ফ্রি।" {
		t.Errorf("notice = %q", got.Notice)
	}
}

// An expanded order at a distance the multiplier does not move — a zero base
// and a zero rate — carries no surcharge, so the receipt gains no row and the
// notice says nothing about one.
func TestAnExpansionThatCostsNothingSaysNothing(t *testing.T) {
	settings := appendixB()
	settings.ints[cfgcontract.PricingDeliveryBase] = 0
	settings.ints[cfgcontract.PricingDeliveryPerKm] = 0

	got, err := tariffOf(t, settings).Quote(contract.QuoteRequest{
		SubtotalMinor: 10000, DistanceM: 8000, ExpansionLevel: 1, Lang: "en",
	})
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if _, charged := rowsOf(t, got)["expansion_surcharge"]; charged {
		t.Errorf("a zero surcharge got a row: %+v", got.Rows)
	}
	if got.Notice != "" {
		t.Errorf("notice = %q, want nothing", got.Notice)
	}
}
