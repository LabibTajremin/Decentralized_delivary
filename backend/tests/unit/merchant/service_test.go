package merchant

import (
	"context"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
)

// OwnedBy answers "which shops are yours", which the order module asks before
// it shows a shop's queue or accepts an order on its behalf. It cannot read the
// merchant table to find out (2.5), so this is the seam.
func TestOwnedBy(t *testing.T) {
	ctx := context.Background()
	repo := newRepo()
	shop := approvedMerchant(t)
	if err := repo.Create(ctx, shop); err != nil {
		t.Fatalf("Create: %v", err)
	}
	service := application.NewService(repo, clock.NewFixed(at(2026, time.March, 2, 12, 0)))

	owned, err := service.OwnedBy(ctx, shop.OwnerUserID)
	if err != nil {
		t.Fatalf("OwnedBy: %v", err)
	}
	if len(owned) != 1 || owned[0] != shop.ID {
		t.Fatalf("owned = %v, want [%s]", owned, shop.ID)
	}

	// An account with no shop is not an error: most accounts have none, and a
	// consumer asking about a customer should get an empty answer rather than
	// a failure it has to special-case.
	none, err := service.OwnedBy(ctx, "usr_customer")
	if err != nil || len(none) != 0 {
		t.Fatalf("owned = %v, err = %v", none, err)
	}

	// A store that is broken is a third answer again, and must not read as
	// "you own no shops" — that would show an owner an empty queue and let
	// them conclude their orders had vanished.
	broken := newRepo()
	broken.byOwnerErr = errStore
	brokenService := application.NewService(broken, clock.NewFixed(at(2026, time.March, 2, 12, 0)))
	if _, err := brokenService.OwnedBy(ctx, shop.OwnerUserID); err == nil {
		t.Fatal("a broken store was reported as no shop")
	}
}
