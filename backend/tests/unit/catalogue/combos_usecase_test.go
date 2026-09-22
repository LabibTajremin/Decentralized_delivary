package catalogue

import (
	"context"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// mealDeal builds a shop with two items and a combo over them.
func mealDeal(t *testing.T) (*harness, domain.Item, domain.Item, domain.Combo) {
	t.Helper()
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")

	kacchi := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)
	borhani := h.item(t, domain.Restaurant, category.ID, "Borhani", 6_000)

	combo, err := h.combos.Create(context.Background(), ownerID, shopID, application.ComboRequest{
		Name: "Kacchi meal", Description: "Kacchi with a borhani.", PriceMinor: 38_000,
		Lines: []application.ComboLineRequest{
			{ItemID: kacchi.ID, Quantity: 1},
			{ItemID: borhani.ID, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Create combo: %v", err)
	}
	return h, kacchi, borhani, combo
}

// TestACombosMembersMustBeOnThisMenu. Trusting the ids gives a bundle that
// resolves to nothing at checkout — which the customer discovers and the owner
// does not.
func TestACombosMembersMustBeOnThisMenu(t *testing.T) {
	h, kacchi, _, combo := mealDeal(t)

	_, err := h.combos.Create(context.Background(), ownerID, shopID, application.ComboRequest{
		Name: "Bad deal", PriceMinor: 10_000,
		Lines: []application.ComboLineRequest{
			{ItemID: kacchi.ID, Quantity: 1},
			{ItemID: "itm_nope", Quantity: 1},
		},
	})
	if got := errs.CodeOf(err); got != "item_not_found" {
		t.Errorf("create: code = %q, want item_not_found", got)
	}

	// The same check applies on an edit.
	_, err = h.combos.Update(context.Background(), ownerID, shopID, combo.ID, application.ComboRequest{
		Name: "Kacchi meal", PriceMinor: 38_000,
		Lines: []application.ComboLineRequest{
			{ItemID: kacchi.ID, Quantity: 1},
			{ItemID: "itm_nope", Quantity: 1},
		},
	})
	if got := errs.CodeOf(err); got != "item_not_found" {
		t.Errorf("update: code = %q, want item_not_found", got)
	}
}

func TestListingAndReadingCombos(t *testing.T) {
	h, _, _, combo := mealDeal(t)

	combos, err := h.combos.List(context.Background(), shopID, false)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(combos) != 1 || combos[0].ID != combo.ID {
		t.Errorf("combos = %+v", combos)
	}

	got, err := h.combos.Get(context.Background(), shopID, combo.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Kacchi meal" {
		t.Errorf("name = %q", got.Name)
	}

	if _, err := h.combos.Get(context.Background(), shopID, "cmb_nope"); errs.CodeOf(err) != "combo_not_found" {
		t.Errorf("code = %q, want combo_not_found", errs.CodeOf(err))
	}
}

func TestHidingACombo(t *testing.T) {
	h, _, _, combo := mealDeal(t)

	hidden, err := h.combos.SetActive(context.Background(), ownerID, shopID, combo.ID, false)
	if err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if hidden.Active {
		t.Error("the combo is still shown")
	}

	shown, err := h.combos.List(context.Background(), shopID, true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(shown) != 0 {
		t.Errorf("a hidden combo is still listed as active: %+v", shown)
	}
}

// TestEditingACombosKeepsWhetherItIsShownAndWhen, so an owner correcting a
// price does not put a hidden bundle back on the menu.
func TestEditingACombosKeepsWhetherItIsShownAndWhen(t *testing.T) {
	h, kacchi, borhani, combo := mealDeal(t)

	if _, err := h.combos.SetActive(context.Background(), ownerID, shopID, combo.ID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if _, err := h.combos.SetAvailability(context.Background(), ownerID, shopID, combo.ID,
		application.AvailabilityRequest{Days: map[string][]string{"1": {"12:00-15:00"}}}); err != nil {
		t.Fatalf("SetAvailability: %v", err)
	}

	updated, err := h.combos.Update(context.Background(), ownerID, shopID, combo.ID, application.ComboRequest{
		Name: "Kacchi meal deal", PriceMinor: 40_000,
		Lines: []application.ComboLineRequest{
			{ItemID: kacchi.ID, Quantity: 1},
			{ItemID: borhani.ID, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Active {
		t.Error("the edit put a hidden combo back on the menu")
	}
	if updated.Availability.Always {
		t.Error("the edit lost the combo's schedule")
	}
	if updated.Name != "Kacchi meal deal" || updated.Price.Minor() != 40_000 {
		t.Errorf("updated = %+v", updated)
	}
	if len(updated.Lines) != 2 || updated.Lines[1].Quantity != 2 {
		t.Errorf("lines = %+v", updated.Lines)
	}
}

func TestSettingACombosAvailability(t *testing.T) {
	h, _, _, combo := mealDeal(t)

	scheduled, err := h.combos.SetAvailability(context.Background(), ownerID, shopID, combo.ID,
		application.AvailabilityRequest{Days: map[string][]string{"1": {"12:00-15:00"}}})
	if err != nil {
		t.Fatalf("SetAvailability: %v", err)
	}
	if !scheduled.Orderable(at(2026, time.March, 2, 13, 0)) {
		t.Error("the lunch combo is not orderable at 13:00 Monday")
	}

	back, err := h.combos.SetAvailability(context.Background(), ownerID, shopID, combo.ID,
		application.AvailabilityRequest{Always: true})
	if err != nil {
		t.Fatalf("SetAvailability: %v", err)
	}
	if !back.Availability.Always {
		t.Error("the combo did not go back to always available")
	}

	if _, err := h.combos.SetAvailability(context.Background(), ownerID, shopID, combo.ID,
		application.AvailabilityRequest{}); errs.CodeOf(err) != "availability_required" {
		t.Errorf("code = %q, want availability_required", errs.CodeOf(err))
	}
	if _, err := h.combos.SetAvailability(context.Background(), ownerID, shopID, combo.ID,
		application.AvailabilityRequest{Days: map[string][]string{"1": {"noon to three"}}}); errs.CodeOf(err) != "invalid_availability" {
		t.Errorf("code = %q, want invalid_availability", errs.CodeOf(err))
	}
}

// TestDeletingACombosLeavesItsItemsAlone: a bundle is an offer over things that
// exist independently of it.
func TestDeletingACombosLeavesItsItemsAlone(t *testing.T) {
	h, kacchi, _, combo := mealDeal(t)

	if err := h.combos.Delete(context.Background(), ownerID, shopID, combo.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := h.combos.Get(context.Background(), shopID, combo.ID); errs.CodeOf(err) != "combo_not_found" {
		t.Errorf("the combo survived: %v", err)
	}
	if _, err := h.items.Get(context.Background(), shopID, kacchi.ID); err != nil {
		t.Errorf("deleting the combo took an item with it: %v", err)
	}
}

func TestACombosThatDoesNotExist(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	ctx := context.Background()

	calls := map[string]func() error{
		"update": func() error {
			_, e := h.combos.Update(ctx, ownerID, shopID, "cmb_nope", application.ComboRequest{Name: "X"})
			return e
		},
		"hide": func() error { _, e := h.combos.SetActive(ctx, ownerID, shopID, "cmb_nope", false); return e },
		"availability": func() error {
			_, e := h.combos.SetAvailability(ctx, ownerID, shopID, "cmb_nope", application.AvailabilityRequest{Always: true})
			return e
		},
		"delete": func() error { return h.combos.Delete(ctx, ownerID, shopID, "cmb_nope") },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if got := errs.CodeOf(call()); got != "combo_not_found" {
				t.Errorf("code = %q, want combo_not_found", got)
			}
		})
	}
}

// TestACombosDraftIsValidatedBeforeItsMembersAreLookedUp, so an obviously
// malformed bundle does not cost a query.
func TestACombosDraftIsValidated(t *testing.T) {
	h, kacchi, _, _ := mealDeal(t)

	cases := map[string]struct {
		req  application.ComboRequest
		code string
	}{
		"one item": {
			application.ComboRequest{Name: "Solo", PriceMinor: 1,
				Lines: []application.ComboLineRequest{{ItemID: kacchi.ID, Quantity: 1}}},
			"combo_too_small",
		},
		"no name": {
			application.ComboRequest{Name: " ", PriceMinor: 1,
				Lines: []application.ComboLineRequest{{ItemID: kacchi.ID, Quantity: 1}, {ItemID: "b", Quantity: 1}}},
			"name_required",
		},
		"the same item twice": {
			application.ComboRequest{Name: "Double", PriceMinor: 1,
				Lines: []application.ComboLineRequest{{ItemID: kacchi.ID, Quantity: 1}, {ItemID: kacchi.ID, Quantity: 1}}},
			"duplicate_combo_item",
		},
		"a quantity of zero": {
			application.ComboRequest{Name: "Nothing", PriceMinor: 1,
				Lines: []application.ComboLineRequest{{ItemID: kacchi.ID, Quantity: 0}, {ItemID: "b", Quantity: 1}}},
			"invalid_quantity",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := h.combos.Create(context.Background(), ownerID, shopID, tc.req)
			if got := errs.CodeOf(err); got != tc.code {
				t.Errorf("code = %q, want %q", got, tc.code)
			}
		})
	}
}

// TestAPharmacyCannotBundleThroughTheUseCaseEither.
func TestAPharmacyCannotBundleThroughTheUseCaseEither(t *testing.T) {
	h := newHarness(t, domain.Pharmacy)
	category := h.category(t, "Cold and flu")
	first := h.item(t, domain.Pharmacy, category.ID, "Napa", 1_200)
	second := h.item(t, domain.Pharmacy, category.ID, "Antihistamine", 2_000)

	_, err := h.combos.Create(context.Background(), ownerID, shopID, application.ComboRequest{
		Name: "Cold pack", PriceMinor: 3_000,
		Lines: []application.ComboLineRequest{
			{ItemID: first.ID, Quantity: 1},
			{ItemID: second.ID, Quantity: 1},
		},
	})
	if got := errs.CodeOf(err); got != "combos_not_allowed" {
		t.Errorf("code = %q, want combos_not_allowed", got)
	}
}

func TestComboWritesReportAStoreThatIsDown(t *testing.T) {
	ctx := context.Background()

	cases := map[string]struct {
		broken func(*memoryRepo)
		call   func(*harness, domain.Item, domain.Item, domain.Combo) error
	}{
		"list": {
			func(r *memoryRepo) { r.combosErr = errStore },
			func(h *harness, _, _ domain.Item, _ domain.Combo) error {
				_, e := h.combos.List(ctx, shopID, false)
				return e
			},
		},
		"read one": {
			func(r *memoryRepo) { r.comboErr = errStore },
			func(h *harness, _, _ domain.Item, c domain.Combo) error {
				_, e := h.combos.Get(ctx, shopID, c.ID)
				return e
			},
		},
		"member lookup": {
			func(r *memoryRepo) { r.itemsByIDErr = errStore },
			func(h *harness, a, b domain.Item, _ domain.Combo) error {
				_, e := h.combos.Create(ctx, ownerID, shopID, application.ComboRequest{
					Name: "X", PriceMinor: 1,
					Lines: []application.ComboLineRequest{{ItemID: a.ID, Quantity: 1}, {ItemID: b.ID, Quantity: 1}},
				})
				return e
			},
		},
		"save": {
			func(r *memoryRepo) { r.saveComboErr = errStore },
			func(h *harness, a, b domain.Item, _ domain.Combo) error {
				_, e := h.combos.Create(ctx, ownerID, shopID, application.ComboRequest{
					Name: "X", PriceMinor: 1,
					Lines: []application.ComboLineRequest{{ItemID: a.ID, Quantity: 1}, {ItemID: b.ID, Quantity: 1}},
				})
				return e
			},
		},
		"save on update": {
			func(r *memoryRepo) { r.saveComboErr = errStore },
			func(h *harness, a, b domain.Item, c domain.Combo) error {
				_, e := h.combos.Update(ctx, ownerID, shopID, c.ID, application.ComboRequest{
					Name: "X", PriceMinor: 1,
					Lines: []application.ComboLineRequest{{ItemID: a.ID, Quantity: 1}, {ItemID: b.ID, Quantity: 1}},
				})
				return e
			},
		},
		"save on hide": {
			func(r *memoryRepo) { r.saveComboErr = errStore },
			func(h *harness, _, _ domain.Item, c domain.Combo) error {
				_, e := h.combos.SetActive(ctx, ownerID, shopID, c.ID, false)
				return e
			},
		},
		"save on availability": {
			func(r *memoryRepo) { r.saveComboErr = errStore },
			func(h *harness, _, _ domain.Item, c domain.Combo) error {
				_, e := h.combos.SetAvailability(ctx, ownerID, shopID, c.ID,
					application.AvailabilityRequest{Always: true})
				return e
			},
		},
		"delete": {
			func(r *memoryRepo) { r.deleteComboErr = errStore },
			func(h *harness, _, _ domain.Item, c domain.Combo) error {
				return h.combos.Delete(ctx, ownerID, shopID, c.ID)
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h, first, second, combo := mealDeal(t)
			tc.broken(h.repo)

			if got := errs.KindOf(tc.call(h, first, second, combo)); got != errs.KindUnavailable {
				t.Errorf("kind = %v, want unavailable", got)
			}
		})
	}
}
