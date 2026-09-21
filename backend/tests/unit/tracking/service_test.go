package tracking

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/contract"
	dispatchx "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/external/dispatch"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// TestTheContractComposesASnapshotWithAPartner is what order and
// notification actually depend on — TrackingContract, not this package's
// own types.
func TestTheContractComposesASnapshotWithAPartner(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	var api contract.TrackingContract = r.service
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_1")
	r.dispatch.jobs["ord_1"] = dispatchx.Job{
		OrderID: "ord_1", Live: true,
		Partner: dispatchx.Partner{ID: "PTR-1", Name: "Karim", Phone: "01700000000", Vehicle: "bike", Lat: 23.7, Lng: 90.4},
	}

	snap, err := api.Snapshot(ctx, "ord_1", "en")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.OrderID != "ord_1" || snap.Status != "picked_up" || !snap.Live {
		t.Fatalf("snap = %+v", snap)
	}
	if snap.Partner == nil || snap.Partner.ID != "PTR-1" || snap.Partner.Name != "Karim" {
		t.Fatalf("partner = %+v", snap.Partner)
	}
}

func TestTheContractComposesASnapshotWithNoPartner(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = orderx.Order{ID: "ord_1", CustomerID: "usr_1", Status: "placed", Live: true}

	snap, err := r.service.Snapshot(ctx, "ord_1", "en")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Partner != nil {
		t.Fatalf("snap.Partner = %+v, want nil", snap.Partner)
	}
}

func TestTheContractSurfacesAFailure(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if _, err := r.service.Snapshot(ctx, "ord_missing", "en"); errs.CodeOf(err) != "order_not_found" {
		t.Fatalf("err = %v", err)
	}
}
