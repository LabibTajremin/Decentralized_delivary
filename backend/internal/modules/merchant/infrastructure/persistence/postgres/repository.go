// Package postgres stores merchants, their documents and their decisions.
//
// Engine-specific SQL is confined to this package (05-architecture.md 2.4).
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/merchant/domain"
)

// uniqueViolation is Postgres' SQLSTATE for a broken unique constraint. The one
// that matters here is merchants_owner_user_id_key: two registrations racing.
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

// Repository stores merchants.
type Repository struct {
	db Querier
	tx TxBeginner
}

// New builds a repository.
func New(db Querier, tx TxBeginner) *Repository { return &Repository{db: db, tx: tx} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool) *Repository { return &Repository{db: pool, tx: pool} }

// merchantColumns is the projection every merchant read shares, so one added
// column cannot be forgotten in one of four places.
const merchantColumns = `
	id, owner_user_id, name, type, status, phone, email, logo_url,
	line1, line2,
	ST_Y(pin::geometry), ST_X(pin::geometry),
	area_code, area_name, district_code, division_code,
	hours, holiday_active, holiday_until, holiday_reason, review_note, created_at`

// scanMerchant reads one row, documents excluded.
//
// Hours and the holiday go through the domain's own decoders rather than being
// assembled field by field: a schedule is a value object with an invariant —
// sorted, non-overlapping — and a row that was written before a rule tightened
// must fail loudly here rather than become an invalid one in memory.
func scanMerchant(row pgx.Row) (domain.Merchant, error) {
	var (
		m            domain.Merchant
		kind, status string
		lat, lng     float64
		encodedHours []byte
		holidayUntil *time.Time
		holiday      domain.Holiday
	)

	if err := row.Scan(&m.ID, &m.OwnerUserID, &m.Name, &kind, &status, &m.Phone, &m.Email, &m.LogoURL,
		&m.Line1, &m.Line2, &lat, &lng,
		&m.Placement.AreaCode, &m.Placement.AreaName, &m.Placement.DistrictCode, &m.Placement.DivisionCode,
		&encodedHours, &holiday.Active, &holidayUntil, &holiday.Reason, &m.ReviewNote, &m.CreatedAt); err != nil {
		return domain.Merchant{}, err
	}

	parsedType, err := domain.ParseType(kind)
	if err != nil {
		return domain.Merchant{}, fmt.Errorf("merchant %s: %w", m.ID, err)
	}
	parsedStatus, err := domain.ParseStatus(status)
	if err != nil {
		return domain.Merchant{}, fmt.Errorf("merchant %s: %w", m.ID, err)
	}
	m.Type = parsedType
	m.Status = parsedStatus
	m.Pin = domain.Pin{Lat: lat, Lng: lng}

	var encoded map[string][]string
	if err := json.Unmarshal(encodedHours, &encoded); err != nil {
		return domain.Merchant{}, fmt.Errorf("merchant %s hours: %w", m.ID, err)
	}
	hours, err := domain.DecodeWeeklyHours(encoded)
	if err != nil {
		return domain.Merchant{}, fmt.Errorf("merchant %s hours: %w", m.ID, err)
	}
	m.Hours = hours

	if holidayUntil != nil {
		holiday.Until = holidayUntil.UTC()
	}
	m.Holiday = holiday

	return m, nil
}

// Merchant returns one merchant by id.
func (r *Repository) Merchant(ctx context.Context, merchantID string) (domain.Merchant, error) {
	return r.one(ctx, `WHERE id = $1`, merchantID)
}

// ByOwner returns the merchant an account owns.
func (r *Repository) ByOwner(ctx context.Context, ownerUserID string) (domain.Merchant, error) {
	return r.one(ctx, `WHERE owner_user_id = $1`, ownerUserID)
}

// one runs a single-row read and attaches the documents.
func (r *Repository) one(ctx context.Context, where string, arg string) (domain.Merchant, error) {
	merchant, err := scanMerchant(r.db.QueryRow(ctx,
		`SELECT `+merchantColumns+` FROM merchants `+where, arg))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Merchant{}, domain.ErrMerchantNotFound
	}
	if err != nil {
		return domain.Merchant{}, fmt.Errorf("read merchant: %w", err)
	}

	documents, err := r.documents(ctx, merchant.ID)
	if err != nil {
		return domain.Merchant{}, err
	}
	merchant.Documents = documents
	return merchant, nil
}

// documents reads a merchant's papers, in the order they are asked for.
func (r *Repository) documents(ctx context.Context, merchantID string) ([]domain.Document, error) {
	rows, err := r.db.Query(ctx, `
		SELECT kind, number, file_url, uploaded_at
		FROM merchant_documents WHERE merchant_id = $1
		ORDER BY kind`, merchantID)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	var out []domain.Document
	for rows.Next() {
		var kind string
		var d domain.Document
		if err := rows.Scan(&kind, &d.Number, &d.FileURL, &d.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		parsed, err := domain.ParseDocumentKind(kind)
		if err != nil {
			return nil, fmt.Errorf("merchant %s: %w", merchantID, err)
		}
		d.Kind = parsed
		d.UploadedAt = d.UploadedAt.UTC()
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read documents: %w", err)
	}
	return out, nil
}

// Create stores a new merchant and its documents.
func (r *Repository) Create(ctx context.Context, m domain.Merchant) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if err := writeMerchant(ctx, tx, m); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
				return domain.ErrAlreadyRegistered
			}
			return err
		}
		return writeDocuments(ctx, tx, m)
	})
}

// Save writes an existing merchant and its documents.
func (r *Repository) Save(ctx context.Context, m domain.Merchant) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if err := writeMerchant(ctx, tx, m); err != nil {
			return err
		}
		return writeDocuments(ctx, tx, m)
	})
}

// Delete removes a merchant. Documents and status events cascade.
func (r *Repository) Delete(ctx context.Context, merchantID string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM merchants WHERE id = $1`, merchantID); err != nil {
		return fmt.Errorf("delete merchant: %w", err)
	}
	return nil
}

// writeMerchant upserts the merchant row.
func writeMerchant(ctx context.Context, q Querier, m domain.Merchant) error {
	// Encode returns map[string][]string. Every value in it is a string, so
	// there is no type encoding/json can refuse and no error branch a test
	// could reach — the discard is deliberate rather than careless.
	encoded, _ := json.Marshal(m.Hours.Encode()) //nolint:errchkjson // map[string][]string cannot fail

	var holidayUntil *time.Time
	if !m.Holiday.Until.IsZero() {
		until := m.Holiday.Until
		holidayUntil = &until
	}

	if _, err := q.Exec(ctx, `
		INSERT INTO merchants (
		    id, owner_user_id, name, type, status, phone, email, logo_url,
		    line1, line2, pin,
		    area_code, area_name, district_code, division_code,
		    hours, holiday_active, holiday_until, holiday_reason, review_note, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
		        ST_SetSRID(ST_MakePoint($11, $12), 4326)::geography,
		        $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
		ON CONFLICT (id) DO UPDATE
		    SET name           = EXCLUDED.name,
		        type           = EXCLUDED.type,
		        status         = EXCLUDED.status,
		        phone          = EXCLUDED.phone,
		        email          = EXCLUDED.email,
		        logo_url       = EXCLUDED.logo_url,
		        line1          = EXCLUDED.line1,
		        line2          = EXCLUDED.line2,
		        pin            = EXCLUDED.pin,
		        area_code      = EXCLUDED.area_code,
		        area_name      = EXCLUDED.area_name,
		        district_code  = EXCLUDED.district_code,
		        division_code  = EXCLUDED.division_code,
		        hours          = EXCLUDED.hours,
		        holiday_active = EXCLUDED.holiday_active,
		        holiday_until  = EXCLUDED.holiday_until,
		        holiday_reason = EXCLUDED.holiday_reason,
		        review_note    = EXCLUDED.review_note,
		        updated_at     = now()`,
		m.ID, m.OwnerUserID, m.Name, m.Type.String(), m.Status.String(), m.Phone, m.Email, m.LogoURL,
		m.Line1, m.Line2, m.Pin.Lng, m.Pin.Lat,
		m.Placement.AreaCode, m.Placement.AreaName, m.Placement.DistrictCode, m.Placement.DivisionCode,
		encoded, m.Holiday.Active, holidayUntil, m.Holiday.Reason, m.ReviewNote, m.CreatedAt); err != nil {
		return fmt.Errorf("save merchant: %w", err)
	}
	return nil
}

// writeDocuments replaces the merchant's papers with the set it now carries.
//
// Deleting what is absent rather than only upserting what is present: the
// domain is the authority on which documents a shop has, and a row left behind
// from before a type change is a licence a reviewer would count as supplied.
func writeDocuments(ctx context.Context, q Querier, m domain.Merchant) error {
	kinds := make([]string, 0, len(m.Documents))
	for _, d := range m.Documents {
		kinds = append(kinds, d.Kind.String())
	}
	if _, err := q.Exec(ctx,
		`DELETE FROM merchant_documents WHERE merchant_id = $1 AND NOT (kind = ANY($2))`,
		m.ID, kinds); err != nil {
		return fmt.Errorf("prune documents: %w", err)
	}

	for _, d := range m.Documents {
		if _, err := q.Exec(ctx, `
			INSERT INTO merchant_documents (merchant_id, kind, number, file_url, uploaded_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (merchant_id, kind) DO UPDATE
			    SET number      = EXCLUDED.number,
			        file_url    = EXCLUDED.file_url,
			        uploaded_at = EXCLUDED.uploaded_at`,
			m.ID, d.Kind.String(), d.Number, d.FileURL, d.UploadedAt); err != nil {
			return fmt.Errorf("save document %s: %w", d.Kind, err)
		}
	}
	return nil
}

// List returns merchants matching a filter, newest first.
//
// The filter is built as parameters rather than interpolated text: every clause
// is optional, and a listing that concatenated an operator-supplied division
// code into SQL would be an injection in the admin console.
func (r *Repository) List(ctx context.Context, f ports.Filter) ([]domain.Merchant, error) {
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 5)

	if f.Status != "" {
		args = append(args, f.Status.String())
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.Type != "" {
		args = append(args, f.Type.String())
		clauses = append(clauses, fmt.Sprintf("type = $%d", len(args)))
	}
	if f.DivisionCode != "" {
		args = append(args, f.DivisionCode)
		clauses = append(clauses, fmt.Sprintf("division_code = $%d", len(args)))
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM merchants%s ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`,
			merchantColumns, where, len(args)-1, len(args)),
		args...)
	if err != nil {
		return nil, fmt.Errorf("list merchants: %w", err)
	}
	defer rows.Close()

	var found []domain.Merchant
	for rows.Next() {
		merchant, scanErr := scanMerchant(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan merchant: %w", scanErr)
		}
		found = append(found, merchant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read merchants: %w", err)
	}

	// Documents are read after the rows are closed rather than inside the loop:
	// pgx holds one connection per open result set, and a nested query on the
	// same connection would deadlock against the pool under load.
	for i := range found {
		documents, docErr := r.documents(ctx, found[i].ID)
		if docErr != nil {
			return nil, docErr
		}
		found[i].Documents = documents
	}
	return found, nil
}

// RecordStatusChange appends to the audit trail.
func (r *Repository) RecordStatusChange(ctx context.Context, e ports.StatusChange) error {
	if _, err := r.db.Exec(ctx, `
		INSERT INTO merchant_status_events (id, merchant_id, from_status, to_status, actor_user_id, note, at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.ID, e.MerchantID, e.From.String(), e.To.String(), e.ActorUserID, e.Note, e.At); err != nil {
		return fmt.Errorf("record status change: %w", err)
	}
	return nil
}

// StatusHistory returns a merchant's decisions, newest first.
func (r *Repository) StatusHistory(ctx context.Context, merchantID string, limit int) ([]ports.StatusChange, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, merchant_id, from_status, to_status, actor_user_id, note, at
		FROM merchant_status_events WHERE merchant_id = $1
		ORDER BY at DESC, id DESC LIMIT $2`, merchantID, limit)
	if err != nil {
		return nil, fmt.Errorf("read status history: %w", err)
	}
	defer rows.Close()

	var out []ports.StatusChange
	for rows.Next() {
		var e ports.StatusChange
		var from, to string
		if err := rows.Scan(&e.ID, &e.MerchantID, &from, &to, &e.ActorUserID, &e.Note, &e.At); err != nil {
			return nil, fmt.Errorf("scan status change: %w", err)
		}
		// Statuses are read back as written rather than re-parsed: history is a
		// record of what happened, and a status we have since retired must
		// still be readable rather than failing the whole page.
		e.From = domain.Status(from)
		e.To = domain.Status(to)
		e.At = e.At.UTC()
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read status history: %w", err)
	}
	return out, nil
}

// inTx runs fn in a transaction, rolling back on any failure.
func (r *Repository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(tx); err != nil {
		// The rollback error is discarded: the original failure is what the
		// caller needs, and reporting a rollback problem instead would hide it.
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
