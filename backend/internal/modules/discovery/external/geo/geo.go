// Package geo is the discovery module's view of the geo module.
//
// Cross-module access goes through a module's own external/ package, depending
// only on the target's contract (05-architecture.md 2.5). Extracting geo into
// its own service is then a change to this file alone.
package geo

import (
	"context"

	geocontract "github.com/rootlogic-lab/delivery/backend/internal/modules/geo/contract"
)

// Point is a coordinate pair, aliased from geo's contract so discovery reads in
// its own vocabulary without copying the type.
type Point = geocontract.Point

// Area is the administrative placement of a point.
type Area = geocontract.Area

// NearbyMerchant is one result of a radius search.
type NearbyMerchant = geocontract.NearbyMerchant

// Service is the part of geo discovery depends on.
//
// Four methods, all reads. Discovery never writes to the spatial index — that
// is merchant's business — and an interface that offered PlaceMerchant here
// would let a search endpoint move a shop.
type Service interface {
	// ResolveDivision places the customer. Discovery asks for the division
	// rather than the area because D3 is about the division, and a customer
	// standing in an upazila we have not drawn an area for must still be able
	// to search.
	ResolveDivision(ctx context.Context, p Point) (Area, error)

	// MerchantsWithinRadius is ALG-01, already division-bounded by geo.
	MerchantsWithinRadius(ctx context.Context, p Point, radiusM float64, limit int) ([]NearbyMerchant, error)

	// CountMerchantsWithinRadius is what the expansion decision needs without
	// transferring rows nobody will read (ALG-02).
	CountMerchantsWithinRadius(ctx context.Context, p Point, radiusM float64) (int, error)

	// DistanceBetween answers a single-merchant reachability question.
	DistanceBetween(ctx context.Context, a, b Point) (float64, error)
}
