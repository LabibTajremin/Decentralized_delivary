package tracking

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/application"
	dispatchx "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/external/dispatch"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// rig is one assembled tracking use case with its fakes reachable.
type rig struct {
	order    *fakeOrder
	dispatch *fakeDispatch
	snapshot *application.SnapshotUseCase
	service  *application.Service
}

func newRig() *rig {
	order := newOrder()
	dispatch := newDispatch()
	snapshot := application.NewSnapshotUseCase(order, dispatch)
	return &rig{order: order, dispatch: dispatch, snapshot: snapshot, service: application.NewService(snapshot)}
}

func liveOrder(id, customerID string) orderx.Order {
	return orderx.Order{ID: id, CustomerID: customerID, Status: "picked_up", Live: true, UpdatedAt: at}
}

func nonLiveOrder(id, customerID string) orderx.Order {
	return orderx.Order{ID: id, CustomerID: customerID, Status: "delivered", Live: false, UpdatedAt: at}
}

func TestTheCustomerCanWatchTheirOwnOrder(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_1")
	r.dispatch.jobs["ord_1"] = dispatchx.Job{
		OrderID: "ord_1", Live: true,
		Partner: dispatchx.Partner{ID: "PTR-1", Name: "Karim", Phone: "01700000000", Lat: 23.7, Lng: 90.4},
	}

	snap, err := r.snapshot.For(ctx, "ord_1", "usr_1", "en")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if snap.Status != "picked_up" || snap.StatusLabel != "On the way" || !snap.Live {
		t.Fatalf("snap = %+v", snap)
	}
	if snap.Partner == nil || snap.Partner.ID != "PTR-1" {
		t.Fatalf("partner = %+v", snap.Partner)
	}
}

func TestTheAssignedPartnerCanWatchTheDelivery(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_owner")
	r.dispatch.jobs["ord_1"] = dispatchx.Job{
		OrderID: "ord_1", Live: true,
		Partner: dispatchx.Partner{ID: "PTR-1"},
	}
	r.dispatch.partners["usr_rider"] = "PTR-1"

	snap, err := r.snapshot.For(ctx, "ord_1", "usr_rider", "en")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if snap.OrderID != "ord_1" {
		t.Fatalf("snap = %+v", snap)
	}
}

func TestAStrangerCannotWatch(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_owner")
	r.dispatch.jobs["ord_1"] = dispatchx.Job{OrderID: "ord_1", Live: true, Partner: dispatchx.Partner{ID: "PTR-1"}}

	if _, err := r.snapshot.For(ctx, "ord_1", "usr_stranger", "en"); errs.CodeOf(err) != "order_not_found" {
		t.Fatalf("err = %v", err)
	}
}

// A partner who was on this job but has since finished it, or was never on
// it, may not watch — only the currently assigned rider may.
func TestAPartnerNotOnThisJobCannotWatch(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_owner")
	r.dispatch.jobs["ord_1"] = dispatchx.Job{OrderID: "ord_1", Live: true, Partner: dispatchx.Partner{ID: "PTR-1"}}
	r.dispatch.partners["usr_other_rider"] = "PTR-2"

	if _, err := r.snapshot.For(ctx, "ord_1", "usr_other_rider", "en"); errs.CodeOf(err) != "order_not_found" {
		t.Fatalf("err = %v", err)
	}
}

// Before a job exists, or after nobody is carrying it any more, there is
// nothing to watch beyond the order's own status — and only its customer may
// see even that, since no partner is assigned to be let in.
func TestNoJobMeansOnlyTheCustomerCanWatch(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = orderx.Order{ID: "ord_1", CustomerID: "usr_1", Status: "placed", Live: true}

	snap, err := r.snapshot.For(ctx, "ord_1", "usr_1", "en")
	if err != nil {
		t.Fatalf("For (customer): %v", err)
	}
	if snap.Partner != nil {
		t.Fatalf("snap = %+v, want no partner before a job exists", snap)
	}

	if _, err := r.snapshot.For(ctx, "ord_1", "usr_someone_else", "en"); errs.CodeOf(err) != "order_not_found" {
		t.Fatalf("err = %v", err)
	}
}

func TestForRefusesAnUnknownOrder(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if _, err := r.snapshot.For(ctx, "ord_missing", "usr_1", "en"); errs.CodeOf(err) != "order_not_found" {
		t.Fatalf("err = %v", err)
	}
}

func TestForSurfacesAnOrderStorageFailure(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.err = errBoom
	if _, err := r.snapshot.For(ctx, "ord_1", "usr_1", "en"); errs.CodeOf(err) != "tracking_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

func TestForSurfacesAJobStorageFailure(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_1")
	r.dispatch.jobErr = errBoom
	if _, err := r.snapshot.For(ctx, "ord_1", "usr_1", "en"); errs.CodeOf(err) != "tracking_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

func TestForSurfacesAPartnerLookupFailure(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_owner")
	r.dispatch.partnerErr = errBoom
	if _, err := r.snapshot.For(ctx, "ord_1", "usr_stranger", "en"); errs.CodeOf(err) != "tracking_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

// A terminal order carries no partner even if dispatch still has the job on
// record — a delivered order's rider is not "live" any more.
func TestATerminalOrderCarriesNoPartner(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = orderx.Order{ID: "ord_1", CustomerID: "usr_1", Status: "delivered", Live: false}
	r.dispatch.jobs["ord_1"] = dispatchx.Job{OrderID: "ord_1", Live: false, Partner: dispatchx.Partner{ID: "PTR-1"}}

	snap, err := r.snapshot.For(ctx, "ord_1", "usr_1", "en")
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if snap.Live || snap.Partner != nil {
		t.Fatalf("snap = %+v, want no partner on a terminal order", snap)
	}
}

// Unscoped is the trusted in-process path — no ownership check — reachable
// only through the contract, exercised here and again in service_test.go.
func TestUnscopedSkipsTheOwnershipCheck(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_owner")

	snap, err := r.snapshot.Unscoped(ctx, "ord_1", "en")
	if err != nil {
		t.Fatalf("Unscoped: %v", err)
	}
	if snap.OrderID != "ord_1" {
		t.Fatalf("snap = %+v", snap)
	}
}

func TestUnscopedRefusesAnUnknownOrder(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if _, err := r.snapshot.Unscoped(ctx, "ord_missing", "en"); errs.CodeOf(err) != "order_not_found" {
		t.Fatalf("err = %v", err)
	}
}

func TestUnscopedSurfacesAJobStorageFailure(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.order.orders["ord_1"] = liveOrder("ord_1", "usr_1")
	r.dispatch.jobErr = errBoom
	if _, err := r.snapshot.Unscoped(ctx, "ord_1", "en"); errs.CodeOf(err) != "tracking_unavailable" {
		t.Fatalf("err = %v", err)
	}
}
