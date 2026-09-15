package discovery

import (
	"context"
	"testing"

	cfgcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/geo"
	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
	pricingapp "github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/application"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// dhanmondi is the point every search in these tests is run from.
var dhanmondi = geo.Point{Lat: 23.7465, Lng: 90.3760}

// dhaka is what geo resolves that point to.
func dhaka() geo.Area {
	return geo.Area{
		AreaCode: "DHA-DHK-DHN", AreaName: "Dhanmondi",
		DistrictCode: "DHA-DHK", DivisionCode: "DHA", DivisionName: "Dhaka",
	}
}

// rig is one assembled search, with every collaborator reachable for assertions.
type rig struct {
	geo      *fakeGeo
	merchant *fakeMerchant
	config   *fakeConfig
	pricing  *fakeConfig
	search   *application.SearchUseCase
}

func newRig(settings cfgcontract.Settings) *rig {
	g := &fakeGeo{area: dhaka()}
	m := &fakeMerchant{}
	c := &fakeConfig{settings: settings}
	// The real pricing service, over its own view of the same settings, so a
	// test can break pricing's configuration without breaking discovery's.
	prices := &fakeConfig{settings: settings}
	return &rig{geo: g, merchant: m, config: c, pricing: prices,
		search: application.NewSearchUseCase(g, m, c, pricingapp.NewService(prices))}
}

func defaultRig() *rig { return newRig(defaultSettings()) }

// A customer with shops around them sees them, nearest-relevant first, with
// every display string already composed.
func TestSearchReturnsWhatIsNearby(t *testing.T) {
	r := defaultRig()
	r.geo.nearby = []geo.NearbyMerchant{
		{MerchantID: "m1", DistanceM: 400},
		{MerchantID: "m2", DistanceM: 3200},
	}
	r.merchant.listed = []merchantcontract.Merchant{
		shop("m1", "Star Kabab", merchantcontract.TypeRestaurant, true),
		shop("m2", "Lazz Pharma", merchantcontract.TypePharmacy, true),
	}

	got, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi, Lang: "en"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got.Total != 2 || len(got.Merchants) != 2 {
		t.Fatalf("found %d merchants, want 2", got.Total)
	}
	if got.Merchants[0].ID != "m1" {
		t.Errorf("nearest shop is %q, want m1", got.Merchants[0].ID)
	}
	if got.Merchants[0].Distance != "400 m" {
		t.Errorf("distance = %q, want a composed string", got.Merchants[0].Distance)
	}
	if got.Merchants[0].Delivery.Amount.Display == "" || got.Merchants[0].Delivery.Expanded {
		t.Errorf("base-radius fee is wrong: %+v", got.Merchants[0].Delivery)
	}
	if got.Expansion.Stage != domain.StageBase || got.RadiusText != "5 km" {
		t.Errorf("expansion is wrong: %+v (%q)", got.Expansion, got.RadiusText)
	}
	// Five is the threshold, so two shops is thin and the offer stands.
	if !got.Expansion.Offered {
		t.Error("two shops did not produce an expansion offer")
	}
	if got.Notice == "" {
		t.Error("no notice was composed for the list header")
	}
	// The placement geo resolved is what config was asked about — area first,
	// which is what makes a per-area radius mean anything.
	if len(r.config.seen) != 1 || r.config.seen[0].AreaCode != "DHA-DHK-DHN" {
		t.Errorf("config was asked about %+v", r.config.seen)
	}
	// The ids handed to merchant are exactly what the radius search returned.
	if len(r.merchant.asked) != 2 || r.merchant.asked[0] != "m1" {
		t.Errorf("merchant was asked about %v", r.merchant.asked)
	}
}

// D2: nothing nearby, so the customer is offered a wider search, and told it
// will cost more.
func TestSearchOffersExpansionWhenThereIsNothing(t *testing.T) {
	r := defaultRig()
	got, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi, Lang: "en"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Total != 0 || len(got.Merchants) != 0 {
		t.Fatalf("an empty radius returned %d merchants", got.Total)
	}
	if !got.Expansion.CanExpand || !got.Expansion.Offered {
		t.Fatalf("no expansion was offered: %+v", got.Expansion)
	}
	if got.NextRadiusText != "10 km" {
		t.Errorf("next radius = %q, want 10 km", got.NextRadiusText)
	}
	if want := "No shops within 5 km. Try searching up to 10 km — delivery will cost more."; got.Notice != want {
		t.Errorf("notice = %q, want %q", got.Notice, want)
	}
}

// D2, the other half: expanding costs more. This is the acceptance criterion
// stated as an assertion — the same shop, at the same distance, quoted at two
// levels.
func TestExpandingRaisesTheDeliveryFee(t *testing.T) {
	near := []geo.NearbyMerchant{{MerchantID: "m1", DistanceM: 6200}}
	listed := []merchantcontract.Merchant{shop("m1", "Star Kabab", merchantcontract.TypeRestaurant, true)}

	base := defaultRig()
	base.geo.nearby = near
	base.merchant.listed = listed
	atBase, err := base.search.Execute(context.Background(), application.Query{Point: dhanmondi, Level: 0})
	if err != nil {
		t.Fatalf("Execute at base: %v", err)
	}

	wide := defaultRig()
	wide.geo.nearby = near
	wide.merchant.listed = listed
	atLevel2, err := wide.search.Execute(context.Background(), application.Query{Point: dhanmondi, Level: 2})
	if err != nil {
		t.Fatalf("Execute at level 2: %v", err)
	}

	baseFee := atBase.Merchants[0].Delivery
	wideFee := atLevel2.Merchants[0].Delivery
	if wideFee.Amount.Minor <= baseFee.Amount.Minor {
		t.Fatalf("expanding did not raise the fee: %d then %d", baseFee.Amount.Minor, wideFee.Amount.Minor)
	}
	if !wideFee.Expanded || baseFee.Expanded {
		t.Errorf("the surcharge was not flagged: base %+v, expanded %+v", baseFee, wideFee)
	}
	// The surcharge is exactly the difference, so the app can show what the
	// customer's choice to widen actually cost.
	if wideFee.Surcharge.Minor != wideFee.Amount.Minor-baseFee.Amount.Minor {
		t.Errorf("surcharge = %d, want %d", wideFee.Surcharge.Minor,
			wideFee.Amount.Minor-baseFee.Amount.Minor)
	}
	if atLevel2.Expansion.RadiusM != 15000 {
		t.Errorf("level 2 searched %v m, want 15000", atLevel2.Expansion.RadiusM)
	}
	// One tariff resolved for the whole page, not one per shop card.
	if len(wide.pricing.seen) != 1 {
		t.Errorf("pricing was asked %d times for one page", len(wide.pricing.seen))
	}
}

// D3: the ladder stops, and the terminal state says so rather than offering a
// button that could only fail.
func TestSearchStopsAtTheDivisionCeiling(t *testing.T) {
	r := defaultRig()
	got, err := r.search.Execute(context.Background(), application.Query{
		Point: dhanmondi, Level: 99, Lang: "en",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Expansion.Level != 4 || got.Expansion.RadiusM != 25000 {
		t.Fatalf("a level beyond the ladder was not clamped: %+v", got.Expansion)
	}
	if got.Expansion.CanExpand || got.NextRadiusText != "" {
		t.Errorf("the ceiling offered a further step: %+v", got.Expansion)
	}
	if want := "No shops found within 25 km, which is as far as we can search in your division."; got.Notice != want {
		t.Errorf("terminal notice = %q, want %q", got.Notice, want)
	}
}

// The ceiling notice when there *are* shops is a different sentence: the
// customer is not stuck, they are simply at the widest view.
func TestCeilingNoticeWithResults(t *testing.T) {
	r := defaultRig()
	r.geo.nearby = []geo.NearbyMerchant{{MerchantID: "m1", DistanceM: 21000}}
	r.merchant.listed = []merchantcontract.Merchant{shop("m1", "Far Shop", merchantcontract.TypeGrocery, true)}

	got, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi, Level: 4, Lang: "en"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := "Within 25 km — as far as we can search in your division."; got.Notice != want {
		t.Errorf("notice = %q, want %q", got.Notice, want)
	}
}

// The plain case: enough shops, room to expand, nothing to suggest.
func TestPlainNotice(t *testing.T) {
	r := defaultRig()
	for i, id := range []string{"m1", "m2", "m3", "m4", "m5"} {
		r.geo.nearby = append(r.geo.nearby, geo.NearbyMerchant{MerchantID: id, DistanceM: float64(100 * (i + 1))})
		r.merchant.listed = append(r.merchant.listed, shop(id, "Shop "+id, merchantcontract.TypeRestaurant, true))
	}
	got, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi, Lang: "en"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Notice != "Within 5 km" {
		t.Errorf("notice = %q, want %q", got.Notice, "Within 5 km")
	}
	if got.Expansion.Offered {
		t.Error("five shops still triggered an expansion offer")
	}
}

// Every notice has a Bengali form, because the audience reads Bengali (1.4).
func TestNoticesAreBengaliByDefault(t *testing.T) {
	cases := []struct {
		name  string
		level int
		want  string
	}{
		{"nothing nearby", 0, "৫ কিমি এর মধ্যে কোনো দোকান নেই। ১০ কিমি পর্যন্ত খুঁজে দেখুন — ডেলিভারি চার্জ বেশি হবে।"},
		{"nothing at the ceiling", 4, "আপনার বিভাগের মধ্যে ২৫ কিমি পর্যন্ত খুঁজে কোনো দোকান পাওয়া যায়নি। এটাই সর্বোচ্চ দূরত্ব।"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := defaultRig().search.Execute(context.Background(), application.Query{Point: dhanmondi, Level: tc.level})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if got.Notice != tc.want {
				t.Errorf("notice = %q, want %q", got.Notice, tc.want)
			}
		})
	}
}

func TestBengaliOfferAndCeilingNoticesWithResults(t *testing.T) {
	thin := defaultRig()
	thin.geo.nearby = []geo.NearbyMerchant{{MerchantID: "m1", DistanceM: 400}}
	thin.merchant.listed = []merchantcontract.Merchant{shop("m1", "Star", merchantcontract.TypeRestaurant, true)}
	got, err := thin.search.Execute(context.Background(), application.Query{Point: dhanmondi})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := "কাছাকাছি কম দোকান আছে। ১০ কিমি পর্যন্ত খুঁজলে আরও পাবেন — ডেলিভারি চার্জ বেশি হবে।"; got.Notice != want {
		t.Errorf("offer notice = %q, want %q", got.Notice, want)
	}

	atCeiling := defaultRig()
	atCeiling.geo.nearby = []geo.NearbyMerchant{{MerchantID: "m1", DistanceM: 400}}
	atCeiling.merchant.listed = []merchantcontract.Merchant{shop("m1", "Star", merchantcontract.TypeRestaurant, true)}
	got, err = atCeiling.search.Execute(context.Background(), application.Query{Point: dhanmondi, Level: 4})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := "২৫ কিমি এর মধ্যে — এটাই আপনার বিভাগের সর্বোচ্চ দূরত্ব।"; got.Notice != want {
		t.Errorf("ceiling notice = %q, want %q", got.Notice, want)
	}

	plenty := defaultRig()
	for i, id := range []string{"m1", "m2", "m3", "m4", "m5"} {
		plenty.geo.nearby = append(plenty.geo.nearby, geo.NearbyMerchant{MerchantID: id, DistanceM: float64(100 * (i + 1))})
		plenty.merchant.listed = append(plenty.merchant.listed, shop(id, "Shop", merchantcontract.TypeRestaurant, true))
	}
	got, err = plenty.search.Execute(context.Background(), application.Query{Point: dhanmondi})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Notice != "৫ কিমি এর মধ্যে" {
		t.Errorf("plain notice = %q", got.Notice)
	}
}

// ALG-02 with auto-expand on: the ladder is walked with counting queries until
// the threshold clears, and the walk stops there rather than going further.
func TestAutoExpandWalksTheLadderUntilThresholdClears(t *testing.T) {
	settings := defaultSettings()
	settings.bools[cfgcontract.DiscoveryAutoExpand] = true
	r := newRig(settings)
	r.geo.counts = []int{0, 2, 9} // 5 km, 10 km, then 15 km clears the five

	got, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Expansion.Level != 2 || got.Expansion.RadiusM != 15000 {
		t.Fatalf("auto-expand settled at %+v, want level 2 / 15000 m", got.Expansion)
	}
	if len(r.geo.counted) != 3 {
		t.Fatalf("counting queries: %v, want three rungs walked", r.geo.counted)
	}
	if r.geo.counted[0] != 5000 || r.geo.counted[1] != 10000 || r.geo.counted[2] != 15000 {
		t.Errorf("the ladder was not walked stepwise: %v", r.geo.counted)
	}
	// One fetch, at the settled radius. The walk itself must not pull rows.
	if len(r.geo.radii) != 1 || r.geo.radii[0] != 15000 {
		t.Errorf("fetches: %v, want one at 15000 m", r.geo.radii)
	}
}

// D3 again, and the sharper case: auto-expand runs out of ladder and stops at
// the ceiling rather than widening further.
func TestAutoExpandStopsAtTheCeiling(t *testing.T) {
	settings := defaultSettings()
	settings.bools[cfgcontract.DiscoveryAutoExpand] = true
	r := newRig(settings)
	r.geo.counts = []int{0, 0, 0, 0, 0}

	got, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Expansion.Level != 4 || !got.Expansion.AtCeiling {
		t.Fatalf("auto-expand did not stop at the ceiling: %+v", got.Expansion)
	}
	if len(r.geo.counted) != 4 {
		t.Errorf("counted %d times, want four (the ceiling is not counted again)", len(r.geo.counted))
	}
}

// With auto-expand off — the default — expansion is the customer's decision,
// because it is their money. No counting queries at all.
func TestWithoutAutoExpandTheLevelIsTheCustomersChoice(t *testing.T) {
	r := defaultRig()
	if _, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi, Level: 1}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(r.geo.counted) != 0 {
		t.Errorf("counting queries were run with auto-expand off: %v", r.geo.counted)
	}
	if len(r.geo.radii) != 1 || r.geo.radii[0] != 10000 {
		t.Errorf("searched %v, want one query at 10000 m", r.geo.radii)
	}
}

func TestSearchFiltersByTypeAndName(t *testing.T) {
	r := defaultRig()
	r.geo.nearby = []geo.NearbyMerchant{
		{MerchantID: "m1", DistanceM: 100},
		{MerchantID: "m2", DistanceM: 200},
		{MerchantID: "m3", DistanceM: 300},
	}
	r.merchant.listed = []merchantcontract.Merchant{
		shop("m1", "Star Kabab", merchantcontract.TypeRestaurant, true),
		shop("m2", "Lazz Pharma", merchantcontract.TypePharmacy, true),
		shop("m3", "Kabab Ghar", merchantcontract.TypeRestaurant, true),
	}

	byType, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi, Type: "pharmacy"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if byType.Total != 1 || byType.Merchants[0].ID != "m2" {
		t.Errorf("type filter returned %+v", byType.Merchants)
	}

	byName, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi, Text: "kabab"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if byName.Total != 2 {
		t.Fatalf("name filter returned %d, want 2", byName.Total)
	}
	// "Kabab Ghar" starts with the query and outranks "Star Kabab", which only
	// contains it — relevance outweighs 200 m (ALG-07).
	if byName.Merchants[0].ID != "m3" {
		t.Errorf("relevance did not outrank distance: %+v", byName.Merchants)
	}
}

func TestSearchRefusesAnUnknownType(t *testing.T) {
	_, err := defaultRig().search.Execute(context.Background(), application.Query{Point: dhanmondi, Type: "hardware"})
	if !errs.Is(err, errs.KindInvalid) || errs.CodeOf(err) != "invalid_merchant_type" {
		t.Fatalf("an unknown shop type was accepted: %v", err)
	}
}

func TestSearchPages(t *testing.T) {
	r := defaultRig()
	for i, id := range []string{"m1", "m2", "m3", "m4", "m5"} {
		r.geo.nearby = append(r.geo.nearby, geo.NearbyMerchant{MerchantID: id, DistanceM: float64(100 * (i + 1))})
		r.merchant.listed = append(r.merchant.listed, shop(id, "Shop "+id, merchantcontract.TypeRestaurant, true))
	}

	cases := []struct {
		name          string
		limit, offset int
		wantIDs       []string
	}{
		{"first page", 2, 0, []string{"m1", "m2"}},
		{"second page", 2, 2, []string{"m3", "m4"}},
		{"past the end", 2, 9, nil},
		{"negative offset is the start", 2, -3, []string{"m1", "m2"}},
		{"no limit is everything", 0, 0, []string{"m1", "m2", "m3", "m4", "m5"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.search.Execute(context.Background(), application.Query{
				Point: dhanmondi, Limit: tc.limit, Offset: tc.offset,
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if len(got.Merchants) != len(tc.wantIDs) {
				t.Fatalf("page has %d entries, want %d", len(got.Merchants), len(tc.wantIDs))
			}
			for i, want := range tc.wantIDs {
				if got.Merchants[i].ID != want {
					t.Errorf("entry %d is %q, want %q", i, got.Merchants[i].ID, want)
				}
			}
			// Total is the whole match count, not the page — the client says
			// "৫টি দোকান" without another request.
			if got.Total != 5 {
				t.Errorf("total = %d, want 5", got.Total)
			}
		})
	}
}

// A shop the radius search found but merchant will not list — suspended, or
// closed for a holiday — must not appear. Keeping those two answers in two
// modules is what stops a WHERE clause deciding it.
func TestUnlistedShopsAreDroppedEvenWhenNearby(t *testing.T) {
	r := defaultRig()
	r.geo.nearby = []geo.NearbyMerchant{
		{MerchantID: "m1", DistanceM: 100},
		{MerchantID: "gone", DistanceM: 150},
	}
	r.merchant.listed = []merchantcontract.Merchant{shop("m1", "Star", merchantcontract.TypeRestaurant, true)}

	got, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Total != 1 || got.Merchants[0].ID != "m1" {
		t.Fatalf("an unlisted shop was shown: %+v", got.Merchants)
	}
}

func TestSearchFailures(t *testing.T) {
	t.Run("the point is outside every division", func(t *testing.T) {
		r := defaultRig()
		r.geo.areaErr = errs.New(errs.KindNotFound, "outside_service_area", "We do not deliver here yet.")
		_, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi})
		if !errs.Is(err, errs.KindNotFound) {
			t.Fatalf("err = %v, want a not-found", err)
		}
	})

	t.Run("configuration is unreachable", func(t *testing.T) {
		r := defaultRig()
		r.config.err = errBoom
		_, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi})
		if errs.CodeOf(err) != "discovery_config_unavailable" {
			t.Fatalf("err = %v, want discovery_config_unavailable", err)
		}
	})

	t.Run("a setting is missing", func(t *testing.T) {
		for _, key := range []string{
			cfgcontract.DiscoveryBaseRadius, cfgcontract.DiscoveryExpansionStep,
			cfgcontract.DiscoveryMaxExpansions, cfgcontract.DiscoveryMinMerchants,
			cfgcontract.DiscoveryAutoExpand, cfgcontract.DiscoveryDivisionCeil,
		} {
			settings := defaultSettings()
			settings.failOn = key
			r := newRig(settings)
			_, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi})
			if errs.CodeOf(err) != "discovery_config_unavailable" {
				t.Errorf("%s: err = %v, want discovery_config_unavailable", key, err)
			}
		}
	})

	// The one invariant. A tuner bug that wrote `false` must stop the search,
	// not widen it.
	t.Run("the division ceiling was turned off", func(t *testing.T) {
		settings := defaultSettings()
		settings.bools[cfgcontract.DiscoveryDivisionCeil] = false
		_, err := newRig(settings).search.Execute(context.Background(), application.Query{Point: dhanmondi})
		if errs.CodeOf(err) != "division_ceiling_disabled" {
			t.Fatalf("err = %v, want division_ceiling_disabled", err)
		}
	})

	t.Run("a setting is present but impossible", func(t *testing.T) {
		settings := defaultSettings()
		settings.ints[cfgcontract.DiscoveryBaseRadius] = 0
		_, err := newRig(settings).search.Execute(context.Background(), application.Query{Point: dhanmondi})
		if errs.CodeOf(err) != "discovery_config_invalid" {
			t.Fatalf("err = %v, want discovery_config_invalid", err)
		}
	})

	t.Run("the radius search fails", func(t *testing.T) {
		r := defaultRig()
		r.geo.nearErr = errBoom
		if _, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi}); err == nil {
			t.Fatal("a failed radius search was reported as an empty neighbourhood")
		}
	})

	t.Run("the count query fails during auto-expand", func(t *testing.T) {
		settings := defaultSettings()
		settings.bools[cfgcontract.DiscoveryAutoExpand] = true
		r := newRig(settings)
		r.geo.countErr = errBoom
		if _, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi}); err == nil {
			t.Fatal("a failed count was reported as an empty neighbourhood")
		}
	})

	t.Run("the merchant listing fails", func(t *testing.T) {
		r := defaultRig()
		r.geo.nearby = []geo.NearbyMerchant{{MerchantID: "m1", DistanceM: 100}}
		r.merchant.listedErr = errBoom
		if _, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi}); err == nil {
			t.Fatal("a failed listing was reported as no shops")
		}
	})

	t.Run("the fee cannot be quoted", func(t *testing.T) {
		r := defaultRig()
		r.geo.nearby = []geo.NearbyMerchant{{MerchantID: "m1", DistanceM: 100}}
		r.merchant.listed = []merchantcontract.Merchant{shop("m1", "Star", merchantcontract.TypeRestaurant, true)}
		r.pricing.err = errBoom
		// A shop card with no fee is a shop the customer cannot decide about.
		// Failing the whole search is the honest answer.
		if _, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi}); err == nil {
			t.Fatal("a shop was shown with no delivery fee")
		}
	})
}

// A distance that cannot be priced. Unreachable while geo is the only source of
// distances, but discovery takes the number on trust from another module, and a
// shop card showing a fee worked out from nonsense would be worse than a search
// that refused.
func TestANonsensicalDistanceFailsTheSearch(t *testing.T) {
	r := defaultRig()
	r.geo.nearby = []geo.NearbyMerchant{{MerchantID: "m1", DistanceM: -1}}
	r.merchant.listed = []merchantcontract.Merchant{shop("m1", "Star", merchantcontract.TypeRestaurant, true)}

	if _, err := r.search.Execute(context.Background(), application.Query{Point: dhanmondi}); err == nil {
		t.Fatal("a shop card was priced over a negative distance")
	}
}
