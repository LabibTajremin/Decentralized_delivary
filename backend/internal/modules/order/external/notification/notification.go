// Package notification is the order module's view of the notification
// module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package notification

import "context"

// Service is the part of notification order depends on.
//
// One method, matching NotificationContract exactly: no adapter struct here,
// the same as order/external/payment — notification's own application.Service
// already has a Notify method of this shape, so it satisfies Service
// structurally and cmd/api wires it in directly.
type Service interface {
	Notify(ctx context.Context, userID, title, body string) error
}
