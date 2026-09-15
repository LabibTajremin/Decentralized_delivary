package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	cartdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	cartpg "github.com/rootlogic-lab/delivery/backend/internal/modules/cart/infrastructure/persistence/postgres"
	merchantdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	merchantpg "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/infrastructure/persistence/postgres"
	taka "github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// withCart runs a test inside a transaction that is always rolled back, with a
// shop already registered — a cart has a foreign key into merchants.
func withCart(t *testing.T, fn func(ctx context.Context, tx pgx.Tx, repo *cartpg.Repository, merchantID string)) {
	t.Helper()
	ctx := context.Background()
	conn := connect(t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	merchantID := "mch_cart"
	shop := sampleMerchant(t, merchantID, "usr_cart_owner", merchantdomain.TypeRestaurant)
	if err := merchantpg.New(tx, tx).Create(ctx, shop); err != nil {
		t.Fatalf("create shop: %v", err)
	}

	fn(ctx, tx, cartpg.New(tx, tx), merchantID)
}

func sampleCart(t *testing.T, id, userID, merchantID string) cartdomain.Cart {
	t.Helper()
	cart, err := cartdomain.NewCart(id, userID, merchantID, time.Now().UTC().Truncate(time.Microsecond))
	if err != nil {
		t.Fatalf("NewCart: %v", err)
	}
	return cart
}

func sampleLine(id, target string, minor int64, quantity int, options ...cartdomain.Option) cartdomain.Line {
	return cartdomain.Line{
		ID: id, Kind: cartdomain.KindItem, TargetID: target, Name: target,
		UnitPrice: taka.Taka(minor), Quantity: quantity, Options: options,
	}
}

func TestACartSurvivesARoundTrip(t *testing.T) {
	withCart(t, func(ctx context.Context, _ pgx.Tx, repo *cartpg.Repository, merchantID string) {
		cart := sampleCart(t, "crt_1", "usr_cart_1", merchantID)
		cart.PlaceAt("adr_1", 23.7465, 90.3760)
		line := sampleLine("cln_1", "itm_1", 25000, 2,
			cartdomain.Option{GroupID: "grp_size", OptionID: "opt_large", Name: "Large", Price: taka.Taka(5000)},
			cartdomain.Option{GroupID: "grp_extra", OptionID: "opt_cheese", Name: "Cheese", Price: taka.Taka(2500)},
		)
		line.Note = "no chilli"
		if err := cart.Add(merchantID, line); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := cart.Add(merchantID, sampleLine("cln_2", "itm_2", 12000, 1)); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := repo.Save(ctx, cart); err != nil {
			t.Fatalf("Save: %v", err)
		}

		got, found, err := repo.OfUser(ctx, "usr_cart_1")
		if err != nil || !found {
			t.Fatalf("OfUser: found = %v, err = %v", found, err)
		}
		if got.ID != cart.ID || got.MerchantID != merchantID || got.AddressID != "adr_1" {
			t.Fatalf("cart = %+v", got)
		}
		if got.Lat != 23.7465 || got.Lng != 90.3760 {
			t.Errorf("delivery point = %v, %v", got.Lat, got.Lng)
		}
		if len(got.Lines) != 2 {
			t.Fatalf("lines = %d, want 2", len(got.Lines))
		}
		// The order the customer added things in, preserved, so the cart screen
		// does not reshuffle itself between reads.
		if got.Lines[0].ID != "cln_1" || got.Lines[1].ID != "cln_2" {
			t.Errorf("line order = %s, %s", got.Lines[0].ID, got.Lines[1].ID)
		}
		first := got.Lines[0]
		if first.Quantity != 2 || first.UnitPrice.Minor() != 25000 || first.Note != "no chilli" {
			t.Errorf("line = %+v", first)
		}
		if len(first.Options) != 2 || first.Options[0].OptionID != "opt_large" ||
			first.Options[0].Price.Minor() != 5000 {
			t.Errorf("options = %+v", first.Options)
		}
		// The second line has none, and comes back with none rather than nil
		// confusion.
		if len(got.Lines[1].Options) != 0 {
			t.Errorf("a line with no options came back with %+v", got.Lines[1].Options)
		}
	})
}

func TestNoCartIsNotAnError(t *testing.T) {
	withCart(t, func(ctx context.Context, _ pgx.Tx, repo *cartpg.Repository, _ string) {
		got, found, err := repo.OfUser(ctx, "usr_nobody")
		if err != nil {
			t.Fatalf("OfUser: %v", err)
		}
		if found || got.ID != "" {
			t.Fatalf("a customer with no cart returned %+v", got)
		}
	})
}

// Save rewrites the lines rather than diffing them. A cart is small, the whole
// of it changed together, and a diff would be a second implementation of what
// the cart now is.
func TestSavingACartReplacesItsLines(t *testing.T) {
	withCart(t, func(ctx context.Context, tx pgx.Tx, repo *cartpg.Repository, merchantID string) {
		cart := sampleCart(t, "crt_2", "usr_cart_2", merchantID)
		if err := cart.Add(merchantID, sampleLine("cln_1", "itm_1", 10000, 1)); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := cart.Add(merchantID, sampleLine("cln_2", "itm_2", 20000, 1,
			cartdomain.Option{GroupID: "g", OptionID: "o", Name: "O", Price: taka.Taka(500)})); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := repo.Save(ctx, cart); err != nil {
			t.Fatalf("Save: %v", err)
		}

		if err := cart.Remove("cln_2"); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if err := cart.SetQuantity("cln_1", 7); err != nil {
			t.Fatalf("SetQuantity: %v", err)
		}
		if err := repo.Save(ctx, cart); err != nil {
			t.Fatalf("Save again: %v", err)
		}

		got, _, err := repo.OfUser(ctx, "usr_cart_2")
		if err != nil {
			t.Fatalf("OfUser: %v", err)
		}
		if len(got.Lines) != 1 || got.Lines[0].ID != "cln_1" || got.Lines[0].Quantity != 7 {
			t.Fatalf("cart = %+v", got.Lines)
		}
		// The removed line's options went with it, by cascade. Checked in SQL
		// rather than through the repository, because the repository can only
		// read options that still have a line — which is the very thing being
		// asserted.
		var orphans int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM cart_line_options o
			WHERE NOT EXISTS (SELECT 1 FROM cart_lines l WHERE l.id = o.line_id)`).
			Scan(&orphans); err != nil {
			t.Fatalf("count orphans: %v", err)
		}
		if orphans != 0 {
			t.Errorf("%d option rows outlived their line", orphans)
		}
	})
}

// One cart per customer. Two open carts is how somebody ends up ordering from a
// shop they had forgotten they were in the middle of.
func TestOneCartPerCustomer(t *testing.T) {
	withCart(t, func(ctx context.Context, _ pgx.Tx, repo *cartpg.Repository, merchantID string) {
		if err := repo.Save(ctx, sampleCart(t, "crt_a", "usr_cart_3", merchantID)); err != nil {
			t.Fatalf("Save: %v", err)
		}
		err := repo.Save(ctx, sampleCart(t, "crt_b", "usr_cart_3", merchantID))
		if err == nil {
			t.Fatal("a customer was given a second cart")
		}
	})
}

func TestDeletingACartTakesItsLinesWithIt(t *testing.T) {
	withCart(t, func(ctx context.Context, _ pgx.Tx, repo *cartpg.Repository, merchantID string) {
		cart := sampleCart(t, "crt_4", "usr_cart_4", merchantID)
		if err := cart.Add(merchantID, sampleLine("cln_1", "itm_1", 10000, 1,
			cartdomain.Option{GroupID: "g", OptionID: "o", Name: "O", Price: taka.Taka(0)})); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := repo.Save(ctx, cart); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := repo.Delete(ctx, "crt_4"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, found, err := repo.OfUser(ctx, "usr_cart_4"); err != nil || found {
			t.Fatalf("the cart survived deletion: found = %v, err = %v", found, err)
		}

		// Deleting a cart that is not there is not an error: the caller's
		// intent is satisfied either way.
		if err := repo.Delete(ctx, "crt_gone"); err != nil {
			t.Errorf("deleting nothing failed: %v", err)
		}
	})
}

// A cart line deliberately outlives the item it points at, because the cart's
// job on the next read is to say "the shop has removed this" — which it cannot
// do if the row vanished with the item.
func TestALineOutlivesTheItemItPointsAt(t *testing.T) {
	withCart(t, func(ctx context.Context, _ pgx.Tx, repo *cartpg.Repository, merchantID string) {
		cart := sampleCart(t, "crt_5", "usr_cart_5", merchantID)
		if err := cart.Add(merchantID, sampleLine("cln_1", "itm_never_existed", 10000, 1)); err != nil {
			t.Fatalf("Add: %v", err)
		}
		if err := repo.Save(ctx, cart); err != nil {
			t.Fatalf("Save: %v", err)
		}
		got, found, err := repo.OfUser(ctx, "usr_cart_5")
		if err != nil || !found {
			t.Fatalf("OfUser: found = %v, err = %v", found, err)
		}
		if got.Lines[0].TargetID != "itm_never_existed" {
			t.Errorf("line = %+v", got.Lines[0])
		}
	})
}
