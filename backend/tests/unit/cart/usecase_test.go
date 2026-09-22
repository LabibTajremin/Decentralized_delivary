package cart

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	catcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/contract"
	merchantcontract "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/contract"
	pricingapp "github.com/rootlogic-lab/delivery/backend/internal/modules/pricing/application"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// rig is one assembled cart with every collaborator reachable for assertions.
type rig struct {
	repo      *fakeRepo
	catalogue *fakeCatalogue
	merchant  *fakeMerchant
	discovery *fakeDiscovery
	pricing   *fakeConfig
	carts     *application.CartUseCase
	service   *application.Service
}

func newRig() *rig {
	repo := newRepo()
	cat := newCatalogue()
	shop := &fakeMerchant{shop: openShop()}
	disco := &fakeDiscovery{reach: contract.Reach{Reachable: true, DivisionCode: "DHA"}}
	// The real pricing service over a fake config, not a stubbed quoter. The
	// thing most worth asserting is that the cart's total is the one ALG-05
	// produces — a stub would agree with itself and with nothing else.
	prices := &fakeConfig{settings: appendixB()}
	uc := application.NewCartUseCase(repo, cat, shop, disco,
		pricingapp.NewService(prices), &clock.Fixed{}, &fakeIDs{})
	return &rig{repo: repo, catalogue: cat, merchant: shop, discovery: disco, pricing: prices,
		carts: uc, service: application.NewService(uc)}
}

// withBurger is a rig whose shop sells one plain item.
func withBurger() *rig {
	r := newRig()
	r.catalogue.items["ITM-burger"] = orderableItem("ITM-burger", "Beef Burger", 25000)
	return r
}

func add(t *testing.T, r *rig, targetID string, quantity int, choices ...application.Choice) application.View {
	t.Helper()
	view, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-1", Kind: "item", TargetID: targetID, Quantity: quantity,
		Choices: choices, Lang: "en",
	})
	if err != nil {
		t.Fatalf("Add %s: %v", targetID, err)
	}
	return view
}

func TestAddOpensACartAndPricesItFromTheCatalogue(t *testing.T) {
	r := withBurger()
	view := add(t, r, "ITM-burger", 2)

	if view.MerchantID != "MER-1" || view.MerchantName != "Star Kabab" {
		t.Fatalf("view = %+v", view)
	}
	if len(view.Lines) != 1 || view.Lines[0].Name != "Beef Burger" {
		t.Fatalf("lines = %+v", view.Lines)
	}
	// The price came from the catalogue, not the request.
	if view.Lines[0].UnitPrice.Minor != 25000 || view.Lines[0].LineTotal.Minor != 50000 {
		t.Errorf("prices = %+v", view.Lines[0])
	}
	if view.Subtotal.Minor != 50000 || view.Subtotal.Display == "" {
		t.Errorf("subtotal = %+v", view.Subtotal)
	}
	if view.Count != 2 {
		t.Errorf("count = %d, want 2", view.Count)
	}
	// No address yet, so there is nothing true to say about reachability.
	if view.Blocker != string(domain.BlockerNoAddress) || view.Orderable {
		t.Errorf("blocker = %q, orderable = %v", view.Blocker, view.Orderable)
	}
	if view.BlockerText == "" {
		t.Error("the blocker has no sentence")
	}
	// Discovery is not asked about a cart with no address: there is nothing to
	// ask about.
	if len(r.discovery.asked) != 0 {
		t.Errorf("discovery was asked about an address-less cart: %v", r.discovery.asked)
	}
}

// The single-merchant rule, refused rather than resolved.
func TestAddingFromAnotherShopIsRefused(t *testing.T) {
	r := withBurger()
	add(t, r, "ITM-burger", 1)

	_, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-2", Kind: "item", TargetID: "ITM-burger", Quantity: 1,
	})
	if !errs.Is(err, errs.KindConflict) || errs.CodeOf(err) != "different_merchant" {
		t.Fatalf("err = %v, want a different_merchant conflict", err)
	}
	// Nothing was destroyed on the way to refusing.
	cart, found, _ := r.repo.OfUser(context.Background(), "USR-1")
	if !found || len(cart.Lines) != 1 {
		t.Errorf("the refused add changed the cart: %+v", cart)
	}
}

// Replace is the deliberate version of what Add refuses to do quietly.
func TestReplaceStartsANewCartAtAnotherShop(t *testing.T) {
	r := withBurger()
	add(t, r, "ITM-burger", 1)
	r.catalogue.items["ITM-rice"] = orderableItem("ITM-rice", "Miniket Rice", 7000)
	r.merchant.shop.ID = "MER-2"

	view, err := r.carts.Replace(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-2", Kind: "item", TargetID: "ITM-rice", Quantity: 1, Lang: "en",
	})
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if view.MerchantID != "MER-2" || len(view.Lines) != 1 || view.Lines[0].TargetID != "ITM-rice" {
		t.Fatalf("view = %+v", view)
	}
	if r.repo.deletes != 1 {
		t.Errorf("the old cart was not deleted: %d deletes", r.repo.deletes)
	}
}

// Replace with no existing cart is just an add.
func TestReplaceWithNoCart(t *testing.T) {
	r := withBurger()
	view, err := r.carts.Replace(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-1", Kind: "item", TargetID: "ITM-burger", Quantity: 1,
	})
	if err != nil {
		t.Fatalf("Replace: %v", err)
	}
	if len(view.Lines) != 1 || r.repo.deletes != 0 {
		t.Fatalf("view = %+v, deletes = %d", view, r.repo.deletes)
	}
}

func TestAddResolvesOptionsAndEnforcesTheGroupRules(t *testing.T) {
	r := newRig()
	r.catalogue.items["ITM-pizza"] = catcontract.Item{
		ID: "ITM-pizza", MerchantID: "MER-1", Name: "Pizza",
		Price: taka(50000), Orderable: true,
		VariantGroups: []catcontract.OptionGroup{{
			ID: "GRP-size", Name: "Size", Required: true, MinChoices: 1, MaxChoices: 1,
			Options: []catcontract.Option{
				{ID: "OPT-small", Name: "Small", Price: taka(0), Available: true},
				{ID: "OPT-large", Name: "Large", Price: taka(15000), Available: true},
				{ID: "OPT-family", Name: "Family", Price: taka(30000)},
			},
		}},
		AddOnGroups: []catcontract.OptionGroup{{
			ID: "GRP-extras", Name: "Extras", MaxChoices: 2,
			Options: []catcontract.Option{
				{ID: "OPT-cheese", Name: "Extra cheese", Price: taka(5000), Available: true},
				{ID: "OPT-olives", Name: "Olives", Price: taka(3000), Available: true},
				{ID: "OPT-corn", Name: "Corn", Price: taka(3000), Available: true},
			},
		}},
	}

	view := add(t, r, "ITM-pizza", 1,
		application.Choice{GroupID: "GRP-size", OptionID: "OPT-large"},
		application.Choice{GroupID: "GRP-extras", OptionID: "OPT-cheese"})
	// 500 + 150 + 50 = 700 taka.
	if view.Lines[0].UnitPrice.Minor != 70000 {
		t.Fatalf("unit price = %d, want 70000", view.Lines[0].UnitPrice.Minor)
	}
	if len(view.Lines[0].Options) != 2 {
		t.Errorf("options = %+v", view.Lines[0].Options)
	}

	cases := []struct {
		name     string
		choices  []application.Choice
		wantCode string
	}{
		{"no size chosen", nil, "option_required"},
		{"unknown group", []application.Choice{{GroupID: "GRP-nope", OptionID: "OPT-large"}}, "unknown_option_group"},
		{"unknown option", []application.Choice{{GroupID: "GRP-size", OptionID: "OPT-nope"}}, "unknown_option"},
		{"an option the shop turned off", []application.Choice{{GroupID: "GRP-size", OptionID: "OPT-family"}}, "option_unavailable"},
		{"the same choice twice", []application.Choice{
			{GroupID: "GRP-size", OptionID: "OPT-large"},
			{GroupID: "GRP-size", OptionID: "OPT-large"},
		}, "duplicate_option"},
		{"two sizes", []application.Choice{
			{GroupID: "GRP-size", OptionID: "OPT-large"},
			{GroupID: "GRP-size", OptionID: "OPT-small"},
		}, "too_many_options"},
		{"too many extras", []application.Choice{
			{GroupID: "GRP-size", OptionID: "OPT-large"},
			{GroupID: "GRP-extras", OptionID: "OPT-cheese"},
			{GroupID: "GRP-extras", OptionID: "OPT-olives"},
			{GroupID: "GRP-extras", OptionID: "OPT-corn"},
		}, "too_many_options"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fresh := newRig()
			fresh.catalogue.items = r.catalogue.items
			_, err := fresh.carts.Add(context.Background(), "USR-1", application.AddRequest{
				MerchantID: "MER-1", Kind: "item", TargetID: "ITM-pizza", Quantity: 1, Choices: tc.choices,
			})
			if errs.CodeOf(err) != tc.wantCode {
				t.Fatalf("err = %v, want %s", err, tc.wantCode)
			}
		})
	}
}

// An optional group's minimum applies to a customer who is using it, not to
// one who ignored it.
func TestAnOptionalGroupWithAMinimumCanBeSkipped(t *testing.T) {
	r := newRig()
	r.catalogue.items["ITM-salad"] = catcontract.Item{
		ID: "ITM-salad", MerchantID: "MER-1", Name: "Salad",
		Price: taka(20000), Orderable: true,
		AddOnGroups: []catcontract.OptionGroup{{
			ID: "GRP-dressing", Name: "Dressings", MinChoices: 2, MaxChoices: 3,
			Options: []catcontract.Option{
				{ID: "OPT-a", Name: "A", Price: taka(0), Available: true},
				{ID: "OPT-b", Name: "B", Price: taka(0), Available: true},
			},
		}},
	}

	if view := add(t, r, "ITM-salad", 1); len(view.Lines) != 1 {
		t.Fatalf("skipping an optional group was refused: %+v", view)
	}

	// Using it, though, means using it properly.
	fresh := newRig()
	fresh.catalogue.items = r.catalogue.items
	_, err := fresh.carts.Add(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-1", Kind: "item", TargetID: "ITM-salad", Quantity: 1,
		Choices: []application.Choice{{GroupID: "GRP-dressing", OptionID: "OPT-a"}},
	})
	if errs.CodeOf(err) != "too_few_options" {
		t.Fatalf("err = %v, want too_few_options", err)
	}
}

func TestAddACombo(t *testing.T) {
	r := newRig()
	r.catalogue.combos["CMB-1"] = catcontract.Combo{
		ID: "CMB-1", MerchantID: "MER-1", Name: "Family Meal",
		Price: taka(120000), Orderable: true,
	}

	view, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-1", Kind: "combo", TargetID: "CMB-1", Quantity: 1, Lang: "en",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if view.Lines[0].Kind != "combo" || view.Lines[0].UnitPrice.Minor != 120000 {
		t.Fatalf("line = %+v", view.Lines[0])
	}

	// A bundle comes as it is. Accepting choices and ignoring them would be
	// worse than refusing them.
	_, err = r.carts.Add(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-1", Kind: "combo", TargetID: "CMB-1", Quantity: 1,
		Choices: []application.Choice{{GroupID: "G", OptionID: "O"}},
	})
	if errs.CodeOf(err) != "combo_has_no_options" {
		t.Fatalf("err = %v, want combo_has_no_options", err)
	}
}

func TestAddRefusals(t *testing.T) {
	r := withBurger()

	t.Run("an unknown kind", func(t *testing.T) {
		_, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
			MerchantID: "MER-1", Kind: "subscription", TargetID: "ITM-burger", Quantity: 1,
		})
		if errs.CodeOf(err) != "invalid_line" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a quantity of zero", func(t *testing.T) {
		_, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
			MerchantID: "MER-1", Kind: "item", TargetID: "ITM-burger", Quantity: 0,
		})
		if errs.CodeOf(err) != "invalid_quantity" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("no such item", func(t *testing.T) {
		_, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
			MerchantID: "MER-1", Kind: "item", TargetID: "ITM-gone", Quantity: 1,
		})
		if !errs.Is(err, errs.KindNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("an item the shop has taken down", func(t *testing.T) {
		fresh := newRig()
		down := orderableItem("ITM-down", "Sold out", 10000)
		down.Orderable = false
		down.UnavailableReason = "out_of_stock"
		fresh.catalogue.items["ITM-down"] = down
		_, err := fresh.carts.Add(context.Background(), "USR-1", application.AddRequest{
			MerchantID: "MER-1", Kind: "item", TargetID: "ITM-down", Quantity: 1,
		})
		if errs.CodeOf(err) != "not_orderable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a bundle the shop has taken down", func(t *testing.T) {
		fresh := newRig()
		fresh.catalogue.combos["CMB-down"] = catcontract.Combo{ID: "CMB-down", Name: "Gone", Price: taka(100)}
		_, err := fresh.carts.Add(context.Background(), "USR-1", application.AddRequest{
			MerchantID: "MER-1", Kind: "combo", TargetID: "CMB-down", Quantity: 1,
		})
		if errs.CodeOf(err) != "not_orderable" {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestSetQuantityRemoveAndClear(t *testing.T) {
	r := withBurger()
	view := add(t, r, "ITM-burger", 1)
	lineID := view.Lines[0].ID

	view, err := r.carts.SetQuantity(context.Background(), "USR-1", lineID, 4, "en")
	if err != nil {
		t.Fatalf("SetQuantity: %v", err)
	}
	if view.Lines[0].Quantity != 4 || view.Count != 4 {
		t.Fatalf("view = %+v", view)
	}

	view, err = r.carts.Remove(context.Background(), "USR-1", lineID, "en")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(view.Lines) != 0 || view.Blocker != string(domain.BlockerEmpty) {
		t.Fatalf("view = %+v", view)
	}

	if err := r.carts.Clear(context.Background(), "USR-1"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, found, _ := r.repo.OfUser(context.Background(), "USR-1"); found {
		t.Error("Clear left the cart behind")
	}
	// Clearing a cart that is not there is not an error. The customer's intent
	// is satisfied either way.
	if err := r.carts.Clear(context.Background(), "USR-1"); err != nil {
		t.Errorf("clearing nothing failed: %v", err)
	}
}

func TestOperationsOnACartThatDoesNotExist(t *testing.T) {
	r := withBurger()
	ctx := context.Background()

	if _, _, err := r.carts.Current(ctx, "USR-1", "en"); err != nil {
		t.Fatalf("Current: %v", err)
	}
	if _, found, _ := r.carts.Current(ctx, "USR-1", "en"); found {
		t.Error("a customer with no cart was reported as having one")
	}

	for name, run := range map[string]func() error{
		"set quantity": func() error {
			_, err := r.carts.SetQuantity(ctx, "USR-1", "CLN-1", 2, "en")
			return err
		},
		"remove": func() error {
			_, err := r.carts.Remove(ctx, "USR-1", "CLN-1", "en")
			return err
		},
		"set address": func() error {
			_, err := r.carts.SetAddress(ctx, "USR-1", "ADR-1", 23.7, 90.4, "en")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); errs.CodeOf(err) != "cart_not_found" {
				t.Fatalf("err = %v, want cart_not_found", err)
			}
		})
	}
}

// The acceptance criterion: a cart invalidates when the address changes.
func TestChangingTheAddressRevalidatesTheCart(t *testing.T) {
	r := withBurger()
	add(t, r, "ITM-burger", 1)

	view, err := r.carts.SetAddress(context.Background(), "USR-1", "ADR-1", 23.7465, 90.3760, "en")
	if err != nil {
		t.Fatalf("SetAddress: %v", err)
	}
	if !view.Orderable || view.Blocker != "" {
		t.Fatalf("a reachable address left the cart blocked: %+v", view)
	}
	if len(r.discovery.asked) != 1 || r.discovery.asked[0].Lat != 23.7465 {
		t.Fatalf("discovery was asked about %v", r.discovery.asked)
	}

	// The customer switches to an address in another division. D3 makes that
	// permanent, and they find out now rather than at checkout.
	r.discovery.reach = contract.Reach{Reason: contract.ReasonOutsideDivision}
	view, err = r.carts.SetAddress(context.Background(), "USR-1", "ADR-2", 22.3284, 91.8118, "en")
	if err != nil {
		t.Fatalf("SetAddress: %v", err)
	}
	if view.Orderable || view.Blocker != string(domain.BlockerOutsideDivision) {
		t.Fatalf("a cross-division address was still orderable: %+v", view)
	}
	if view.BlockerText == "" {
		t.Error("no sentence explaining the block")
	}
	// Re-asked on the second change, not cached from the first.
	if len(r.discovery.asked) != 2 || r.discovery.asked[1].Lat != 22.3284 {
		t.Errorf("discovery was asked about %v", r.discovery.asked)
	}
}

func TestSetAddressNeedsAnAddress(t *testing.T) {
	r := withBurger()
	add(t, r, "ITM-burger", 1)
	if _, err := r.carts.SetAddress(context.Background(), "USR-1", "", 0, 0, "en"); errs.CodeOf(err) != "invalid_address" {
		t.Fatalf("err = %v, want invalid_address", err)
	}
}

// Revalidation: one batched read for all the item lines, not one per line.
func TestRevalidationBatchesTheMenuRead(t *testing.T) {
	r := newRig()
	for _, id := range []string{"ITM-1", "ITM-2", "ITM-3"} {
		r.catalogue.items[id] = orderableItem(id, id, 10000)
		add(t, r, id, 1)
	}
	before := r.catalogue.batches
	if _, _, err := r.carts.Current(context.Background(), "USR-1", "en"); err != nil {
		t.Fatalf("Current: %v", err)
	}
	if r.catalogue.batches-before != 1 {
		t.Errorf("revalidating three lines made %d menu reads, want one", r.catalogue.batches-before)
	}
}

func TestRevalidationReportsWhatTheShopChanged(t *testing.T) {
	r := withBurger()
	view := add(t, r, "ITM-burger", 2)
	if view.Lines[0].Issue != "" {
		t.Fatalf("a fresh line already had an issue: %+v", view.Lines[0])
	}

	t.Run("the price went up", func(t *testing.T) {
		dearer := orderableItem("ITM-burger", "Beef Burger", 28000)
		r.catalogue.items["ITM-burger"] = dearer

		got, _, err := r.carts.Current(context.Background(), "USR-1", "en")
		if err != nil {
			t.Fatalf("Current: %v", err)
		}
		if got.Lines[0].Issue != string(domain.IssuePriceChanged) {
			t.Fatalf("issue = %q", got.Lines[0].Issue)
		}
		if got.Lines[0].IssueText != "The price has changed" {
			t.Errorf("issueText = %q", got.Lines[0].IssueText)
		}
		// Still orderable, at the new price, and the subtotal follows.
		if !got.Lines[0].Orderable || got.Subtotal.Minor != 56000 {
			t.Errorf("view = %+v", got)
		}
	})

	t.Run("the shop deleted it", func(t *testing.T) {
		delete(r.catalogue.items, "ITM-burger")
		got, _, err := r.carts.Current(context.Background(), "USR-1", "en")
		if err != nil {
			t.Fatalf("Current: %v", err)
		}
		if got.Lines[0].Issue != string(domain.IssueRemoved) || got.Lines[0].Orderable {
			t.Fatalf("line = %+v", got.Lines[0])
		}
		// A line nobody can order contributes nothing to the total.
		if got.Subtotal.Minor != 0 {
			t.Errorf("subtotal = %d, want 0", got.Subtotal.Minor)
		}
	})
}

// A variant the customer chose has gone. The line cannot be priced without the
// choice they made, so it is not orderable — repricing it without the choice
// would be quietly changing their order.
func TestAChosenOptionDisappearing(t *testing.T) {
	r := newRig()
	r.catalogue.items["ITM-pizza"] = catcontract.Item{
		ID: "ITM-pizza", MerchantID: "MER-1", Name: "Pizza", Price: taka(50000), Orderable: true,
		VariantGroups: []catcontract.OptionGroup{{
			ID: "GRP-size", Name: "Size", Required: true, MinChoices: 1, MaxChoices: 1,
			Options: []catcontract.Option{{ID: "OPT-large", Name: "Large", Price: taka(15000), Available: true}},
		}},
	}
	add(t, r, "ITM-pizza", 1, application.Choice{GroupID: "GRP-size", OptionID: "OPT-large"})

	withoutLarge := r.catalogue.items["ITM-pizza"]
	withoutLarge.VariantGroups[0].Options = nil
	r.catalogue.items["ITM-pizza"] = withoutLarge

	got, _, err := r.carts.Current(context.Background(), "USR-1", "en")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if got.Lines[0].Issue != string(domain.IssueUnavailable) || got.Lines[0].Orderable {
		t.Fatalf("line = %+v", got.Lines[0])
	}
}

// A combo that has been deleted reads as "removed" rather than as an outage.
// The customer's answer is the same either way, and failing the whole cart read
// because one line vanished would take the rest of the cart with it.
func TestACombosDisappearanceDoesNotFailTheWholeCart(t *testing.T) {
	r := newRig()
	r.catalogue.combos["CMB-1"] = catcontract.Combo{ID: "CMB-1", Name: "Meal", Price: taka(100000), Orderable: true}
	r.catalogue.items["ITM-1"] = orderableItem("ITM-1", "Coke", 3000)

	if _, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-1", Kind: "combo", TargetID: "CMB-1", Quantity: 1,
	}); err != nil {
		t.Fatalf("Add combo: %v", err)
	}
	add(t, r, "ITM-1", 1)

	delete(r.catalogue.combos, "CMB-1")
	got, _, err := r.carts.Current(context.Background(), "USR-1", "en")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if len(got.Lines) != 2 {
		t.Fatalf("lines = %d, want both still shown", len(got.Lines))
	}
	if got.Lines[0].Issue != string(domain.IssueRemoved) {
		t.Errorf("the combo line = %+v", got.Lines[0])
	}
	if got.Lines[1].Issue != "" || !got.Lines[1].Orderable {
		t.Errorf("the surviving line was affected: %+v", got.Lines[1])
	}
}

// A combo the shop turned off, as distinct from one it deleted.
func TestACombosBeingTurnedOff(t *testing.T) {
	r := newRig()
	r.catalogue.combos["CMB-1"] = catcontract.Combo{ID: "CMB-1", Name: "Meal", Price: taka(100000), Orderable: true}
	if _, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-1", Kind: "combo", TargetID: "CMB-1", Quantity: 1,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	off := r.catalogue.combos["CMB-1"]
	off.Orderable = false
	r.catalogue.combos["CMB-1"] = off

	got, _, err := r.carts.Current(context.Background(), "USR-1", "en")
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if got.Lines[0].Issue != string(domain.IssueUnavailable) || got.Lines[0].Orderable {
		t.Fatalf("line = %+v", got.Lines[0])
	}
}

func TestTheShopsOwnStateBlocksCheckout(t *testing.T) {
	cases := []struct {
		name    string
		shop    merchantcontract.Merchant
		blocker domain.Blocker
	}{
		{"suspended", merchantcontract.Merchant{ID: "MER-1", Name: "Star Kabab"}, domain.BlockerShopUnavailable},
		{"closed for the evening", merchantcontract.Merchant{
			ID: "MER-1", Name: "Star Kabab", IsListed: true, OpenStatus: "বন্ধ — খুলবে 09:00",
		}, domain.BlockerShopClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := withBurger()
			add(t, r, "ITM-burger", 1)
			if _, err := r.carts.SetAddress(context.Background(), "USR-1", "ADR-1", 23.7, 90.4, "en"); err != nil {
				t.Fatalf("SetAddress: %v", err)
			}
			r.merchant.shop = tc.shop

			got, _, err := r.carts.Current(context.Background(), "USR-1", "en")
			if err != nil {
				t.Fatalf("Current: %v", err)
			}
			if got.Blocker != string(tc.blocker) || got.Orderable {
				t.Fatalf("blocker = %q, want %q", got.Blocker, tc.blocker)
			}
			// A closed shop's own status is the sentence, because it says when
			// to come back.
			if tc.blocker == domain.BlockerShopClosed && got.BlockerText != tc.shop.OpenStatus {
				t.Errorf("blockerText = %q, want the shop's own status", got.BlockerText)
			}
		})
	}
}

func TestDownstreamFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("the cart cannot be read", func(t *testing.T) {
		r := withBurger()
		r.repo.readErr = errBoom
		if _, _, err := r.carts.Current(ctx, "USR-1", "en"); errs.CodeOf(err) != "cart_unavailable" {
			t.Fatalf("err = %v", err)
		}
		if _, err := r.carts.Add(ctx, "USR-1", application.AddRequest{
			MerchantID: "MER-1", Kind: "item", TargetID: "ITM-burger", Quantity: 1,
		}); errs.CodeOf(err) != "cart_unavailable" {
			t.Fatalf("add: err = %v", err)
		}
		if _, err := r.carts.Replace(ctx, "USR-1", application.AddRequest{MerchantID: "MER-1"}); errs.CodeOf(err) != "cart_unavailable" {
			t.Fatalf("replace: err = %v", err)
		}
		if err := r.carts.Clear(ctx, "USR-1"); errs.CodeOf(err) != "cart_unavailable" {
			t.Fatalf("clear: err = %v", err)
		}
	})

	t.Run("the cart cannot be written", func(t *testing.T) {
		r := withBurger()
		r.repo.saveErr = errBoom
		if _, err := r.carts.Add(ctx, "USR-1", application.AddRequest{
			MerchantID: "MER-1", Kind: "item", TargetID: "ITM-burger", Quantity: 1,
		}); errs.CodeOf(err) != "cart_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the old cart cannot be deleted on replace", func(t *testing.T) {
		r := withBurger()
		add(t, r, "ITM-burger", 1)
		r.repo.deleteErr = errBoom
		if _, err := r.carts.Replace(ctx, "USR-1", application.AddRequest{MerchantID: "MER-2"}); errs.CodeOf(err) != "cart_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the cart cannot be deleted on clear", func(t *testing.T) {
		r := withBurger()
		add(t, r, "ITM-burger", 1)
		r.repo.deleteErr = errBoom
		if err := r.carts.Clear(ctx, "USR-1"); errs.CodeOf(err) != "cart_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the shop cannot be read", func(t *testing.T) {
		r := withBurger()
		add(t, r, "ITM-burger", 1)
		r.merchant.err = errBoom
		if _, _, err := r.carts.Current(ctx, "USR-1", "en"); err == nil {
			t.Fatal("a cart was rendered with no shop")
		}
	})

	t.Run("the menu cannot be read", func(t *testing.T) {
		r := withBurger()
		add(t, r, "ITM-burger", 1)
		r.catalogue.itemsErr = errBoom
		if _, _, err := r.carts.Current(ctx, "USR-1", "en"); err == nil {
			t.Fatal("a cart was rendered with no menu")
		}
	})

	t.Run("reachability cannot be checked", func(t *testing.T) {
		r := withBurger()
		add(t, r, "ITM-burger", 1)
		if _, err := r.carts.SetAddress(ctx, "USR-1", "ADR-1", 23.7, 90.4, "en"); err != nil {
			t.Fatalf("SetAddress: %v", err)
		}
		r.discovery.err = errBoom
		if _, _, err := r.carts.Current(ctx, "USR-1", "en"); err == nil {
			t.Fatal("a cart was declared orderable with no reachability answer")
		}
	})
}

// A request naming no shop cannot open a cart. Reported as something the
// customer can act on rather than as a nil cart that fails later.
func TestAddWithNoShopCannotOpenACart(t *testing.T) {
	r := withBurger()
	_, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
		Kind: "item", TargetID: "ITM-burger", Quantity: 1,
	})
	if errs.CodeOf(err) != "invalid_cart" {
		t.Fatalf("err = %v, want invalid_cart", err)
	}
}

func TestAddABundleThatIsNotThere(t *testing.T) {
	r := newRig()
	_, err := r.carts.Add(context.Background(), "USR-1", application.AddRequest{
		MerchantID: "MER-1", Kind: "combo", TargetID: "CMB-gone", Quantity: 1,
	})
	if !errs.Is(err, errs.KindNotFound) {
		t.Fatalf("err = %v, want a not-found", err)
	}
}
