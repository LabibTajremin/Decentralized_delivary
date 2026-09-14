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

// MerchantPlacement is a merchant's location as the merchant module states it.
//
// Geo resolves the administrative placement itself rather than taking it from
// the caller: the division decides what a customer can ever see (D3), and a
// division code supplied by another module is a division code that can be wrong.
type MerchantPlacement struct {
	MerchantID string
	Lat        float64
	Lng        float64
	// Active is whether the merchant should appear in radius searches at all.
	// Approval and suspension are the merchant module's decisions; geo only
	// records the answer so the search can filter on it in the index.
	Active bool
}

// GeoContract is the geo module's public interface.
type GeoContract interface {
	// ResolveArea places a point in the administrative hierarchy, requiring
	// both a division and a mapped area. This is what a delivery address needs:
	// config resolution, pricing and dispatch all key off the area, so an
	// address we cannot place to that precision is an order that reaches
	// checkout and then cannot be priced.
	ResolveArea(ctx context.Context, p Point) (Area, error)

	// ResolveDivision places a point when only the division is required. The
	// returned Area always carries a DivisionCode; its AreaCode is empty when
	// the point falls outside every area we have drawn.
	//
	// Separate from ResolveArea because D1 and delivery want different things.
	// A merchant may register from anywhere in Bangladesh, and whether they may
	// must not depend on how finely we have mapped their upazila — so
	// registration asks only "is this in a division", while an address still
	// has to land in an area.
	ResolveDivision(ctx context.Context, p Point) (Area, error)

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

	// PlaceMerchant records where a merchant is and whether it is searchable.
	//
	// The merchant module owns the merchant record; geo owns the spatial index
	// over it. Writing through this method rather than letting merchant insert
	// into geo's table keeps the rule that a module never touches another's
	// storage, and keeps the D3 division code derived by the module that owns
	// the boundaries.
	PlaceMerchant(ctx context.Context, m MerchantPlacement) error

	// RemoveMerchant drops a merchant from the spatial index entirely. Used
	// when a registration is abandoned; suspension sets Active instead, so the
	// location survives a reinstatement.
	RemoveMerchant(ctx context.Context, merchantID string) error
}
