// Package contract is the discovery module's only public surface.
//
// Consumed by: cart (Appendix A).
//
// Cart holds a merchant a customer chose and a delivery address they may have
// changed since. It needs one answer — may this address still order from this
// shop, and how far is it — without re-implementing the radius ladder or
// reading geo itself.
package contract

import "context"

// Point is a plain coordinate pair, as geo's contract states it.
type Point struct {
	Lat float64
	Lng float64
}

// Reach is whether one merchant is visible from one point, and at what cost.
type Reach struct {
	MerchantID string
	// AreaCode, DistrictCode and DivisionCode are where the *customer* is, as
	// discovery had to resolve it to answer at all. Carried so a caller that
	// needs the placement — to price the delivery, say — does not make a second
	// geo call for an answer this one already has.
	AreaCode     string
	DistrictCode string
	DivisionCode string
	// DistanceM is the straight-line distance in metres.
	DistanceM float64
	// DistanceText is the distance as the customer should read it, composed by
	// the server (2.9).
	DistanceText string
	// Reachable is whether this merchant is inside the widest radius the
	// customer's division allows. False also covers a merchant in another
	// division entirely — D3 makes that unreachable at any radius.
	Reachable bool
	// RequiredLevel is the expansion level needed to see this merchant, 0 when
	// it is inside the base radius. Meaningless when Reachable is false.
	RequiredLevel int
	// Expanded is whether seeing this merchant needs an expanded radius, which
	// is what makes the delivery fee higher (D2).
	Expanded bool
	// Reason names why an unreachable merchant is unreachable:
	// "outside_division", "beyond_max_radius", or "" when it is reachable.
	Reason string
}

// The reasons a merchant may be out of reach.
const (
	ReasonOutsideDivision = "outside_division"
	ReasonBeyondMaxRadius = "beyond_max_radius"
)

// DiscoveryContract is the discovery module's public interface.
type DiscoveryContract interface {
	// Reach reports whether a merchant is visible from a point, applying the
	// same radius ladder and the same division ceiling a search would.
	Reach(ctx context.Context, from Point, merchantID string) (Reach, error)
}
