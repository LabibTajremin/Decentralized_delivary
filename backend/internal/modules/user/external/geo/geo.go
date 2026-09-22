// Package geo is the user module's view of the geo module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5). Extracting geo into
// its own service is then a change to this file alone.
package geo

import (
	"context"

	geocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
)

// Point is a coordinate pair.
type Point = geocontract.Point

// Area is the administrative placement of a point.
type Area = geocontract.Area

// Service is the part of the geo module the user module depends on.
//
// One method. The user module places addresses and does nothing else spatial,
// and an interface that declared more would let it quietly grow a dependency on
// merchant search or distance.
type Service interface {
	ResolveArea(ctx context.Context, p Point) (Area, error)
}
