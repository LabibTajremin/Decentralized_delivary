// Package ports declares what notification's use cases need from the
// outside world. Every implementation lives in infrastructure/.
package ports

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/domain"
)

// Repository stores notifications and the devices registered to receive
// push, behind one interface: both are written by the same use cases and a
// module never needs two storage seams for what is one small table pair.
type Repository interface {
	// SaveNotification records one delivery attempt, sent or failed.
	SaveNotification(ctx context.Context, n domain.Notification) error

	// NotificationsForUser lists the most recent notifications, newest
	// first, for a user's own inbox.
	NotificationsForUser(ctx context.Context, userID string, limit int) ([]domain.Notification, error)

	// SaveDeviceToken registers a device, replacing any earlier token this
	// user registered on the same platform.
	SaveDeviceToken(ctx context.Context, t domain.DeviceToken) error

	// DeviceTokensForUser returns every device currently registered to a
	// user — at most one per platform.
	DeviceTokensForUser(ctx context.Context, userID string) ([]domain.DeviceToken, error)
}

// PushSender delivers to one registered device.
type PushSender interface {
	Send(ctx context.Context, token, title, body string) error
}

// SMSSender delivers a plain text message.
type SMSSender interface {
	Send(ctx context.Context, phone, body string) error
}
