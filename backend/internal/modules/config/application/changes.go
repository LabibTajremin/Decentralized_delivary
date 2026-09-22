package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/application/ports"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// ListChangesUseCase serves the configuration audit log — D5's own
// requirement that every change, admin or tuner, is recorded with its
// reasoning and can be read back.
type ListChangesUseCase struct {
	repo ports.ConfigRepository
}

// NewListChangesUseCase wires the use case.
func NewListChangesUseCase(repo ports.ConfigRepository) *ListChangesUseCase {
	return &ListChangesUseCase{repo: repo}
}

// Execute lists audit entries, newest first.
func (uc *ListChangesUseCase) Execute(ctx context.Context, filter ports.ChangeFilter) ([]domain.Change, error) {
	changes, err := uc.repo.Changes(ctx, filter)
	if err != nil {
		return nil, errs.Wrap(err, errs.KindUnavailable, "config_unavailable",
			"We could not load the change history. Please try again.")
	}
	return changes, nil
}
