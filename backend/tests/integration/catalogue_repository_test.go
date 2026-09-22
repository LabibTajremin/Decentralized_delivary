package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	catports "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/application/ports"
	catdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/domain"
	catpg "github.com/rootlogic-lab/delivery/backend/internal/modules/catalogue/infrastructure/persistence/postgres"
	merchantdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	merchantpg "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/infrastructure/persistence/postgres"
	taka "github.com/rootlogic-lab/delivery/backend/internal/shared/money"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/schedule"
)

// withCatalogue runs a test inside a transaction that is always rolled back,
// with a shop already registered — a catalogue row has a foreign key into
// merchants, so there is no such thing as a catalogue without one.
func withCatalogue(t *testing.T, kind catdomain.MerchantType, fn func(ctx context.Context, tx pgx.Tx, repo *catpg.Repository, merchantID string)) {
	t.Helper()
	ctx := context.Background()
	conn := connect(t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	merchantID := "mch_cat_" + string(kind)
	merchant := sampleMerchant(t, merchantID, "usr_cat_"+string(kind),
		merchantdomain.Type(kind))
	if err := merchantpg.New(tx, tx).Create(ctx, merchant); err != nil {
		t.Fatalf("create shop: %v", err)
	}

	fn(ctx, tx, catpg.New(tx, tx), merchantID)
}

// newCategory stores a section and returns it.
func newCategory(t *testing.T, ctx context.Context, repo *catpg.Repository, merchantID, id, name string, sortOrder int) catdomain.Category {
	t.Helper()
	category, err := catdomain.NewCategory(id, merchantID, name, sortOrder)
	if err != nil {
		t.Fatalf("NewCategory: %v", err)
	}
	if err := repo.SaveCategory(ctx, category); err != nil {
		t.Fatalf("SaveCategory: %v", err)
	}
	return category
}

// newItem stores an item of the right shape for its shop type and returns it.
func newItem(t *testing.T, ctx context.Context, repo *catpg.Repository, merchantID, categoryID, id, name string, kind catdomain.MerchantType, priceMinor int64) catdomain.Item {
	t.Helper()
	draft := catdomain.ItemDraft{
		CategoryID: categoryID, Name: name, Price: taka.Taka(priceMinor),
	}
	if catdomain.CapabilitiesFor(kind).RequiresUnit {
		draft.Attributes.Unit = catdomain.UnitPiece
	}

	item, err := catdomain.NewItem(id, merchantID, kind, draft)
	if err != nil {
		t.Fatalf("NewItem: %v", err)
	}
	if err := repo.SaveItem(ctx, item); err != nil {
		t.Fatalf("SaveItem: %v", err)
	}
	return item
}

// ------------------------------------------------------------- categories

func TestACategoryRoundTripsThroughPostgres(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		newCategory(t, ctx, repo, merchantID, "cat_1", "কাচ্চি ও বিরিয়ানি", 3)

		got, err := repo.Category(ctx, merchantID, "cat_1")
		if err != nil {
			t.Fatalf("Category: %v", err)
		}
		if got.Name != "কাচ্চি ও বিরিয়ানি" || got.SortOrder != 3 || !got.Active {
			t.Errorf("category = %+v", got)
		}

		// Saving again updates rather than duplicating.
		if err := repo.SaveCategory(ctx, got.WithActive(false).WithSortOrder(1)); err != nil {
			t.Fatalf("SaveCategory: %v", err)
		}
		got, err = repo.Category(ctx, merchantID, "cat_1")
		if err != nil {
			t.Fatalf("Category: %v", err)
		}
		if got.Active || got.SortOrder != 1 {
			t.Errorf("after update = %+v", got)
		}
	})
}

// TestACategoryIsScopedToItsShop: a query that can return another shop's row is
// one careless call away from letting somebody file an item under a
// competitor's menu.
func TestACategoryIsScopedToItsShop(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)

		if _, err := repo.Category(ctx, "mch_someone_else", "cat_1"); !errors.Is(err, catdomain.ErrCategoryNotFound) {
			t.Errorf("error = %v, want ErrCategoryNotFound", err)
		}
	})
}

func TestCategoriesComeBackInTheOwnersOrder(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		newCategory(t, ctx, repo, merchantID, "cat_2", "Drinks", 2)
		newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)

		categories, err := repo.Categories(ctx, merchantID)
		if err != nil {
			t.Fatalf("Categories: %v", err)
		}
		if len(categories) != 2 || categories[0].ID != "cat_1" {
			t.Errorf("categories = %+v, want the owner's order", categories)
		}
	})
}

// TestDeletingASectionWithItemsIsRefusedByTheDatabase. The use case checks
// first and gives a readable message; this is the constraint behind it, which
// is what stops a direct write from deleting a shop's food.
func TestDeletingASectionWithItemsIsRefusedByTheDatabase(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, tx pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)
		newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Kacchi", catdomain.Restaurant, 35_000)

		n, err := repo.CountItemsInCategory(ctx, merchantID, category.ID)
		if err != nil {
			t.Fatalf("CountItemsInCategory: %v", err)
		}
		if n != 1 {
			t.Errorf("count = %d, want 1", n)
		}

		err = repo.DeleteCategory(ctx, merchantID, category.ID)
		if err == nil {
			t.Fatal("a section with food in it was deleted")
		}
		// The transaction is poisoned by the constraint violation, so this test
		// ends here; the rollback in the helper cleans up.
		_ = tx
	})
}

// ------------------------------------------------------------------- items

// TestAnItemRoundTripsWithEveryFieldItsTypeCarries, because a column written
// and never read back is a column that can be wrong for a year.
func TestAnItemRoundTripsWithEveryFieldItsTypeCarries(t *testing.T) {
	withCatalogue(t, catdomain.Pharmacy, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Painkillers", 1)

		item, err := catdomain.NewItem("itm_1", merchantID, catdomain.Pharmacy, catdomain.ItemDraft{
			CategoryID: category.ID, Name: "নাপা", Description: "প্যারাসিটামল ৫০০ মিগ্রা",
			ImageURL: "/static/demo/napa.png", Price: taka.Taka(1_200), SortOrder: 2,
			Attributes: catdomain.Attributes{
				Unit: catdomain.UnitStrip, PackSize: "10 tablets", Brand: "Beximco",
				GenericName: "Paracetamol", Strength: "500mg", RequiresPrescription: true,
			},
			Stock: catdomain.Stock{Tracked: true, Quantity: 40},
		})
		if err != nil {
			t.Fatalf("NewItem: %v", err)
		}
		if err := repo.SaveItem(ctx, item); err != nil {
			t.Fatalf("SaveItem: %v", err)
		}

		got, err := repo.Item(ctx, merchantID, "itm_1")
		if err != nil {
			t.Fatalf("Item: %v", err)
		}

		if got.Name != "নাপা" || got.Description != "প্যারাসিটামল ৫০০ মিগ্রা" {
			t.Errorf("text = %q / %q", got.Name, got.Description)
		}
		if got.ImageURL != "/static/demo/napa.png" || got.SortOrder != 2 {
			t.Errorf("image = %q, sort = %d", got.ImageURL, got.SortOrder)
		}
		if got.Price.Minor() != 1_200 || got.Price.Display() != "৳ 12" {
			t.Errorf("price = %s (%d minor)", got.Price, got.Price.Minor())
		}
		if got.MerchantType != catdomain.Pharmacy || got.CategoryID != category.ID {
			t.Errorf("placement = %q / %q", got.MerchantType, got.CategoryID)
		}
		if got.Attributes.Unit != catdomain.UnitStrip || got.Attributes.PackSize != "10 tablets" ||
			got.Attributes.Brand != "Beximco" {
			t.Errorf("shared attributes = %+v", got.Attributes)
		}
		if got.Attributes.GenericName != "Paracetamol" || got.Attributes.Strength != "500mg" ||
			!got.Attributes.RequiresPrescription {
			t.Errorf("pharmacy attributes = %+v", got.Attributes)
		}
		if !got.Stock.Tracked || got.Stock.Quantity != 40 {
			t.Errorf("stock = %+v", got.Stock)
		}
		if !got.Active || !got.Availability.Always {
			t.Errorf("defaults = active %v, always %v", got.Active, got.Availability.Always)
		}
	})
}

// TestARestaurantDishStoresNoShelfCount, which is the per-type difference as
// the schema sees it.
func TestARestaurantDishStoresNoShelfCount(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)

		item, err := catdomain.NewItem("itm_1", merchantID, catdomain.Restaurant, catdomain.ItemDraft{
			CategoryID: category.ID, Name: "Kacchi", Price: taka.Taka(35_000),
			Attributes: catdomain.Attributes{IsVegetarian: false, PreparationMinutes: 40},
		})
		if err != nil {
			t.Fatalf("NewItem: %v", err)
		}
		if err := repo.SaveItem(ctx, item); err != nil {
			t.Fatalf("SaveItem: %v", err)
		}

		got, err := repo.Item(ctx, merchantID, "itm_1")
		if err != nil {
			t.Fatalf("Item: %v", err)
		}
		if got.Stock.Tracked {
			t.Error("a restaurant dish stored a shelf count")
		}
		if !got.Stock.InStock() {
			t.Error("a restaurant dish reads as out of stock")
		}
		if got.Attributes.PreparationMinutes != 40 {
			t.Errorf("preparation = %d", got.Attributes.PreparationMinutes)
		}
		if got.Attributes.Unit != "" {
			t.Errorf("a restaurant dish stored a unit: %q", got.Attributes.Unit)
		}
	})
}

// TestTheDatabaseRefusesAPrescriptionOutsideAPharmacy. The domain enforces it
// and so does the schema: this is what stops a direct write or a future import
// from creating a restaurant dish that needs one.
func TestTheDatabaseRefusesAPrescriptionOutsideAPharmacy(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, tx pgx.Tx, _ *catpg.Repository, merchantID string) {
		newCategory(t, ctx, catpg.New(tx, tx), merchantID, "cat_1", "Biryani", 1)

		_, err := tx.Exec(ctx, `
			INSERT INTO catalogue_items (id, merchant_id, category_id, merchant_type,
			    name, price_minor, requires_prescription)
			VALUES ($1, $2, $3, 'restaurant', 'Kacchi', 100, TRUE)`,
			"itm_direct", merchantID, "cat_1")
		if err == nil {
			t.Error("the database accepted a restaurant dish needing a prescription")
		}
	})
}

func TestAnItemsAvailabilitySurvivesStorage(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Breakfast", 1)
		item := newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Paratha", catdomain.Restaurant, 3_000)

		breakfast, err := catdomain.NewAvailability(map[time.Weekday][]schedule.Window{
			time.Monday: {mustWindow(t, "07:00-11:00")},
			time.Friday: {mustWindow(t, "07:00-10:00"), mustWindow(t, "15:00-18:00")},
		})
		if err != nil {
			t.Fatalf("NewAvailability: %v", err)
		}
		if err := repo.SaveItem(ctx, item.WithAvailability(breakfast)); err != nil {
			t.Fatalf("SaveItem: %v", err)
		}

		got, err := repo.Item(ctx, merchantID, "itm_1")
		if err != nil {
			t.Fatalf("Item: %v", err)
		}
		if got.Availability.Always {
			t.Fatal("the schedule was lost")
		}

		monday := time.Date(2026, time.March, 2, 8, 0, 0, 0, schedule.Bangladesh)
		if !got.Availability.AvailableAt(monday) {
			t.Error("unavailable at 08:00 Monday, inside the breakfast window")
		}
		if got.Availability.AvailableAt(monday.Add(6 * time.Hour)) {
			t.Error("available at 14:00 Monday, outside it")
		}

		friday := time.Date(2026, time.March, 6, 16, 0, 0, 0, schedule.Bangladesh)
		if !got.Availability.AvailableAt(friday) {
			t.Error("unavailable at 16:00 Friday, inside the second window")
		}
	})
}

func TestItemsAreListedFilteredAndPaged(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		biryani := newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)
		drinks := newCategory(t, ctx, repo, merchantID, "cat_2", "Drinks", 2)

		newItem(t, ctx, repo, merchantID, biryani.ID, "itm_1", "Kacchi Biryani", catdomain.Restaurant, 35_000)
		newItem(t, ctx, repo, merchantID, biryani.ID, "itm_2", "Morog Polao", catdomain.Restaurant, 28_000)
		hidden := newItem(t, ctx, repo, merchantID, drinks.ID, "itm_3", "Borhani", catdomain.Restaurant, 6_000)

		if err := repo.SaveItem(ctx, hidden.WithActive(false)); err != nil {
			t.Fatalf("SaveItem: %v", err)
		}

		cases := map[string]struct {
			filter catports.ItemFilter
			want   int
		}{
			"everything":     {catports.ItemFilter{MerchantID: merchantID, Limit: 10}, 3},
			"one section":    {catports.ItemFilter{MerchantID: merchantID, CategoryID: biryani.ID, Limit: 10}, 2},
			"shown only":     {catports.ItemFilter{MerchantID: merchantID, ActiveOnly: true, Limit: 10}, 2},
			"by name":        {catports.ItemFilter{MerchantID: merchantID, Search: "biryani", Limit: 10}, 1},
			"name any case":  {catports.ItemFilter{MerchantID: merchantID, Search: "BIRYANI", Limit: 10}, 1},
			"a partial word": {catports.ItemFilter{MerchantID: merchantID, Search: "polao", Limit: 10}, 1},
			"first page":     {catports.ItemFilter{MerchantID: merchantID, Limit: 2}, 2},
			"second page":    {catports.ItemFilter{MerchantID: merchantID, Limit: 2, Offset: 2}, 1},
			"another shop":   {catports.ItemFilter{MerchantID: "mch_other", Limit: 10}, 0},
		}

		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				items, err := repo.Items(ctx, tc.filter)
				if err != nil {
					t.Fatalf("Items: %v", err)
				}
				if len(items) != tc.want {
					t.Errorf("found %d items, want %d", len(items), tc.want)
				}
			})
		}
	})
}

// TestASearchTermCannotReachTheSQL. Every clause is a placeholder, so a search
// box on the busiest endpoint in the product is data and never code.
func TestASearchTermCannotReachTheSQL(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)
		newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Kacchi", catdomain.Restaurant, 35_000)

		items, err := repo.Items(ctx, catports.ItemFilter{
			MerchantID: merchantID,
			Search:     "'; DROP TABLE catalogue_items; --",
			Limit:      10,
		})
		if err != nil {
			t.Fatalf("Items: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("found %d items for a nonsense search", len(items))
		}

		// The table is still there.
		if _, err := repo.Item(ctx, merchantID, "itm_1"); err != nil {
			t.Errorf("the item is gone: %v", err)
		}
	})
}

func TestItemsAreFetchedInBatches(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)
		newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Kacchi", catdomain.Restaurant, 35_000)
		newItem(t, ctx, repo, merchantID, category.ID, "itm_2", "Borhani", catdomain.Restaurant, 6_000)

		items, err := repo.ItemsByID(ctx, merchantID, []string{"itm_2", "itm_1", "itm_gone"})
		if err != nil {
			t.Fatalf("ItemsByID: %v", err)
		}
		if len(items) != 2 {
			t.Errorf("found %d items, want the two that exist", len(items))
		}

		// An id belonging to another shop is simply absent.
		items, err = repo.ItemsByID(ctx, "mch_other", []string{"itm_1"})
		if err != nil {
			t.Fatalf("ItemsByID: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("another shop's item was returned: %+v", items)
		}
	})
}

// ------------------------------------------------------------ option groups

// TestOptionGroupsRoundTripAndAreReplacedWholesale. A group left behind from
// before an edit is a choice a customer would still be offered.
func TestOptionGroupsRoundTripAndAreReplacedWholesale(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Pizza", 1)
		item := newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Margherita", catdomain.Restaurant, 55_000)

		size, err := catdomain.NewVariantGroup("vgr_size", "Size", true, 1, 1, []catdomain.VariantOption{
			mustVariantOption(t, "vop_regular", "Regular", 0),
			mustVariantOption(t, "vop_large", "Large", 15_000),
		})
		if err != nil {
			t.Fatalf("NewVariantGroup: %v", err)
		}
		crust, err := catdomain.NewVariantGroup("vgr_crust", "Crust", false, 0, 1, []catdomain.VariantOption{
			mustVariantOption(t, "vop_thin", "Thin", 0),
		})
		if err != nil {
			t.Fatalf("NewVariantGroup: %v", err)
		}
		size.SortOrder, crust.SortOrder = 1, 2

		extras, err := catdomain.NewAddOnGroup("agr_extras", "Extras", 0, 2, []catdomain.AddOn{
			mustAddOnOption(t, "aop_olives", "Olives", 2_000),
			mustAddOnOption(t, "aop_cheese", "Extra cheese", 3_000),
		})
		if err != nil {
			t.Fatalf("NewAddOnGroup: %v", err)
		}

		withVariants, err := item.WithVariantGroups([]catdomain.VariantGroup{size, crust})
		if err != nil {
			t.Fatalf("WithVariantGroups: %v", err)
		}
		withAll, err := withVariants.WithAddOnGroups([]catdomain.AddOnGroup{extras})
		if err != nil {
			t.Fatalf("WithAddOnGroups: %v", err)
		}
		if err := repo.SaveItem(ctx, withAll); err != nil {
			t.Fatalf("SaveItem: %v", err)
		}

		got, err := repo.Item(ctx, merchantID, "itm_1")
		if err != nil {
			t.Fatalf("Item: %v", err)
		}
		if len(got.VariantGroups) != 2 {
			t.Fatalf("variant groups = %d", len(got.VariantGroups))
		}
		if got.VariantGroups[0].Name != "Size" || !got.VariantGroups[0].Required {
			t.Errorf("the first group = %+v", got.VariantGroups[0])
		}
		if len(got.VariantGroups[0].Options) != 2 {
			t.Fatalf("size options = %d", len(got.VariantGroups[0].Options))
		}
		if got.VariantGroups[0].Options[1].PriceDelta.Minor() != 15_000 {
			t.Errorf("large = %s", got.VariantGroups[0].Options[1].PriceDelta)
		}
		if len(got.AddOnGroups) != 1 || len(got.AddOnGroups[0].Options) != 2 {
			t.Errorf("add-on groups = %+v", got.AddOnGroups)
		}
		if got.AddOnGroups[0].Options[0].Price.Minor() != 2_000 {
			t.Errorf("olives = %s", got.AddOnGroups[0].Options[0].Price)
		}

		// The owner drops the crust group and one extra.
		trimmedSize, err := catdomain.NewVariantGroup("vgr_size", "Size", true, 1, 1, []catdomain.VariantOption{
			mustVariantOption(t, "vop_regular", "Regular", 0),
		})
		if err != nil {
			t.Fatalf("NewVariantGroup: %v", err)
		}
		trimmedExtras, err := catdomain.NewAddOnGroup("agr_extras", "Extras", 0, 1, []catdomain.AddOn{
			mustAddOnOption(t, "aop_olives", "Olives", 2_500),
		})
		if err != nil {
			t.Fatalf("NewAddOnGroup: %v", err)
		}

		trimmed, err := got.WithVariantGroups([]catdomain.VariantGroup{trimmedSize})
		if err != nil {
			t.Fatalf("WithVariantGroups: %v", err)
		}
		trimmed, err = trimmed.WithAddOnGroups([]catdomain.AddOnGroup{trimmedExtras})
		if err != nil {
			t.Fatalf("WithAddOnGroups: %v", err)
		}
		if err := repo.SaveItem(ctx, trimmed); err != nil {
			t.Fatalf("SaveItem: %v", err)
		}

		got, err = repo.Item(ctx, merchantID, "itm_1")
		if err != nil {
			t.Fatalf("Item: %v", err)
		}
		if len(got.VariantGroups) != 1 || got.VariantGroups[0].Name != "Size" {
			t.Errorf("the crust group survived: %+v", got.VariantGroups)
		}
		if len(got.VariantGroups[0].Options) != 1 {
			t.Errorf("the large option survived: %+v", got.VariantGroups[0].Options)
		}
		if len(got.AddOnGroups[0].Options) != 1 || got.AddOnGroups[0].Options[0].Price.Minor() != 2_500 {
			t.Errorf("extras = %+v", got.AddOnGroups[0].Options)
		}
	})
}

// TestAnItemWithNoOptionsSavesCleanly exercises the prune statements with an
// empty list, which the ANY($2) form has to handle.
func TestAnItemWithNoOptionsSavesCleanly(t *testing.T) {
	withCatalogue(t, catdomain.Grocery, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Rice", 1)
		item := newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Chal", catdomain.Grocery, 7_500)

		got, err := repo.Item(ctx, merchantID, item.ID)
		if err != nil {
			t.Fatalf("Item: %v", err)
		}
		if len(got.VariantGroups) != 0 || len(got.AddOnGroups) != 0 {
			t.Errorf("groups = %+v / %+v", got.VariantGroups, got.AddOnGroups)
		}
	})
}

// TestABulkWriteIsOneTransaction, which is what stops a half-applied
// re-pricing.
func TestABulkWriteIsOneTransaction(t *testing.T) {
	withCatalogue(t, catdomain.Grocery, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Rice", 1)
		first := newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Chal", catdomain.Grocery, 7_500)
		second := newItem(t, ctx, repo, merchantID, category.ID, "itm_2", "Atap chal", catdomain.Grocery, 8_000)

		repriced, err := first.WithPrice(taka.Taka(9_000))
		if err != nil {
			t.Fatalf("WithPrice: %v", err)
		}
		hidden := second.WithActive(false)

		if err := repo.SaveItems(ctx, []catdomain.Item{repriced, hidden}); err != nil {
			t.Fatalf("SaveItems: %v", err)
		}

		got, err := repo.ItemsByID(ctx, merchantID, []string{"itm_1", "itm_2"})
		if err != nil {
			t.Fatalf("ItemsByID: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("items = %d", len(got))
		}
		if got[0].Price.Minor() != 9_000 {
			t.Errorf("price = %s", got[0].Price)
		}
		if got[1].Active {
			t.Error("the second item is still shown")
		}
	})
}

func TestDeletingAnItemTakesItsOptionsWithIt(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, tx pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Pizza", 1)
		item := newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Margherita", catdomain.Restaurant, 55_000)

		size, err := catdomain.NewVariantGroup("vgr_size", "Size", false, 0, 1, []catdomain.VariantOption{
			mustVariantOption(t, "vop_regular", "Regular", 0),
		})
		if err != nil {
			t.Fatalf("NewVariantGroup: %v", err)
		}
		withSizes, err := item.WithVariantGroups([]catdomain.VariantGroup{size})
		if err != nil {
			t.Fatalf("WithVariantGroups: %v", err)
		}
		if err := repo.SaveItem(ctx, withSizes); err != nil {
			t.Fatalf("SaveItem: %v", err)
		}

		if err := repo.DeleteItem(ctx, merchantID, item.ID); err != nil {
			t.Fatalf("DeleteItem: %v", err)
		}
		if _, err := repo.Item(ctx, merchantID, item.ID); !errors.Is(err, catdomain.ErrItemNotFound) {
			t.Errorf("error = %v, want ErrItemNotFound", err)
		}

		var groups int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM catalogue_variant_groups WHERE item_id = $1`, item.ID).Scan(&groups); err != nil {
			t.Fatalf("count groups: %v", err)
		}
		if groups != 0 {
			t.Errorf("%d variant groups outlived their item", groups)
		}
	})
}

// ------------------------------------------------------------------ combos

func TestACombosRoundTripsThroughPostgres(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)
		first := newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Kacchi", catdomain.Restaurant, 35_000)
		second := newItem(t, ctx, repo, merchantID, category.ID, "itm_2", "Borhani", catdomain.Restaurant, 6_000)

		combo, err := catdomain.NewCombo("cmb_1", merchantID, catdomain.Restaurant, catdomain.ComboDraft{
			Name: "কাচ্চি মিল", Description: "Kacchi with a borhani.",
			ImageURL: "/static/demo/meal.png", Price: taka.Taka(38_000), SortOrder: 1,
			Lines: []catdomain.ComboLine{
				{ItemID: first.ID, Quantity: 1},
				{ItemID: second.ID, Quantity: 2},
			},
		})
		if err != nil {
			t.Fatalf("NewCombo: %v", err)
		}
		if err := repo.SaveCombo(ctx, combo); err != nil {
			t.Fatalf("SaveCombo: %v", err)
		}

		got, err := repo.Combo(ctx, merchantID, "cmb_1")
		if err != nil {
			t.Fatalf("Combo: %v", err)
		}
		if got.Name != "কাচ্চি মিল" || got.Price.Minor() != 38_000 {
			t.Errorf("combo = %+v", got)
		}
		if len(got.Lines) != 2 {
			t.Fatalf("lines = %d", len(got.Lines))
		}
		if got.Lines[1].Quantity != 2 {
			t.Errorf("the second line = %+v", got.Lines[1])
		}
		if !got.Active || !got.Availability.Always {
			t.Errorf("defaults = active %v, always %v", got.Active, got.Availability.Always)
		}

		// Editing replaces the lines rather than adding to them.
		trimmed, err := catdomain.NewCombo("cmb_1", merchantID, catdomain.Restaurant, catdomain.ComboDraft{
			Name: "Kacchi meal", Price: taka.Taka(36_000),
			Lines: []catdomain.ComboLine{
				{ItemID: first.ID, Quantity: 1},
				{ItemID: second.ID, Quantity: 1},
			},
		})
		if err != nil {
			t.Fatalf("NewCombo: %v", err)
		}
		if err := repo.SaveCombo(ctx, trimmed); err != nil {
			t.Fatalf("SaveCombo: %v", err)
		}

		got, err = repo.Combo(ctx, merchantID, "cmb_1")
		if err != nil {
			t.Fatalf("Combo: %v", err)
		}
		if len(got.Lines) != 2 || got.Lines[1].Quantity != 1 {
			t.Errorf("lines after the edit = %+v", got.Lines)
		}
	})
}

func TestCombosAreListedAndFilteredByVisibility(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)
		first := newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Kacchi", catdomain.Restaurant, 35_000)
		second := newItem(t, ctx, repo, merchantID, category.ID, "itm_2", "Borhani", catdomain.Restaurant, 6_000)

		lines := []catdomain.ComboLine{
			{ItemID: first.ID, Quantity: 1}, {ItemID: second.ID, Quantity: 1},
		}
		shown, err := catdomain.NewCombo("cmb_1", merchantID, catdomain.Restaurant,
			catdomain.ComboDraft{Name: "Shown", Price: taka.Taka(38_000), Lines: lines})
		if err != nil {
			t.Fatalf("NewCombo: %v", err)
		}
		hidden, err := catdomain.NewCombo("cmb_2", merchantID, catdomain.Restaurant,
			catdomain.ComboDraft{Name: "Hidden", Price: taka.Taka(39_000), Lines: lines})
		if err != nil {
			t.Fatalf("NewCombo: %v", err)
		}
		if err := repo.SaveCombo(ctx, shown); err != nil {
			t.Fatalf("SaveCombo: %v", err)
		}
		if err := repo.SaveCombo(ctx, hidden.WithActive(false)); err != nil {
			t.Fatalf("SaveCombo: %v", err)
		}

		all, err := repo.Combos(ctx, merchantID, false)
		if err != nil {
			t.Fatalf("Combos: %v", err)
		}
		if len(all) != 2 {
			t.Errorf("all = %d, want 2", len(all))
		}

		active, err := repo.Combos(ctx, merchantID, true)
		if err != nil {
			t.Fatalf("Combos: %v", err)
		}
		if len(active) != 1 || active[0].ID != "cmb_1" {
			t.Errorf("active = %+v", active)
		}
	})
}

// TestAnItemInAComboCannotBeDeleted. The bundle would otherwise resolve to
// nothing at checkout, which the customer discovers and the owner does not.
func TestAnItemInAComboCannotBeDeleted(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)
		first := newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Kacchi", catdomain.Restaurant, 35_000)
		second := newItem(t, ctx, repo, merchantID, category.ID, "itm_2", "Borhani", catdomain.Restaurant, 6_000)

		combo, err := catdomain.NewCombo("cmb_1", merchantID, catdomain.Restaurant, catdomain.ComboDraft{
			Name: "Meal", Price: taka.Taka(38_000),
			Lines: []catdomain.ComboLine{
				{ItemID: first.ID, Quantity: 1}, {ItemID: second.ID, Quantity: 1},
			},
		})
		if err != nil {
			t.Fatalf("NewCombo: %v", err)
		}
		if err := repo.SaveCombo(ctx, combo); err != nil {
			t.Fatalf("SaveCombo: %v", err)
		}

		if err := repo.DeleteItem(ctx, merchantID, first.ID); err == nil {
			t.Error("an item a combo still sells was deleted")
		}
	})
}

func TestDeletingAComboLeavesItsItemsAlone(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)
		first := newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Kacchi", catdomain.Restaurant, 35_000)
		second := newItem(t, ctx, repo, merchantID, category.ID, "itm_2", "Borhani", catdomain.Restaurant, 6_000)

		combo, err := catdomain.NewCombo("cmb_1", merchantID, catdomain.Restaurant, catdomain.ComboDraft{
			Name: "Meal", Price: taka.Taka(38_000),
			Lines: []catdomain.ComboLine{
				{ItemID: first.ID, Quantity: 1}, {ItemID: second.ID, Quantity: 1},
			},
		})
		if err != nil {
			t.Fatalf("NewCombo: %v", err)
		}
		if err := repo.SaveCombo(ctx, combo); err != nil {
			t.Fatalf("SaveCombo: %v", err)
		}

		if err := repo.DeleteCombo(ctx, merchantID, "cmb_1"); err != nil {
			t.Fatalf("DeleteCombo: %v", err)
		}
		if _, err := repo.Combo(ctx, merchantID, "cmb_1"); !errors.Is(err, catdomain.ErrComboNotFound) {
			t.Errorf("error = %v, want ErrComboNotFound", err)
		}
		if _, err := repo.Item(ctx, merchantID, first.ID); err != nil {
			t.Errorf("deleting the combo took an item with it: %v", err)
		}
	})
}

// TestACombosAvailabilitySurvivesStorage.
func TestACombosAvailabilitySurvivesStorage(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Biryani", 1)
		first := newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Kacchi", catdomain.Restaurant, 35_000)
		second := newItem(t, ctx, repo, merchantID, category.ID, "itm_2", "Borhani", catdomain.Restaurant, 6_000)

		combo, err := catdomain.NewCombo("cmb_1", merchantID, catdomain.Restaurant, catdomain.ComboDraft{
			Name: "Lunch deal", Price: taka.Taka(38_000),
			Lines: []catdomain.ComboLine{
				{ItemID: first.ID, Quantity: 1}, {ItemID: second.ID, Quantity: 1},
			},
		})
		if err != nil {
			t.Fatalf("NewCombo: %v", err)
		}

		lunch, err := catdomain.NewAvailability(map[time.Weekday][]schedule.Window{
			time.Monday: {mustWindow(t, "12:00-15:00")},
		})
		if err != nil {
			t.Fatalf("NewAvailability: %v", err)
		}
		if err := repo.SaveCombo(ctx, combo.WithAvailability(lunch)); err != nil {
			t.Fatalf("SaveCombo: %v", err)
		}

		got, err := repo.Combo(ctx, merchantID, "cmb_1")
		if err != nil {
			t.Fatalf("Combo: %v", err)
		}
		if got.Availability.Always {
			t.Fatal("the combo's schedule was lost")
		}
		monday := time.Date(2026, time.March, 2, 13, 0, 0, 0, schedule.Bangladesh)
		if !got.Orderable(monday) {
			t.Error("the lunch combo is not orderable at 13:00 Monday")
		}
		if got.Orderable(monday.Add(6 * time.Hour)) {
			t.Error("the lunch combo is orderable at 19:00")
		}
	})
}

// mustVariantOption builds a variant option or fails the test.
func mustVariantOption(t *testing.T, id, name string, delta int64) catdomain.VariantOption {
	t.Helper()
	option, err := catdomain.NewVariantOption(id, name, taka.Taka(delta))
	if err != nil {
		t.Fatalf("NewVariantOption(%q): %v", name, err)
	}
	return option
}

// mustAddOnOption builds an add-on or fails the test.
func mustAddOnOption(t *testing.T, id, name string, price int64) catdomain.AddOn {
	t.Helper()
	option, err := catdomain.NewAddOn(id, name, taka.Taka(price))
	if err != nil {
		t.Fatalf("NewAddOn(%q): %v", name, err)
	}
	return option
}

// TestDeletingAnEmptySectionSucceeds, which is the path the use case allows
// once the owner has moved the items out.
func TestDeletingAnEmptySectionSucceeds(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, _ pgx.Tx, repo *catpg.Repository, merchantID string) {
		newCategory(t, ctx, repo, merchantID, "cat_1", "Seasonal", 1)

		if err := repo.DeleteCategory(ctx, merchantID, "cat_1"); err != nil {
			t.Fatalf("DeleteCategory: %v", err)
		}
		if _, err := repo.Category(ctx, merchantID, "cat_1"); !errors.Is(err, catdomain.ErrCategoryNotFound) {
			t.Errorf("error = %v, want ErrCategoryNotFound", err)
		}

		// Another shop's id does not delete this shop's section.
		newCategory(t, ctx, repo, merchantID, "cat_2", "Biryani", 1)
		if err := repo.DeleteCategory(ctx, "mch_other", "cat_2"); err != nil {
			t.Fatalf("DeleteCategory: %v", err)
		}
		if _, err := repo.Category(ctx, merchantID, "cat_2"); err != nil {
			t.Errorf("another shop's delete removed this one's section: %v", err)
		}
	})
}

// TestAGroupWithNoOptionsDoesNotBreakAMenu. The domain refuses to build one,
// so this row can only arrive by hand or from before that rule — and a menu
// that crashes on it is a shop that cannot trade. The reader skips the empty
// LEFT JOIN row instead.
func TestAGroupWithNoOptionsDoesNotBreakAMenu(t *testing.T) {
	withCatalogue(t, catdomain.Restaurant, func(ctx context.Context, tx pgx.Tx, repo *catpg.Repository, merchantID string) {
		category := newCategory(t, ctx, repo, merchantID, "cat_1", "Pizza", 1)
		item := newItem(t, ctx, repo, merchantID, category.ID, "itm_1", "Margherita", catdomain.Restaurant, 55_000)

		if _, err := tx.Exec(ctx, `
			INSERT INTO catalogue_variant_groups (id, item_id, name, required, min_choices, max_choices)
			VALUES ('vgr_empty', $1, 'Size', FALSE, 0, 1)`, item.ID); err != nil {
			t.Fatalf("insert empty variant group: %v", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO catalogue_addon_groups (id, item_id, name, min_choices, max_choices)
			VALUES ('agr_empty', $1, 'Extras', 0, 1)`, item.ID); err != nil {
			t.Fatalf("insert empty add-on group: %v", err)
		}

		got, err := repo.Item(ctx, merchantID, item.ID)
		if err != nil {
			t.Fatalf("Item: %v", err)
		}
		if len(got.VariantGroups) != 1 || len(got.VariantGroups[0].Options) != 0 {
			t.Errorf("variant groups = %+v, want one group with no options", got.VariantGroups)
		}
		if len(got.AddOnGroups) != 1 || len(got.AddOnGroups[0].Options) != 0 {
			t.Errorf("add-on groups = %+v, want one group with no options", got.AddOnGroups)
		}
	})
}
