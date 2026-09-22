package payment

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

func TestRecordingACashCollection(t *testing.T) {
	ctx := context.Background()
	r := newRig()

	if err := r.collections.Record(ctx, "ord_1", "PTR-1", 50000); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(r.repo.collections) != 1 {
		t.Fatalf("%d collections written, want 1", len(r.repo.collections))
	}
	var got domain.Collection
	for _, c := range r.repo.collections {
		got = c
	}
	if got.OrderID != "ord_1" || got.PartnerID != "PTR-1" || got.Amount.Minor() != 50000 || got.Status != domain.StatusHeld {
		t.Fatalf("collection = %+v", got)
	}
}

// A retried delivery hook must not double-book the same order's cash.
func TestRecordingACollectionTwiceIsIdempotent(t *testing.T) {
	ctx := context.Background()
	r := newRig()

	if err := r.collections.Record(ctx, "ord_1", "PTR-1", 50000); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := r.collections.Record(ctx, "ord_1", "PTR-1", 50000); err != nil {
		t.Fatalf("Record again: %v", err)
	}
	if len(r.repo.collections) != 1 {
		t.Fatalf("%d collections written, want 1", len(r.repo.collections))
	}
}

func TestRecordingACollectionFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("no partner", func(t *testing.T) {
		r := newRig()
		if err := r.collections.Record(ctx, "ord_1", "", 50000); errs.CodeOf(err) != "invalid_collection" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("storage unreachable", func(t *testing.T) {
		r := newRig()
		r.repo.createCollectionErr = errBoom
		if err := r.collections.Record(ctx, "ord_1", "PTR-1", 50000); errs.CodeOf(err) != "payment_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})
}

func seedHeld(t *testing.T, r *rig, id, orderID, partnerID string, minor int64) {
	t.Helper()
	c, err := domain.NewCollection(id, orderID, partnerID, taka(minor), at)
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	r.repo.collections[id] = c
}

func TestMyLedger(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	seedHeld(t, r, "COL-1", "ord_1", "PTR-1", 30000)
	seedHeld(t, r, "COL-2", "ord_2", "PTR-1", 20000)
	seedHeld(t, r, "COL-3", "ord_3", "PTR-2", 99000)

	ledger, err := r.collections.MyLedger(ctx, "PTR-1", "en")
	if err != nil {
		t.Fatalf("MyLedger: %v", err)
	}
	if ledger.Outstanding.Minor != 50000 || len(ledger.Held) != 2 {
		t.Fatalf("ledger = %+v", ledger)
	}
}

func TestMyLedgerForUser(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	seedHeld(t, r, "COL-1", "ord_1", "PTR-1", 30000)
	r.dispatch.partners["usr_1"] = "PTR-1"

	ledger, err := r.collections.MyLedgerForUser(ctx, "usr_1", "en")
	if err != nil {
		t.Fatalf("MyLedgerForUser: %v", err)
	}
	if ledger.PartnerID != "PTR-1" || ledger.Outstanding.Minor != 30000 {
		t.Fatalf("ledger = %+v", ledger)
	}

	// An account that never registered as a partner has no ledger to see.
	if _, err := r.collections.MyLedgerForUser(ctx, "usr_customer", "en"); errs.CodeOf(err) != "payment_not_found" {
		t.Fatalf("err = %v", err)
	}

	r.dispatch.err = errBoom
	if _, err := r.collections.MyLedgerForUser(ctx, "usr_1", "en"); errs.CodeOf(err) != "payment_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

func TestReconciling(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	seedHeld(t, r, "COL-1", "ord_1", "PTR-1", 30000)
	seedHeld(t, r, "COL-2", "ord_2", "PTR-1", 20000)

	ledger, err := r.collections.Reconcile(ctx, "PTR-1", []string{"COL-1", "COL-2"}, "DEPOSIT-9", "en")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if ledger.Outstanding.Minor != 0 || ledger.Remitted.Minor != 50000 {
		t.Fatalf("ledger = %+v", ledger)
	}
	for _, id := range []string{"COL-1", "COL-2"} {
		c := r.repo.collections[id]
		if c.Status != domain.StatusRemitted || c.RemittanceRef != "DEPOSIT-9" {
			t.Fatalf("%s = %+v", id, c)
		}
	}
}

// The batch is all-or-nothing: one bad id refuses the whole reconciliation,
// and nothing already in the batch is left half-remitted.
func TestReconcilingIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	seedHeld(t, r, "COL-1", "ord_1", "PTR-1", 30000)
	seedHeld(t, r, "COL-2", "ord_2", "PTR-2", 20000) // somebody else's

	_, err := r.collections.Reconcile(ctx, "PTR-1", []string{"COL-1", "COL-2"}, "DEPOSIT-9", "en")
	if errs.CodeOf(err) != "collection_not_remittable" {
		t.Fatalf("err = %v", err)
	}
	// COL-1 must not have moved even though it was valid on its own.
	if r.repo.collections["COL-1"].Status != domain.StatusHeld {
		t.Fatalf("a partially valid batch remitted something: %+v", r.repo.collections["COL-1"])
	}
}

func TestReconcileRefusesWhatItMust(t *testing.T) {
	ctx := context.Background()

	t.Run("nothing named", func(t *testing.T) {
		r := newRig()
		if _, err := r.collections.Reconcile(ctx, "PTR-1", nil, "DEPOSIT-9", "en"); errs.CodeOf(err) != "nothing_to_reconcile" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("no reference", func(t *testing.T) {
		r := newRig()
		seedHeld(t, r, "COL-1", "ord_1", "PTR-1", 30000)
		if _, err := r.collections.Reconcile(ctx, "PTR-1", []string{"COL-1"}, "", "en"); errs.CodeOf(err) != "reference_required" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("storage unreachable", func(t *testing.T) {
		r := newRig()
		seedHeld(t, r, "COL-1", "ord_1", "PTR-1", 30000)
		r.repo.remitErr = errBoom
		if _, err := r.collections.Reconcile(ctx, "PTR-1", []string{"COL-1"}, "DEPOSIT-9", "en"); errs.CodeOf(err) != "payment_unavailable" {
			t.Fatalf("err = %v", err)
		}
	})

	// The remittance itself succeeded; only the ledger re-read that follows it
	// failed. The remittance must not be undone just because reporting it back
	// could not be read.
	t.Run("the ledger re-read fails after a good reconcile", func(t *testing.T) {
		r := newRig()
		seedHeld(t, r, "COL-1", "ord_1", "PTR-1", 30000)
		r.repo.forPartnerErr = errBoom
		if _, err := r.collections.Reconcile(ctx, "PTR-1", []string{"COL-1"}, "DEPOSIT-9", "en"); errs.CodeOf(err) != "payment_unavailable" {
			t.Fatalf("err = %v", err)
		}
		if r.repo.collections["COL-1"].Status != domain.StatusRemitted {
			t.Fatalf("a failed re-read undid the remittance: %+v", r.repo.collections["COL-1"])
		}
	})
}
