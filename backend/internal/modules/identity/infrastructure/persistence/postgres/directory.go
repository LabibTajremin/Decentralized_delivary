// Package postgres stores identity accounts.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/identity/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// ErrAccountSuspended means the account exists but has been deactivated.
var ErrAccountSuspended = errors.New("account is suspended")

// Querier is the database surface this directory needs.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Directory maps phone numbers to accounts.
type Directory struct {
	db  Querier
	ids id.Generator
}

// New builds a directory.
func New(db Querier, ids id.Generator) *Directory { return &Directory{db: db, ids: ids} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool, ids id.Generator) *Directory {
	return &Directory{db: pool, ids: ids}
}

// EnsureUser returns the account id for a phone and role, creating it on first
// sign-in.
//
// One statement, not a select followed by an insert. Two devices verifying
// codes at the same moment would both find no account and both try to create
// one; the unique constraint would reject the loser and the user would see a
// failed sign-in for no reason they could understand.
func (d *Directory) EnsureUser(ctx context.Context, phone domain.Phone, role domain.Role) (string, bool, error) {
	newID := d.ids.New("usr")

	var storedID string
	var isActive bool
	var createdAt, updatedAt any
	err := d.db.QueryRow(ctx, `
		INSERT INTO identity_accounts (id, phone, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (phone, role) DO UPDATE
		    SET updated_at = now()
		RETURNING id, is_active, created_at, updated_at`,
		newID, phone.String(), role.String()).Scan(&storedID, &isActive, &createdAt, &updatedAt)
	if err != nil {
		return "", false, fmt.Errorf("ensure account: %w", err)
	}

	// A suspended account must not be able to sign in. Checking here rather
	// than in the use case keeps it true for every caller of the directory.
	if !isActive {
		return "", false, ErrAccountSuspended
	}

	// The id we generated came back only if the insert really inserted, which
	// is how "was this their first sign-in" is known without a second query.
	return storedID, storedID == newID, nil
}

// PhoneFor is the reverse lookup: the number behind an account id.
func (d *Directory) PhoneFor(ctx context.Context, userID string) (domain.Phone, bool, error) {
	var raw string
	var isActive bool
	err := d.db.QueryRow(ctx, `
		SELECT phone, is_active FROM identity_accounts WHERE id = $1`,
		userID).Scan(&raw, &isActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Phone{}, false, nil
	}
	if err != nil {
		return domain.Phone{}, false, fmt.Errorf("phone for account: %w", err)
	}
	if !isActive {
		return domain.Phone{}, false, nil
	}
	phone, err := domain.NewPhone(raw)
	if err != nil {
		// A phone stored by EnsureUser has already been validated once; this
		// can only mean the stored value and the validator have drifted.
		return domain.Phone{}, false, fmt.Errorf("stored phone %q does not parse: %w", raw, err)
	}
	return phone, true, nil
}
