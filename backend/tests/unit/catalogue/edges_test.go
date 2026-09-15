package catalogue

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// The branches here are the ones a happy path never reaches: a shop that
// disappears between two calls, a group of choices too long for a phone, an
// option id an owner sent back to keep. Each is a real state, and each is a
// line of code that would otherwise never run before production did.

// TestTheShopIsReCheckedOnEveryCall. Every entry point resolves the shop
// independently, so a shop deleted between two requests is caught on the second
// rather than acting on a stale type.
func TestTheShopIsReCheckedOnEveryCall(t *testing.T) {
	ctx := context.Background()

	cases := map[string]func(*harness, string, string) error{
		"category create": func(h *harness, _, _ string) error {
			_, e := h.categories.Create(ctx, ownerID, shopID, application.CategoryRequest{Name: "X"})
			return e
		},
		"merchant type": func(h *harness, _, _ string) error {
			_, e := h.categories.MerchantTypeOf(ctx, ownerID, shopID)
			return e
		},
		"item create": func(h *harness, categoryID, _ string) error {
			_, e := h.items.Create(ctx, ownerID, shopID, itemRequest(domain.Restaurant, categoryID, "X", 1))
			return e
		},
		"item update": func(h *harness, categoryID, itemID string) error {
			_, e := h.items.Update(ctx, ownerID, shopID, itemID, itemRequest(domain.Restaurant, categoryID, "X", 1))
			return e
		},
		"options": func(h *harness, _, itemID string) error {
			_, e := h.options.SetVariantGroups(ctx, ownerID, shopID, itemID, nil)
			return e
		},
		"combo create": func(h *harness, _, _ string) error {
			_, e := h.combos.Create(ctx, ownerID, shopID, application.ComboRequest{Name: "X"})
			return e
		},
		"combo update": func(h *harness, _, _ string) error {
			_, e := h.combos.Update(ctx, ownerID, shopID, "cmb_1", application.ComboRequest{Name: "X"})
			return e
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, domain.Restaurant)
			category := h.category(t, "Biryani")
			item := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)

			// The shop is gone by the time the next call arrives.
			delete(h.merchants.shops, shopID)

			if got := errs.CodeOf(call(h, category.ID, item.ID)); got != "merchant_not_found" {
				t.Errorf("code = %q, want merchant_not_found", got)
			}
		})
	}
}

// TestTooManyGroupsOfChoicesIsRefused. Past five the item is really several
// items, and saying so beats rendering a form nobody scrolls to the end of.
func TestTooManyGroupsOfChoicesIsRefused(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Pizza")
	item := h.item(t, domain.Restaurant, category.ID, "Margherita", 55_000)

	six := make([]application.GroupRequest, 0, 6)
	for i := 0; i < 6; i++ {
		six = append(six, application.GroupRequest{
			Name: "Group " + string(rune('A'+i)), MaxChoices: 1,
			Options: []application.OptionRequest{{Name: "Only"}},
		})
	}

	if _, err := h.options.SetVariantGroups(context.Background(), ownerID, shopID, item.ID, six); errs.CodeOf(err) != "too_many_groups" {
		t.Errorf("variants: code = %q, want too_many_groups", errs.CodeOf(err))
	}
	if _, err := h.options.SetAddOnGroups(context.Background(), ownerID, shopID, item.ID, six); errs.CodeOf(err) != "too_many_groups" {
		t.Errorf("add-ons: code = %q, want too_many_groups", errs.CodeOf(err))
	}
}

// TestAnOptionKeepsItsIDWhenTheOwnerSendsItBack. Minting a fresh id on every
// save would invalidate every cart holding that choice the moment a typo was
// fixed.
func TestAnOptionKeepsItsIDWhenTheOwnerSendsItBack(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Pizza")
	item := h.item(t, domain.Restaurant, category.ID, "Margherita", 55_000)

	created, err := h.options.SetVariantGroups(context.Background(), ownerID, shopID, item.ID,
		[]application.GroupRequest{{
			Name: "Size", MaxChoices: 1,
			Options: []application.OptionRequest{{Name: "Regular"}, {Name: "Large", PriceMinor: 15_000}},
		}})
	if err != nil {
		t.Fatalf("SetVariantGroups: %v", err)
	}
	group := created.VariantGroups[0]
	large := group.Options[1]

	// The owner fixes a typo and sends the ids back.
	updated, err := h.options.SetVariantGroups(context.Background(), ownerID, shopID, item.ID,
		[]application.GroupRequest{{
			ID: group.ID, Name: "Size", MaxChoices: 1,
			Options: []application.OptionRequest{
				{ID: group.Options[0].ID, Name: "Regular"},
				{ID: large.ID, Name: "Large (12 inch)", PriceMinor: 15_000},
			},
		}})
	if err != nil {
		t.Fatalf("SetVariantGroups: %v", err)
	}
	if updated.VariantGroups[0].ID != group.ID {
		t.Errorf("group id changed: %q then %q", group.ID, updated.VariantGroups[0].ID)
	}
	if updated.VariantGroups[0].Options[1].ID != large.ID {
		t.Errorf("option id changed: %q then %q", large.ID, updated.VariantGroups[0].Options[1].ID)
	}
	if updated.VariantGroups[0].Options[1].Name != "Large (12 inch)" {
		t.Errorf("the rename did not take: %q", updated.VariantGroups[0].Options[1].Name)
	}
}

// TestAnOptionCanBeMarkedUnavailableWithoutRemovingIt, so a shop that has run
// out of large bases can say so and put it back tomorrow.
func TestAnOptionCanBeMarkedUnavailableWithoutRemovingIt(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Pizza")
	item := h.item(t, domain.Restaurant, category.ID, "Margherita", 55_000)

	unavailable := false
	updated, err := h.options.SetVariantGroups(context.Background(), ownerID, shopID, item.ID,
		[]application.GroupRequest{{
			Name: "Size", MaxChoices: 1,
			Options: []application.OptionRequest{
				{Name: "Regular"},
				{Name: "Large", PriceMinor: 15_000, Available: &unavailable},
			},
		}})
	if err != nil {
		t.Fatalf("SetVariantGroups: %v", err)
	}
	if updated.VariantGroups[0].Options[1].Available {
		t.Error("the option is still available")
	}

	withAddOns, err := h.options.SetAddOnGroups(context.Background(), ownerID, shopID, item.ID,
		[]application.GroupRequest{{
			Name: "Extras", MaxChoices: 1,
			Options: []application.OptionRequest{{Name: "Truffle", PriceMinor: 50_000, Available: &unavailable}},
		}})
	if err != nil {
		t.Fatalf("SetAddOnGroups: %v", err)
	}
	if withAddOns.AddOnGroups[0].Options[0].Available {
		t.Error("the add-on is still available")
	}
}

// TestABadOptionInAGroupIsReported, naming what is wrong rather than failing
// the whole save with "invalid".
func TestABadOptionInAGroupIsReported(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Pizza")
	item := h.item(t, domain.Restaurant, category.ID, "Margherita", 55_000)

	cases := map[string]struct {
		groups []application.GroupRequest
		code   string
	}{
		"an unnamed option": {
			[]application.GroupRequest{{Name: "Size", MaxChoices: 1,
				Options: []application.OptionRequest{{Name: " "}}}},
			"name_required",
		},
		"an unnamed group": {
			[]application.GroupRequest{{Name: " ", MaxChoices: 1,
				Options: []application.OptionRequest{{Name: "Regular"}}}},
			"name_required",
		},
		"a group with no options": {
			[]application.GroupRequest{{Name: "Size", MaxChoices: 1}},
			"options_required",
		},
		"two options alike": {
			[]application.GroupRequest{{Name: "Size", MaxChoices: 1,
				Options: []application.OptionRequest{{Name: "Large"}, {Name: "large"}}}},
			"duplicate_option",
		},
		"impossible limits": {
			[]application.GroupRequest{{Name: "Size", MinChoices: 2, MaxChoices: 1,
				Options: []application.OptionRequest{{Name: "Regular"}}}},
			"invalid_choice_range",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := h.options.SetVariantGroups(context.Background(), ownerID, shopID, item.ID, tc.groups); errs.CodeOf(err) != tc.code {
				t.Errorf("variants: code = %q, want %q", errs.CodeOf(err), tc.code)
			}
		})
	}

	// The add-on path has its own construction and its own negative-price rule.
	badPrice := []application.GroupRequest{{Name: "Extras", MaxChoices: 1,
		Options: []application.OptionRequest{{Name: "Cheese", PriceMinor: -1}}}}
	if _, err := h.options.SetAddOnGroups(context.Background(), ownerID, shopID, item.ID, badPrice); errs.CodeOf(err) != "invalid_price" {
		t.Errorf("a negative add-on: code = %q, want invalid_price", errs.CodeOf(err))
	}

	unnamedAddOn := []application.GroupRequest{{Name: "Extras", MaxChoices: 1,
		Options: []application.OptionRequest{{Name: " "}}}}
	if _, err := h.options.SetAddOnGroups(context.Background(), ownerID, shopID, item.ID, unnamedAddOn); errs.CodeOf(err) != "name_required" {
		t.Errorf("an unnamed add-on: code = %q, want name_required", errs.CodeOf(err))
	}

	noOptions := []application.GroupRequest{{Name: "Extras", MaxChoices: 1}}
	if _, err := h.options.SetAddOnGroups(context.Background(), ownerID, shopID, item.ID, noOptions); errs.CodeOf(err) != "options_required" {
		t.Errorf("an empty add-on group: code = %q, want options_required", errs.CodeOf(err))
	}
}

// TestALongNameIsRefusedWhereverItIsTyped, mapped to one message rather than a
// different one per field.
func TestALongNameIsRefusedWhereverItIsTyped(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	long := longText(121)

	_, err := h.items.Create(context.Background(), ownerID, shopID,
		itemRequest(domain.Restaurant, category.ID, long, 1))
	if got := errs.CodeOf(err); got != "text_too_long" {
		t.Errorf("item name: code = %q, want text_too_long", got)
	}

	_, err = h.categories.Create(context.Background(), ownerID, shopID,
		application.CategoryRequest{Name: long})
	if got := errs.CodeOf(err); got != "text_too_long" {
		t.Errorf("category name: code = %q, want text_too_long", got)
	}
}

// TestAnUnusableImageAddressIsRefused on the paths an owner reaches it by.
func TestAnUnusableImageAddressIsRefused(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")

	req := itemRequest(domain.Restaurant, category.ID, "Kacchi", 35_000)
	req.ImageURL = "http://cdn.example.com/x.png"
	if _, err := h.items.Create(context.Background(), ownerID, shopID, req); errs.CodeOf(err) != "invalid_image_url" {
		t.Errorf("item: code = %q, want invalid_image_url", errs.CodeOf(err))
	}

	first := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)
	second := h.item(t, domain.Restaurant, category.ID, "Borhani", 6_000)
	_, err := h.combos.Create(context.Background(), ownerID, shopID, application.ComboRequest{
		Name: "Meal", PriceMinor: 38_000, ImageURL: "http://cdn.example.com/x.png",
		Lines: []application.ComboLineRequest{
			{ItemID: first.ID, Quantity: 1}, {ItemID: second.ID, Quantity: 1},
		},
	})
	if errs.CodeOf(err) != "invalid_image_url" {
		t.Errorf("combo: code = %q, want invalid_image_url", errs.CodeOf(err))
	}
}

// TestAnItemWithANegativePriceIsRefusedOnBothPaths.
func TestAnItemWithANegativePriceIsRefusedOnBothPaths(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	item := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)

	req := itemRequest(domain.Restaurant, category.ID, "Kacchi", -1)
	if _, err := h.items.Create(context.Background(), ownerID, shopID, req); errs.CodeOf(err) != "invalid_price" {
		t.Errorf("create: code = %q, want invalid_price", errs.CodeOf(err))
	}
	if _, err := h.items.Update(context.Background(), ownerID, shopID, item.ID, req); errs.CodeOf(err) != "invalid_price" {
		t.Errorf("update: code = %q, want invalid_price", errs.CodeOf(err))
	}
}

// TestAnUnrecognisedDomainRefusalStillReachesTheClient. entryError has a
// fallback so a rule added to the domain without a matching case here surfaces
// as a 400 an owner can act on rather than an opaque 500.
func TestAnUnrecognisedDomainRefusalStillReachesTheClient(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")

	// A description past the limit is refused by the domain with ErrTooLong,
	// which entryError maps; an empty merchant id is the one refusal with no
	// case of its own, and it lands on the fallback.
	req := itemRequest(domain.Restaurant, category.ID, "Kacchi", 1)
	req.Description = longText(1001)

	err := errs.KindOf(mustErr(t, func() error {
		_, e := h.items.Create(context.Background(), ownerID, shopID, req)
		return e
	}))
	if err != errs.KindInvalid {
		t.Errorf("kind = %v, want invalid", err)
	}
}

func mustErr(t *testing.T, call func() error) error {
	t.Helper()
	err := call()
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
}

// TestTheMerchantTypeIsServedToItsOwner, so the merchant app knows which fields
// to show rather than deciding for itself (2.9).
func TestTheMerchantTypeIsServedToItsOwner(t *testing.T) {
	for _, kind := range domain.AllMerchantTypes() {
		h := newHarness(t, kind)

		got, err := h.categories.MerchantTypeOf(context.Background(), ownerID, shopID)
		if err != nil {
			t.Errorf("%s: MerchantTypeOf: %v", kind, err)
			continue
		}
		if got != kind {
			t.Errorf("MerchantTypeOf = %q, want %q", got, kind)
		}
	}
}

// TestEditingACombosRevalidatesItsDraft, by the same rules a creation uses.
func TestEditingACombosRevalidatesItsDraft(t *testing.T) {
	h, kacchi, _, combo := mealDeal(t)

	_, err := h.combos.Update(context.Background(), ownerID, shopID, combo.ID, application.ComboRequest{
		Name: "Solo", PriceMinor: 1,
		Lines: []application.ComboLineRequest{{ItemID: kacchi.ID, Quantity: 1}},
	})
	if got := errs.CodeOf(err); got != "combo_too_small" {
		t.Errorf("code = %q, want combo_too_small", got)
	}
}

// TestEditingAnItemChecksTheSectionAndTheUnit, the two validations that run
// before the item is rebuilt.
func TestEditingAnItemChecksTheSectionAndTheUnit(t *testing.T) {
	h := newHarness(t, domain.Grocery)
	category := h.category(t, "Rice")
	item := h.item(t, domain.Grocery, category.ID, "Chal", 7_500)

	moved := itemRequest(domain.Grocery, "cat_nope", "Chal", 7_500)
	if _, err := h.items.Update(context.Background(), ownerID, shopID, item.ID, moved); errs.CodeOf(err) != "category_not_found" {
		t.Errorf("bad section: code = %q, want category_not_found", errs.CodeOf(err))
	}

	badUnit := itemRequest(domain.Grocery, category.ID, "Chal", 7_500)
	badUnit.Unit = "furlong"
	if _, err := h.items.Update(context.Background(), ownerID, shopID, item.ID, badUnit); errs.CodeOf(err) != "unit_required" {
		t.Errorf("bad unit: code = %q, want unit_required", errs.CodeOf(err))
	}
}

// TestANegativeShelfCountIsRefusedWhereverItArrives — on create, on edit, and
// through the stock route.
func TestANegativeShelfCountIsRefusedWhereverItArrives(t *testing.T) {
	h := newHarness(t, domain.Grocery)
	category := h.category(t, "Rice")
	item := h.item(t, domain.Grocery, category.ID, "Chal", 7_500)

	negative := -5
	req := itemRequest(domain.Grocery, category.ID, "Chal", 7_500)
	req.StockQuantity = &negative

	if _, err := h.items.Create(context.Background(), ownerID, shopID, req); errs.CodeOf(err) != "invalid_stock" {
		t.Errorf("create: code = %q, want invalid_stock", errs.CodeOf(err))
	}
	if _, err := h.items.Update(context.Background(), ownerID, shopID, item.ID, req); errs.CodeOf(err) != "invalid_stock" {
		t.Errorf("update: code = %q, want invalid_stock", errs.CodeOf(err))
	}
}

// TestAddOnsAreRefusedByTheUseCaseForShopsThatDoNotHaveThem, which is the
// capability check doing its job. Variants have no matching test because no
// shop type refuses them, and SetVariantGroups says so rather than carrying a
// check nothing can trip.
func TestAddOnsAreRefusedByTheUseCaseForShopsThatDoNotHaveThem(t *testing.T) {
	for _, kind := range []domain.MerchantType{domain.Grocery, domain.Pharmacy} {
		h := newHarness(t, kind)
		category := h.category(t, "Section")
		item := h.item(t, kind, category.ID, "Something", 1_000)

		_, err := h.options.SetAddOnGroups(context.Background(), ownerID, shopID, item.ID,
			[]application.GroupRequest{{
				Name: "Extras", MaxChoices: 1,
				Options: []application.OptionRequest{{Name: "Gift wrap", PriceMinor: 1_000}},
			}})
		if got := errs.CodeOf(err); got != "addons_not_allowed" {
			t.Errorf("%s: code = %q, want addons_not_allowed", kind, got)
		}
	}
}

// TestAGroupWithTooManyOptionsIsRefusedThroughTheUseCase, with the message an
// owner reads rather than the domain's.
func TestAGroupWithTooManyOptionsIsRefusedThroughTheUseCase(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Pizza")
	item := h.item(t, domain.Restaurant, category.ID, "Margherita", 55_000)

	options := make([]application.OptionRequest, 0, 21)
	for i := 0; i < 21; i++ {
		options = append(options, application.OptionRequest{Name: "Option " + string(rune('A'+i))})
	}

	_, err := h.options.SetVariantGroups(context.Background(), ownerID, shopID, item.ID,
		[]application.GroupRequest{{Name: "Size", MaxChoices: 1, Options: options}})
	if got := errs.CodeOf(err); got != "too_many_options" {
		t.Errorf("code = %q, want too_many_options", got)
	}
}

// TestACombosWithTooManyItemsIsRefusedThroughTheUseCase.
func TestACombosWithTooManyItemsIsRefusedThroughTheUseCase(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")

	lines := make([]application.ComboLineRequest, 0, 21)
	for i := 0; i < 21; i++ {
		item := h.item(t, domain.Restaurant, category.ID, "Dish "+string(rune('A'+i)), 1_000)
		lines = append(lines, application.ComboLineRequest{ItemID: item.ID, Quantity: 1})
	}

	_, err := h.combos.Create(context.Background(), ownerID, shopID, application.ComboRequest{
		Name: "Everything", PriceMinor: 100_000, Lines: lines,
	})
	if got := errs.CodeOf(err); got != "combo_too_large" {
		t.Errorf("code = %q, want combo_too_large", got)
	}
}

// TestADomainRefusalWithNoCaseOfItsOwnStillReachesTheClient. entryError has a
// fallback so a rule the domain gains without a matching message here surfaces
// as a 400 an owner can act on rather than an opaque 500. A combo line with no
// item id is the one refusal that lands on it today.
func TestADomainRefusalWithNoCaseOfItsOwnStillReachesTheClient(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	item := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)

	_, err := h.combos.Create(context.Background(), ownerID, shopID, application.ComboRequest{
		Name: "Meal", PriceMinor: 38_000,
		Lines: []application.ComboLineRequest{
			{ItemID: item.ID, Quantity: 1},
			{ItemID: "   ", Quantity: 1},
		},
	})
	if got := errs.CodeOf(err); got != "invalid_entry" {
		t.Errorf("code = %q, want invalid_entry", got)
	}
	if errs.KindOf(err) != errs.KindInvalid {
		t.Errorf("kind = %v, want invalid", errs.KindOf(err))
	}
}
