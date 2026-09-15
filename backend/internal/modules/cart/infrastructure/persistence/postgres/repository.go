// Package postgres stores carts.
//
// Engine-specific SQL is confined to this package (05-architecture.md 2.4).
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/cart/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// Querier is the read and write surface this repository needs.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// TxBeginner starts a transaction.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Repository stores carts.
type Repository struct {
	db Querier
	tx TxBeginner
}

// New builds a repository.
func New(db Querier, tx TxBeginner) *Repository { return &Repository{db: db, tx: tx} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool) *Repository { return &Repository{db: pool, tx: pool} }

// OfUser returns the customer's open cart.
//
// Two queries rather than a join: a join across lines and their options
// multiplies the rows and leaves the assembly to do the de-duplication anyway.
// A cart is one customer's screen — there is no n+1 here to avoid.
func (r *Repository) OfUser(ctx context.Context, userID string) (domain.Cart, bool, error) {
	var cart domain.Cart
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, merchant_id, address_id, lat, lng, updated_at
		FROM carts WHERE user_id = $1`, userID).
		Scan(&cart.ID, &cart.UserID, &cart.MerchantID, &cart.AddressID,
			&cart.Lat, &cart.Lng, &cart.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Cart{}, false, nil
	}
	if err != nil {
		return domain.Cart{}, false, err
	}

	lines, err := r.linesOf(ctx, cart.ID)
	if err != nil {
		return domain.Cart{}, false, err
	}
	cart.Lines = lines
	return cart, true, nil
}

// linesOf reads a cart's lines and their chosen options.
func (r *Repository) linesOf(ctx context.Context, cartID string) ([]domain.Line, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, kind, target_id, name, unit_price_minor, quantity, note
		FROM cart_lines WHERE cart_id = $1 ORDER BY position, id`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []domain.Line
	index := map[string]int{}
	for rows.Next() {
		var line domain.Line
		var kind string
		var minor int64
		if err := rows.Scan(&line.ID, &kind, &line.TargetID, &line.Name, &minor,
			&line.Quantity, &line.Note); err != nil {
			return nil, err
		}
		line.Kind = domain.Kind(kind)
		line.UnitPrice = money.Taka(minor)
		index[line.ID] = len(lines)
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	options, err := r.db.Query(ctx, `
		SELECT o.line_id, o.group_id, o.option_id, o.name, o.price_minor
		FROM cart_line_options o
		JOIN cart_lines l ON l.id = o.line_id
		WHERE l.cart_id = $1
		ORDER BY o.position, o.group_id, o.option_id`, cartID)
	if err != nil {
		return nil, err
	}
	defer options.Close()

	for options.Next() {
		var lineID string
		var option domain.Option
		var minor int64
		if err := options.Scan(&lineID, &option.GroupID, &option.OptionID,
			&option.Name, &minor); err != nil {
			return nil, err
		}
		option.Price = money.Taka(minor)
		at, ok := index[lineID]
		if !ok {
			// An option whose line is not in the set we just read. Impossible
			// through the join above; skipped rather than indexed blindly,
			// because a panic here would be a 500 on a cart screen.
			continue
		}
		lines[at].Options = append(lines[at].Options, option)
	}
	if err := options.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// Save writes a cart and its lines in one transaction.
//
// The lines are deleted and rewritten rather than diffed. A cart is small, the
// whole of it changed together, and a diff would be a second implementation of
// "what the cart now is" that could disagree with the first.
func (r *Repository) Save(ctx context.Context, c domain.Cart) error {
	tx, err := r.tx.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO carts (id, user_id, merchant_id, address_id, lat, lng, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			merchant_id = EXCLUDED.merchant_id,
			address_id  = EXCLUDED.address_id,
			lat         = EXCLUDED.lat,
			lng         = EXCLUDED.lng,
			updated_at  = EXCLUDED.updated_at`,
		c.ID, c.UserID, c.MerchantID, c.AddressID, c.Lat, c.Lng, c.UpdatedAt); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM cart_lines WHERE cart_id = $1`, c.ID); err != nil {
		return err
	}

	for position, line := range c.Lines {
		if _, err := tx.Exec(ctx, `
			INSERT INTO cart_lines (id, cart_id, kind, target_id, name,
				unit_price_minor, quantity, note, position)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			line.ID, c.ID, string(line.Kind), line.TargetID, line.Name,
			line.UnitPrice.Minor(), line.Quantity, line.Note, position); err != nil {
			return err
		}
		for optionPosition, option := range line.Options {
			if _, err := tx.Exec(ctx, `
				INSERT INTO cart_line_options (line_id, group_id, option_id, name, price_minor, position)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				line.ID, option.GroupID, option.OptionID, option.Name,
				option.Price.Minor(), optionPosition); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// Delete removes a cart entirely. The lines and options go with it by cascade.
func (r *Repository) Delete(ctx context.Context, cartID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM carts WHERE id = $1`, cartID)
	return err
}
