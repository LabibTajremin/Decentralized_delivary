package application

import (
	"context"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/domain"
	identityx "github.com/rootlogic-lab/delivery/backend/internal/modules/notification/external/identity"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// NotifyUseCase delivers one message, push first, falling back to SMS.
type NotifyUseCase struct {
	repo     ports.Repository
	push     ports.PushSender
	sms      ports.SMSSender
	identity identityx.Service
	clock    clock.Clock
	ids      id.Generator
}

// NewNotifyUseCase wires the use case.
func NewNotifyUseCase(repo ports.Repository, push ports.PushSender, sms ports.SMSSender, identity identityx.Service, c clock.Clock, ids id.Generator) *NotifyUseCase {
	return &NotifyUseCase{repo: repo, push: push, sms: sms, identity: identity, clock: c, ids: ids}
}

// Notify delivers a message already composed by the caller (2.9 — the label
// belongs to the module that knows what happened, not to this one), trying
// every device registered for push, and falling back to SMS when none of
// them accept it.
//
// Whichever channel actually delivered the message is recorded on the
// notification, along with a failure when neither could.
func (uc *NotifyUseCase) Notify(ctx context.Context, userID, title, body string) error {
	now := uc.clock.Now()

	devices, err := uc.repo.DeviceTokensForUser(ctx, userID)
	if err != nil {
		return storageError(err)
	}
	// Every registered device gets the push, not just the first one that
	// accepts it — a person with a phone and a tablet both signed in should
	// hear on both, and stopping at the first success would leave the
	// second silent for no reason the first device's own state explains.
	pushed := false
	for _, device := range devices {
		if err := uc.push.Send(ctx, device.Token, title, body); err == nil {
			pushed = true
		}
	}
	if pushed {
		return uc.record(ctx, userID, title, body, domain.ChannelPush, domain.StatusSent, now)
	}

	// Push had nothing to send to, or every device refused it. SMS is the
	// one other channel this platform has for reaching a person.
	phone, found, err := uc.identity.PhoneFor(ctx, userID)
	if err != nil {
		return storageError(err)
	}
	if found {
		if err := uc.sms.Send(ctx, phone, body); err == nil {
			return uc.record(ctx, userID, title, body, domain.ChannelSMS, domain.StatusSent, now)
		}
	}

	if err := uc.record(ctx, userID, title, body, domain.ChannelNone, domain.StatusFailed, now); err != nil {
		return err
	}
	return errs.New(errs.KindUnavailable, "notification_failed",
		"We could not deliver that notification on any channel.")
}

func (uc *NotifyUseCase) record(ctx context.Context, userID, title, body string, channel domain.Channel, status domain.Status, now time.Time) error {
	n, err := domain.NewNotification(uc.ids.New("ntf"), userID, title, body, channel, status, now)
	if err != nil {
		return notificationError(err)
	}
	if err := uc.repo.SaveNotification(ctx, n); err != nil {
		return storageError(err)
	}
	return nil
}
