// Package contract is the geo module's only public surface.
//
// Other modules may not import geo's domain, application or infrastructure
// (05-architecture.md 2.5). They depend on this interface and reach it through
// their own external/ service, so extracting geo into a real service later
// means rewriting one file per consumer.
//
// Consumed by: discovery, dispatch, pricing, merchant, user (Appendix A).
package contract

import "context"

// Point is a plain coordinate pair. The contract uses primitives rather than
// geo's domain types so a consumer never links against geo's internals, and so
// the same shape survives a move to HTTP or gRPC.
type Point struct {
	Lat float64
	Lng float64
}

// Area is the administrative placement of a point.
type Area struct {
	AreaCode     string
	AreaName     string
	DistrictCode string
	DivisionCode string
	DivisionName string
}

// NearbyMerchant is one result of a radius search.
type NearbyMerchant struct {
	MerchantID string
	Lat        float64
	Lng        float64
	DistanceM  float64
}

// GeoContract is the geo module's public interface.
type GeoContract interface {
	// ResolveArea places a point in the administrative hierarchy.
	ResolveArea(ctx context.Context, p Point) (Area, error)

	// MerchantsWithinRadius returns merchants within radiusM metres of p,
	// nearest first, never crossing the division boundary (D3).
	MerchantsWithinRadius(ctx context.Context, p Point, radiusM float64, limit int) ([]NearbyMerchant, error)

	// CountMerchantsWithinRadius returns only how many merchants are in range,
	// which is what the radius-expansion decision needs (ALG-02).
	CountMerchantsWithinRadius(ctx context.Context, p Point, radiusM float64) (int, error)

	// DistanceBetween returns the great-circle distance in metres. Pricing and
	// dispatch use it rather than computing distance themselves, so every part
	// of the system agrees on one number.
	DistanceBetween(ctx context.Context, a, b Point) (float64, error)
}
