package catalogue

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// The service is what a customer sees. These tests hold two lines: it shows
// only what a customer may see, and every number on it is computed by the
// server (2.9).

func TestTheMenuShowsOnlyWhatACustomerMaySee(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	biryani := h.category(t, "Biryani")
	winter := h.category(t, "Winter specials")

	kacchi := h.item(t, domain.Restaurant, biryani.ID, "Kacchi", 35_000)
	hidden := h.item(t, domain.Restaurant, biryani.ID, "Off menu", 1_000)
	h.item(t, domain.Restaurant, winter.ID, "Haleem", 20_000)

	if _, err := h.items.SetActive(context.Background(), ownerID, shopID, hidden.ID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if _, err := h.categories.SetActive(context.Background(), ownerID, shopID, winter.ID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	menu, err := h.service.Menu(context.Background(), shopID)
	if err != nil {
		t.Fatalf("Menu: %v", err)
	}

	if len(menu.Categories) != 1 || menu.Categories[0].ID != biryani.ID {
		t.Errorf("categories = %+v, want the hidden section gone", menu.Categories)
	}
	if len(menu.Items) != 1 || menu.Items[0].ID != kacchi.ID {
		t.Errorf("items = %+v, want only the shown dish in a shown section", menu.Items)
	}
	if menu.MerchantID != shopID {
		t.Errorf("merchant = %q", menu.MerchantID)
	}
}

// TestHidingASectionTakesItsItemsWithIt, so an owner hiding "Winter specials"
// does not have to hide each dish too.
func TestHidingASectionTakesItsItemsWithIt(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	winter := h.category(t, "Winter specials")
	h.item(t, domain.Restaurant, winter.ID, "Haleem", 20_000)

	if _, err := h.categories.SetActive(context.Background(), ownerID, shopID, winter.ID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	menu, err := h.service.Menu(context.Background(), shopID)
	if err != nil {
		t.Fatalf("Menu: %v", err)
	}
	if len(menu.Items) != 0 {
		t.Errorf("items = %+v, want none from a hidden section", menu.Items)
	}
}

// TestEveryPriceCrossesAsBothANumberAndAString: the client renders the string
// and never computes with the number (2.9).
func TestEveryPriceCrossesAsBothANumberAndAString(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	item := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)

	if _, err := h.options.SetVariantGroups(context.Background(), ownerID, shopID, item.ID,
		[]application.GroupRequest{{
			Name: "Size", Required: true, MinChoices: 1, MaxChoices: 1,
			Options: []application.OptionRequest{
				{Name: "Half", PriceMinor: -10_000},
				{Name: "Full", PriceMinor: 0},
			},
		}}); err != nil {
		t.Fatalf("SetVariantGroups: %v", err)
	}
	if _, err := h.options.SetAddOnGroups(context.Background(), ownerID, shopID, item.ID,
		[]application.GroupRequest{{
			Name: "Extras", MaxChoices: 2,
			Options: []application.OptionRequest{{Name: "Extra egg", PriceMinor: 3_000}},
		}}); err != nil {
		t.Fatalf("SetAddOnGroups: %v", err)
	}

	got, err := h.service.Item(context.Background(), shopID, item.ID)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}

	if got.Price.Minor != 35_000 || got.Price.Display != "৳ 350" || got.Price.Currency != "BDT" {
		t.Errorf("price = %+v", got.Price)
	}
	if len(got.VariantGroups) != 1 || len(got.VariantGroups[0].Options) != 2 {
		t.Fatalf("variant groups = %+v", got.VariantGroups)
	}
	half := got.VariantGroups[0].Options[0]
	if half.Price.Minor != -10_000 || half.Price.Display != "-৳ 100" {
		t.Errorf("a discount variant = %+v", half.Price)
	}
	if len(got.AddOnGroups) != 1 || got.AddOnGroups[0].Options[0].Price.Display != "৳ 30" {
		t.Errorf("add-on groups = %+v", got.AddOnGroups)
	}
	if !got.VariantGroups[0].Required || got.VariantGroups[0].MinChoices != 1 {
		t.Errorf("the required group lost its limits: %+v", got.VariantGroups[0])
	}
}

// TestOptionGroupsComeBackInTheOwnersOrder, not whatever order the store
// happened to return.
func TestOptionGroupsComeBackInTheOwnersOrder(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Pizza")
	item := h.item(t, domain.Restaurant, category.ID, "Margherita", 55_000)

	if _, err := h.options.SetVariantGroups(context.Background(), ownerID, shopID, item.ID,
		[]application.GroupRequest{
			{Name: "Crust", SortOrder: 2, MaxChoices: 1,
				Options: []application.OptionRequest{{Name: "Thin"}}},
			{Name: "Size", SortOrder: 1, MaxChoices: 1,
				Options: []application.OptionRequest{{Name: "Regular"}}},
		}); err != nil {
		t.Fatalf("SetVariantGroups: %v", err)
	}

	got, err := h.service.Item(context.Background(), shopID, item.ID)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if len(got.VariantGroups) != 2 || got.VariantGroups[0].Name != "Size" {
		t.Errorf("groups = %+v, want Size first", got.VariantGroups)
	}
}

func TestAddOnGroupsComeBackInTheOwnersOrder(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Pizza")
	item := h.item(t, domain.Restaurant, category.ID, "Margherita", 55_000)

	if _, err := h.options.SetAddOnGroups(context.Background(), ownerID, shopID, item.ID,
		[]application.GroupRequest{
			{Name: "Drinks", SortOrder: 2, MaxChoices: 1,
				Options: []application.OptionRequest{{Name: "Cola", PriceMinor: 4_000}}},
			{Name: "Toppings", SortOrder: 1, MaxChoices: 2,
				Options: []application.OptionRequest{{Name: "Olives", PriceMinor: 2_000}}},
		}); err != nil {
		t.Fatalf("SetAddOnGroups: %v", err)
	}

	got, err := h.service.Item(context.Background(), shopID, item.ID)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if len(got.AddOnGroups) != 2 || got.AddOnGroups[0].Name != "Toppings" {
		t.Errorf("groups = %+v, want Toppings first", got.AddOnGroups)
	}
}

// TestTheContractSaysWhyAnItemIsNotOrderable, so the app renders the right
// message rather than a generic one.
func TestTheContractSaysWhyAnItemIsNotOrderable(t *testing.T) {
	h := newHarness(t, domain.Grocery)
	category := h.category(t, "Rice")
	item := h.item(t, domain.Grocery, category.ID, "Chal", 7_500) // counted, zero

	got, err := h.service.Item(context.Background(), shopID, item.ID)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if got.Orderable {
		t.Error("a sold-out item is orderable")
	}
	if got.UnavailableReason != "out_of_stock" {
		t.Errorf("reason = %q", got.UnavailableReason)
	}

	if _, err := h.items.SetStock(context.Background(), ownerID, shopID, item.ID, 10); err != nil {
		t.Fatalf("SetStock: %v", err)
	}
	got, err = h.service.Item(context.Background(), shopID, item.ID)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if !got.Orderable || got.UnavailableReason != "" {
		t.Errorf("a restocked item = %+v", got)
	}
}

// TestTheContractCarriesThePerTypeFieldsAndNoOthers.
func TestTheContractCarriesThePerTypeFieldsAndNoOthers(t *testing.T) {
	h := newHarness(t, domain.Pharmacy)
	category := h.category(t, "Painkillers")

	item, err := h.items.Create(context.Background(), ownerID, shopID, application.ItemRequest{
		CategoryID: category.ID, Name: "Napa", PriceMinor: 1_200,
		Unit: "strip", GenericName: "Paracetamol", Strength: "500mg",
		RequiresPrescription: true, Brand: "Beximco",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := h.service.Item(context.Background(), shopID, item.ID)
	if err != nil {
		t.Fatalf("Item: %v", err)
	}
	if got.GenericName != "Paracetamol" || got.Strength != "500mg" || !got.RequiresPrescription {
		t.Errorf("pharmacy fields = %+v", got)
	}
	if got.Unit != "strip" || got.Brand != "Beximco" {
		t.Errorf("shared fields = %+v", got)
	}
	if got.IsVegetarian || got.PreparationMinutes != 0 {
		t.Errorf("a pharmacy item carried restaurant fields: %+v", got)
	}
}

// TestItemsAreReturnedInTheOrderAsked, so a cart can zip the result against its
// own lines without a second index.
func TestItemsAreReturnedInTheOrderAsked(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	first := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)
	second := h.item(t, domain.Restaurant, category.ID, "Morog polao", 28_000)

	got, err := h.service.Items(context.Background(), shopID, []string{second.ID, first.ID})
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if len(got) != 2 || got[0].ID != second.ID || got[1].ID != first.ID {
		t.Errorf("items = %+v, want the order asked for", got)
	}
}

// TestAStaleItemIdIsSkippedRatherThanFailingTheBatch: the caller is a cart
// revalidating lines it has held since before the shop edited its menu, and it
// needs to know which survived.
func TestAStaleItemIDIsSkippedRatherThanFailingTheBatch(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	item := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)

	got, err := h.service.Items(context.Background(), shopID, []string{"itm_gone", item.ID})
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if len(got) != 1 || got[0].ID != item.ID {
		t.Errorf("items = %+v, want the surviving one", got)
	}
}

// --------------------------------------------------------- combo savings

// TestTheServerWorksOutWhatACombosSaves. A client doing this arithmetic is
// exactly what 2.9 forbids.
func TestTheServerWorksOutWhatACombosSaves(t *testing.T) {
	h, _, _, combo := mealDeal(t)

	got, err := h.service.Combo(context.Background(), shopID, combo.ID)
	if err != nil {
		t.Fatalf("Combo: %v", err)
	}

	// 35,000 + 6,000 apart; 38,000 together.
	if got.Price.Minor != 38_000 {
		t.Errorf("price = %+v", got.Price)
	}
	if got.Savings.Minor != 3_000 || got.Savings.Display != "৳ 30" {
		t.Errorf("savings = %+v, want ৳ 30", got.Savings)
	}
	if !got.Orderable {
		t.Error("the combo is not orderable")
	}
	if len(got.Lines) != 2 || got.Lines[0].Name != "Kacchi" {
		t.Errorf("lines = %+v, want the member names filled in", got.Lines)
	}
}

// TestACombosPricedAboveItsPartsSavesNothing, rather than rendering as
// "save -৳40".
func TestACombosPricedAboveItsPartsSavesNothing(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	first := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)
	second := h.item(t, domain.Restaurant, category.ID, "Borhani", 6_000)

	combo, err := h.combos.Create(context.Background(), ownerID, shopID, application.ComboRequest{
		Name: "Bad deal", PriceMinor: 50_000,
		Lines: []application.ComboLineRequest{
			{ItemID: first.ID, Quantity: 1},
			{ItemID: second.ID, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := h.service.Combo(context.Background(), shopID, combo.ID)
	if err != nil {
		t.Fatalf("Combo: %v", err)
	}
	if got.Savings.Minor != 0 {
		t.Errorf("savings = %+v, want nothing", got.Savings)
	}
}

// TestACombosWithASoldOutMemberIsNotOrderable. Letting it be ordered moves the
// disappointment from the menu screen to the doorstep.
func TestACombosWithASoldOutMemberIsNotOrderable(t *testing.T) {
	h := newHarness(t, domain.Grocery)
	category := h.category(t, "Basket")

	rice := h.item(t, domain.Grocery, category.ID, "Chal", 7_500)
	oil := h.item(t, domain.Grocery, category.ID, "Tel", 18_000)
	for _, item := range []domain.Item{rice, oil} {
		if _, err := h.items.SetStock(context.Background(), ownerID, shopID, item.ID, 10); err != nil {
			t.Fatalf("SetStock: %v", err)
		}
	}

	combo, err := h.combos.Create(context.Background(), ownerID, shopID, application.ComboRequest{
		Name: "Weekly basket", PriceMinor: 24_000,
		Lines: []application.ComboLineRequest{
			{ItemID: rice.ID, Quantity: 1},
			{ItemID: oil.ID, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Create combo: %v", err)
	}

	got, err := h.service.Combo(context.Background(), shopID, combo.ID)
	if err != nil {
		t.Fatalf("Combo: %v", err)
	}
	if !got.Orderable {
		t.Fatal("the combo is not orderable while both members are in stock")
	}

	// The oil sells out.
	if _, err := h.items.SetStock(context.Background(), ownerID, shopID, oil.ID, 0); err != nil {
		t.Fatalf("SetStock: %v", err)
	}
	got, err = h.service.Combo(context.Background(), shopID, combo.ID)
	if err != nil {
		t.Fatalf("Combo: %v", err)
	}
	if got.Orderable {
		t.Error("a combo whose member is sold out is still orderable")
	}
}

// TestACombosWithAMemberThatIsGoneIsNotOrderable, and does not crash working
// out the savings for an item it cannot find.
func TestACombosWithAMemberThatIsGoneIsNotOrderable(t *testing.T) {
	h, _, borhani, combo := mealDeal(t)

	if err := h.items.Delete(context.Background(), ownerID, shopID, borhani.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := h.service.Combo(context.Background(), shopID, combo.ID)
	if err != nil {
		t.Fatalf("Combo: %v", err)
	}
	if got.Orderable {
		t.Error("a combo missing a member is orderable")
	}
	// The surviving member still counts toward the saving; the missing one
	// contributes nothing rather than a zero-priced line.
	if got.Savings.Minor != 0 {
		t.Errorf("savings = %+v", got.Savings)
	}
}

func TestTheMenuCarriesItsCombos(t *testing.T) {
	h, _, _, combo := mealDeal(t)

	menu, err := h.service.Menu(context.Background(), shopID)
	if err != nil {
		t.Fatalf("Menu: %v", err)
	}
	if len(menu.Combos) != 1 || menu.Combos[0].ID != combo.ID {
		t.Errorf("combos = %+v", menu.Combos)
	}
	if menu.Combos[0].Savings.Minor != 3_000 {
		t.Errorf("savings = %+v", menu.Combos[0].Savings)
	}
}

// -------------------------------------------------------------- failures

func TestTheServiceReportsWhatIsMissing(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	ctx := context.Background()

	if _, err := h.service.Item(ctx, shopID, "itm_nope"); errs.CodeOf(err) != "item_not_found" {
		t.Errorf("item: code = %q", errs.CodeOf(err))
	}
	if _, err := h.service.Combo(ctx, shopID, "cmb_nope"); errs.CodeOf(err) != "combo_not_found" {
		t.Errorf("combo: code = %q", errs.CodeOf(err))
	}
}

func TestTheServiceReportsAStoreThatIsDown(t *testing.T) {
	ctx := context.Background()

	cases := map[string]struct {
		broken func(*memoryRepo)
		call   func(*harness, domain.Combo) error
	}{
		"menu categories": {
			func(r *memoryRepo) { r.categoriesErr = errStore },
			func(h *harness, _ domain.Combo) error { _, e := h.service.Menu(ctx, shopID); return e },
		},
		"menu items": {
			func(r *memoryRepo) { r.itemsErr = errStore },
			func(h *harness, _ domain.Combo) error { _, e := h.service.Menu(ctx, shopID); return e },
		},
		"menu combos": {
			func(r *memoryRepo) { r.combosErr = errStore },
			func(h *harness, _ domain.Combo) error { _, e := h.service.Menu(ctx, shopID); return e },
		},
		"one item": {
			func(r *memoryRepo) { r.itemErr = errStore },
			func(h *harness, _ domain.Combo) error { _, e := h.service.Item(ctx, shopID, "itm_1"); return e },
		},
		"several items": {
			func(r *memoryRepo) { r.itemsByIDErr = errStore },
			func(h *harness, _ domain.Combo) error {
				_, e := h.service.Items(ctx, shopID, []string{"itm_1"})
				return e
			},
		},
		"one combo": {
			func(r *memoryRepo) { r.comboErr = errStore },
			func(h *harness, c domain.Combo) error { _, e := h.service.Combo(ctx, shopID, c.ID); return e },
		},
		"combo members": {
			func(r *memoryRepo) { r.itemsByIDErr = errStore },
			func(h *harness, c domain.Combo) error { _, e := h.service.Combo(ctx, shopID, c.ID); return e },
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h, _, _, combo := mealDeal(t)
			tc.broken(h.repo)

			if got := errs.KindOf(tc.call(h, combo)); got != errs.KindUnavailable {
				t.Errorf("kind = %v, want unavailable", got)
			}
		})
	}
}

// TestMoneyCrossesTheBoundaryInBothForms covers the exported conversion the
// transport also uses, so the two cannot drift.
func TestMoneyCrossesTheBoundaryInBothForms(t *testing.T) {
	got := application.ToContractMoney(money.Taka(124_050))
	if got.Minor != 124_050 || got.Currency != "BDT" || got.Display != "৳ 1,240.50" {
		t.Errorf("money = %+v", got)
	}
}
