// Package geo is the dispatch module's view of the geo module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5).
package geo

import (
	"context"

	geocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
)

// Point is a coordinate pair.
type Point = geocontract.Point

// Area is the administrative placement of a point.
type Area = geocontract.Area

// Service is the part of geo dispatch depends on.
//
// Every distance in the system comes from geo so that pricing, dispatch and the
// client all quote one number — a rider told they are 2 km from a shop that
// charged the customer for 3 km is a rider in an argument.
//
// ResolveDivision places a rider so the dispatch settings they work under are
// the ones configured where they are standing. The lenient resolution, not the
// strict one: a rider in an upazila nobody has drawn an area for still gets
// offered work, which is D1 read from the partner's side.
type Service interface {
	DistanceBetween(ctx context.Context, a, b Point) (float64, error)
	ResolveDivision(ctx context.Context, p Point) (Area, error)
}
