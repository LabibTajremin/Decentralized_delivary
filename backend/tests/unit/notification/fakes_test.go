package notification

import (
	"context"
	"errors"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

var errBoom = errors.New("boom")

var at = time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)

// fakeRepo is notification's storage, in memory.
type fakeRepo struct {
	notifications  []domain.Notification
	devices        map[string]map[domain.Platform]domain.DeviceToken // userID -> platform -> token
	saveErr        error
	listErr        error
	saveDeviceErr  error
	listDevicesErr error
}

func newRepo() *fakeRepo {
	return &fakeRepo{devices: map[string]map[domain.Platform]domain.DeviceToken{}}
}

func (f *fakeRepo) SaveNotification(_ context.Context, n domain.Notification) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.notifications = append(f.notifications, n)
	return nil
}

func (f *fakeRepo) NotificationsForUser(_ context.Context, userID string, limit int) ([]domain.Notification, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.Notification
	for i := len(f.notifications) - 1; i >= 0; i-- {
		n := f.notifications[i]
		if n.UserID == userID {
			out = append(out, n)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeRepo) SaveDeviceToken(_ context.Context, t domain.DeviceToken) error {
	if f.saveDeviceErr != nil {
		return f.saveDeviceErr
	}
	if f.devices[t.UserID] == nil {
		f.devices[t.UserID] = map[domain.Platform]domain.DeviceToken{}
	}
	f.devices[t.UserID][t.Platform] = t
	return nil
}

func (f *fakeRepo) DeviceTokensForUser(_ context.Context, userID string) ([]domain.DeviceToken, error) {
	if f.listDevicesErr != nil {
		return nil, f.listDevicesErr
	}
	var out []domain.DeviceToken
	for _, t := range f.devices[userID] {
		out = append(out, t)
	}
	return out, nil
}

// fakePush is notification's push gateway.
type fakePush struct {
	// fail lists tokens that refuse a push — every other token succeeds.
	fail  map[string]bool
	sends []pushSend
}

type pushSend struct{ token, title, body string }

func newPush() *fakePush { return &fakePush{fail: map[string]bool{}} }

func (f *fakePush) Send(_ context.Context, token, title, body string) error {
	f.sends = append(f.sends, pushSend{token, title, body})
	if f.fail[token] {
		return errBoom
	}
	return nil
}

// fakeSMS is notification's SMS fallback.
type fakeSMS struct {
	err   error
	sends []smsSend
}

type smsSend struct{ phone, body string }

func (f *fakeSMS) Send(_ context.Context, phone, body string) error {
	f.sends = append(f.sends, smsSend{phone, body})
	return f.err
}

// fakeIdentity is notification's view of identity, mapping a user to a
// phone.
type fakeIdentity struct {
	phones map[string]string
	err    error
}

func newIdentity() *fakeIdentity { return &fakeIdentity{phones: map[string]string{}} }

func (f *fakeIdentity) PhoneFor(_ context.Context, userID string) (string, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	phone, ok := f.phones[userID]
	return phone, ok, nil
}

// fakeIDs mints predictable ids.
type fakeIDs struct{ n int }

func (f *fakeIDs) New(prefix string) string {
	f.n++
	return prefix + "_test"
}

var _ id.Generator = (*fakeIDs)(nil)
