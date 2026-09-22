package tracking

import (
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/tracking/domain"
)

func TestChangedDetectsAStatusMove(t *testing.T) {
	a := domain.Snapshot{Status: "placed"}
	b := domain.Snapshot{Status: "accepted"}
	if !b.Changed(a) {
		t.Error("a status change was not detected")
	}
	if a.Changed(a) {
		t.Error("an identical snapshot was reported as changed")
	}
}

func TestChangedDetectsAPartnerAppearingOrLeaving(t *testing.T) {
	// Same status on both sides, so it is the partner appearing — not the
	// status move already covered above — that this test proves triggers a
	// change.
	none := domain.Snapshot{Status: "picked_up"}
	withRider := domain.Snapshot{Status: "picked_up", Partner: &domain.Partner{ID: "PTR-1", Lat: 23.7, Lng: 90.4}}

	if !withRider.Changed(none) {
		t.Error("a rider appearing was not detected")
	}
	if !none.Changed(withRider) {
		t.Error("a rider leaving was not detected")
	}
}

func TestChangedDetectsTheRiderMoving(t *testing.T) {
	a := domain.Snapshot{Status: "picked_up", Partner: &domain.Partner{ID: "PTR-1", Lat: 23.7, Lng: 90.4}}
	b := domain.Snapshot{Status: "picked_up", Partner: &domain.Partner{ID: "PTR-1", Lat: 23.71, Lng: 90.4}}
	if !b.Changed(a) {
		t.Error("the rider's own movement was not detected")
	}
}

func TestChangedIsFalseWhenNothingMoved(t *testing.T) {
	a := domain.Snapshot{Status: "picked_up", Partner: &domain.Partner{ID: "PTR-1", Lat: 23.7, Lng: 90.4}}
	b := domain.Snapshot{Status: "picked_up", Partner: &domain.Partner{ID: "PTR-1", Lat: 23.7, Lng: 90.4}}
	if b.Changed(a) {
		t.Error("an identical rider position was reported as changed")
	}
}
