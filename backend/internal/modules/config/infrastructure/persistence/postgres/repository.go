// Package postgres implements the config repository.
//
// Engine-specific SQL is confined here (05-architecture.md 2.4). The precedence
// rule deliberately is not: resolution happens in the domain, so the ordering
// that decides every fee in the country is expressed once, in Go, where it is
// readable and testable — not spread across a query's ORDER BY.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
)

// Querier is the read surface this repository needs.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// TxBeginner starts a transaction. An override and its audit entry are written
// together or not at all.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Repository reads and writes configuration.
type Repository struct {
	db Querier
	tx TxBeginner
}

// New builds a repository over anything that can query and begin.
func New(db Querier, tx TxBeginner) *Repository { return &Repository{db: db, tx: tx} }

// NewFromPool is the production constructor.
func NewFromPool(pool *pgxpool.Pool) *Repository { return &Repository{db: pool, tx: pool} }

// OverridesFor loads every override that could apply to a placement.
//
// One query for all four scopes rather than four queries: this runs on every
// pricing and discovery request, and three extra round trips per request is a
// cost paid on the hot path for no benefit.
func (r *Repository) OverridesFor(ctx context.Context, placement domain.Placement) ([]domain.Override, error) {
	rows, err := r.db.Query(ctx, `
		SELECT config_key, scope_level, scope_code, value, pinned
		FROM config_overrides
		WHERE scope_level = 'global'
		   OR (scope_level = 'division' AND scope_code = $1)
		   OR (scope_level = 'district' AND scope_code = $2)
		   OR (scope_level = 'area'     AND scope_code = $3)`,
		placement.DivisionCode, placement.DistrictCode, placement.AreaCode)
	if err != nil {
		return nil, fmt.Errorf("load config overrides: %w", err)
	}
	defer rows.Close()
	return scanOverrides(rows)
}

// AllOverrides returns every override at one scope.
func (r *Repository) AllOverrides(ctx context.Context, scope domain.Scope) ([]domain.Override, error) {
	rows, err := r.db.Query(ctx, `
		SELECT config_key, scope_level, scope_code, value, pinned
		FROM config_overrides
		WHERE scope_level = $1 AND scope_code = $2
		ORDER BY config_key`,
		scope.Level.String(), scope.Code)
	if err != nil {
		return nil, fmt.Errorf("load overrides for %s: %w", scope, err)
	}
	defer rows.Close()
	return scanOverrides(rows)
}

// scanOverrides reads override rows, skipping any whose key or type no longer
// matches the registry.
//
// Skipping rather than failing is deliberate: a key removed from the registry
// leaves rows behind, and one stale row must not stop the whole system loading
// its configuration. Resolve applies the same rule, so a skipped row behaves
// exactly as if it were absent.
func scanOverrides(rows pgx.Rows) ([]domain.Override, error) {
	var out []domain.Override
	for rows.Next() {
		var key, level, code, raw string
		var pinned bool
		if err := rows.Scan(&key, &level, &code, &raw, &pinned); err != nil {
			return nil, fmt.Errorf("scan config override: %w", err)
		}

		def, err := domain.Lookup(domain.Key(key))
		if err != nil {
			continue
		}
		value, err := domain.Parse(def.Kind, raw)
		if err != nil {
			continue
		}
		parsedLevel, err := domain.ParseLevel(level)
		if err != nil {
			continue
		}
		scope, err := domain.NewScope(parsedLevel, code)
		if err != nil {
			continue
		}
		out = append(out, domain.Override{
			Key:    domain.Key(key),
			Scope:  scope,
			Value:  value,
			Pinned: pinned,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read config overrides: %w", err)
	}
	return out, nil
}

// SaveOverride writes the override and its audit entry in one transaction.
func (r *Repository) SaveOverride(ctx context.Context, o domain.Override, change domain.Change) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO config_overrides (config_key, scope_level, scope_code, value, pinned)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (config_key, scope_level, scope_code) DO UPDATE
			    SET value = EXCLUDED.value,
			        pinned = EXCLUDED.pinned,
			        updated_at = now()`,
			string(o.Key), o.Scope.Level.String(), o.Scope.Code, o.Value.String(), o.Pinned); err != nil {
			return fmt.Errorf("save override: %w", err)
		}
		return insertChange(ctx, tx, change)
	})
}

// DeleteOverride removes an override and records the removal.
func (r *Repository) DeleteOverride(ctx context.Context, key domain.Key, scope domain.Scope, change domain.Change) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			DELETE FROM config_overrides
			WHERE config_key = $1 AND scope_level = $2 AND scope_code = $3`,
			string(key), scope.Level.String(), scope.Code); err != nil {
			return fmt.Errorf("delete override: %w", err)
		}
		return insertChange(ctx, tx, change)
	})
}

// insertChange appends one audit entry.
func insertChange(ctx context.Context, tx pgx.Tx, c domain.Change) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO config_changes
		    (id, config_key, scope_level, scope_code, old_value, new_value,
		     actor_kind, actor_id, reason, changed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		c.ID, string(c.Key), c.Scope.Level.String(), c.Scope.Code,
		c.OldValue.String(), c.NewValue.String(),
		c.Actor.Kind, c.Actor.ID, c.Reason, c.At); err != nil {
		return fmt.Errorf("record config change: %w", err)
	}
	return nil
}

// defaultChangeLimit caps an unfiltered audit query. The log grows without
// bound, and an admin screen asking for "recent changes" must not pull years of
// history to render one page.
const defaultChangeLimit = 100

// Changes returns audit entries, newest first.
func (r *Repository) Changes(ctx context.Context, filter ports.ChangeFilter) ([]domain.Change, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultChangeLimit
	}

	var level, code string
	if filter.Scope != nil {
		level, code = filter.Scope.Level.String(), filter.Scope.Code
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, config_key, scope_level, scope_code, old_value, new_value,
		       actor_kind, actor_id, reason, changed_at
		FROM config_changes
		WHERE ($1 = '' OR config_key = $1)
		  AND ($2 = '' OR (scope_level = $2 AND scope_code = $3))
		ORDER BY changed_at DESC, id DESC
		LIMIT $4`,
		string(filter.Key), level, code, limit)
	if err != nil {
		return nil, fmt.Errorf("read config changes: %w", err)
	}
	defer rows.Close()

	var out []domain.Change
	for rows.Next() {
		var c domain.Change
		var key, levelText, codeText, oldRaw, newRaw string
		if err := rows.Scan(&c.ID, &key, &levelText, &codeText, &oldRaw, &newRaw,
			&c.Actor.Kind, &c.Actor.ID, &c.Reason, &c.At); err != nil {
			return nil, fmt.Errorf("scan config change: %w", err)
		}

		c.Key = domain.Key(key)
		parsedLevel, err := domain.ParseLevel(levelText)
		if err != nil {
			return nil, fmt.Errorf("audit entry %s: %w", c.ID, err)
		}
		c.Scope, err = domain.NewScope(parsedLevel, codeText)
		if err != nil {
			return nil, fmt.Errorf("audit entry %s: %w", c.ID, err)
		}

		// An audit entry survives its key being removed from the registry. It
		// is a historical record, so the values are reported as written rather
		// than being dropped for failing today's validation.
		if def, defErr := domain.Lookup(c.Key); defErr == nil {
			if v, parseErr := domain.Parse(def.Kind, oldRaw); parseErr == nil {
				c.OldValue = v
			}
			if v, parseErr := domain.Parse(def.Kind, newRaw); parseErr == nil {
				c.NewValue = v
			}
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read config changes: %w", err)
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
		// The rollback error is deliberately discarded: the original failure is
		// what the caller needs, and reporting a rollback problem instead would
		// hide it.
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
