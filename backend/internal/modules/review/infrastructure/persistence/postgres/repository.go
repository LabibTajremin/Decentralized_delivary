// Package postgres stores reviews and support tickets.
//
// Engine-specific SQL is confined to this package (05-architecture.md 2.4).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/review/domain"
)

// Querier is the read and write surface this repository needs.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Repository stores reviews and support tickets.
type Repository struct {
	db Querier
}

// New builds a repository.
func New(db Querier) *Repository { return &Repository{db: db} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool) *Repository { return &Repository{db: pool} }

// SaveReview records one review.
func (r *Repository) SaveReview(ctx context.Context, rv domain.Review) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO reviews (id, order_id, rater_id, subject, subject_id, rating, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		rv.ID, rv.OrderID, rv.RaterID, string(rv.Subject), rv.SubjectID, rv.Rating, rv.Comment, rv.CreatedAt)
	if err != nil {
		return fmt.Errorf("save review: %w", err)
	}
	return nil
}

// ExistsForRater reports whether this rater already reviewed this subject
// from this order.
func (r *Repository) ExistsForRater(ctx context.Context, orderID, raterID string, subject domain.Subject, subjectID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM reviews
			WHERE order_id = $1 AND rater_id = $2 AND subject = $3 AND subject_id = $4)`,
		orderID, raterID, string(subject), subjectID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check existing review: %w", err)
	}
	return exists, nil
}

// RatingFor reduces one subject's reviews to a mean and a count.
func (r *Repository) RatingFor(ctx context.Context, subject domain.Subject, subjectID string) (domain.Rating, error) {
	var count int
	var average *float64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*), AVG(rating) FROM reviews WHERE subject = $1 AND subject_id = $2`,
		string(subject), subjectID).Scan(&count, &average)
	if err != nil {
		return domain.Rating{}, fmt.Errorf("aggregate rating: %w", err)
	}
	rating := domain.Rating{Subject: subject, SubjectID: subjectID, Count: count}
	if average != nil {
		rating.Average = *average
	}
	return rating, nil
}

// ReviewsFor lists a subject's reviews, newest first.
func (r *Repository) ReviewsFor(ctx context.Context, subject domain.Subject, subjectID string, limit int) ([]domain.Review, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, rater_id, subject, subject_id, rating, comment, created_at
		FROM reviews
		WHERE subject = $1 AND subject_id = $2
		ORDER BY created_at DESC
		LIMIT $3`, string(subject), subjectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Review, 0, limit)
	for rows.Next() {
		var rv domain.Review
		var subjectCol string
		if err := rows.Scan(&rv.ID, &rv.OrderID, &rv.RaterID, &subjectCol, &rv.SubjectID,
			&rv.Rating, &rv.Comment, &rv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		rv.Subject = domain.Subject(subjectCol)
		out = append(out, rv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	return out, nil
}

// SaveTicket inserts a new ticket, or updates one already resolved.
func (r *Repository) SaveTicket(ctx context.Context, t domain.Ticket) error {
	var resolvedAt *time.Time
	if !t.ResolvedAt.IsZero() {
		resolvedAt = &t.ResolvedAt
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO support_tickets (id, order_id, raised_by, subject, status, resolution, note, agent_id, created_at, resolved_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE
		    SET status = EXCLUDED.status, resolution = EXCLUDED.resolution,
		        note = EXCLUDED.note, agent_id = EXCLUDED.agent_id, resolved_at = EXCLUDED.resolved_at`,
		t.ID, t.OrderID, t.RaisedBy, t.Subject, string(t.Status), string(t.Resolution),
		t.Note, t.AgentID, t.CreatedAt, resolvedAt)
	if err != nil {
		return fmt.Errorf("save ticket: %w", err)
	}
	return nil
}

// Ticket returns one ticket by id.
func (r *Repository) Ticket(ctx context.Context, id string) (domain.Ticket, bool, error) {
	t, err := scanTicket(r.db.QueryRow(ctx, `
		SELECT id, order_id, raised_by, subject, status, resolution, note, agent_id, created_at, resolved_at
		FROM support_tickets WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Ticket{}, false, nil
		}
		return domain.Ticket{}, false, fmt.Errorf("get ticket: %w", err)
	}
	return t, true, nil
}

// TicketsRaisedBy lists a customer's own tickets, newest first.
func (r *Repository) TicketsRaisedBy(ctx context.Context, userID string) ([]domain.Ticket, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, raised_by, subject, status, resolution, note, agent_id, created_at, resolved_at
		FROM support_tickets WHERE raised_by = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list tickets: %w", err)
	}
	return scanTickets(rows)
}

// OpenTickets lists tickets nobody has resolved yet, oldest first.
func (r *Repository) OpenTickets(ctx context.Context, limit int) ([]domain.Ticket, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, raised_by, subject, status, resolution, note, agent_id, created_at, resolved_at
		FROM support_tickets WHERE status = 'open' ORDER BY created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list open tickets: %w", err)
	}
	return scanTickets(rows)
}

func scanTickets(rows pgx.Rows) ([]domain.Ticket, error) {
	defer rows.Close()
	var out []domain.Ticket
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ticket: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tickets: %w", err)
	}
	return out, nil
}

// row is the subset of pgx.Row and pgx.Rows this package scans from —
// QueryRow's single-row result and Query's per-row cursor share the same
// Scan signature, so one function reads both.
type row interface {
	Scan(dest ...any) error
}

func scanTicket(rw row) (domain.Ticket, error) {
	var t domain.Ticket
	var status, resolution string
	var resolvedAt *time.Time
	if err := rw.Scan(&t.ID, &t.OrderID, &t.RaisedBy, &t.Subject, &status, &resolution,
		&t.Note, &t.AgentID, &t.CreatedAt, &resolvedAt); err != nil {
		return domain.Ticket{}, err
	}
	t.Status = domain.TicketStatus(status)
	t.Resolution = domain.Resolution(resolution)
	if resolvedAt != nil {
		t.ResolvedAt = *resolvedAt
	}
	return t, nil
}
