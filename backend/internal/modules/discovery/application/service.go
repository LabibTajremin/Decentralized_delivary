package application

import (
	"context"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/contract"
	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/external/geo"
)

// Service implements contract.DiscoveryContract.
//
// A thin adapter over ReachUseCase, for the same reason every module has one:
// the contract speaks primitives so a consumer never links discovery's
// internals, and the day discovery becomes its own service the consumer's
// external/ package changes and nothing else does.
type Service struct {
	reach *ReachUseCase
}

// NewService wires the public service.
func NewService(reach *ReachUseCase) *Service { return &Service{reach: reach} }

// Reach reports whether a merchant is visible from a point.
//
// The language is fixed to Bengali here rather than taken as an argument: the
// caller is another module, not a request handler, and a contract that carried
// a locale would put every consumer in the business of guessing one.
func (s *Service) Reach(ctx context.Context, from contract.Point, merchantID string) (contract.Reach, error) {
	return s.reach.Execute(ctx, geo.Point{Lat: from.Lat, Lng: from.Lng}, merchantID, "")
}
