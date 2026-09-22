// Package postgres stores payments and cash collections.
//
// Engine-specific SQL is confined to this package (05-architecture.md 2.4).
// Neither table references another module's schema: payment is the isolated
// module (2.6), and an order id here is an opaque string, never a foreign key
// into a table this module does not own.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/payment/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/money"
)

// uniqueViolation is Postgres' SQLSTATE for a broken unique constraint.
const uniqueViolation = "23505"

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

// Repository stores payments and cash collections.
type Repository struct {
	db Querier
	tx TxBeginner
}

// New builds a repository.
func New(db Querier, tx TxBeginner) *Repository { return &Repository{db: db, tx: tx} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool) *Repository { return &Repository{db: pool, tx: pool} }

// ------------------------------------------------------------------ payments

const paymentColumns = `
	id, order_id, customer_id, gateway, reference, gateway_ref,
	amount_minor, currency, status, failure_reason, refund_reason,
	created_at, updated_at`

func scanPayment(row pgx.Row) (domain.Payment, error) {
	var p domain.Payment
	var status, currency string
	var minor int64
	err := row.Scan(&p.ID, &p.OrderID, &p.CustomerID, &p.Gateway, &p.Reference, &p.GatewayRef,
		&minor, &currency, &status, &p.FailureReason, &p.RefundReason,
		&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return domain.Payment{}, err
	}
	p.Status = domain.Status(status)
	p.Amount, err = money.New(minor, money.Currency(currency))
	if err != nil {
		return domain.Payment{}, err
	}
	return p, nil
}

// CreatePayment writes a new gateway attempt.
func (r *Repository) CreatePayment(ctx context.Context, p domain.Payment) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO payments (
			id, order_id, customer_id, gateway, reference, gateway_ref,
			amount_minor, currency, status, failure_reason, refund_reason,
			created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'','',$10,$11)`,
		p.ID, p.OrderID, p.CustomerID, p.Gateway, p.Reference, p.GatewayRef,
		p.Amount.Minor(), string(p.Amount.Currency()), string(p.Status),
		p.CreatedAt, p.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		// The partial unique index on (order_id) WHERE status = 'pending'
		// caught a second checkout racing this one for the same order. The
		// one that won is the answer; this one never existed.
		return errs.New(errs.KindConflict, "checkout_in_progress",
			"A checkout for that order is already in progress.")
	}
	return err
}

// Payment returns one attempt by its own id.
func (r *Repository) Payment(ctx context.Context, id string) (domain.Payment, error) {
	p, err := scanPayment(r.db.QueryRow(ctx, `SELECT`+paymentColumns+` FROM payments WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, errs.New(errs.KindNotFound, "payment_not_found", "No such payment.")
	}
	return p, err
}

// ByReference finds the attempt a webhook is answering.
func (r *Repository) ByReference(ctx context.Context, reference string) (domain.Payment, error) {
	p, err := scanPayment(r.db.QueryRow(ctx,
		`SELECT`+paymentColumns+` FROM payments WHERE reference = $1`, reference))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, errs.New(errs.KindNotFound, "payment_not_found", "No such payment.")
	}
	return p, err
}

// PendingForOrder returns the open attempt for an order, if there is one.
func (r *Repository) PendingForOrder(ctx context.Context, orderID string) (domain.Payment, bool, error) {
	p, err := scanPayment(r.db.QueryRow(ctx,
		`SELECT`+paymentColumns+` FROM payments WHERE order_id = $1 AND status = 'pending'`, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, false, nil
	}
	if err != nil {
		return domain.Payment{}, false, err
	}
	return p, true, nil
}

// LatestForOrder returns the most recent attempt for an order, whatever its
// state.
func (r *Repository) LatestForOrder(ctx context.Context, orderID string) (domain.Payment, bool, error) {
	p, err := scanPayment(r.db.QueryRow(ctx,
		`SELECT`+paymentColumns+` FROM payments WHERE order_id = $1 ORDER BY created_at DESC LIMIT 1`, orderID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, false, nil
	}
	if err != nil {
		return domain.Payment{}, false, err
	}
	return p, true, nil
}

// Save writes a payment, checking it is still where the caller thought it was.
func (r *Repository) Save(ctx context.Context, p domain.Payment, expected domain.Status) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE payments
		SET status = $1, gateway_ref = $2, failure_reason = $3, refund_reason = $4, updated_at = $5
		WHERE id = $6 AND status = $7`,
		string(p.Status), p.GatewayRef, p.FailureReason, p.RefundReason, p.UpdatedAt,
		p.ID, string(expected))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.New(errs.KindConflict, "payment_moved",
			"That payment has already moved on.")
	}
	return nil
}

// ------------------------------------------------------------------ cash collections

const collectionColumns = `
	id, order_id, partner_id, amount_minor, currency, status, remittance_ref,
	collected_at, COALESCE(remitted_at, 'epoch'::timestamptz), created_at, updated_at`

func scanCollection(row pgx.Row) (domain.Collection, error) {
	var c domain.Collection
	var status, currency string
	var minor int64
	var remittedAt time.Time
	err := row.Scan(&c.ID, &c.OrderID, &c.PartnerID, &minor, &currency, &status, &c.RemittanceRef,
		&c.CollectedAt, &remittedAt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return domain.Collection{}, err
	}
	c.Status = domain.CollectionStatus(status)
	c.Amount, err = money.New(minor, money.Currency(currency))
	if err != nil {
		return domain.Collection{}, err
	}
	// An epoch timestamp is how a NULL comes back through the COALESCE above —
	// restored to the zero time so nothing above this package has to know
	// that convention.
	if remittedAt.Unix() != 0 {
		c.RemittedAt = remittedAt
	}
	return c, nil
}

// CreateCollection records a rider taking cash for a delivered order.
func (r *Repository) CreateCollection(ctx context.Context, c domain.Collection) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO cash_collections (
			id, order_id, partner_id, amount_minor, currency, status, remittance_ref,
			collected_at, remitted_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,'',$7,NULL,$8,$9)`,
		c.ID, c.OrderID, c.PartnerID, c.Amount.Minor(), string(c.Amount.Currency()), string(c.Status),
		c.CollectedAt, c.CreatedAt, c.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return errs.New(errs.KindConflict, "collection_exists",
			"That order's cash has already been recorded.")
	}
	return err
}

// Collection returns one record by its own id.
func (r *Repository) Collection(ctx context.Context, id string) (domain.Collection, error) {
	c, err := scanCollection(r.db.QueryRow(ctx,
		`SELECT`+collectionColumns+` FROM cash_collections WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Collection{}, errs.New(errs.KindNotFound, "collection_not_found", "No such collection.")
	}
	return c, err
}

// ForPartner lists a partner's collections, held first, oldest first within
// each state — what a "please remit" screen and a ledger read both want.
func (r *Repository) ForPartner(ctx context.Context, partnerID string) ([]domain.Collection, error) {
	rows, err := r.db.Query(ctx, `
		SELECT`+collectionColumns+`
		FROM cash_collections
		WHERE partner_id = $1
		ORDER BY (status <> 'held'), collected_at`, partnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Collection
	for rows.Next() {
		c, scanErr := scanCollection(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Remit marks a batch of a partner's held collections as remitted, inside one
// transaction: every id must belong to this partner and still be held, or none
// of them move.
func (r *Repository) Remit(ctx context.Context, partnerID string, collectionIDs []string, reference string, now time.Time) error {
	tx, err := r.tx.Begin(ctx)
	if err != nil {
		return err
	}
	for _, id := range collectionIDs {
		tag, execErr := tx.Exec(ctx, `
			UPDATE cash_collections
			SET status = 'remitted', remittance_ref = $1, remitted_at = $2, updated_at = $2
			WHERE id = $3 AND partner_id = $4 AND status = 'held'`,
			reference, now, id, partnerID)
		if execErr != nil {
			_ = tx.Rollback(ctx)
			return execErr
		}
		if tag.RowsAffected() == 0 {
			_ = tx.Rollback(ctx)
			return errs.New(errs.KindConflict, "collection_not_remittable",
				"One of those collections is not yours to remit, or has already been remitted.").
				With("collection_id", id)
		}
	}
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return err
	}
	return nil
}
