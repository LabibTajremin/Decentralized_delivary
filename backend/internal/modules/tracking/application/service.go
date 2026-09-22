package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/contract"
)

// Service implements contract.TrackingContract.
type Service struct {
	snapshot *SnapshotUseCase
}

// NewService wires the public service.
func NewService(snapshot *SnapshotUseCase) *Service { return &Service{snapshot: snapshot} }

// Snapshot composes one moment of a delivery for a trusted in-process caller.
func (s *Service) Snapshot(ctx context.Context, orderID, lang string) (contract.Snapshot, error) {
	snap, err := s.snapshot.Unscoped(ctx, orderID, lang)
	if err != nil {
		return contract.Snapshot{}, err
	}
	out := contract.Snapshot{
		OrderID: snap.OrderID, Status: snap.Status, StatusLabel: snap.StatusLabel, Live: snap.Live,
	}
	if snap.Partner != nil {
		out.Partner = &contract.Partner{
			ID: snap.Partner.ID, Name: snap.Partner.Name, Phone: snap.Partner.Phone,
			Vehicle: snap.Partner.Vehicle, Lat: snap.Partner.Lat, Lng: snap.Partner.Lng,
		}
	}
	return out, nil
}
