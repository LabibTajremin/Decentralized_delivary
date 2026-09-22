package catalogue

import (
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/schedule"
)

// These are the acceptance criterion at the level it bites: an item of one shop
// type must not be constructible in the shape of another. Each case below is a
// field that belongs to one type and is refused on the others.

// draftFor returns a minimal valid draft for a shop type.
func draftFor(kind domain.MerchantType) domain.ItemDraft {
	draft := domain.ItemDraft{
		CategoryID: "cat_1",
		Name:       "Something",
		Price:      money.Taka(12_000),
	}
	if domain.CapabilitiesFor(kind).RequiresUnit {
		draft.Attributes.Unit = domain.UnitPiece
	}
	return draft
}

func newItem(t *testing.T, kind domain.MerchantType, draft domain.ItemDraft) domain.Item {
	t.Helper()
	item, err := domain.NewItem("itm_1", "mch_1", kind, draft)
	if err != nil {
		t.Fatalf("NewItem(%s): %v", kind, err)
	}
	return item
}

// ------------------------------------------------------- the type barrier

// TestAGroceryItemMustSayHowItIsSold. Without a unit, "Rice 500" is a price
// with no meaning.
func TestAGroceryItemMustSayHowItIsSold(t *testing.T) {
	for _, kind := range []domain.MerchantType{domain.Grocery, domain.Pharmacy} {
		draft := draftFor(kind)
		draft.Attributes.Unit = ""

		if _, err := domain.NewItem("itm_1", "mch_1", kind, draft); !errors.Is(err, domain.ErrUnitRequired) {
			t.Errorf("%s with no unit: error = %v, want ErrUnitRequired", kind, err)
		}
	}
}

// TestARestaurantItemIsNotSoldByUnit, and a unit sent for one is refused rather
// than dropped — an owner who typed it expects to see it, and silently
// discarding it is how "the app lost my changes" starts.
func TestARestaurantItemIsNotSoldByUnit(t *testing.T) {
	draft := draftFor(domain.Restaurant)
	draft.Attributes.Unit = domain.UnitKilogram

	if _, err := domain.NewItem("itm_1", "mch_1", domain.Restaurant, draft); !errors.Is(err, domain.ErrUnitNotAllowed) {
		t.Errorf("error = %v, want ErrUnitNotAllowed", err)
	}
}

// TestOnlyAPharmacyItemMayNeedAPrescription.
func TestOnlyAPharmacyItemMayNeedAPrescription(t *testing.T) {
	for _, kind := range []domain.MerchantType{domain.Restaurant, domain.Grocery} {
		draft := draftFor(kind)
		draft.Attributes.RequiresPrescription = true

		if _, err := domain.NewItem("itm_1", "mch_1", kind, draft); !errors.Is(err, domain.ErrPrescriptionNotAllowed) {
			t.Errorf("%s: error = %v, want ErrPrescriptionNotAllowed", kind, err)
		}
	}

	draft := draftFor(domain.Pharmacy)
	draft.Attributes.RequiresPrescription = true
	draft.Attributes.GenericName = "Paracetamol"
	draft.Attributes.Strength = "500mg"

	item := newItem(t, domain.Pharmacy, draft)
	if !item.Attributes.RequiresPrescription {
		t.Error("a pharmacy item lost its prescription flag")
	}
	if item.Attributes.GenericName != "Paracetamol" || item.Attributes.Strength != "500mg" {
		t.Errorf("attributes = %+v", item.Attributes)
	}
}

// TestFieldsOfAnotherTypeAreNotCarried: a restaurant dish has no strength, and
// a consumer reading one gets an empty string rather than nonsense.
func TestFieldsOfAnotherTypeAreNotCarried(t *testing.T) {
	draft := draftFor(domain.Restaurant)
	draft.Attributes.GenericName = "Paracetamol"
	draft.Attributes.Strength = "500mg"
	draft.Attributes.Brand = "Beximco"
	draft.Attributes.PackSize = "1 kg"

	item := newItem(t, domain.Restaurant, draft)
	if item.Attributes.GenericName != "" || item.Attributes.Strength != "" {
		t.Errorf("a restaurant dish carried pharmacy fields: %+v", item.Attributes)
	}
	if item.Attributes.Brand != "" || item.Attributes.PackSize != "" {
		t.Errorf("a restaurant dish carried grocery fields: %+v", item.Attributes)
	}
}

// TestRestaurantFieldsAreOnlyOnRestaurantItems, the same rule in the other
// direction.
func TestRestaurantFieldsAreOnlyOnRestaurantItems(t *testing.T) {
	draft := draftFor(domain.Grocery)
	draft.Attributes.IsVegetarian = true
	draft.Attributes.PreparationMinutes = 25

	item := newItem(t, domain.Grocery, draft)
	if item.Attributes.IsVegetarian || item.Attributes.PreparationMinutes != 0 {
		t.Errorf("a grocery item carried restaurant fields: %+v", item.Attributes)
	}

	restaurant := draftFor(domain.Restaurant)
	restaurant.Attributes.IsVegetarian = true
	restaurant.Attributes.PreparationMinutes = 25
	dish := newItem(t, domain.Restaurant, restaurant)
	if !dish.Attributes.IsVegetarian || dish.Attributes.PreparationMinutes != 25 {
		t.Errorf("a restaurant dish lost its own fields: %+v", dish.Attributes)
	}
}

// TestANegativePreparationTimeIsIgnored rather than refused: it is a hint on a
// screen, not a rule, and a 400 over it would block a save for nothing.
func TestANegativePreparationTimeIsIgnored(t *testing.T) {
	draft := draftFor(domain.Restaurant)
	draft.Attributes.PreparationMinutes = -5

	item := newItem(t, domain.Restaurant, draft)
	if item.Attributes.PreparationMinutes != 0 {
		t.Errorf("preparation minutes = %d, want 0", item.Attributes.PreparationMinutes)
	}
}

// ---------------------------------------------------------- stock policy

// TestAKitchenHasNoShelfCount, and offering one would be a number nobody
// updates.
func TestAKitchenHasNoShelfCount(t *testing.T) {
	item := newItem(t, domain.Restaurant, draftFor(domain.Restaurant))
	if item.Stock.Tracked {
		t.Error("a restaurant dish tracks stock")
	}
	if !item.Stock.InStock() {
		t.Error("a restaurant dish reads as out of stock")
	}

	draft := draftFor(domain.Restaurant)
	draft.Stock = domain.Stock{Tracked: true, Quantity: 5}
	if _, err := domain.NewItem("itm_1", "mch_1", domain.Restaurant, draft); !errors.Is(err, domain.ErrStockNotTracked) {
		t.Errorf("error = %v, want ErrStockNotTracked", err)
	}
}

// TestACountedShopWithNoCountStartsSoldOut. Guessing the other way sells
// something the shop does not have.
func TestACountedShopWithNoCountStartsSoldOut(t *testing.T) {
	item := newItem(t, domain.Grocery, draftFor(domain.Grocery))
	if !item.Stock.Tracked {
		t.Error("a grocery item does not track stock")
	}
	if item.Stock.Quantity != 0 || item.Stock.InStock() {
		t.Errorf("stock = %+v, want a counted zero", item.Stock)
	}
}

func TestSettingAShelfCount(t *testing.T) {
	item := newItem(t, domain.Grocery, draftFor(domain.Grocery))

	stocked, err := item.WithStock(domain.Stock{Tracked: true, Quantity: 12})
	if err != nil {
		t.Fatalf("WithStock: %v", err)
	}
	if stocked.Stock.Quantity != 12 || !stocked.Stock.InStock() {
		t.Errorf("stock = %+v", stocked.Stock)
	}

	// The type policy still applies on an update, not only at creation.
	dish := newItem(t, domain.Restaurant, draftFor(domain.Restaurant))
	if _, err := dish.WithStock(domain.Stock{Tracked: true, Quantity: 1}); !errors.Is(err, domain.ErrStockNotTracked) {
		t.Errorf("error = %v, want ErrStockNotTracked", err)
	}
}

// ----------------------------------------------------------- basic shape

func TestAnItemNeedsAShopACategoryANameAndAPrice(t *testing.T) {
	cases := map[string]struct {
		merchantID string
		kind       domain.MerchantType
		mutate     func(*domain.ItemDraft)
		want       error
	}{
		"no shop":        {"  ", domain.Restaurant, func(*domain.ItemDraft) {}, domain.ErrEmptyMerchant},
		"unknown type":   {"mch_1", "hardware", func(*domain.ItemDraft) {}, domain.ErrUnknownMerchantType},
		"no category":    {"mch_1", domain.Restaurant, func(d *domain.ItemDraft) { d.CategoryID = " " }, domain.ErrNoCategory},
		"no name":        {"mch_1", domain.Restaurant, func(d *domain.ItemDraft) { d.Name = " " }, domain.ErrEmptyName},
		"negative price": {"mch_1", domain.Restaurant, func(d *domain.ItemDraft) { d.Price = money.Taka(-1) }, domain.ErrNegativePrice},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			draft := draftFor(domain.Restaurant)
			tc.mutate(&draft)
			if _, err := domain.NewItem("itm_1", tc.merchantID, tc.kind, draft); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestAFreeItemIsAllowed(t *testing.T) {
	draft := draftFor(domain.Restaurant)
	draft.Price = money.Taka(0)

	item := newItem(t, domain.Restaurant, draft)
	if !item.Price.IsZero() {
		t.Errorf("price = %s", item.Price)
	}
}

func TestANewItemIsShownAndAlwaysAvailable(t *testing.T) {
	item := newItem(t, domain.Restaurant, draftFor(domain.Restaurant))
	if !item.Active {
		t.Error("a new item is hidden")
	}
	if !item.Availability.Always {
		t.Error("a new item is on a schedule")
	}
	if item.SortOrder != 0 {
		t.Errorf("sort order = %d", item.SortOrder)
	}
}

func TestANegativeSortOrderOnAnItemBecomesZero(t *testing.T) {
	draft := draftFor(domain.Restaurant)
	draft.SortOrder = -3

	if item := newItem(t, domain.Restaurant, draft); item.SortOrder != 0 {
		t.Errorf("sort order = %d, want 0", item.SortOrder)
	}
}

func TestChangingAnItemsPrice(t *testing.T) {
	item := newItem(t, domain.Restaurant, draftFor(domain.Restaurant))

	repriced, err := item.WithPrice(money.Taka(40_000))
	if err != nil {
		t.Fatalf("WithPrice: %v", err)
	}
	if repriced.Price.Minor() != 40_000 {
		t.Errorf("price = %s", repriced.Price)
	}
	if _, err := item.WithPrice(money.Taka(-1)); !errors.Is(err, domain.ErrNegativePrice) {
		t.Errorf("error = %v, want ErrNegativePrice", err)
	}
}

// ------------------------------------------- three reasons it is not orderable

// TestACustomerIsToldWhichReasonApplies. "Sold out", "not on the menu right
// now" and "the shop took it down" are different things to tell someone, and
// collapsing them into one message is how a customer waits for a dish that is
// never coming back.
func TestACustomerIsToldWhichReasonApplies(t *testing.T) {
	noon := at(2026, time.March, 2, 12, 0)

	breakfast, err := domain.NewAvailability(map[time.Weekday][]schedule.Window{
		time.Monday: {mustWindow(t, "07:00-11:00")},
	})
	if err != nil {
		t.Fatalf("NewAvailability: %v", err)
	}

	orderable := newItem(t, domain.Restaurant, draftFor(domain.Restaurant))

	soldOut := newItem(t, domain.Grocery, draftFor(domain.Grocery)) // counted, zero
	scheduled := orderable.WithAvailability(breakfast)
	hidden := orderable.WithActive(false)

	cases := map[string]struct {
		item      domain.Item
		orderable bool
		reason    string
	}{
		"orderable":      {orderable, true, ""},
		"sold out":       {soldOut, false, "out_of_stock"},
		"outside window": {scheduled, false, "not_available_now"},
		"taken down":     {hidden, false, "unavailable"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.item.Orderable(noon); got != tc.orderable {
				t.Errorf("Orderable() = %v, want %v", got, tc.orderable)
			}
			if got := tc.item.UnavailableReason(noon); got != tc.reason {
				t.Errorf("UnavailableReason() = %q, want %q", got, tc.reason)
			}
		})
	}
}

// TestBeingHiddenOutranksTheOtherReasons: an owner who took an item down does
// not want a customer told it is merely sold out.
func TestBeingHiddenOutranksTheOtherReasons(t *testing.T) {
	item := newItem(t, domain.Grocery, draftFor(domain.Grocery)) // counted, zero, so also sold out
	hidden := item.WithActive(false)

	if got := hidden.UnavailableReason(at(2026, time.March, 2, 12, 0)); got != "unavailable" {
		t.Errorf("reason = %q, want unavailable", got)
	}
}

// -------------------------------------------------- variants and add-ons

// TestAddOnsAreRefusedOnTheShopTypesThatDoNotHaveThem, which is the rule the
// phase's acceptance criterion names most directly.
func TestAddOnsAreRefusedOnTheShopTypesThatDoNotHaveThem(t *testing.T) {
	group, err := domain.NewAddOnGroup("agr_1", "Extras", 0, 1, []domain.AddOn{
		mustAddOn(t, "aop_1", "Extra cheese", 3_000),
	})
	if err != nil {
		t.Fatalf("NewAddOnGroup: %v", err)
	}

	for _, kind := range []domain.MerchantType{domain.Grocery, domain.Pharmacy} {
		item := newItem(t, kind, draftFor(kind))
		if _, err := item.WithAddOnGroups([]domain.AddOnGroup{group}); !errors.Is(err, domain.ErrAddOnsNotAllowed) {
			t.Errorf("%s: error = %v, want ErrAddOnsNotAllowed", kind, err)
		}
	}

	dish := newItem(t, domain.Restaurant, draftFor(domain.Restaurant))
	withAddOns, err := dish.WithAddOnGroups([]domain.AddOnGroup{group})
	if err != nil {
		t.Fatalf("a restaurant dish refused add-ons: %v", err)
	}
	if len(withAddOns.AddOnGroups) != 1 {
		t.Errorf("groups = %d", len(withAddOns.AddOnGroups))
	}
}

// TestEmptyingTheGroupsIsAlwaysAllowed, whatever the shop type — otherwise a
// grocery that somehow acquired add-ons could never be cleaned up.
func TestEmptyingTheGroupsIsAlwaysAllowed(t *testing.T) {
	for _, kind := range domain.AllMerchantTypes() {
		item := newItem(t, kind, draftFor(kind))

		if _, err := item.WithAddOnGroups(nil); err != nil {
			t.Errorf("%s: clearing add-ons: %v", kind, err)
		}
		if _, err := item.WithVariantGroups(nil); err != nil {
			t.Errorf("%s: clearing variants: %v", kind, err)
		}
	}
}

// TestEveryShopTypeHasVariants: every type sells the same thing in more than
// one size.
func TestEveryShopTypeHasVariants(t *testing.T) {
	group, err := domain.NewVariantGroup("vgr_1", "Size", true, 1, 1, []domain.VariantOption{
		mustVariant(t, "vop_1", "Small", -5_000),
		mustVariant(t, "vop_2", "Large", 5_000),
	})
	if err != nil {
		t.Fatalf("NewVariantGroup: %v", err)
	}

	for _, kind := range domain.AllMerchantTypes() {
		item := newItem(t, kind, draftFor(kind))
		withVariants, err := item.WithVariantGroups([]domain.VariantGroup{group})
		if err != nil {
			t.Errorf("%s refused variants: %v", kind, err)
			continue
		}
		if len(withVariants.VariantGroups) != 1 {
			t.Errorf("%s: groups = %d", kind, len(withVariants.VariantGroups))
		}
	}
}

// TestAVariantMayCostLess: a small size legitimately does, and modelling that
// as a separate item would double every menu.
func TestAVariantMayCostLess(t *testing.T) {
	option := mustVariant(t, "vop_1", "Small", -5_000)
	if !option.PriceDelta.IsNegative() {
		t.Errorf("delta = %s, want a discount", option.PriceDelta)
	}
}

// TestAnAddOnMayNotCostLess: an extra is a thing with a price, and a negative
// one is a discount wearing a disguise.
func TestAnAddOnMayNotCostLess(t *testing.T) {
	if _, err := domain.NewAddOn("aop_1", "Extra cheese", money.Taka(-100)); !errors.Is(err, domain.ErrNegativePrice) {
		t.Errorf("error = %v, want ErrNegativePrice", err)
	}
}

func mustVariant(t *testing.T, id, name string, delta int64) domain.VariantOption {
	t.Helper()
	option, err := domain.NewVariantOption(id, name, money.Taka(delta))
	if err != nil {
		t.Fatalf("NewVariantOption(%q): %v", name, err)
	}
	return option
}

func mustAddOn(t *testing.T, id, name string, price int64) domain.AddOn {
	t.Helper()
	option, err := domain.NewAddOn(id, name, money.Taka(price))
	if err != nil {
		t.Fatalf("NewAddOn(%q): %v", name, err)
	}
	return option
}
