package integration

import (
	"context"
	"testing"
	"time"

	paymentdomain "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	paymentpg "github.com/rootlogic-lab/delivery/backend/internal/modules/payment/infrastructure/persistence/postgres"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	taka "github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// withPayment runs a test inside a transaction that is always rolled back.
// Payment does not reference another module's schema (2.6), so there is no
// seed data to set up first — only the two tables this module owns.
func withPayment(t *testing.T, fn func(ctx context.Context, repo *paymentpg.Repository)) {
	t.Helper()
	ctx := context.Background()
	conn := connect(t)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	fn(ctx, paymentpg.New(tx, tx))
}

// paymentClock is a fixed instant the whole file shares, truncated because
// Postgres keeps microseconds and Go keeps nanoseconds.
func paymentClock() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func samplePayment(t *testing.T, id, orderID, customerID string, minor int64) paymentdomain.Payment {
	t.Helper()
	p, err := paymentdomain.NewPayment(id, orderID, customerID, "manual", taka.Taka(minor), paymentClock())
	if err != nil {
		t.Fatalf("NewPayment: %v", err)
	}
	return p
}

func sampleCollection(t *testing.T, id, orderID, partnerID string, minor int64) paymentdomain.Collection {
	t.Helper()
	c, err := paymentdomain.NewCollection(id, orderID, partnerID, taka.Taka(minor), paymentClock())
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	return c
}

// A payment round-trips with its amount, its gateway and its reference —
// the identifier a webhook is matched back by.
func TestAPaymentRoundTrips(t *testing.T) {
	withPayment(t, func(ctx context.Context, repo *paymentpg.Repository) {
		saved := samplePayment(t, "pay_1", "ord_1", "usr_1", 50000)
		if err := repo.CreatePayment(ctx, saved); err != nil {
			t.Fatalf("CreatePayment: %v", err)
		}

		got, err := repo.Payment(ctx, saved.ID)
		if err != nil {
			t.Fatalf("Payment: %v", err)
		}
		if got.OrderID != "ord_1" || got.CustomerID != "usr_1" || got.Gateway != "manual" {
			t.Fatalf("payment = %+v", got)
		}
		if got.Amount.Minor() != 50000 || got.Amount.Currency() != taka.BDT {
			t.Fatalf("amount = %+v", got.Amount)
		}
		if got.Status != paymentdomain.StatusPending {
			t.Fatalf("status = %q", got.Status)
		}

		byRef, err := repo.ByReference(ctx, saved.Reference)
		if err != nil || byRef.ID != saved.ID {
			t.Fatalf("ByReference = %+v, %v", byRef, err)
		}

		if _, err := repo.Payment(ctx, "pay_missing"); errs.CodeOf(err) != "payment_not_found" {
			t.Fatalf("err = %v", err)
		}
		if _, err := repo.ByReference(ctx, "ref_missing"); errs.CodeOf(err) != "payment_not_found" {
			t.Fatalf("err = %v", err)
		}
	})
}

// One open checkout per order: the partial unique index refuses a second
// pending attempt while the first is still open.
//
// A single failed statement leaves the rest of a Postgres transaction
// unusable until it is rolled back (SQLSTATE 25P02) — true here only because
// this test's isolation wraps everything in one transaction; the production
// repository runs each call over the pool directly, where a refused write
// never touches any other request. So the refusal is checked here, and
// PendingForOrder is checked in TestPendingForOrderFindsTheOpenAttempt, each
// in a transaction of its own — not because they are unrelated facts, but
// because proving both in one would be proving something about this test
// harness, not about the repository.
func TestOnlyOnePendingPaymentPerOrder(t *testing.T) {
	withPayment(t, func(ctx context.Context, repo *paymentpg.Repository) {
		if err := repo.CreatePayment(ctx, samplePayment(t, "pay_1", "ord_1", "usr_1", 50000)); err != nil {
			t.Fatalf("CreatePayment: %v", err)
		}
		err := repo.CreatePayment(ctx, samplePayment(t, "pay_2", "ord_1", "usr_1", 50000))
		if errs.CodeOf(err) != "checkout_in_progress" {
			t.Fatalf("a second open checkout for one order = %v", err)
		}
	})
}

func TestPendingForOrderFindsTheOpenAttempt(t *testing.T) {
	withPayment(t, func(ctx context.Context, repo *paymentpg.Repository) {
		if err := repo.CreatePayment(ctx, samplePayment(t, "pay_1", "ord_1", "usr_1", 50000)); err != nil {
			t.Fatalf("CreatePayment: %v", err)
		}

		pending, found, err := repo.PendingForOrder(ctx, "ord_1")
		if err != nil || !found || pending.ID != "pay_1" {
			t.Fatalf("PendingForOrder = %+v, %v, %v", pending, found, err)
		}
		_, found, err = repo.PendingForOrder(ctx, "ord_none")
		if err != nil || found {
			t.Fatalf("an order with no attempt = %v, %v", found, err)
		}
	})
}

// A retried checkout after a failed attempt is a second row, and
// LatestForOrder is which one is authoritative.
func TestLatestForOrderIsTheMostRecentAttempt(t *testing.T) {
	withPayment(t, func(ctx context.Context, repo *paymentpg.Repository) {
		first := samplePayment(t, "pay_1", "ord_1", "usr_1", 50000)
		if err := repo.CreatePayment(ctx, first); err != nil {
			t.Fatalf("CreatePayment: %v", err)
		}
		if err := first.Fail("card declined", paymentClock()); err != nil {
			t.Fatalf("Fail: %v", err)
		}
		if err := repo.Save(ctx, first, paymentdomain.StatusPending); err != nil {
			t.Fatalf("Save: %v", err)
		}

		second := samplePayment(t, "pay_2", "ord_1", "usr_1", 50000)
		second.CreatedAt = first.CreatedAt.Add(time.Second)
		if err := repo.CreatePayment(ctx, second); err != nil {
			t.Fatalf("CreatePayment (retry): %v", err)
		}

		latest, found, err := repo.LatestForOrder(ctx, "ord_1")
		if err != nil || !found || latest.ID != "pay_2" {
			t.Fatalf("LatestForOrder = %+v, %v, %v", latest, found, err)
		}
	})
}

// The compare-and-set that makes a webhook idempotent: a save from a status
// the row is no longer in is refused rather than silently applied.
func TestSavingAPaymentIsCompareAndSet(t *testing.T) {
	withPayment(t, func(ctx context.Context, repo *paymentpg.Repository) {
		p := samplePayment(t, "pay_1", "ord_1", "usr_1", 50000)
		if err := repo.CreatePayment(ctx, p); err != nil {
			t.Fatalf("CreatePayment: %v", err)
		}

		if err := p.Capture("gw_ref_1", taka.Taka(50000), paymentClock()); err != nil {
			t.Fatalf("Capture: %v", err)
		}
		if err := repo.Save(ctx, p, paymentdomain.StatusPending); err != nil {
			t.Fatalf("Save: %v", err)
		}

		got, err := repo.Payment(ctx, p.ID)
		if err != nil || got.Status != paymentdomain.StatusCaptured || got.GatewayRef != "gw_ref_1" {
			t.Fatalf("payment = %+v, err = %v", got, err)
		}

		// A second save still claiming the payment was pending finds it
		// already moved.
		err = repo.Save(ctx, p, paymentdomain.StatusPending)
		if errs.CodeOf(err) != "payment_moved" {
			t.Fatalf("a stale save = %v, want payment_moved", err)
		}
	})
}

// A collection round-trips, and the null remitted_at comes back as the zero
// time rather than an epoch timestamp leaking out of the COALESCE.
func TestACollectionRoundTrips(t *testing.T) {
	withPayment(t, func(ctx context.Context, repo *paymentpg.Repository) {
		saved := sampleCollection(t, "col_1", "ord_1", "PTR-1", 50000)
		if err := repo.CreateCollection(ctx, saved); err != nil {
			t.Fatalf("CreateCollection: %v", err)
		}

		got, err := repo.Collection(ctx, saved.ID)
		if err != nil {
			t.Fatalf("Collection: %v", err)
		}
		if got.OrderID != "ord_1" || got.PartnerID != "PTR-1" || got.Amount.Minor() != 50000 {
			t.Fatalf("collection = %+v", got)
		}
		if got.Status != paymentdomain.StatusHeld || !got.RemittedAt.IsZero() {
			t.Fatalf("a held collection has a remit time: %+v", got)
		}

		if _, err := repo.Collection(ctx, "col_missing"); errs.CodeOf(err) != "collection_not_found" {
			t.Fatalf("err = %v", err)
		}
	})
}

// One collection per order: a delivery happens once, so its cash is recorded
// once, guarded at the database.
func TestOneCollectionPerOrder(t *testing.T) {
	withPayment(t, func(ctx context.Context, repo *paymentpg.Repository) {
		if err := repo.CreateCollection(ctx, sampleCollection(t, "col_1", "ord_1", "PTR-1", 50000)); err != nil {
			t.Fatalf("CreateCollection: %v", err)
		}
		err := repo.CreateCollection(ctx, sampleCollection(t, "col_2", "ord_1", "PTR-2", 40000))
		if errs.CodeOf(err) != "collection_exists" {
			t.Fatalf("a second collection for one order = %v", err)
		}
	})
}

// ForPartner returns held before remitted, oldest first within each — what a
// "please remit" screen and the ledger both read.
func TestForPartnerOrdersHeldFirst(t *testing.T) {
	withPayment(t, func(ctx context.Context, repo *paymentpg.Repository) {
		now := paymentClock()

		older := sampleCollection(t, "col_older", "ord_1", "PTR-1", 10000)
		older.CollectedAt = now.Add(-time.Hour)
		newer := sampleCollection(t, "col_newer", "ord_2", "PTR-1", 20000)
		newer.CollectedAt = now
		remitted := sampleCollection(t, "col_remitted", "ord_3", "PTR-1", 30000)
		for _, c := range []paymentdomain.Collection{older, newer, remitted} {
			if err := repo.CreateCollection(ctx, c); err != nil {
				t.Fatalf("CreateCollection: %v", err)
			}
		}
		if err := repo.Remit(ctx, "PTR-1", []string{"col_remitted"}, "DEPOSIT-1", now); err != nil {
			t.Fatalf("Remit: %v", err)
		}

		list, err := repo.ForPartner(ctx, "PTR-1")
		if err != nil {
			t.Fatalf("ForPartner: %v", err)
		}
		if len(list) != 3 {
			t.Fatalf("list = %+v", list)
		}
		if list[0].ID != "col_older" || list[1].ID != "col_newer" || list[2].ID != "col_remitted" {
			t.Fatalf("order = %s, %s, %s", list[0].ID, list[1].ID, list[2].ID)
		}
		if list[2].RemittanceRef != "DEPOSIT-1" || list[2].RemittedAt.IsZero() {
			t.Fatalf("remitted row = %+v", list[2])
		}

		none, err := repo.ForPartner(ctx, "PTR-nobody")
		if err != nil || len(none) != 0 {
			t.Fatalf("a partner with nothing = %+v, %v", none, err)
		}
	})
}

// Remit is all-or-nothing, inside one transaction: a batch with one collection
// that is not this partner's to remit rolls the whole batch back.
func TestRemitIsAllOrNothing(t *testing.T) {
	withPayment(t, func(ctx context.Context, repo *paymentpg.Repository) {
		mine := sampleCollection(t, "col_mine", "ord_1", "PTR-1", 10000)
		theirs := sampleCollection(t, "col_theirs", "ord_2", "PTR-2", 20000)
		for _, c := range []paymentdomain.Collection{mine, theirs} {
			if err := repo.CreateCollection(ctx, c); err != nil {
				t.Fatalf("CreateCollection: %v", err)
			}
		}

		err := repo.Remit(ctx, "PTR-1", []string{"col_mine", "col_theirs"}, "DEPOSIT-1", paymentClock())
		if errs.CodeOf(err) != "collection_not_remittable" {
			t.Fatalf("err = %v", err)
		}

		// col_mine must not have moved even though it was valid on its own.
		got, err := repo.Collection(ctx, "col_mine")
		if err != nil || got.Status != paymentdomain.StatusHeld {
			t.Fatalf("a partially valid batch remitted something: %+v, %v", got, err)
		}
	})
}

// Remitting an already-remitted collection is refused the same way as
// remitting somebody else's — the WHERE clause encodes both invariants at
// once.
func TestRemittingAnAlreadyRemittedCollection(t *testing.T) {
	withPayment(t, func(ctx context.Context, repo *paymentpg.Repository) {
		c := sampleCollection(t, "col_1", "ord_1", "PTR-1", 10000)
		if err := repo.CreateCollection(ctx, c); err != nil {
			t.Fatalf("CreateCollection: %v", err)
		}
		if err := repo.Remit(ctx, "PTR-1", []string{"col_1"}, "DEPOSIT-1", paymentClock()); err != nil {
			t.Fatalf("Remit: %v", err)
		}
		err := repo.Remit(ctx, "PTR-1", []string{"col_1"}, "DEPOSIT-2", paymentClock())
		if errs.CodeOf(err) != "collection_not_remittable" {
			t.Fatalf("a second remittance = %v", err)
		}
	})
}

// NewFromPool is what the running binary uses; New is what these tests use.
func TestThePaymentProductionConstructor(t *testing.T) {
	if paymentpg.NewFromPool(nil) == nil {
		t.Fatal("NewFromPool built nothing")
	}
}
