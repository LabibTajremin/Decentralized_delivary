// Package application holds notification's use cases: delivering a message
// with a fallback, registering a device, and reading a user's own inbox.
package application

import (
	"errors"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// notificationError turns a domain rule into a failure a caller can act on.
//
// ErrNoDeviceUser has no case here: the transport never lets an empty user id
// reach Register — h.caller refuses first — so a request that got this far
// with one is a malformed application-layer call, not a person to give a
// specific message to, and falls to the generic default below.
func notificationError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNoUser), errors.Is(err, domain.ErrNoTitle), errors.Is(err, domain.ErrNoBody):
		return errs.Wrap(err, errs.KindInvalid, "invalid_notification", "We could not send that notification.")
	case errors.Is(err, domain.ErrNoPlatform), errors.Is(err, domain.ErrBadPlatform), errors.Is(err, domain.ErrNoDeviceToken):
		return errs.Wrap(err, errs.KindInvalid, "invalid_device", "We could not register that device.")
	default:
		return errs.Wrap(err, errs.KindInvalid, "invalid_request", "We could not process that.")
	}
}

// storageError reports notification storage we could not read or write.
func storageError(err error) error {
	return errs.Wrap(err, errs.KindUnavailable, "notification_unavailable",
		"We could not reach notifications just now. Please try again.")
}
