package notification

import (
	"context"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/application"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// rig is one assembled notification module with every collaborator
// reachable.
type rig struct {
	repo     *fakeRepo
	push     *fakePush
	sms      *fakeSMS
	identity *fakeIdentity
	clock    *clock.Fixed

	notify   *application.NotifyUseCase
	register *application.RegisterDeviceUseCase
	reads    *application.ReadUseCase
	service  *application.Service
}

func newRig() *rig {
	repo := newRepo()
	push := newPush()
	sms := &fakeSMS{}
	identity := newIdentity()
	clk := clock.NewFixed(at)
	ids := &fakeIDs{}

	notify := application.NewNotifyUseCase(repo, push, sms, identity, clk, ids)
	return &rig{
		repo: repo, push: push, sms: sms, identity: identity, clock: clk,
		notify:   notify,
		register: application.NewRegisterDeviceUseCase(repo),
		reads:    application.NewReadUseCase(repo),
		service:  application.NewService(notify),
	}
}

func TestNotifyDeliversByPushWhenADeviceIsRegistered(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.register.Register(ctx, "usr_1", "android", "tok_1"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := r.notify.Notify(ctx, "usr_1", "Title", "Body"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if len(r.push.sends) != 1 || r.push.sends[0].token != "tok_1" {
		t.Fatalf("push sends = %v", r.push.sends)
	}
	if len(r.sms.sends) != 0 {
		t.Fatalf("SMS was used when push succeeded: %v", r.sms.sends)
	}
	if len(r.repo.notifications) != 1 || r.repo.notifications[0].Channel != domain.ChannelPush {
		t.Fatalf("notifications = %+v", r.repo.notifications)
	}
}

// The one behavior this whole module exists for: push fails, so the
// message goes out as a text instead.
func TestNotifyFallsBackToSMSWhenPushFails(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.register.Register(ctx, "usr_1", "android", "tok_bad"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	r.push.fail["tok_bad"] = true
	r.identity.phones["usr_1"] = "+8801700000000"

	if err := r.notify.Notify(ctx, "usr_1", "Title", "Body"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if len(r.sms.sends) != 1 || r.sms.sends[0].phone != "+8801700000000" {
		t.Fatalf("SMS sends = %v", r.sms.sends)
	}
	if r.repo.notifications[0].Channel != domain.ChannelSMS || r.repo.notifications[0].Status != domain.StatusSent {
		t.Fatalf("notification = %+v", r.repo.notifications[0])
	}
}

// No device registered at all goes straight to SMS — there was never a push
// to attempt.
func TestNotifyGoesStraightToSMSWithNoDeviceRegistered(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.identity.phones["usr_1"] = "+8801700000000"

	if err := r.notify.Notify(ctx, "usr_1", "Title", "Body"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if len(r.push.sends) != 0 {
		t.Fatalf("push was attempted with no device registered: %v", r.push.sends)
	}
	if len(r.sms.sends) != 1 {
		t.Fatalf("SMS sends = %v", r.sms.sends)
	}
}

// Two devices registered — one on each platform — and the first fails: the
// second is still tried before falling back.
func TestNotifyTriesEveryDeviceBeforeFallingBack(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.register.Register(ctx, "usr_1", "ios", "tok_bad"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := r.register.Register(ctx, "usr_1", "android", "tok_good"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	r.push.fail["tok_bad"] = true

	if err := r.notify.Notify(ctx, "usr_1", "Title", "Body"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if len(r.push.sends) != 2 {
		t.Fatalf("push sends = %v, want both devices tried", r.push.sends)
	}
	if len(r.sms.sends) != 0 {
		t.Fatalf("SMS was used when a second device succeeded: %v", r.sms.sends)
	}
}

// Push and SMS both fail, and there is no third channel — the caller finds
// out, and the attempt is still recorded.
func TestNotifyReportsWhenNeitherChannelDelivers(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.register.Register(ctx, "usr_1", "android", "tok_bad"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	r.push.fail["tok_bad"] = true
	r.identity.phones["usr_1"] = "+8801700000000"
	r.sms.err = errBoom

	if err := r.notify.Notify(ctx, "usr_1", "Title", "Body"); errs.CodeOf(err) != "notification_failed" {
		t.Fatalf("err = %v", err)
	}
	if r.repo.notifications[0].Channel != domain.ChannelNone || r.repo.notifications[0].Status != domain.StatusFailed {
		t.Fatalf("notification = %+v", r.repo.notifications[0])
	}
}

// No device and no phone on file: nothing to even attempt SMS against.
func TestNotifyReportsWhenNeitherChannelExists(t *testing.T) {
	ctx := context.Background()
	r := newRig()

	if err := r.notify.Notify(ctx, "usr_1", "Title", "Body"); errs.CodeOf(err) != "notification_failed" {
		t.Fatalf("err = %v", err)
	}
	if r.repo.notifications[0].Channel != domain.ChannelNone {
		t.Fatalf("notification = %+v", r.repo.notifications[0])
	}
}

func TestNotifySurfacesADeviceLookupFailure(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.repo.listDevicesErr = errBoom
	if err := r.notify.Notify(ctx, "usr_1", "Title", "Body"); errs.CodeOf(err) != "notification_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

func TestNotifySurfacesAPhoneLookupFailure(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.identity.err = errBoom
	if err := r.notify.Notify(ctx, "usr_1", "Title", "Body"); errs.CodeOf(err) != "notification_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

func TestNotifySurfacesASaveFailureAfterASuccessfulSend(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.register.Register(ctx, "usr_1", "android", "tok_1"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	r.repo.saveErr = errBoom
	if err := r.notify.Notify(ctx, "usr_1", "Title", "Body"); errs.CodeOf(err) != "notification_unavailable" {
		t.Fatalf("err = %v", err)
	}
}

// A malformed call — no user, no title, no body — is refused at the point
// where the attempt is recorded, the same validation every other write in
// this module goes through.
func TestNotifyRejectsAMalformedCall(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	if err := r.notify.Notify(ctx, "", "Title", "Body"); errs.CodeOf(err) != "invalid_notification" {
		t.Fatalf("err = %v", err)
	}
}

func TestNotifySurfacesASaveFailureAfterEverythingFailed(t *testing.T) {
	ctx := context.Background()
	r := newRig()
	r.repo.saveErr = errBoom
	if err := r.notify.Notify(ctx, "usr_1", "Title", "Body"); errs.CodeOf(err) != "notification_unavailable" {
		t.Fatalf("err = %v", err)
	}
}
