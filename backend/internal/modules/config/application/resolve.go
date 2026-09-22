// Package application holds the config use cases.
package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// ResolveUseCase produces the effective configuration for a placement.
type ResolveUseCase struct {
	repo ports.ConfigRepository
}

// NewResolveUseCase wires the use case.
func NewResolveUseCase(repo ports.ConfigRepository) *ResolveUseCase {
	return &ResolveUseCase{repo: repo}
}

// Execute resolves configuration for a placement.
//
// A repository failure is an error rather than a silent fall back to defaults.
// Quietly serving global defaults during a database outage would mean an area
// with a 15 km radius silently reverting to 5 km, and customers being told
// there are no merchants near them — a wrong answer delivered confidently is
// worse than an error the caller can retry.
func (uc *ResolveUseCase) Execute(ctx context.Context, placement domain.Placement) (domain.Resolved, error) {
	overrides, err := uc.repo.OverridesFor(ctx, placement)
	if err != nil {
		return domain.Resolved{}, errs.Wrap(err, errs.KindUnavailable, "config_unavailable",
			"We could not load settings for this area. Please try again.")
	}
	return domain.Resolve(placement, overrides), nil
}
