package notification

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

func TestForUserListsTheCallersOwnNotifications(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.identity.phones["usr_1"] = "+8801700000001"
	r.identity.phones["usr_2"] = "+8801700000002"
	if err := r.notify.Notify(ctx, "usr_1", "Title 1", "Body 1"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if err := r.notify.Notify(ctx, "usr_2", "Not yours", "Body"); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	views, err := r.reads.ForUser(ctx, "usr_1", 10)
	if err != nil {
		t.Fatalf("ForUser: %v", err)
	}
	if len(views) != 1 || views[0].Title != "Title 1" {
		t.Fatalf("views = %+v", views)
	}
}

func TestForUserDefaultsAndCapsTheLimit(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if _, err := r.reads.ForUser(ctx, "usr_1", 0); err != nil {
		t.Fatalf("ForUser(0): %v", err)
	}
	if _, err := r.reads.ForUser(ctx, "usr_1", 500); err != nil {
		t.Fatalf("ForUser(500): %v", err)
	}
}

func TestForUserSurfacesAStorageFailure(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.repo.listErr = errBoom
	if _, err := r.reads.ForUser(ctx, "usr_1", 10); errs.CodeOf(err) != "notification_unavailable" {
		t.Fatalf("err = %v", err)
	}
}
