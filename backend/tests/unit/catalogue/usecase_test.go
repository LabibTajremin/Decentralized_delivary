package catalogue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

const (
	ownerID = "usr_owner"
	shopID  = "mch_shop"
)

// harness is a wired catalogue module over in-memory dependencies.
type harness struct {
	repo       *memoryRepo
	merchants  *fakeMerchants
	clock      *clock.Fixed
	categories *application.CategoryUseCase
	items      *application.ItemUseCase
	options    *application.OptionUseCase
	combos     *application.ComboUseCase
	service    *application.Service
}

func newHarness(t *testing.T, kind domain.MerchantType) *harness {
	t.Helper()
	repo := newRepo()
	merchants := newMerchants()
	merchants.add(shopID, ownerID, kind)

	fixed := clock.NewFixed(at(2026, time.March, 2, 12, 0))
	ids := &countingIDs{}

	return &harness{
		repo: repo, merchants: merchants, clock: fixed,
		categories: application.NewCategoryUseCase(repo, repo, merchants, ids),
		items:      application.NewItemUseCase(repo, repo, merchants, fixed, ids),
		options:    application.NewOptionUseCase(repo, merchants, ids),
		combos:     application.NewComboUseCase(repo, repo, merchants, ids),
		service:    application.NewService(repo, fixed),
	}
}

func (h *harness) category(t *testing.T, name string) domain.Category {
	t.Helper()
	category, err := h.categories.Create(context.Background(), ownerID, shopID,
		application.CategoryRequest{Name: name})
	if err != nil {
		t.Fatalf("Create category: %v", err)
	}
	return category
}

// itemRequest builds a valid item request for the harness's shop type.
func itemRequest(kind domain.MerchantType, categoryID, name string, priceMinor int64) application.ItemRequest {
	req := application.ItemRequest{
		CategoryID: categoryID, Name: name, PriceMinor: priceMinor,
	}
	if domain.CapabilitiesFor(kind).RequiresUnit {
		req.Unit = domain.UnitPiece.String()
	}
	return req
}

func (h *harness) item(t *testing.T, kind domain.MerchantType, categoryID, name string, priceMinor int64) domain.Item {
	t.Helper()
	item, err := h.items.Create(context.Background(), ownerID, shopID,
		itemRequest(kind, categoryID, name, priceMinor))
	if err != nil {
		t.Fatalf("Create item %q: %v", name, err)
	}
	return item
}

// TestTheServiceSatisfiesItsContract: consumers depend on the interface, so it
// must be the interface the service actually implements.
func TestTheServiceSatisfiesItsContract(t *testing.T) {
	var _ contract.CatalogueContract = (*application.Service)(nil)
}

// ------------------------------------------------------------- ownership

// TestOnlyTheOwnerMayChangeAMenu is the property that matters most here: the
// merchant id is in the path, so without this check any signed-in account could
// rewrite any menu in the country.
func TestOnlyTheOwnerMayChangeAMenu(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	item := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)
	combo := h.combo(t, category.ID)

	const intruder = "usr_someone_else"
	ctx := context.Background()

	calls := map[string]func() error{
		"create category": func() error {
			_, e := h.categories.Create(ctx, intruder, shopID, application.CategoryRequest{Name: "X"})
			return e
		},
		"update category": func() error {
			_, e := h.categories.Update(ctx, intruder, shopID, category.ID, application.CategoryRequest{Name: "X"})
			return e
		},
		"hide category": func() error {
			_, e := h.categories.SetActive(ctx, intruder, shopID, category.ID, false)
			return e
		},
		"delete category": func() error { return h.categories.Delete(ctx, intruder, shopID, category.ID) },
		"merchant type": func() error {
			_, e := h.categories.MerchantTypeOf(ctx, intruder, shopID)
			return e
		},
		"create item": func() error {
			_, e := h.items.Create(ctx, intruder, shopID, itemRequest(domain.Restaurant, category.ID, "X", 1))
			return e
		},
		"update item": func() error {
			_, e := h.items.Update(ctx, intruder, shopID, item.ID, itemRequest(domain.Restaurant, category.ID, "X", 1))
			return e
		},
		"hide item": func() error {
			_, e := h.items.SetActive(ctx, intruder, shopID, item.ID, false)
			return e
		},
		"set stock": func() error {
			_, e := h.items.SetStock(ctx, intruder, shopID, item.ID, 5)
			return e
		},
		"set availability": func() error {
			_, e := h.items.SetAvailability(ctx, intruder, shopID, item.ID, application.AvailabilityRequest{Always: true})
			return e
		},
		"delete item": func() error { return h.items.Delete(ctx, intruder, shopID, item.ID) },
		"bulk update": func() error {
			_, e := h.items.BulkUpdate(ctx, intruder, shopID, []application.BulkChange{{ItemID: item.ID}})
			return e
		},
		"set variants": func() error {
			_, e := h.options.SetVariantGroups(ctx, intruder, shopID, item.ID, nil)
			return e
		},
		"set add-ons": func() error {
			_, e := h.options.SetAddOnGroups(ctx, intruder, shopID, item.ID, nil)
			return e
		},
		"create combo": func() error {
			_, e := h.combos.Create(ctx, intruder, shopID, application.ComboRequest{Name: "X"})
			return e
		},
		"update combo": func() error {
			_, e := h.combos.Update(ctx, intruder, shopID, combo.ID, application.ComboRequest{Name: "X"})
			return e
		},
		"hide combo": func() error {
			_, e := h.combos.SetActive(ctx, intruder, shopID, combo.ID, false)
			return e
		},
		"combo availability": func() error {
			_, e := h.combos.SetAvailability(ctx, intruder, shopID, combo.ID, application.AvailabilityRequest{Always: true})
			return e
		},
		"delete combo": func() error { return h.combos.Delete(ctx, intruder, shopID, combo.ID) },
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			if got := errs.CodeOf(err); got != "not_your_shop" {
				t.Errorf("code = %q, want not_your_shop", got)
			}
			if errs.KindOf(err) != errs.KindForbidden {
				t.Errorf("kind = %v, want forbidden", errs.KindOf(err))
			}
		})
	}
}

// TestAShopThatDoesNotExistIsNotFound, with the merchant module's own wording
// rather than a second one invented here.
func TestAShopThatDoesNotExistIsNotFound(t *testing.T) {
	h := newHarness(t, domain.Restaurant)

	_, err := h.categories.Create(context.Background(), ownerID, "mch_nope",
		application.CategoryRequest{Name: "X"})
	if got := errs.CodeOf(err); got != "merchant_not_found" {
		t.Errorf("code = %q, want merchant_not_found", got)
	}
}

// TestAShopWhoseTypeWeCannotReadIsAnInternalFault, not something an owner can
// fix — and defaulting it would silently give a pharmacy a restaurant's rules.
func TestAShopWhoseTypeWeCannotReadIsAnInternalFault(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	broken := h.merchants.shops[shopID]
	broken.Type = "hardware"
	h.merchants.shops[shopID] = broken

	_, err := h.categories.Create(context.Background(), ownerID, shopID,
		application.CategoryRequest{Name: "X"})
	if got := errs.CodeOf(err); got != "unknown_merchant_type" {
		t.Errorf("code = %q, want unknown_merchant_type", got)
	}
	if errs.KindOf(err) != errs.KindInternal {
		t.Errorf("kind = %v, want internal", errs.KindOf(err))
	}
}

// ------------------------------------------------------------ categories

func TestANewSectionGoesToTheEndOfTheMenu(t *testing.T) {
	h := newHarness(t, domain.Restaurant)

	first := h.category(t, "Biryani")
	second := h.category(t, "Drinks")

	if first.SortOrder != 1 || second.SortOrder != 2 {
		t.Errorf("sort orders = %d, %d; want a section appended rather than inserted",
			first.SortOrder, second.SortOrder)
	}

	// An explicit position is honoured.
	third, err := h.categories.Create(context.Background(), ownerID, shopID,
		application.CategoryRequest{Name: "Starters", SortOrder: 1})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if third.SortOrder != 1 {
		t.Errorf("sort order = %d, want the position asked for", third.SortOrder)
	}
}

func TestTheMenuHasASectionLimit(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	for i := 0; i < 50; i++ {
		h.category(t, "Section")
	}

	_, err := h.categories.Create(context.Background(), ownerID, shopID,
		application.CategoryRequest{Name: "One more"})
	if got := errs.CodeOf(err); got != "too_many_categories" {
		t.Errorf("code = %q, want too_many_categories", got)
	}
	if errs.KindOf(err) != errs.KindConflict {
		t.Errorf("kind = %v, want conflict", errs.KindOf(err))
	}
}

func TestSectionsAreListedInTheOwnersOrder(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	h.category(t, "Biryani")
	h.category(t, "Drinks")

	categories, err := h.categories.List(context.Background(), shopID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(categories) != 2 || categories[0].Name != "Biryani" {
		t.Errorf("categories = %+v", categories)
	}
}

// TestASectionWithFoodInItCannotBeDeleted. Cascading would silently delete the
// food; reassigning it would be a decision the owner did not make.
func TestASectionWithFoodInItCannotBeDeleted(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")
	item := h.item(t, domain.Restaurant, category.ID, "Kacchi", 35_000)

	err := h.categories.Delete(context.Background(), ownerID, shopID, category.ID)
	if got := errs.CodeOf(err); got != "category_not_empty" {
		t.Errorf("code = %q, want category_not_empty", got)
	}

	// Emptied, it goes.
	if err := h.items.Delete(context.Background(), ownerID, shopID, item.ID); err != nil {
		t.Fatalf("Delete item: %v", err)
	}
	if err := h.categories.Delete(context.Background(), ownerID, shopID, category.ID); err != nil {
		t.Errorf("Delete category: %v", err)
	}
}

func TestHidingASectionKeepsItForLater(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Winter specials")

	hidden, err := h.categories.SetActive(context.Background(), ownerID, shopID, category.ID, false)
	if err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if hidden.Active {
		t.Error("the section is still shown")
	}

	back, err := h.categories.SetActive(context.Background(), ownerID, shopID, category.ID, true)
	if err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if !back.Active {
		t.Error("the section did not come back")
	}
}

func TestRenamingASection(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	category := h.category(t, "Biryani")

	renamed, err := h.categories.Update(context.Background(), ownerID, shopID, category.ID,
		application.CategoryRequest{Name: "Rice dishes", SortOrder: 4})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if renamed.Name != "Rice dishes" || renamed.SortOrder != 4 {
		t.Errorf("renamed = %+v", renamed)
	}

	_, err = h.categories.Update(context.Background(), ownerID, shopID, category.ID,
		application.CategoryRequest{Name: " "})
	if got := errs.CodeOf(err); got != "name_required" {
		t.Errorf("code = %q, want name_required", got)
	}
}

func TestASectionThatDoesNotExist(t *testing.T) {
	h := newHarness(t, domain.Restaurant)
	ctx := context.Background()

	calls := map[string]func() error{
		"update": func() error {
			_, e := h.categories.Update(ctx, ownerID, shopID, "cat_nope", application.CategoryRequest{Name: "X"})
			return e
		},
		"hide":   func() error { _, e := h.categories.SetActive(ctx, ownerID, shopID, "cat_nope", false); return e },
		"delete": func() error { return h.categories.Delete(ctx, ownerID, shopID, "cat_nope") },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			if got := errs.CodeOf(err); got != "category_not_found" {
				t.Errorf("code = %q, want category_not_found", got)
			}
		})
	}
}

func TestCategoryWritesReportAStoreThatIsDown(t *testing.T) {
	ctx := context.Background()

	cases := map[string]struct {
		broken func(*memoryRepo)
		call   func(*harness, string) error
	}{
		"list": {
			func(r *memoryRepo) { r.categoriesErr = errStore },
			func(h *harness, _ string) error { _, e := h.categories.List(ctx, shopID); return e },
		},
		"list before create": {
			func(r *memoryRepo) { r.categoriesErr = errStore },
			func(h *harness, _ string) error {
				_, e := h.categories.Create(ctx, ownerID, shopID, application.CategoryRequest{Name: "X"})
				return e
			},
		},
		"save": {
			func(r *memoryRepo) { r.saveCategoryErr = errStore },
			func(h *harness, _ string) error {
				_, e := h.categories.Create(ctx, ownerID, shopID, application.CategoryRequest{Name: "X"})
				return e
			},
		},
		"read one": {
			func(r *memoryRepo) { r.categoryErr = errStore },
			func(h *harness, id string) error {
				_, e := h.categories.Update(ctx, ownerID, shopID, id, application.CategoryRequest{Name: "X"})
				return e
			},
		},
		"save on update": {
			func(r *memoryRepo) { r.saveCategoryErr = errStore },
			func(h *harness, id string) error {
				_, e := h.categories.Update(ctx, ownerID, shopID, id, application.CategoryRequest{Name: "X"})
				return e
			},
		},
		"save on hide": {
			func(r *memoryRepo) { r.saveCategoryErr = errStore },
			func(h *harness, id string) error {
				_, e := h.categories.SetActive(ctx, ownerID, shopID, id, false)
				return e
			},
		},
		"count before delete": {
			func(r *memoryRepo) { r.countErr = errStore },
			func(h *harness, id string) error { return h.categories.Delete(ctx, ownerID, shopID, id) },
		},
		"delete": {
			func(r *memoryRepo) { r.deleteCategErr = errStore },
			func(h *harness, id string) error { return h.categories.Delete(ctx, ownerID, shopID, id) },
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, domain.Restaurant)
			category := h.category(t, "Biryani")
			tc.broken(h.repo)

			err := tc.call(h, category.ID)
			if errs.KindOf(err) != errs.KindUnavailable {
				t.Errorf("kind = %v, want unavailable", errs.KindOf(err))
			}
			if !errors.Is(err, errStore) {
				t.Errorf("the cause was lost: %v", err)
			}
		})
	}
}

// combo builds a two-item combo for the harness's shop.
func (h *harness) combo(t *testing.T, categoryID string) domain.Combo {
	t.Helper()
	first := h.item(t, domain.Restaurant, categoryID, "Kacchi", 35_000)
	second := h.item(t, domain.Restaurant, categoryID, "Borhani", 6_000)

	combo, err := h.combos.Create(context.Background(), ownerID, shopID, application.ComboRequest{
		Name: "Kacchi meal", PriceMinor: 38_000,
		Lines: []application.ComboLineRequest{
			{ItemID: first.ID, Quantity: 1},
			{ItemID: second.ID, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Create combo: %v", err)
	}
	return combo
}
