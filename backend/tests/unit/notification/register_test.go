package notification

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

func TestRegisterSavesADevice(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.register.Register(ctx, "usr_1", "ios", "tok_1"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if r.repo.devices["usr_1"]["ios"].Token != "tok_1" {
		t.Fatalf("devices = %+v", r.repo.devices)
	}
}

// A second registration on the same platform replaces the first — a
// reinstalled app's new token, not a second row nothing will ever clean up.
func TestRegisterReplacesAnEarlierTokenOnTheSamePlatform(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.register.Register(ctx, "usr_1", "ios", "tok_old"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := r.register.Register(ctx, "usr_1", "ios", "tok_new"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(r.repo.devices["usr_1"]) != 1 || r.repo.devices["usr_1"]["ios"].Token != "tok_new" {
		t.Fatalf("devices = %+v", r.repo.devices["usr_1"])
	}
}

// The transport never lets an empty user id through — h.caller refuses
// first — so this is the application layer's own defensive check, not a
// message a real client will ever see.
func TestRegisterRejectsAnEmptyUserID(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.register.Register(ctx, "", "ios", "tok_1"); errs.CodeOf(err) != "invalid_request" {
		t.Fatalf("err = %v", err)
	}
}

func TestRegisterRejectsAnInvalidPlatform(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.register.Register(ctx, "usr_1", "windows", "tok_1"); errs.CodeOf(err) != "invalid_device" {
		t.Fatalf("err = %v", err)
	}
}

func TestRegisterRejectsAnEmptyToken(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.register.Register(ctx, "usr_1", "ios", ""); errs.CodeOf(err) != "invalid_device" {
		t.Fatalf("err = %v", err)
	}
}

func TestRegisterSurfacesAStorageFailure(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.repo.saveDeviceErr = errBoom
	if err := r.register.Register(ctx, "usr_1", "ios", "tok_1"); errs.CodeOf(err) != "notification_unavailable" {
		t.Fatalf("err = %v", err)
	}
}
