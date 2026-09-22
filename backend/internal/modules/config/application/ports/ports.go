// Package ports declares what the config use cases need from the outside world.
package ports

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
)

// ConfigRepository stores overrides and the audit log.
//
// Reads and writes are separated deliberately: the read path is on every
// pricing and discovery request and has to be cheap, while the write path is an
// admin action a few times a day and can afford to be careful.
type ConfigRepository interface {
	// OverridesFor returns every override that could apply to a placement —
	// its area, its district, its division, and global. Returning all of them
	// and resolving in the domain keeps the precedence rule in one place
	// instead of encoding it in SQL.
	OverridesFor(ctx context.Context, placement domain.Placement) ([]domain.Override, error)

	// AllOverrides returns every override at one scope, for the admin UI.
	AllOverrides(ctx context.Context, scope domain.Scope) ([]domain.Override, error)

	// SaveOverride writes an override and its audit entry together.
	//
	// One method rather than two, because a change that is applied but not
	// recorded is exactly the change someone will need to explain later. The
	// implementation is responsible for making them atomic.
	SaveOverride(ctx context.Context, override domain.Override, change domain.Change) error

	// DeleteOverride removes an override, reverting to the next scope up, and
	// records that too.
	DeleteOverride(ctx context.Context, key domain.Key, scope domain.Scope, change domain.Change) error

	// Changes returns audit entries, newest first.
	Changes(ctx context.Context, filter ChangeFilter) ([]domain.Change, error)
}

// ChangeFilter narrows an audit query.
type ChangeFilter struct {
	// Key, when set, limits to one variable.
	Key domain.Key
	// Scope, when set, limits to one scope.
	Scope *domain.Scope
	// Limit caps the result. Zero means the repository's default.
	Limit int
}
