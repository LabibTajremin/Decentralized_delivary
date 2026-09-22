// Package catalogue tests the catalogue domain, use cases and transport.
package catalogue

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/schedule"
)

// at builds an instant in Bangladesh local time.
func at(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, schedule.Bangladesh)
}

// longText builds a string of n Bengali characters.
func longText(n int) string {
	out := make([]rune, n)
	for i := range out {
		out[i] = 'অ'
	}
	return string(out)
}

// --------------------------------------------- the per-type capability table

// TestCapabilitiesAreExactlyThis is the phase's acceptance criterion, pinned.
// Every per-type rule in the module reads from this table, so this test is what
// makes changing one of those rules deliberate rather than incidental.
func TestCapabilitiesAreExactlyThis(t *testing.T) {
	want := map[domain.MerchantType]domain.Capabilities{
		domain.Restaurant: {Variants: true, AddOns: true, Combos: true},
		domain.Grocery:    {Variants: true, Combos: true, TracksStock: true, RequiresUnit: true},
		domain.Pharmacy:   {Variants: true, TracksStock: true, RequiresUnit: true, Prescriptions: true},
	}

	for kind, expected := range want {
		if got := domain.CapabilitiesFor(kind); got != expected {
			t.Errorf("%s = %+v, want %+v", kind, got, expected)
		}
	}
}

// TestAnUnknownShopTypeGetsNothing: CapabilitiesFor fails closed rather than
// inheriting a restaurant's rules, which is what a default of "the first case"
// would do.
func TestAnUnknownShopTypeGetsNothing(t *testing.T) {
	if got := domain.CapabilitiesFor(domain.MerchantType("hardware")); got != (domain.Capabilities{}) {
		t.Errorf("CapabilitiesFor(hardware) = %+v, want nothing", got)
	}
}

// TestOnlyARestaurantHasAddOns states the rule the acceptance criterion is
// really about, in the form a reader will check it against.
func TestOnlyARestaurantHasAddOns(t *testing.T) {
	if !domain.CapabilitiesFor(domain.Restaurant).AddOns {
		t.Error("a restaurant has no add-ons")
	}
	for _, kind := range []domain.MerchantType{domain.Grocery, domain.Pharmacy} {
		if domain.CapabilitiesFor(kind).AddOns {
			t.Errorf("%s has add-ons", kind)
		}
	}
}

// TestOnlyAPharmacyHasPrescriptions. Getting this wrong in the other direction
// would make us a delivery channel for unlicensed medicine.
func TestOnlyAPharmacyHasPrescriptions(t *testing.T) {
	if !domain.CapabilitiesFor(domain.Pharmacy).Prescriptions {
		t.Error("a pharmacy cannot mark a prescription item")
	}
	for _, kind := range []domain.MerchantType{domain.Restaurant, domain.Grocery} {
		if domain.CapabilitiesFor(kind).Prescriptions {
			t.Errorf("%s can require a prescription", kind)
		}
	}
}

// TestOnlyShopsWithAShelfCountStock: a kitchen cooks to order, and a count
// there would be a number nobody updates.
func TestOnlyShopsWithAShelfCountStock(t *testing.T) {
	if domain.CapabilitiesFor(domain.Restaurant).TracksStock {
		t.Error("a restaurant counts stock")
	}
	for _, kind := range []domain.MerchantType{domain.Grocery, domain.Pharmacy} {
		if !domain.CapabilitiesFor(kind).TracksStock {
			t.Errorf("%s does not count stock", kind)
		}
	}
}

func TestMerchantTypesRoundTripThroughTheirCodes(t *testing.T) {
	for _, kind := range domain.AllMerchantTypes() {
		parsed, err := domain.ParseMerchantType(kind.String())
		if err != nil {
			t.Fatalf("ParseMerchantType(%q): %v", kind, err)
		}
		if parsed != kind {
			t.Errorf("ParseMerchantType(%q) = %q", kind, parsed)
		}
	}

	if parsed, err := domain.ParseMerchantType("  PHARMACY "); err != nil || parsed != domain.Pharmacy {
		t.Errorf("a padded, upper-case type = %q, %v", parsed, err)
	}
	if _, err := domain.ParseMerchantType("hardware"); !errors.Is(err, domain.ErrUnknownMerchantType) {
		t.Errorf("error = %v, want ErrUnknownMerchantType", err)
	}
}

// ------------------------------------------------------------- categories

func TestACategoryNeedsAShopAndAName(t *testing.T) {
	if _, err := domain.NewCategory("cat_1", "  ", "Biryani", 1); !errors.Is(err, domain.ErrEmptyMerchant) {
		t.Errorf("no shop: error = %v", err)
	}
	if _, err := domain.NewCategory("cat_1", "mch_1", "   ", 1); !errors.Is(err, domain.ErrEmptyName) {
		t.Errorf("no name: error = %v", err)
	}
	if _, err := domain.NewCategory("cat_1", "mch_1", longText(121), 1); !errors.Is(err, domain.ErrNameTooLong) {
		t.Errorf("long name: error = %v", err)
	}
}

// TestNameLengthIsCountedInCharactersNotBytes: a Bengali name is three bytes
// per character, and a byte limit would give it a third of the room.
func TestNameLengthIsCountedInCharactersNotBytes(t *testing.T) {
	if _, err := domain.NewCategory("cat_1", "mch_1", longText(120), 1); err != nil {
		t.Errorf("a 120-character Bengali name was refused: %v", err)
	}
}

func TestANewCategoryIsShownAndTrimmed(t *testing.T) {
	category, err := domain.NewCategory("cat_1", "mch_1", "  Biryani  ", 3)
	if err != nil {
		t.Fatalf("NewCategory: %v", err)
	}
	if category.Name != "Biryani" {
		t.Errorf("name = %q", category.Name)
	}
	if !category.Active {
		t.Error("a new category is hidden")
	}
	if category.SortOrder != 3 {
		t.Errorf("sort order = %d", category.SortOrder)
	}
}

func TestANegativeSortOrderBecomesZero(t *testing.T) {
	category, err := domain.NewCategory("cat_1", "mch_1", "Biryani", -5)
	if err != nil {
		t.Fatalf("NewCategory: %v", err)
	}
	if category.SortOrder != 0 {
		t.Errorf("sort order = %d, want 0", category.SortOrder)
	}
	if moved := category.WithSortOrder(-2); moved.SortOrder != 0 {
		t.Errorf("WithSortOrder(-2) = %d, want 0", moved.SortOrder)
	}
	if moved := category.WithSortOrder(7); moved.SortOrder != 7 {
		t.Errorf("WithSortOrder(7) = %d", moved.SortOrder)
	}
}

// TestRenamingKeepsWhetherItIsShown: an owner fixing a typo in a hidden
// section's name does not expect it to reappear on the menu.
func TestRenamingKeepsWhetherItIsShown(t *testing.T) {
	category, err := domain.NewCategory("cat_1", "mch_1", "Winter specials", 1)
	if err != nil {
		t.Fatalf("NewCategory: %v", err)
	}
	hidden := category.WithActive(false)

	renamed, err := hidden.WithName("Winter menu")
	if err != nil {
		t.Fatalf("WithName: %v", err)
	}
	if renamed.Active {
		t.Error("renaming a hidden section put it back on the menu")
	}
	if renamed.Name != "Winter menu" || renamed.ID != category.ID {
		t.Errorf("renamed = %+v", renamed)
	}

	if _, err := hidden.WithName("  "); !errors.Is(err, domain.ErrEmptyName) {
		t.Errorf("renaming to nothing: error = %v", err)
	}
}

// ------------------------------------------------------------------ stock

// TestUntrackedIsNotTheSameAsZero: a kitchen has no count at all, and reading
// that as zero would show every dish as sold out.
func TestUntrackedIsNotTheSameAsZero(t *testing.T) {
	untracked := domain.Untracked()
	if untracked.Tracked {
		t.Error("Untracked() is tracked")
	}
	if !untracked.InStock() {
		t.Error("an untracked item reads as out of stock")
	}

	none, err := domain.NewStock(0)
	if err != nil {
		t.Fatalf("NewStock(0): %v", err)
	}
	if !none.Tracked {
		t.Error("a counted zero is untracked")
	}
	if none.InStock() {
		t.Error("a counted zero reads as in stock")
	}
}

func TestAStockCountCannotBeNegative(t *testing.T) {
	if _, err := domain.NewStock(-1); !errors.Is(err, domain.ErrNegativeStock) {
		t.Errorf("error = %v, want ErrNegativeStock", err)
	}
	if stock, err := domain.NewStock(12); err != nil || stock.Quantity != 12 || !stock.InStock() {
		t.Errorf("NewStock(12) = %+v, %v", stock, err)
	}
}

// ----------------------------------------------------------- availability

// TestAlmostEverythingIsAlwaysAvailable, so that is the default and a schedule
// is the thing that takes configuring.
func TestAlmostEverythingIsAlwaysAvailable(t *testing.T) {
	always := domain.AlwaysAvailable()
	if !always.AvailableAt(at(2026, time.March, 2, 3, 0)) {
		t.Error("an always-available item was unavailable at 03:00")
	}
	if len(always.Encode()) != 0 {
		t.Errorf("Encode() = %v, want nothing stored", always.Encode())
	}
}

func TestABreakfastMenuEndsAtEleven(t *testing.T) {
	availability, err := domain.NewAvailability(map[time.Weekday][]schedule.Window{
		time.Monday: {mustWindow(t, "07:00-11:00")},
	})
	if err != nil {
		t.Fatalf("NewAvailability: %v", err)
	}

	if !availability.AvailableAt(at(2026, time.March, 2, 8, 0)) {
		t.Error("unavailable at 08:00 Monday")
	}
	if availability.AvailableAt(at(2026, time.March, 2, 12, 0)) {
		t.Error("available at noon Monday")
	}
	if availability.AvailableAt(at(2026, time.March, 3, 8, 0)) {
		t.Error("available on Tuesday, which was left out")
	}
}

// TestAnAvailabilityThatNeverOpensIsRefused: it makes an item invisible in a
// way that looks like a bug to its owner. Hiding it is what Active is for.
func TestAnAvailabilityThatNeverOpensIsRefused(t *testing.T) {
	if _, err := domain.NewAvailability(nil); err == nil {
		t.Error("an empty schedule was accepted")
	}
	if _, err := domain.NewAvailability(map[time.Weekday][]schedule.Window{
		time.Monday: {{Open: 600, Close: 100}},
	}); err == nil {
		t.Error("an impossible window was accepted")
	}
}

func TestAvailabilitySurvivesStorage(t *testing.T) {
	availability, err := domain.NewAvailability(map[time.Weekday][]schedule.Window{
		time.Monday: {mustWindow(t, "07:00-11:00")},
		time.Friday: {mustWindow(t, "15:00-24:00")},
	})
	if err != nil {
		t.Fatalf("NewAvailability: %v", err)
	}

	decoded, err := domain.DecodeAvailability(availability.Encode())
	if err != nil {
		t.Fatalf("DecodeAvailability: %v", err)
	}
	if decoded.Always {
		t.Error("a scheduled availability decoded as always")
	}
	if !decoded.AvailableAt(at(2026, time.March, 2, 8, 0)) {
		t.Error("the decoded schedule lost Monday morning")
	}

	// Nothing stored means always available, which is also how a row written
	// before schedules existed reads back.
	empty, err := domain.DecodeAvailability(nil)
	if err != nil {
		t.Fatalf("DecodeAvailability(nil): %v", err)
	}
	if !empty.Always {
		t.Error("an empty stored schedule did not decode as always available")
	}

	if _, err := domain.DecodeAvailability(map[string][]string{"1": {"nine to five"}}); err == nil {
		t.Error("an unreadable stored schedule was accepted")
	}
}

// mustWindow parses a window or fails the test.
func mustWindow(t *testing.T, text string) schedule.Window {
	t.Helper()
	w, err := schedule.ParseWindow(text)
	if err != nil {
		t.Fatalf("ParseWindow(%q): %v", text, err)
	}
	return w
}

// ------------------------------------------------------------------ units

func TestUnitsRoundTripThroughTheirCodes(t *testing.T) {
	for _, unit := range domain.AllUnits() {
		parsed, err := domain.ParseUnit(unit.String())
		if err != nil {
			t.Fatalf("ParseUnit(%q): %v", unit, err)
		}
		if parsed != unit {
			t.Errorf("ParseUnit(%q) = %q", unit, parsed)
		}
	}
	if _, err := domain.ParseUnit("furlong"); !errors.Is(err, domain.ErrUnknownUnit) {
		t.Errorf("error = %v, want ErrUnknownUnit", err)
	}
	if parsed, err := domain.ParseUnit("  KG "); err != nil || parsed != domain.UnitKilogram {
		t.Errorf("a padded unit = %q, %v", parsed, err)
	}
}

// ------------------------------------------------------------ image URLs

func TestImageAddressesWeAccept(t *testing.T) {
	cases := map[string]struct {
		in      string
		want    string
		refused bool
	}{
		"empty":             {"", "", false},
		"served path":       {"/static/demo/x.png", "/static/demo/x.png", false},
		"https":             {"https://cdn.example.com/x.png", "https://cdn.example.com/x.png", false},
		"plain http":        {"http://cdn.example.com/x.png", "", true},
		"bare https":        {"https://", "", true},
		"protocol-relative": {"//cdn.example.com/x.png", "", true},
		"relative":          {"x.png", "", true},
		"javascript":        {"javascript:alert(1)", "", true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			item, err := domain.NewItem("itm_1", "mch_1", domain.Restaurant, domain.ItemDraft{
				CategoryID: "cat_1", Name: "Kacchi", Price: money.Taka(35_000), ImageURL: tc.in,
			})
			if tc.refused {
				if !errors.Is(err, domain.ErrInvalidImageURL) {
					t.Errorf("error = %v, want ErrInvalidImageURL", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewItem: %v", err)
			}
			if item.ImageURL != tc.want {
				t.Errorf("image = %q, want %q", item.ImageURL, tc.want)
			}
		})
	}
}

// TestNamesAndDescriptionsAreTrimmed keeps stored text tidy without an owner
// having to notice a trailing space.
func TestNamesAndDescriptionsAreTrimmed(t *testing.T) {
	item, err := domain.NewItem("itm_1", "mch_1", domain.Restaurant, domain.ItemDraft{
		CategoryID: "  cat_1  ", Name: "  Kacchi  ", Description: "  Slow cooked.  ",
		Price: money.Taka(35_000),
	})
	if err != nil {
		t.Fatalf("NewItem: %v", err)
	}
	if item.Name != "Kacchi" || item.Description != "Slow cooked." || item.CategoryID != "cat_1" {
		t.Errorf("item = %+v", item)
	}
}

func TestADescriptionHasALimit(t *testing.T) {
	_, err := domain.NewItem("itm_1", "mch_1", domain.Restaurant, domain.ItemDraft{
		CategoryID: "cat_1", Name: "Kacchi", Description: longText(1001), Price: money.Taka(1),
	})
	if !errors.Is(err, domain.ErrTooLong) {
		t.Errorf("error = %v, want ErrTooLong", err)
	}
}

func TestShortTextFieldsHaveALimit(t *testing.T) {
	long := strings.Repeat("a", 61)
	cases := map[string]domain.Attributes{
		"pack size": {Unit: domain.UnitKilogram, PackSize: long},
		"brand":     {Unit: domain.UnitKilogram, Brand: long},
	}
	for name, attributes := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NewItem("itm_1", "mch_1", domain.Grocery, domain.ItemDraft{
				CategoryID: "cat_1", Name: "Rice", Price: money.Taka(1), Attributes: attributes,
			})
			if !errors.Is(err, domain.ErrTooLong) {
				t.Errorf("error = %v, want ErrTooLong", err)
			}
		})
	}

	for name, attributes := range map[string]domain.Attributes{
		"generic name": {Unit: domain.UnitStrip, GenericName: long},
		"strength":     {Unit: domain.UnitStrip, Strength: long},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NewItem("itm_1", "mch_1", domain.Pharmacy, domain.ItemDraft{
				CategoryID: "cat_1", Name: "Napa", Price: money.Taka(1), Attributes: attributes,
			})
			if !errors.Is(err, domain.ErrTooLong) {
				t.Errorf("error = %v, want ErrTooLong", err)
			}
		})
	}
}
