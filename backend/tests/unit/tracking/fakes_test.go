package tracking

import (
	"context"
	"errors"
	"time"

	dispatchx "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/external/dispatch"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

var errBoom = errors.New("boom")

var at = time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

// fakeOrder is tracking's view of the order module.
type fakeOrder struct {
	orders map[string]orderx.Order
	err    error
	calls  int

	// toggleAfter, when nonzero, is the call count after which later is
	// returned instead of whatever orders holds — a deterministic way to
	// prove a stream notices a change without racing a real clock against a
	// scripted mutation.
	toggleAfter int
	later       orderx.Order

	// failAfter, when nonzero, is the call count after which every call
	// fails — proving a stream stops rather than looping on a storage
	// failure forever.
	failAfter int
}

func newOrder() *fakeOrder { return &fakeOrder{orders: map[string]orderx.Order{}} }

func (f *fakeOrder) Order(_ context.Context, orderID string) (orderx.Order, error) {
	f.calls++
	if f.failAfter > 0 && f.calls > f.failAfter {
		return orderx.Order{}, errBoom
	}
	if f.err != nil {
		return orderx.Order{}, f.err
	}
	if f.toggleAfter > 0 && f.calls > f.toggleAfter {
		return f.later, nil
	}
	o, ok := f.orders[orderID]
	if !ok {
		return orderx.Order{}, errs.New(errs.KindNotFound, "order_not_found", "We could not find that order.")
	}
	return o, nil
}

// fakeDispatch is tracking's view of the dispatch module.
type fakeDispatch struct {
	jobs       map[string]dispatchx.Job
	partners   map[string]string // userID -> partnerID
	jobErr     error
	partnerErr error
}

func newDispatch() *fakeDispatch {
	return &fakeDispatch{jobs: map[string]dispatchx.Job{}, partners: map[string]string{}}
}

func (f *fakeDispatch) JobForOrder(_ context.Context, orderID string) (dispatchx.Job, bool, error) {
	if f.jobErr != nil {
		return dispatchx.Job{}, false, f.jobErr
	}
	job, ok := f.jobs[orderID]
	return job, ok, nil
}

func (f *fakeDispatch) PartnerOfUser(_ context.Context, userID string) (string, bool, error) {
	if f.partnerErr != nil {
		return "", false, f.partnerErr
	}
	id, ok := f.partners[userID]
	return id, ok, nil
}
