package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/config/domain"
)

// Service implements contract.ConfigContract.
//
// It is the single adapter between config's domain and the primitives other
// modules see, so a consumer never links config's internals.
type Service struct {
	resolve *ResolveUseCase
}

// NewService wires the public service.
func NewService(resolve *ResolveUseCase) *Service { return &Service{resolve: resolve} }

// Settings resolves configuration for a placement.
func (s *Service) Settings(ctx context.Context, p contract.Placement) (contract.Settings, error) {
	resolved, err := s.resolve.Execute(ctx, domain.Placement{
		AreaCode:     p.AreaCode,
		DistrictCode: p.DistrictCode,
		DivisionCode: p.DivisionCode,
	})
	if err != nil {
		return nil, err
	}
	return snapshot{resolved: resolved}, nil
}

// snapshot adapts domain.Resolved to contract.Settings.
type snapshot struct {
	resolved domain.Resolved
}

func (s snapshot) Int(key string) (int64, error)     { return s.resolved.Int(domain.Key(key)) }
func (s snapshot) Bool(key string) (bool, error)     { return s.resolved.Bool(domain.Key(key)) }
func (s snapshot) Ratio(key string) (float64, error) { return s.resolved.Ratio(domain.Key(key)) }
