package catalogue

import (
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/schedule"
)

// ------------------------------------------------------- variant groups

func TestAGroupOfChoicesNeedsANameAndAtLeastOneOption(t *testing.T) {
	one := []domain.VariantOption{mustVariant(t, "vop_1", "Small", 0)}

	if _, err := domain.NewVariantGroup("vgr_1", "  ", false, 0, 1, one); !errors.Is(err, domain.ErrEmptyName) {
		t.Errorf("no name: error = %v", err)
	}
	if _, err := domain.NewVariantGroup("vgr_1", "Size", false, 0, 1, nil); !errors.Is(err, domain.ErrNoOptions) {
		t.Errorf("no options: error = %v", err)
	}
	if _, err := domain.NewVariantOption("vop_1", "  ", money.Taka(0)); !errors.Is(err, domain.ErrEmptyName) {
		t.Errorf("unnamed option: error = %v", err)
	}
}

// TestTwoOptionsCannotShareAName: the name is what a customer picks by, so two
// "Large" on one pizza is a support call whichever the kitchen makes.
func TestTwoOptionsCannotShareAName(t *testing.T) {
	options := []domain.VariantOption{
		mustVariant(t, "vop_1", "Large", 0),
		mustVariant(t, "vop_2", "large", 500), // differs only in case
	}
	if _, err := domain.NewVariantGroup("vgr_1", "Size", false, 0, 1, options); !errors.Is(err, domain.ErrDuplicateOption) {
		t.Errorf("error = %v, want ErrDuplicateOption", err)
	}

	addOns := []domain.AddOn{
		mustAddOn(t, "aop_1", "Cheese", 0),
		mustAddOn(t, "aop_2", "CHEESE", 500),
	}
	if _, err := domain.NewAddOnGroup("agr_1", "Extras", 0, 1, addOns); !errors.Is(err, domain.ErrDuplicateOption) {
		t.Errorf("add-ons: error = %v, want ErrDuplicateOption", err)
	}
}

func TestAGroupHasAnOptionLimit(t *testing.T) {
	options := make([]domain.VariantOption, 0, 21)
	for i := 0; i < 21; i++ {
		options = append(options, mustVariant(t, "vop", string(rune('A'+i)), 0))
	}
	if _, err := domain.NewVariantGroup("vgr_1", "Size", false, 0, 1, options); !errors.Is(err, domain.ErrTooManyOptions) {
		t.Errorf("error = %v, want ErrTooManyOptions", err)
	}
}

// TestARequiredGroupMustBeChosen: a pizza has no price until a size is picked,
// so a required group with a minimum of zero would charge for nothing chosen.
func TestARequiredGroupMustBeChosen(t *testing.T) {
	options := []domain.VariantOption{
		mustVariant(t, "vop_1", "Small", -5_000),
		mustVariant(t, "vop_2", "Large", 5_000),
	}

	group, err := domain.NewVariantGroup("vgr_1", "Size", true, 0, 1, options)
	if err != nil {
		t.Fatalf("NewVariantGroup: %v", err)
	}
	if group.MinChoices != 1 {
		t.Errorf("a required group has a minimum of %d, want 1", group.MinChoices)
	}
}

func TestChoiceLimitsThatDoNotMakeSenseAreRefused(t *testing.T) {
	options := []domain.VariantOption{
		mustVariant(t, "vop_1", "A", 0),
		mustVariant(t, "vop_2", "B", 0),
	}

	cases := map[string][2]int{
		"minimum above maximum":    {2, 1},
		"maximum past the options": {0, 5},
	}
	for name, limits := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NewVariantGroup("vgr_1", "Size", false, limits[0], limits[1], options)
			if !errors.Is(err, domain.ErrInvalidChoiceRange) {
				t.Errorf("error = %v, want ErrInvalidChoiceRange", err)
			}
		})
	}
}

func TestChoiceLimitsAreNormalised(t *testing.T) {
	options := []domain.VariantOption{
		mustVariant(t, "vop_1", "A", 0),
		mustVariant(t, "vop_2", "B", 0),
	}

	// A maximum of zero means "pick one", the ordinary single-choice group.
	group, err := domain.NewVariantGroup("vgr_1", "Size", false, 0, 0, options)
	if err != nil {
		t.Fatalf("NewVariantGroup: %v", err)
	}
	if group.MaxChoices != 1 || group.MinChoices != 0 {
		t.Errorf("limits = %d..%d, want 0..1", group.MinChoices, group.MaxChoices)
	}

	// A negative minimum is nonsense from a client, not a reason to refuse.
	group, err = domain.NewVariantGroup("vgr_2", "Sauce", false, -3, 2, options)
	if err != nil {
		t.Fatalf("NewVariantGroup: %v", err)
	}
	if group.MinChoices != 0 {
		t.Errorf("minimum = %d, want 0", group.MinChoices)
	}
}

// --------------------------------------------------------- add-on groups

func TestAnAddOnGroupDefaultsToAllOfThem(t *testing.T) {
	options := []domain.AddOn{
		mustAddOn(t, "aop_1", "Cheese", 3_000),
		mustAddOn(t, "aop_2", "Olives", 2_000),
	}

	group, err := domain.NewAddOnGroup("agr_1", "Extras", 0, 0, options)
	if err != nil {
		t.Fatalf("NewAddOnGroup: %v", err)
	}
	if group.MaxChoices != 2 {
		t.Errorf("maximum = %d, want all of them", group.MaxChoices)
	}

	// A maximum past the options is clamped rather than refused: it means the
	// same thing as "all of them".
	group, err = domain.NewAddOnGroup("agr_2", "Extras", 0, 99, options)
	if err != nil {
		t.Fatalf("NewAddOnGroup: %v", err)
	}
	if group.MaxChoices != 2 {
		t.Errorf("maximum = %d, want clamped to 2", group.MaxChoices)
	}
}

func TestAnAddOnGroupNeedsANameAndOptions(t *testing.T) {
	if _, err := domain.NewAddOnGroup("agr_1", " ", 0, 1, []domain.AddOn{mustAddOn(t, "a", "Cheese", 0)}); !errors.Is(err, domain.ErrEmptyName) {
		t.Errorf("no name: error = %v", err)
	}
	if _, err := domain.NewAddOnGroup("agr_1", "Extras", 0, 1, nil); !errors.Is(err, domain.ErrNoOptions) {
		t.Errorf("no options: error = %v", err)
	}
}

func TestAnAddOnGroupRefusesImpossibleLimits(t *testing.T) {
	options := []domain.AddOn{
		mustAddOn(t, "aop_1", "Cheese", 0),
		mustAddOn(t, "aop_2", "Olives", 0),
	}
	if _, err := domain.NewAddOnGroup("agr_1", "Extras", 2, 1, options); !errors.Is(err, domain.ErrInvalidChoiceRange) {
		t.Errorf("error = %v, want ErrInvalidChoiceRange", err)
	}

	// A negative minimum is normalised, not refused.
	group, err := domain.NewAddOnGroup("agr_2", "Extras", -1, 2, options)
	if err != nil {
		t.Fatalf("NewAddOnGroup: %v", err)
	}
	if group.MinChoices != 0 {
		t.Errorf("minimum = %d, want 0", group.MinChoices)
	}
}

func TestOptionsStartAvailable(t *testing.T) {
	if !mustVariant(t, "vop_1", "Large", 0).Available {
		t.Error("a new variant option is unavailable")
	}
	if !mustAddOn(t, "aop_1", "Cheese", 0).Available {
		t.Error("a new add-on is unavailable")
	}
}

// -------------------------------------------------------------- combos

func TestACombosPriceIsStatedNotDerived(t *testing.T) {
	combo := newCombo(t, domain.Restaurant, 45_000, []domain.ComboLine{
		{ItemID: "itm_1", Quantity: 1},
		{ItemID: "itm_2", Quantity: 2},
	})
	if combo.Price.Minor() != 45_000 {
		t.Errorf("price = %s", combo.Price)
	}
	if got := combo.ItemIDs(); len(got) != 2 || got[0] != "itm_1" {
		t.Errorf("ItemIDs() = %v", got)
	}
}

// TestACombosOfOneIsAPriceChange, not a bundle, so two members is the minimum.
func TestACombosOfOneIsAPriceChange(t *testing.T) {
	_, err := domain.NewCombo("cmb_1", "mch_1", domain.Restaurant, domain.ComboDraft{
		Name: "Meal", Price: money.Taka(1),
		Lines: []domain.ComboLine{{ItemID: "itm_1", Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrEmptyCombo) {
		t.Errorf("error = %v, want ErrEmptyCombo", err)
	}
}

func TestACombosMembershipIsChecked(t *testing.T) {
	big := make([]domain.ComboLine, 0, 21)
	for i := 0; i < 21; i++ {
		big = append(big, domain.ComboLine{ItemID: string(rune('a' + i)), Quantity: 1})
	}

	cases := map[string]struct {
		lines []domain.ComboLine
		want  error
	}{
		"too many": {big, domain.ErrComboTooLarge},
		"an item twice": {[]domain.ComboLine{
			{ItemID: "itm_1", Quantity: 1}, {ItemID: "itm_1", Quantity: 1},
		}, domain.ErrDuplicateComboLine},
		"no item id": {[]domain.ComboLine{
			{ItemID: " ", Quantity: 1}, {ItemID: "itm_2", Quantity: 1},
		}, domain.ErrItemNotFound},
		"zero quantity": {[]domain.ComboLine{
			{ItemID: "itm_1", Quantity: 0}, {ItemID: "itm_2", Quantity: 1},
		}, domain.ErrInvalidQuantity},
		"absurd quantity": {[]domain.ComboLine{
			{ItemID: "itm_1", Quantity: 11}, {ItemID: "itm_2", Quantity: 1},
		}, domain.ErrInvalidQuantity},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NewCombo("cmb_1", "mch_1", domain.Restaurant, domain.ComboDraft{
				Name: "Meal", Price: money.Taka(1), Lines: tc.lines,
			})
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestAPharmacyDoesNotBundleMedicines. Making that easy is not something we
// should do.
func TestAPharmacyDoesNotBundleMedicines(t *testing.T) {
	_, err := domain.NewCombo("cmb_1", "mch_1", domain.Pharmacy, domain.ComboDraft{
		Name: "Cold pack", Price: money.Taka(10_000),
		Lines: []domain.ComboLine{{ItemID: "itm_1", Quantity: 1}, {ItemID: "itm_2", Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrCombosNotAllowed) {
		t.Errorf("error = %v, want ErrCombosNotAllowed", err)
	}

	// A grocery does run them.
	if _, err := domain.NewCombo("cmb_2", "mch_2", domain.Grocery, domain.ComboDraft{
		Name: "Weekly basket", Price: money.Taka(100_000),
		Lines: []domain.ComboLine{{ItemID: "itm_1", Quantity: 1}, {ItemID: "itm_2", Quantity: 1}},
	}); err != nil {
		t.Errorf("a grocery combo was refused: %v", err)
	}
}

func TestACombosOwnFieldsAreValidated(t *testing.T) {
	lines := []domain.ComboLine{{ItemID: "itm_1", Quantity: 1}, {ItemID: "itm_2", Quantity: 1}}

	cases := map[string]struct {
		merchantID string
		kind       domain.MerchantType
		mutate     func(*domain.ComboDraft)
		want       error
	}{
		"no shop":          {"  ", domain.Restaurant, func(*domain.ComboDraft) {}, domain.ErrEmptyMerchant},
		"unknown type":     {"mch_1", "hardware", func(*domain.ComboDraft) {}, domain.ErrUnknownMerchantType},
		"no name":          {"mch_1", domain.Restaurant, func(d *domain.ComboDraft) { d.Name = " " }, domain.ErrEmptyName},
		"long description": {"mch_1", domain.Restaurant, func(d *domain.ComboDraft) { d.Description = longText(1001) }, domain.ErrTooLong},
		"bad image":        {"mch_1", domain.Restaurant, func(d *domain.ComboDraft) { d.ImageURL = "http://x/y.png" }, domain.ErrInvalidImageURL},
		"negative price":   {"mch_1", domain.Restaurant, func(d *domain.ComboDraft) { d.Price = money.Taka(-1) }, domain.ErrNegativePrice},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			draft := domain.ComboDraft{Name: "Meal", Price: money.Taka(45_000), Lines: lines}
			tc.mutate(&draft)
			if _, err := domain.NewCombo("cmb_1", tc.merchantID, tc.kind, draft); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestANewCombosDefaults(t *testing.T) {
	combo := newCombo(t, domain.Restaurant, 45_000, []domain.ComboLine{
		{ItemID: "itm_1", Quantity: 1}, {ItemID: "itm_2", Quantity: 1},
	})
	if !combo.Active {
		t.Error("a new combo is hidden")
	}
	if !combo.Availability.Always {
		t.Error("a new combo is on a schedule")
	}
	if !combo.Orderable(at(2026, time.March, 2, 12, 0)) {
		t.Error("a new combo is not orderable")
	}

	hidden := combo.WithActive(false)
	if hidden.Orderable(at(2026, time.March, 2, 12, 0)) {
		t.Error("a hidden combo is orderable")
	}
}

func TestACombosNegativeSortOrderBecomesZero(t *testing.T) {
	combo, err := domain.NewCombo("cmb_1", "mch_1", domain.Restaurant, domain.ComboDraft{
		Name: "Meal", Price: money.Taka(1), SortOrder: -4,
		Lines: []domain.ComboLine{{ItemID: "itm_1", Quantity: 1}, {ItemID: "itm_2", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("NewCombo: %v", err)
	}
	if combo.SortOrder != 0 {
		t.Errorf("sort order = %d, want 0", combo.SortOrder)
	}
}

func TestACombosAvailabilityCanBeScheduled(t *testing.T) {
	combo := newCombo(t, domain.Restaurant, 45_000, []domain.ComboLine{
		{ItemID: "itm_1", Quantity: 1}, {ItemID: "itm_2", Quantity: 1},
	})

	lunch, err := domain.NewAvailability(map[time.Weekday][]schedule.Window{
		time.Monday: {mustWindow(t, "12:00-15:00")},
	})
	if err != nil {
		t.Fatalf("NewAvailability: %v", err)
	}

	scheduled := combo.WithAvailability(lunch)
	if !scheduled.Orderable(at(2026, time.March, 2, 13, 0)) {
		t.Error("the lunch combo is not orderable at 13:00 Monday")
	}
	if scheduled.Orderable(at(2026, time.March, 2, 18, 0)) {
		t.Error("the lunch combo is orderable at 18:00")
	}
}

func newCombo(t *testing.T, kind domain.MerchantType, priceMinor int64, lines []domain.ComboLine) domain.Combo {
	t.Helper()
	combo, err := domain.NewCombo("cmb_1", "mch_1", kind, domain.ComboDraft{
		Name: "Meal deal", Price: money.Taka(priceMinor), Lines: lines,
	})
	if err != nil {
		t.Fatalf("NewCombo: %v", err)
	}
	return combo
}

// TestAnOptionNameHasALimit covers every entry point into the shared name
// check, because each is a place an owner types into.
func TestAnOptionNameHasALimit(t *testing.T) {
	long := longText(121)

	if _, err := domain.NewVariantOption("vop_1", long, money.Taka(0)); !errors.Is(err, domain.ErrNameTooLong) {
		t.Errorf("variant option: error = %v", err)
	}
	if _, err := domain.NewAddOn("aop_1", long, money.Taka(0)); !errors.Is(err, domain.ErrNameTooLong) {
		t.Errorf("add-on: error = %v", err)
	}
	if _, err := domain.NewVariantGroup("vgr_1", long, false, 0, 1,
		[]domain.VariantOption{mustVariant(t, "vop_1", "A", 0)}); !errors.Is(err, domain.ErrNameTooLong) {
		t.Errorf("variant group: error = %v", err)
	}
	if _, err := domain.NewAddOnGroup("agr_1", long, 0, 1,
		[]domain.AddOn{mustAddOn(t, "aop_1", "A", 0)}); !errors.Is(err, domain.ErrNameTooLong) {
		t.Errorf("add-on group: error = %v", err)
	}
}

// TestAnItemBuiltByHandIsStillGuarded. domain.Item has exported fields, so a
// caller can assemble one with a type NewItem would have rejected. The
// capability checks re-read the type rather than trusting that the item got
// here legitimately, which is what stops a hand-built value from acquiring
// options its shop type does not have.
func TestAnItemBuiltByHandIsStillGuarded(t *testing.T) {
	handBuilt := domain.Item{ID: "itm_1", MerchantID: "mch_1", MerchantType: "hardware"}

	group, err := domain.NewVariantGroup("vgr_1", "Size", false, 0, 1,
		[]domain.VariantOption{mustVariant(t, "vop_1", "Large", 0)})
	if err != nil {
		t.Fatalf("NewVariantGroup: %v", err)
	}

	if _, err := handBuilt.WithVariantGroups([]domain.VariantGroup{group}); !errors.Is(err, domain.ErrVariantsNotAllowed) {
		t.Errorf("error = %v, want ErrVariantsNotAllowed", err)
	}
}
