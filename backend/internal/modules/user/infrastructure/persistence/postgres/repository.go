// Package postgres stores profiles and addresses.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/user/domain"
)

// Querier is the read and write surface these repositories need.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// TxBeginner starts a transaction.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Repository stores profiles and addresses.
type Repository struct {
	db Querier
	tx TxBeginner
}

// New builds a repository.
func New(db Querier, tx TxBeginner) *Repository { return &Repository{db: db, tx: tx} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool) *Repository { return &Repository{db: pool, tx: pool} }

// Profile returns a user's profile.
func (r *Repository) Profile(ctx context.Context, userID string) (domain.Profile, error) {
	var name, email, language string
	err := r.db.QueryRow(ctx, `
		SELECT name, email, language FROM user_profiles WHERE user_id = $1`,
		userID).Scan(&name, &email, &language)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	if err != nil {
		return domain.Profile{}, fmt.Errorf("read profile: %w", err)
	}

	parsed, err := domain.ParseLanguage(language)
	if err != nil {
		// A stored language outside the supported set is a schema drift, not a
		// reason to lock someone out of their profile. The default applies and
		// their next save corrects the row.
		parsed = domain.DefaultLanguage
	}
	return domain.NewProfile(userID, name, email, parsed)
}

// SaveProfile writes a profile, creating it if absent.
func (r *Repository) SaveProfile(ctx context.Context, profile domain.Profile) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_profiles (user_id, name, email, language)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		    SET name = EXCLUDED.name,
		        email = EXCLUDED.email,
		        language = EXCLUDED.language,
		        updated_at = now()`,
		profile.UserID, profile.Name, profile.Email, profile.Language.String())
	if err != nil {
		return fmt.Errorf("save profile: %w", err)
	}
	return nil
}

// addressColumns is the projection every address read shares, so one added
// column cannot be forgotten in one of three places.
const addressColumns = `
	id, user_id, label, recipient_name, recipient_phone,
	line1, line2, instructions,
	ST_Y(pin::geometry), ST_X(pin::geometry),
	area_code, area_name, district_code, division_code, is_default`

// scanAddress reads one row.
func scanAddress(row pgx.Row) (domain.Address, error) {
	var a domain.Address
	var lat, lng float64
	err := row.Scan(&a.ID, &a.UserID, &a.Label, &a.RecipientName, &a.RecipientPhone,
		&a.Line1, &a.Line2, &a.Instructions, &lat, &lng,
		&a.Placement.AreaCode, &a.Placement.AreaName,
		&a.Placement.DistrictCode, &a.Placement.DivisionCode, &a.IsDefault)
	if err != nil {
		return domain.Address{}, err
	}
	a.Pin = domain.Pin{Lat: lat, Lng: lng}
	return a, nil
}

// Addresses returns a user's addresses, oldest first.
func (r *Repository) Addresses(ctx context.Context, userID string) ([]domain.Address, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+addressColumns+`
		FROM user_addresses WHERE user_id = $1
		ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list addresses: %w", err)
	}
	defer rows.Close()

	var out []domain.Address
	for rows.Next() {
		address, scanErr := scanAddress(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan address: %w", scanErr)
		}
		out = append(out, address)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read addresses: %w", err)
	}
	return out, nil
}

// Address returns one address belonging to a user.
func (r *Repository) Address(ctx context.Context, userID, addressID string) (domain.Address, error) {
	// The user id is in the WHERE clause, not checked afterwards. A query that
	// can return another user's address is one careless call away from leaking
	// a home address and a phone number.
	address, err := scanAddress(r.db.QueryRow(ctx, `
		SELECT `+addressColumns+`
		FROM user_addresses WHERE user_id = $1 AND id = $2`, userID, addressID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Address{}, domain.ErrAddressNotFound
	}
	if err != nil {
		return domain.Address{}, fmt.Errorf("read address: %w", err)
	}
	return address, nil
}

// DefaultAddress returns the user's default.
func (r *Repository) DefaultAddress(ctx context.Context, userID string) (domain.Address, error) {
	address, err := scanAddress(r.db.QueryRow(ctx, `
		SELECT `+addressColumns+`
		FROM user_addresses WHERE user_id = $1 AND is_default
		LIMIT 1`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Address{}, domain.ErrNoDefaultAddress
	}
	if err != nil {
		return domain.Address{}, fmt.Errorf("read default address: %w", err)
	}
	return address, nil
}

// SaveAddresses writes a user's whole list in one transaction.
//
// The whole list rather than one row, because "exactly one default" is a
// property of the set: writing rows individually leaves a window where two are
// default or none is, and a concurrent read in that window returns a wrong
// delivery address. The partial unique index would also reject the intermediate
// state, so a row-at-a-time update would simply fail.
func (r *Repository) SaveAddresses(ctx context.Context, userID string, addresses []domain.Address) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		// Clear the flag first so the unique index never sees two defaults
		// mid-transaction.
		if _, err := tx.Exec(ctx,
			`UPDATE user_addresses SET is_default = FALSE WHERE user_id = $1 AND is_default`,
			userID); err != nil {
			return fmt.Errorf("clear defaults: %w", err)
		}

		for _, a := range addresses {
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_addresses (
				    id, user_id, label, recipient_name, recipient_phone,
				    line1, line2, instructions, pin,
				    area_code, area_name, district_code, division_code, is_default)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8,
				        ST_SetSRID(ST_MakePoint($9, $10), 4326)::geography,
				        $11, $12, $13, $14, $15)
				ON CONFLICT (id) DO UPDATE
				    SET label = EXCLUDED.label,
				        recipient_name = EXCLUDED.recipient_name,
				        recipient_phone = EXCLUDED.recipient_phone,
				        line1 = EXCLUDED.line1,
				        line2 = EXCLUDED.line2,
				        instructions = EXCLUDED.instructions,
				        pin = EXCLUDED.pin,
				        area_code = EXCLUDED.area_code,
				        area_name = EXCLUDED.area_name,
				        district_code = EXCLUDED.district_code,
				        division_code = EXCLUDED.division_code,
				        is_default = EXCLUDED.is_default,
				        updated_at = now()`,
				a.ID, userID, a.Label, a.RecipientName, a.RecipientPhone,
				a.Line1, a.Line2, a.Instructions, a.Pin.Lng, a.Pin.Lat,
				a.Placement.AreaCode, a.Placement.AreaName,
				a.Placement.DistrictCode, a.Placement.DivisionCode, a.IsDefault); err != nil {
				return fmt.Errorf("save address %s: %w", a.ID, err)
			}
		}
		return nil
	})
}

// DeleteAddress removes one address.
func (r *Repository) DeleteAddress(ctx context.Context, userID, addressID string) error {
	if _, err := r.db.Exec(ctx,
		`DELETE FROM user_addresses WHERE user_id = $1 AND id = $2`,
		userID, addressID); err != nil {
		return fmt.Errorf("delete address: %w", err)
	}
	return nil
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
