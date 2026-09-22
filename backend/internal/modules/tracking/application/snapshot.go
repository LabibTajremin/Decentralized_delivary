// Package application holds tracking's one use case: composing a moment of a
// delivery for whoever is allowed to watch it.
package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/domain"
	dispatchx "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/external/dispatch"
	orderx "github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/external/order"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// SnapshotUseCase composes one moment of a delivery from order and dispatch.
//
// It owns no storage of its own — order already has the status and the
// history, dispatch already has where the rider is, and a third copy of
// either would only be a second place for the two to disagree.
type SnapshotUseCase struct {
	order    orderx.Service
	dispatch dispatchx.Service
}

// NewSnapshotUseCase wires the use case.
func NewSnapshotUseCase(order orderx.Service, dispatch dispatchx.Service) *SnapshotUseCase {
	return &SnapshotUseCase{order: order, dispatch: dispatch}
}

// For composes a snapshot for a caller, scoped to what they may watch: the
// customer who placed the order, or the partner currently carrying it.
//
// Refused as not-found rather than forbidden for anyone else — an order's
// existence is itself something only its own parties should be able to
// confirm.
func (uc *SnapshotUseCase) For(ctx context.Context, orderID, callerID, lang string) (domain.Snapshot, error) {
	order, err := uc.order.Order(ctx, orderID)
	if err != nil {
		return domain.Snapshot{}, notFoundOr(err)
	}
	job, found, err := uc.dispatch.JobForOrder(ctx, orderID)
	if err != nil {
		return domain.Snapshot{}, storageError(err)
	}

	if order.CustomerID != callerID {
		partnerID, isPartner, err := uc.dispatch.PartnerOfUser(ctx, callerID)
		if err != nil {
			return domain.Snapshot{}, storageError(err)
		}
		if !isPartner || !found || job.Partner.ID == "" || job.Partner.ID != partnerID {
			return domain.Snapshot{}, notFound()
		}
	}

	return build(order, job, found, lang), nil
}

// Unscoped composes a snapshot for a trusted in-process caller — another
// backend module reached through TrackingContract, not an end user. Nothing
// outside this process can reach it without going through For's ownership
// check first.
func (uc *SnapshotUseCase) Unscoped(ctx context.Context, orderID, lang string) (domain.Snapshot, error) {
	order, err := uc.order.Order(ctx, orderID)
	if err != nil {
		return domain.Snapshot{}, notFoundOr(err)
	}
	job, found, err := uc.dispatch.JobForOrder(ctx, orderID)
	if err != nil {
		return domain.Snapshot{}, storageError(err)
	}
	return build(order, job, found, lang), nil
}

func build(order orderx.Order, job dispatchx.Job, jobFound bool, lang string) domain.Snapshot {
	snap := domain.Snapshot{
		OrderID:     order.ID,
		Status:      order.Status,
		StatusLabel: statusLabel(order.Status, lang),
		Live:        order.Live,
		UpdatedAt:   order.UpdatedAt,
	}
	if jobFound && job.Live && job.Partner.ID != "" {
		snap.Partner = &domain.Partner{
			ID: job.Partner.ID, Name: job.Partner.Name, Phone: job.Partner.Phone,
			Vehicle: job.Partner.Vehicle, Lat: job.Partner.Lat, Lng: job.Partner.Lng,
		}
	}
	return snap
}

func notFound() error {
	return errs.New(errs.KindNotFound, "order_not_found", "We could not find that order.")
}

func notFoundOr(err error) error {
	if errs.Is(err, errs.KindNotFound) {
		return notFound()
	}
	return storageError(err)
}

func storageError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "tracking_unavailable",
		"We could not reach tracking just now. Please try again.")
}
