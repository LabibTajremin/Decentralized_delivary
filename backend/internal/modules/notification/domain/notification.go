// Package domain holds notification's own small rules: what a message needs
// to be sent at all, and which channel it went out on.
package domain

import (
	"errors"
	"time"
)

// Channel is where a notification actually went.
type Channel string

const (
	// ChannelPush means a registered device received it.
	ChannelPush Channel = "push"
	// ChannelSMS means push had nothing to send to, or send failed, and the
	// message went out as a text instead.
	ChannelSMS Channel = "sms"
	// ChannelNone means neither channel could deliver it.
	ChannelNone Channel = "none"
)

// Status is whether delivery, on some channel, succeeded.
type Status string

// The two outcomes a delivery attempt can end in.
const (
	StatusSent   Status = "sent"
	StatusFailed Status = "failed"
)

// Errors NewNotification returns.
var (
	ErrNoUser  = errors.New("a notification needs a recipient")
	ErrNoTitle = errors.New("a notification needs a title")
	ErrNoBody  = errors.New("a notification needs a body")
)

// Notification is one message this platform told somebody, and where it went.
type Notification struct {
	ID        string
	UserID    string
	Title     string
	Body      string
	Channel   Channel
	Status    Status
	CreatedAt time.Time
}

// NewNotification records the outcome of one delivery attempt.
func NewNotification(id, userID, title, body string, channel Channel, status Status, now time.Time) (Notification, error) {
	if userID == "" {
		return Notification{}, ErrNoUser
	}
	if title == "" {
		return Notification{}, ErrNoTitle
	}
	if body == "" {
		return Notification{}, ErrNoBody
	}
	return Notification{
		ID: id, UserID: userID, Title: title, Body: body,
		Channel: channel, Status: status, CreatedAt: now,
	}, nil
}
