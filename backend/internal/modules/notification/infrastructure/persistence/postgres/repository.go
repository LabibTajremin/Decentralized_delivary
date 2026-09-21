// Package postgres stores notifications and registered device tokens.
//
// Engine-specific SQL is confined to this package (05-architecture.md 2.4).
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/notification/domain"
)

// Querier is the read and write surface this repository needs.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Repository stores notifications and device tokens.
type Repository struct {
	db Querier
}

// New builds a repository.
func New(db Querier) *Repository { return &Repository{db: db} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool) *Repository { return &Repository{db: pool} }

// SaveNotification records one delivery attempt.
func (r *Repository) SaveNotification(ctx context.Context, n domain.Notification) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO notifications (id, user_id, title, body, channel, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		n.ID, n.UserID, n.Title, n.Body, string(n.Channel), string(n.Status), n.CreatedAt)
	if err != nil {
		return fmt.Errorf("save notification: %w", err)
	}
	return nil
}

// NotificationsForUser lists a user's own inbox, most recent first.
func (r *Repository) NotificationsForUser(ctx context.Context, userID string, limit int) ([]domain.Notification, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, title, body, channel, status, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Notification, 0, limit)
	for rows.Next() {
		var n domain.Notification
		var channel, status string
		if err := rows.Scan(&n.ID, &n.UserID, &n.Title, &n.Body, &channel, &status, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		n.Channel, n.Status = domain.Channel(channel), domain.Status(status)
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	return out, nil
}

// SaveDeviceToken upserts a user's token for one platform.
func (r *Repository) SaveDeviceToken(ctx context.Context, t domain.DeviceToken) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO device_tokens (user_id, platform, token, registered_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id, platform) DO UPDATE
		    SET token = EXCLUDED.token, registered_at = now()`,
		t.UserID, string(t.Platform), t.Token)
	if err != nil {
		return fmt.Errorf("save device token: %w", err)
	}
	return nil
}

// DeviceTokensForUser returns every device registered to a user.
func (r *Repository) DeviceTokensForUser(ctx context.Context, userID string) ([]domain.DeviceToken, error) {
	rows, err := r.db.Query(ctx, `
		SELECT user_id, platform, token FROM device_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("list device tokens: %w", err)
	}
	defer rows.Close()

	var out []domain.DeviceToken
	for rows.Next() {
		var t domain.DeviceToken
		var platform string
		if err := rows.Scan(&t.UserID, &platform, &t.Token); err != nil {
			return nil, fmt.Errorf("scan device token: %w", err)
		}
		t.Platform = domain.Platform(platform)
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list device tokens: %w", err)
	}
	return out, nil
}
