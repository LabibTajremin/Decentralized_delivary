package catalogue

import (
	"context"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// TestTheShopTypeDecidesWhatAnItemMayCarry, at the use-case level and in the
// wording an owner actually reads. This is the acceptance criterion as it
// reaches the merchant app.
func TestTheShopTypeDecidesWhatAnItemMayCarry(t *testing.T) {
	cases := map[string]struct {
		kind   domain.MerchantType
		mutate func(*application.ItemRequest)
		code   string
	}{
		"a grocery item needs a unit": {
			domain.Grocery, func(r *application.ItemRequest) { r.Unit = "" }, "unit_required",
		},
		"a pharmacy item needs a unit": {
			domain.Pharmacy, func(r *application.ItemRequest) { r.Unit = "" }, "unit_required",
		},
		"a restaurant dish has no unit": {
			domain.Restaurant, func(r *application.ItemRequest) { r.Unit = "kg" }, "unit_not_allowed",
		},
		"only a pharmacy needs a prescription": {
			domain.Grocery, func(r *application.ItemRequest) { r.RequiresPrescription = true }, "prescription_not_allowed",
		},
		"a restaurant cannot need a prescription": {
			domain.Restaurant, func(r *application.ItemRequest) { r.RequiresPrescription = true }, "prescription_not_allowed",
		},
		"an unknown unit": {
			domain.Grocery, func(r *application.ItemRequest) { r.Unit = "furlong" }, "unit_required",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, tc.kind)
			category := h.category(t, "Section")

			req := itemRequest(tc.kind, category.ID, "Something", 12_000)
			tc.mutate(&req)

			_, err := h.items.Create(context.Background(), ownerID, shopID, req)
			if got := errs.CodeOf(err); got != tc.code {
				t.Errorf("code = %q, want %q", got, tc.code)
			}
			if errs.KindOf(err) != errs.KindInvalid {
				t.Errorf("kind = %v, want invalid", errs.KindOf(err))
			}
		})
	}
}

// TestEachShopTypeCanBuildItsOwnItem, the positive side of the same rule.
func TestEachShopTypeCanBuildItsOwnItem(t *testing.T) {
	t.Run("restaurant", func(t *testing.T) {
		h := newHarness(t, domain.Restaurant)
		category := h.category(t, "Biryani")

		item, err := h.items.Create(context.Background(), ownerID, shopID, application.ItemRequest{
			CategoryID: category.ID, Name: "Kacchi", PriceMinor: 35_000,
			IsVegetarian: false, PreparationMinutes: 40,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if item.Attributes.PreparationMinutes != 40 {
			t.Errorf("preparation = %d", item.Attributes.PreparationMinutes)
		}
		if item.Stock.Tracked {
			t.Error("a restaurant dish tracks stock")
		}
	})

	t.Run("grocery", func(t *testing.T) {
		h := newHarness(t, domain.Grocery)
		category := h.category(t, "Rice")

		quantity := 40
		item, err := h.items.Create(context.Background(), ownerID, shopID, application.ItemRequest{
			CategoryID: category.ID, Name: "Miniket chal", PriceMinor: 7_500,
			Unit: "kg", PackSize: "1 kg", Brand: "Pran", StockQuantity: &quantity,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if item.Attributes.Unit != domain.UnitKilogram || item.Attributes.Brand != "Pran" {
			t.Errorf("attributes = %+v", item.Attributes)
		}
		if !item.Stock.Tracked || item.Stock.Quantity != 40 {
			t.Errorf("stock = %+v", item.Stock)
		}
	})

	t.Run("pharmacy", func(t *testing.T) {
		h := newHarness(t, domain.Pharmacy)
		category := h.category(t, "Painkillers")

		item, err := h.items.Create(context.Background(), ownerID, shopID, application.ItemRequest{
			CategoryID: category.ID, Name: "Napa", PriceMinor: 1_200,
			Unit: "strip", GenericName: "Paracetamol", Strength: "500mg",
			RequiresPrescription: false,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if item.Attributes.GenericName != "Paracetamol" || item.Attributes.Strength != "500mg" {
			t.Errorf("attributes = %+v", item.Attributes)
		}
	})
}

func TestAnItemMustBeFiledUnderASectionOfThisShop(t *testing.T) {
	h := newHarness(t, domain.Restaurant)

	_, err := h.items.Create(context.Background(), ownerID, shopID,
		itemRequest(domain.Restaurant, "cat_nope", "Kacchi", 35_000))
	if got := errs.CodeOf(err); got != "category_not_found" {
		t.Errorf("code = %q, want category_not_found", got)
	}

	_, err = h.items.Create(context.Background(), ownerID, shopID,
		itemRequest(domain.Restaurant, "", "Kacchi", 35_000))
	if got := errs.CodeOf(err); got != "category_required" {
		t.Errorf("code = %q, want category_required", got)
	}
}

// TestEditingRevalidatesByTheSameRulesAsCreating: a patch path with its own
// weaker checks is how an item ends up in a shape creation would refuse.
func TestEditingRevalidatesByTheSameRulesAsCreating(t *testing.T) {
	h := newHarness(t, domain.Grocery)
	category := h.category(t, "Rice")
	item := h.item(t, domain.Grocery, category.ID, "Chal", 7_500)

	req := itemRequest(domain.Grocery, category.ID, "Chal", 7_500)
	req.Unit = ""

	_, err := h.items.Update(context.Background(), ownerID, shopID, item.ID, req)
	if got := errs.CodeOf(err); got != "unit_required" {
		t.Errorf("code = %q, want unit_required", got)
	}
}

// TestAnEditThatSaysNothingAboutStockLeavesItAlone. Resetting it would take a
// shop's inventory offline because somebody fixed a typo.
func TestAnEditThatSaysNothingAboutStockLeavesItAlone(t *testing.T) {
	h := newHarness(t, domain.Grocery)
	category := h.category(t, "Rice")

	quantity := 40
	req := itemRequest(domain.Grocery, category.ID, "Chal", 7_500)
	req.StockQuantity = &quantity

	item, err := h.items.Create(context.Background(), ownerID, shopID, req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	renamed := itemRequest(domain.Grocery, category.ID, "Miniket chal", 7_500)
	updated, err := h.items.Update(context.Background(), ownerID, shopID, item.ID, renamed)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Stock.Quantity != 40 {
		t.Errorf("stock = %d, want the shelf count kept", updated.Stock.Quantity)
	}
	if updated.Name != "Miniket chal" {
		t.Errorf("name = %q", updated.Name)
	}
}

// TestAnEditKeepsTheOptionGroupsAndAvailability, for the same reason: an owner
// changing a price has not asked to lose the sizes.
func TestAnEditKeepsTheOptionGroupsAndAvailability(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Pizza")
	item := h.item(t, domain.Restaurant, category.ID, "Margherita", 55_000)

	withSizes, err := h.options.SetVariantGroups(context.Background(), ownerID, shopID, item.ID,
		[]application.GroupRequest{{
			Name: "Size", Required: true, MinChoices: 1, MaxChoices: 1,
			Options: []application.OptionRequest{
				{Name: "Regular", PriceMinor: 0},
				{Name: "Large", PriceMinor: 15_000},
			},
		}})
	if err != nil {
		t.Fatalf("SetVariantGroups: %v", err)
	}
	if len(withSizes.VariantGroups) != 1 {
		t.Fatalf("groups = %d", len(withSizes.VariantGroups))
	}

	hidden, err := h.items.SetActive(context.Background(), ownerID, shopID, item.ID, false)
	if err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if hidden.Active {
		t.Error("the item is still shown")
	}

	repriced, err := h.items.Update(context.Background(), ownerID, shopID, item.ID,
		itemRequest(domain.Restaurant, category.ID, "Margherita", 60_000))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(repriced.VariantGroups) != 1 {
		t.Errorf("the edit lost the sizes: %d groups", len(repriced.VariantGroups))
	}
	if repriced.Active {
		t.Error("the edit put a hidden item back on the menu")
	}
	if repriced.Price.Minor() != 60_000 {
		t.Errorf("price = %s", repriced.Price)
	}
}

func TestSettingAShelfCountThroughTheUseCase(t *testing.T) {
	h := newHarness(t, domain.Grocery)
	category := h.category(t, "Rice")
	item := h.item(t, domain.Grocery, category.ID, "Chal", 7_500)

	stocked, err := h.items.SetStock(context.Background(), ownerID, shopID, item.ID, 25)
	if err != nil {
		t.Fatalf("SetStock: %v", err)
	}
	if stocked.Stock.Quantity != 25 {
		t.Errorf("stock = %d", stocked.Stock.Quantity)
	}

	_, err = h.items.SetStock(context.Background(), ownerID, shopID, item.ID, -1)
	if got := errs.CodeOf(err); got != "invalid_stock" {
		t.Errorf("code = %q, want invalid_stock", got)
	}
}

// TestAKitchenCannotBeGivenAShelfCount through the use case either.
func TestAKitchenCannotBeGivenAShelfCount(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	item := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)

	_, err := h.items.SetStock(context.Background(), ownerID, shopID, item.ID, 5)
	if got := errs.CodeOf(err); got != "stock_not_tracked" {
		t.Errorf("code = %q, want stock_not_tracked", got)
	}
}

func TestSettingAnItemsAvailability(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Breakfast")
	item := h.item(t, domain.Restaurant, category.ID, "Paratha", 3_000)

	scheduled, err := h.items.SetAvailability(context.Background(), ownerID, shopID, item.ID,
		application.AvailabilityRequest{Days: map[string][]string{"1": {"07:00-11:00"}}})
	if err != nil {
		t.Fatalf("SetAvailability: %v", err)
	}
	if scheduled.Availability.Always {
		t.Error("the item is still always available")
	}
	if !scheduled.Orderable(at(2026, time.March, 2, 8, 0)) {
		t.Error("unavailable at 08:00 Monday")
	}
	if scheduled.Orderable(at(2026, time.March, 2, 14, 0)) {
		t.Error("available at 14:00 Monday")
	}

	back, err := h.items.SetAvailability(context.Background(), ownerID, shopID, item.ID,
		application.AvailabilityRequest{Always: true})
	if err != nil {
		t.Fatalf("SetAvailability: %v", err)
	}
	if !back.Availability.Always {
		t.Error("the item did not go back to always available")
	}
}

// TestAskingForAScheduleWithoutOneIsRefused: an empty map decodes to "always",
// which is the opposite of what the caller meant.
func TestAskingForAScheduleWithoutOneIsRefused(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Breakfast")
	item := h.item(t, domain.Restaurant, category.ID, "Paratha", 3_000)

	_, err := h.items.SetAvailability(context.Background(), ownerID, shopID, item.ID,
		application.AvailabilityRequest{Always: false})
	if got := errs.CodeOf(err); got != "availability_required" {
		t.Errorf("code = %q, want availability_required", got)
	}

	_, err = h.items.SetAvailability(context.Background(), ownerID, shopID, item.ID,
		application.AvailabilityRequest{Days: map[string][]string{"1": {"nine to five"}}})
	if got := errs.CodeOf(err); got != "invalid_availability" {
		t.Errorf("code = %q, want invalid_availability", got)
	}
}

// ------------------------------------------------------------- listing

func TestListingFiltersAndPages(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	biryani := h.category(t, "Biryani")
	drinks := h.category(t, "Drinks")

	h.item(t, domain.Restaurant, biryani.ID, "Kacchi", 35_000)
	h.item(t, domain.Restaurant, biryani.ID, "Morog polao", 28_000)
	borhani := h.item(t, domain.Restaurant, drinks.ID, "Borhani", 6_000)

	if _, err := h.items.SetActive(context.Background(), ownerID, shopID, borhani.ID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	cases := map[string]struct {
		req  application.ListRequest
		want int
	}{
		"everything":     {application.ListRequest{}, 3},
		"one section":    {application.ListRequest{CategoryID: biryani.ID}, 2},
		"shown only":     {application.ListRequest{ActiveOnly: true}, 2},
		"by name":        {application.ListRequest{Search: "kacchi"}, 1},
		"name, any case": {application.ListRequest{Search: "KACCHI"}, 1},
		"first page":     {application.ListRequest{Limit: 2}, 2},
		"second page":    {application.ListRequest{Limit: 2, Offset: 2}, 1},
		"past the end":   {application.ListRequest{Limit: 2, Offset: 10}, 0},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			items, err := h.items.List(context.Background(), shopID, tc.req)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(items) != tc.want {
				t.Errorf("found %d items, want %d", len(items), tc.want)
			}
		})
	}
}

func TestANegativeOffsetIsRefused(t *testing.T) {
	h := newHarness(t, domain.Restaurant)

	_, err := h.items.List(context.Background(), shopID, application.ListRequest{Offset: -1})
	if got := errs.CodeOf(err); got != "invalid_page" {
		t.Errorf("code = %q, want invalid_page", got)
	}
}

func TestAnItemThatDoesNotExist(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	ctx := context.Background()

	calls := map[string]func() error{
		"get": func() error { _, e := h.items.Get(ctx, shopID, "itm_nope"); return e },
		"update": func() error {
			_, e := h.items.Update(ctx, ownerID, shopID, "itm_nope", itemRequest(domain.Restaurant, "cat_1", "X", 1))
			return e
		},
		"hide":  func() error { _, e := h.items.SetActive(ctx, ownerID, shopID, "itm_nope", false); return e },
		"stock": func() error { _, e := h.items.SetStock(ctx, ownerID, shopID, "itm_nope", 1); return e },
		"availability": func() error {
			_, e := h.items.SetAvailability(ctx, ownerID, shopID, "itm_nope", application.AvailabilityRequest{Always: true})
			return e
		},
		"delete":   func() error { return h.items.Delete(ctx, ownerID, shopID, "itm_nope") },
		"variants": func() error { _, e := h.options.SetVariantGroups(ctx, ownerID, shopID, "itm_nope", nil); return e },
		"add-ons":  func() error { _, e := h.options.SetAddOnGroups(ctx, ownerID, shopID, "itm_nope", nil); return e },
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if got := errs.CodeOf(call()); got != "item_not_found" {
				t.Errorf("code = %q, want item_not_found", got)
			}
		})
	}
}

// --------------------------------------------------------- bulk update

// TestABulkUpdateIsAllOrNothing. A menu where the first thirty items moved and
// the rest did not is a shop selling at two price lists.
func TestABulkUpdateIsAllOrNothing(t *testing.T) {
	h := newHarness(t, domain.Grocery)
	category := h.category(t, "Rice")
	first := h.item(t, domain.Grocery, category.ID, "Chal", 7_500)
	second := h.item(t, domain.Grocery, category.ID, "Atap chal", 8_000)

	newPrice := int64(9_000)
	_, err := h.items.BulkUpdate(context.Background(), ownerID, shopID, []application.BulkChange{
		{ItemID: first.ID, PriceMinor: &newPrice},
		{ItemID: "itm_nope", PriceMinor: &newPrice},
	})
	if got := errs.CodeOf(err); got != "item_not_found" {
		t.Fatalf("code = %q, want item_not_found", got)
	}

	// Nothing was written, including the line that was fine.
	unchanged, err := h.items.Get(context.Background(), shopID, first.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if unchanged.Price.Minor() != 7_500 {
		t.Errorf("price = %s, want the whole request refused", unchanged.Price)
	}
	_ = second
}

func TestABulkUpdateChangesPricesStockAndVisibility(t *testing.T) {
	h := newHarness(t, domain.Grocery)
	category := h.category(t, "Rice")
	first := h.item(t, domain.Grocery, category.ID, "Chal", 7_500)
	second := h.item(t, domain.Grocery, category.ID, "Atap chal", 8_000)

	price := int64(9_000)
	stock := 60
	hidden := false

	updated, err := h.items.BulkUpdate(context.Background(), ownerID, shopID, []application.BulkChange{
		{ItemID: first.ID, PriceMinor: &price, Stock: &stock},
		{ItemID: second.ID, Active: &hidden},
	})
	if err != nil {
		t.Fatalf("BulkUpdate: %v", err)
	}
	if len(updated) != 2 {
		t.Fatalf("updated = %d", len(updated))
	}
	if updated[0].Price.Minor() != 9_000 || updated[0].Stock.Quantity != 60 {
		t.Errorf("first = price %s stock %d", updated[0].Price, updated[0].Stock.Quantity)
	}
	if updated[1].Active {
		t.Error("the second item is still shown")
	}
}

func TestABulkUpdateHasLimits(t *testing.T) {
	h := newHarness(t, domain.Grocery)

	_, err := h.items.BulkUpdate(context.Background(), ownerID, shopID, nil)
	if got := errs.CodeOf(err); got != "no_changes" {
		t.Errorf("empty: code = %q, want no_changes", got)
	}

	tooMany := make([]application.BulkChange, 201)
	_, err = h.items.BulkUpdate(context.Background(), ownerID, shopID, tooMany)
	if got := errs.CodeOf(err); got != "too_many_changes" {
		t.Errorf("too many: code = %q, want too_many_changes", got)
	}
}

func TestABulkUpdateValidatesEachChange(t *testing.T) {
	h := newHarness(t, domain.Grocery)
	category := h.category(t, "Rice")
	item := h.item(t, domain.Grocery, category.ID, "Chal", 7_500)

	negativePrice := int64(-1)
	negativeStock := -5

	cases := map[string]struct {
		change application.BulkChange
		code   string
	}{
		"negative price": {application.BulkChange{ItemID: item.ID, PriceMinor: &negativePrice}, "invalid_price"},
		"negative stock": {application.BulkChange{ItemID: item.ID, Stock: &negativeStock}, "invalid_stock"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := h.items.BulkUpdate(context.Background(), ownerID, shopID,
				[]application.BulkChange{tc.change})
			if got := errs.CodeOf(err); got != tc.code {
				t.Errorf("code = %q, want %q", got, tc.code)
			}
		})
	}
}

// TestABulkUpdateCannotGiveAKitchenAShelfCount, because the stock policy is a
// property of the shop type and not of the route used to reach it.
func TestABulkUpdateCannotGiveAKitchenAShelfCount(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	item := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)

	stock := 5
	_, err := h.items.BulkUpdate(context.Background(), ownerID, shopID,
		[]application.BulkChange{{ItemID: item.ID, Stock: &stock}})
	if got := errs.CodeOf(err); got != "stock_not_tracked" {
		t.Errorf("code = %q, want stock_not_tracked", got)
	}
}

func TestItemWritesReportAStoreThatIsDown(t *testing.T) {
	ctx := context.Background()

	cases := map[string]struct {
		broken func(*memoryRepo)
		call   func(*harness, string, string) error
	}{
		"list": {
			func(r *memoryRepo) { r.itemsErr = errStore },
			func(h *harness, _, _ string) error {
				_, e := h.items.List(ctx, shopID, application.ListRequest{})
				return e
			},
		},
		"read one": {
			func(r *memoryRepo) { r.itemErr = errStore },
			func(h *harness, _, itemID string) error { _, e := h.items.Get(ctx, shopID, itemID); return e },
		},
		"category check": {
			func(r *memoryRepo) { r.categoryErr = errStore },
			func(h *harness, categoryID, _ string) error {
				_, e := h.items.Create(ctx, ownerID, shopID, itemRequest(domain.Restaurant, categoryID, "X", 1))
				return e
			},
		},
		"save": {
			func(r *memoryRepo) { r.saveItemErr = errStore },
			func(h *harness, categoryID, _ string) error {
				_, e := h.items.Create(ctx, ownerID, shopID, itemRequest(domain.Restaurant, categoryID, "X", 1))
				return e
			},
		},
		"save on hide": {
			func(r *memoryRepo) { r.saveItemErr = errStore },
			func(h *harness, _, itemID string) error {
				_, e := h.items.SetActive(ctx, ownerID, shopID, itemID, false)
				return e
			},
		},
		"delete": {
			func(r *memoryRepo) { r.deleteItemErr = errStore },
			func(h *harness, _, itemID string) error { return h.items.Delete(ctx, ownerID, shopID, itemID) },
		},
		"bulk read": {
			func(r *memoryRepo) { r.itemsByIDErr = errStore },
			func(h *harness, _, itemID string) error {
				_, e := h.items.BulkUpdate(ctx, ownerID, shopID, []application.BulkChange{{ItemID: itemID}})
				return e
			},
		},
		"bulk write": {
			func(r *memoryRepo) { r.saveItemsErr = errStore },
			func(h *harness, _, itemID string) error {
				_, e := h.items.BulkUpdate(ctx, ownerID, shopID, []application.BulkChange{{ItemID: itemID}})
				return e
			},
		},
		"options write": {
			func(r *memoryRepo) { r.saveItemErr = errStore },
			func(h *harness, _, itemID string) error {
				_, e := h.options.SetVariantGroups(ctx, ownerID, shopID, itemID, nil)
				return e
			},
		},
		"options read": {
			func(r *memoryRepo) { r.itemErr = errStore },
			func(h *harness, _, itemID string) error {
				_, e := h.options.SetAddOnGroups(ctx, ownerID, shopID, itemID, nil)
				return e
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, domain.Restaurant)
			category := h.category(t, "Biryani")
			item := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)
			tc.broken(h.repo)

			err := tc.call(h, category.ID, item.ID)
			if errs.KindOf(err) != errs.KindUnavailable {
				t.Errorf("kind = %v, want unavailable", errs.KindOf(err))
			}
		})
	}
}

// TestUpdateAlsoFailsWhenTheStoreIsDown covers the save on the edit path, which
// the create path's failure does not reach.
func TestUpdateAlsoFailsWhenTheStoreIsDown(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	item := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)
	h.repo.saveItemErr = errStore

	_, err := h.items.Update(context.Background(), ownerID, shopID, item.ID,
		itemRequest(domain.Restaurant, category.ID, "Kacchi", 36_000))
	if errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("kind = %v, want unavailable", errs.KindOf(err))
	}
}

// TestSettingStockOnAMissingItemReportsTheStore covers the save failure behind
// SetStock rather than the lookup.
func TestSettingStockOnABrokenStore(t *testing.T) {
	h := newHarness(t, domain.Grocery)
	category := h.category(t, "Rice")
	item := h.item(t, domain.Grocery, category.ID, "Chal", 7_500)
	h.repo.saveItemErr = errStore

	if _, err := h.items.SetStock(context.Background(), ownerID, shopID, item.ID, 5); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("stock: kind = %v, want unavailable", errs.KindOf(err))
	}
	if _, err := h.items.SetAvailability(context.Background(), ownerID, shopID, item.ID,
		application.AvailabilityRequest{Always: true}); errs.KindOf(err) != errs.KindUnavailable {
		t.Errorf("availability: kind = %v, want unavailable", errs.KindOf(err))
	}
}

// TestTheUseCaseExposesTheClockItDecidesAgainst, so transport renders
// "orderable" against the same instant the rules used rather than its own.
func TestTheUseCaseExposesTheClockItDecidesAgainst(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	if !h.items.Now().Equal(h.clock.Now()) {
		t.Errorf("Now() = %v, want the injected clock", h.items.Now())
	}
}
