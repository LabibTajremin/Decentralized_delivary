package application

import (
	"context"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/application/ports"
)

// View is one notification as a screen shows it.
//
// Title and Body are already in whatever language they were composed in at
// creation time — the caller that sent the notification chose that, not this
// read (see docs/technical/notification.md).
type View struct {
	ID        string
	Title     string
	Body      string
	Channel   string
	Status    string
	CreatedAt time.Time
}

// ReadUseCase serves a user's own notification inbox.
type ReadUseCase struct {
	repo ports.Repository
}

// NewReadUseCase wires the use case.
func NewReadUseCase(repo ports.Repository) *ReadUseCase { return &ReadUseCase{repo: repo} }

// ForUser lists a user's most recent notifications, newest first.
func (uc *ReadUseCase) ForUser(ctx context.Context, userID string, limit int) ([]View, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	notifications, err := uc.repo.NotificationsForUser(ctx, userID, limit)
	if err != nil {
		return nil, storageError(err)
	}
	out := make([]View, 0, len(notifications))
	for _, n := range notifications {
		out = append(out, View{
			ID: n.ID, Title: n.Title, Body: n.Body,
			Channel: string(n.Channel), Status: string(n.Status),
			CreatedAt: n.CreatedAt,
		})
	}
	return out, nil
}
