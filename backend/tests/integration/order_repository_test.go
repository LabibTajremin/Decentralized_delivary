package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	merchantdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
	merchantpg "github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/infrastructure/persistence/postgres"
	orderports "github.com/rootlogic-lab/delivery/backend/internal/modules/order/application/ports"
	orderdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/order/domain"
	orderpg "github.com/rootlogic-lab/delivery/backend/internal/modules/order/infrastructure/persistence/postgres"
	taka "github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// withOrders runs a test inside a transaction that is always rolled back, with
// a shop already registered — the seed data an order needs to be about
// something.
func withOrders(t *testing.T, fn func(ctx context.Context, tx pgx.Tx, repo *orderpg.Repository, merchantID string)) {
	t.Helper()
	ctx := context.Background()
	conn := connect(t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	merchantID := "mch_order"
	shop := sampleMerchant(t, merchantID, "usr_order_owner", merchantdomain.TypeRestaurant)
	if err := merchantpg.New(tx, tx).Create(ctx, shop); err != nil {
		t.Fatalf("create shop: %v", err)
	}

	fn(ctx, tx, orderpg.New(tx, tx), merchantID)
}

func sampleOrder(t *testing.T, id, code, customerID, merchantID string) orderdomain.Order {
	t.Helper()
	order, err := orderdomain.NewOrder(orderdomain.Draft{
		ID: id, EventID: id + "_e1", Code: code, CustomerID: customerID, MerchantID: merchantID,
		Payment: orderdomain.PaymentCash,
		Lines: []orderdomain.Line{
			{
				ID: id + "_l1", Kind: "item", TargetID: "itm_1", Name: "Kacchi",
				UnitPrice: taka.Taka(35000), Quantity: 2, Note: "no chilli",
				Options: []orderdomain.Option{
					{GroupID: "grp_size", OptionID: "opt_full", Name: "Full", Price: taka.Taka(5000)},
					{GroupID: "grp_drink", OptionID: "opt_borhani", Name: "Borhani", Price: taka.Taka(4000)},
				},
			},
			{
				ID: id + "_l2", Kind: "combo", TargetID: "cmb_1", Name: "Family Meal",
				UnitPrice: taka.Taka(120000), Quantity: 1,
			},
		},
		Charges: orderdomain.Charges{
			Subtotal: taka.Taka(208000), Delivery: taka.Taka(7000),
			ExpansionSurcharge: taka.Taka(2000), Total: taka.Taka(215000),
			Expanded: true, DistanceM: 6200,
		},
		Destination: orderdomain.Destination{
			AddressID: "adr_1", Label: "Home", Recipient: "Fardin",
			Phone: "+8801711111111", Line1: "House 5", SingleLine: "House 5, Dhanmondi",
			Lat: 23.746, Lng: 90.375, AreaCode: "DHK-DHM", AreaName: "Dhanmondi",
			Directions: "Second gate",
		},
		Pickup: orderdomain.Pickup{
			MerchantID: merchantID, Name: "Star Kabab", Phone: "+8801711000001",
			SingleLine: "Road 27, Dhanmondi", Lat: 23.7455, Lng: 90.3738,
		},
		ExpansionLevel: 1,
		CODLimit:       taka.Taka(500000),
		Now:            time.Now().UTC().Truncate(time.Microsecond),
	})
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	return order
}

func TestAnOrderSurvivesARoundTrip(t *testing.T) {
	withOrders(t, func(ctx context.Context, _ pgx.Tx, repo *orderpg.Repository, merchantID string) {
		order := sampleOrder(t, "ord_1", "ABC234", "usr_1", merchantID)
		if err := repo.Create(ctx, order); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := repo.Order(ctx, "ord_1")
		if err != nil {
			t.Fatalf("Order: %v", err)
		}
		if got.Code != "ABC234" || got.Status != orderdomain.StatusPlaced ||
			got.Payment != orderdomain.PaymentCash {
			t.Fatalf("order = %+v", got)
		}
		// Money in poisha, frozen.
		if got.Charges.Total.Minor() != 215000 || got.Charges.ExpansionSurcharge.Minor() != 2000 {
			t.Errorf("charges = %+v", got.Charges)
		}
		if !got.Charges.Expanded || got.ExpansionLevel != 1 || got.Charges.DistanceM != 6200 {
			t.Errorf("expansion = %+v / level %d", got.Charges, got.ExpansionLevel)
		}
		// Both ends, copied.
		if got.Destination.Recipient != "Fardin" || got.Destination.Directions != "Second gate" ||
			got.Pickup.Name != "Star Kabab" {
			t.Errorf("places = %+v / %+v", got.Destination, got.Pickup)
		}

		if len(got.Lines) != 2 {
			t.Fatalf("lines = %d", len(got.Lines))
		}
		// The order the customer added things in, preserved.
		if got.Lines[0].ID != "ord_1_l1" || got.Lines[1].Kind != "combo" {
			t.Errorf("line order = %+v", got.Lines)
		}
		if got.Lines[0].Note != "no chilli" || got.Lines[0].UnitPrice.Minor() != 35000 {
			t.Errorf("line = %+v", got.Lines[0])
		}
		if len(got.Lines[0].Options) != 2 || got.Lines[0].Options[0].Name != "Full" {
			t.Errorf("options = %+v", got.Lines[0].Options)
		}
		if len(got.Lines[1].Options) != 0 {
			t.Errorf("a combo came back with options: %+v", got.Lines[1].Options)
		}

		// The history starts at the placement, so it is complete from the
		// beginning rather than from the first change.
		if len(got.Events) != 1 || got.Events[0].Status != orderdomain.StatusPlaced ||
			got.Events[0].Actor != orderdomain.ActorCustomer {
			t.Errorf("events = %+v", got.Events)
		}
	})
}

func TestReadingAnOrderThatIsNotThere(t *testing.T) {
	withOrders(t, func(ctx context.Context, _ pgx.Tx, repo *orderpg.Repository, _ string) {
		if _, err := repo.Order(ctx, "ord_nope"); err == nil {
			t.Fatal("a missing order was found")
		}
	})
}

// The concurrency guard. Two riders tapping "picked up" at the same moment
// cannot both succeed: the expected status is in the UPDATE's own WHERE clause.
func TestATransitionOnlyAppliesOnce(t *testing.T) {
	withOrders(t, func(ctx context.Context, _ pgx.Tx, repo *orderpg.Repository, merchantID string) {
		order := sampleOrder(t, "ord_2", "DEF345", "usr_1", merchantID)
		if err := repo.Create(ctx, order); err != nil {
			t.Fatalf("Create: %v", err)
		}

		at := time.Now().UTC().Truncate(time.Microsecond)
		event := orderdomain.Event{
			ID: "ord_2_e2", Status: orderdomain.StatusAccepted,
			Actor: orderdomain.ActorMerchant, ActorID: "usr_order_owner", At: at,
		}
		if err := repo.AppendTransition(ctx, "ord_2", orderdomain.StatusPlaced, event); err != nil {
			t.Fatalf("AppendTransition: %v", err)
		}

		// The same move again, from the status the caller still thinks it is
		// in. Refused, because it is no longer there.
		event.ID = "ord_2_e3"
		if err := repo.AppendTransition(ctx, "ord_2", orderdomain.StatusPlaced, event); err == nil {
			t.Fatal("the same transition applied twice")
		}

		got, err := repo.Order(ctx, "ord_2")
		if err != nil {
			t.Fatalf("Order: %v", err)
		}
		if got.Status != orderdomain.StatusAccepted {
			t.Fatalf("status = %q", got.Status)
		}
		// And the refused attempt left no event behind.
		if len(got.Events) != 2 {
			t.Errorf("events = %+v", got.Events)
		}
	})
}

func TestListingOrders(t *testing.T) {
	withOrders(t, func(ctx context.Context, _ pgx.Tx, repo *orderpg.Repository, merchantID string) {
		for i, id := range []string{"ord_a", "ord_b", "ord_c"} {
			order := sampleOrder(t, id, "CODE"+string(rune('A'+i)), "usr_list", merchantID)
			order.PlacedAt = order.PlacedAt.Add(time.Duration(i) * time.Minute)
			if err := repo.Create(ctx, order); err != nil {
				t.Fatalf("Create %s: %v", id, err)
			}
		}
		// One of somebody else's, to prove the scoping.
		if err := repo.Create(ctx, sampleOrder(t, "ord_other", "OTHER1", "usr_other", merchantID)); err != nil {
			t.Fatalf("Create: %v", err)
		}

		orders, total, err := repo.Orders(ctx, orderports.Filter{CustomerID: "usr_list", Limit: 10})
		if err != nil {
			t.Fatalf("Orders: %v", err)
		}
		if total != 3 || len(orders) != 3 {
			t.Fatalf("got %d of %d", len(orders), total)
		}
		// Newest first.
		if orders[0].ID != "ord_c" || orders[2].ID != "ord_a" {
			t.Errorf("order = %s, %s, %s", orders[0].ID, orders[1].ID, orders[2].ID)
		}
		// Lines and history came back with every one, in one sweep rather than
		// a query each.
		for _, o := range orders {
			if len(o.Lines) != 2 || len(o.Events) != 1 {
				t.Errorf("%s: %d lines, %d events", o.ID, len(o.Lines), len(o.Events))
			}
			if len(o.Lines[0].Options) != 2 {
				t.Errorf("%s: options = %+v", o.ID, o.Lines[0].Options)
			}
		}

		// The shop's queue is the same rows scoped the other way.
		shopOrders, shopTotal, err := repo.Orders(ctx, orderports.Filter{MerchantID: merchantID, Limit: 10})
		if err != nil {
			t.Fatalf("Orders: %v", err)
		}
		if shopTotal != 4 || len(shopOrders) != 4 {
			t.Fatalf("shop queue = %d of %d", len(shopOrders), shopTotal)
		}

		// Paging keeps the total honest.
		page, pageTotal, err := repo.Orders(ctx, orderports.Filter{CustomerID: "usr_list", Limit: 2, Offset: 1})
		if err != nil {
			t.Fatalf("Orders: %v", err)
		}
		if pageTotal != 3 || len(page) != 2 || page[0].ID != "ord_b" {
			t.Fatalf("page = %+v of %d", page, pageTotal)
		}

		// A page past the end is empty, not an error.
		none, stillThree, err := repo.Orders(ctx, orderports.Filter{CustomerID: "usr_list", Limit: 2, Offset: 99})
		if err != nil {
			t.Fatalf("Orders: %v", err)
		}
		if len(none) != 0 || stillThree != 3 {
			t.Fatalf("past the end: %d of %d", len(none), stillThree)
		}
	})
}

func TestTheLiveFilterAndStatusFilter(t *testing.T) {
	withOrders(t, func(ctx context.Context, _ pgx.Tx, repo *orderpg.Repository, merchantID string) {
		for _, id := range []string{"ord_live", "ord_done"} {
			if err := repo.Create(ctx, sampleOrder(t, id, "C"+id[4:], "usr_f", merchantID)); err != nil {
				t.Fatalf("Create: %v", err)
			}
		}
		done := orderdomain.Event{
			ID: "e_done", Status: orderdomain.StatusCancelled,
			Actor: orderdomain.ActorCustomer, At: time.Now().UTC(),
		}
		if err := repo.AppendTransition(ctx, "ord_done", orderdomain.StatusPlaced, done); err != nil {
			t.Fatalf("AppendTransition: %v", err)
		}

		live, total, err := repo.Orders(ctx, orderports.Filter{
			CustomerID: "usr_f", LiveOnly: true, Limit: 10,
		})
		if err != nil {
			t.Fatalf("Orders: %v", err)
		}
		if total != 1 || live[0].ID != "ord_live" {
			t.Fatalf("live = %+v of %d", live, total)
		}

		byStatus, statusTotal, err := repo.Orders(ctx, orderports.Filter{
			CustomerID: "usr_f", Statuses: []orderdomain.Status{orderdomain.StatusCancelled}, Limit: 10,
		})
		if err != nil {
			t.Fatalf("Orders: %v", err)
		}
		if statusTotal != 1 || byStatus[0].ID != "ord_done" {
			t.Fatalf("by status = %+v of %d", byStatus, statusTotal)
		}
	})
}

// A filter with neither a customer nor a merchant would return everybody's
// orders. It returns nothing instead: that is a caller bug, and answering it
// with the whole table is how a caller bug becomes a data leak.
func TestAnUnscopedFilterReturnsNothing(t *testing.T) {
	withOrders(t, func(ctx context.Context, _ pgx.Tx, repo *orderpg.Repository, merchantID string) {
		if err := repo.Create(ctx, sampleOrder(t, "ord_x", "XYZ234", "usr_x", merchantID)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		orders, total, err := repo.Orders(ctx, orderports.Filter{Limit: 10})
		if err != nil {
			t.Fatalf("Orders: %v", err)
		}
		if total != 0 || len(orders) != 0 {
			t.Fatalf("an unscoped filter returned %d of %d", len(orders), total)
		}
	})
}

// The phase's second acceptance criterion, at the storage level: one key, one
// order, enforced by a primary key rather than by a check the caller might skip.
func TestAnIdempotencyKeyIsClaimedOnce(t *testing.T) {
	withOrders(t, func(ctx context.Context, _ pgx.Tx, repo *orderpg.Repository, merchantID string) {
		if err := repo.Create(ctx, sampleOrder(t, "ord_k1", "KEY234", "usr_k", merchantID)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := repo.ClaimIdempotencyKey(ctx, "usr_k", "attempt-1", "ord_k1"); err != nil {
			t.Fatalf("ClaimIdempotencyKey: %v", err)
		}

		got, found, err := repo.ByIdempotencyKey(ctx, "usr_k", "attempt-1")
		if err != nil || !found {
			t.Fatalf("found = %v, err = %v", found, err)
		}
		if got.ID != "ord_k1" || len(got.Lines) != 2 {
			t.Fatalf("order = %+v", got)
		}

		// The key is scoped to the customer: two customers using the same
		// client library may generate the same key.
		if _, found, err := repo.ByIdempotencyKey(ctx, "usr_other", "attempt-1"); err != nil || found {
			t.Fatalf("another customer's key resolved: found = %v, err = %v", found, err)
		}
		// An unknown key is not an error.
		if _, found, err := repo.ByIdempotencyKey(ctx, "usr_k", "never-used"); err != nil || found {
			t.Fatalf("found = %v, err = %v", found, err)
		}
	})
}

// A second claim on the same key fails, so a racing retry cannot write a second
// order and then overwrite the mapping. Its own test because the failed INSERT
// poisons the surrounding transaction, which is exactly what it should do.
func TestAnIdempotencyKeyCannotBeClaimedTwice(t *testing.T) {
	withOrders(t, func(ctx context.Context, _ pgx.Tx, repo *orderpg.Repository, merchantID string) {
		for _, id := range []string{"ord_d1", "ord_d2"} {
			if err := repo.Create(ctx, sampleOrder(t, id, "D"+id[4:]+"23", "usr_d", merchantID)); err != nil {
				t.Fatalf("Create %s: %v", id, err)
			}
		}
		if err := repo.ClaimIdempotencyKey(ctx, "usr_d", "attempt-1", "ord_d1"); err != nil {
			t.Fatalf("ClaimIdempotencyKey: %v", err)
		}
		if err := repo.ClaimIdempotencyKey(ctx, "usr_d", "attempt-1", "ord_d2"); err == nil {
			t.Fatal("a key was claimed twice")
		}
	})
}

// The order code is unique, so support can search on what a customer reads out.
func TestOrderCodesAreUnique(t *testing.T) {
	withOrders(t, func(ctx context.Context, _ pgx.Tx, repo *orderpg.Repository, merchantID string) {
		if err := repo.Create(ctx, sampleOrder(t, "ord_c1", "SAME23", "usr_1", merchantID)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := repo.Create(ctx, sampleOrder(t, "ord_c2", "SAME23", "usr_2", merchantID)); err == nil {
			t.Fatal("two orders share a code")
		}
	})
}

func TestOrderNewFromPoolIsWired(t *testing.T) {
	if repo := orderpg.NewFromPool(nil); repo == nil {
		t.Error("NewFromPool must return a repository")
	}
}
