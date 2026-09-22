// Package contract is the merchant module's only public surface.
//
// Consumed by: discovery, catalogue, order (Appendix A).
//
// Discovery asks which of the ids a radius search returned may actually be
// shown; catalogue asks what kind of shop it is attaching items to; order asks
// where to collect from and whether the shop is taking orders at all. None of
// them may read a merchant table, which is why everything they need is a
// primitive here.
package contract

import "context"

// Type is what kind of shop this is, as a primitive.
type Type string

// The merchant types, mirroring the domain so a consumer never links it.
const (
	TypeRestaurant Type = "restaurant"
	TypeGrocery    Type = "grocery"
	TypePharmacy   Type = "pharmacy"
)

// Merchant is a shop as the rest of the system sees it.
//
// Note what is absent: documents, the owner's contact details, the review note
// and the status history. Those exist for the owner and for an admin, and a
// contract that handed them to discovery would put a trade licence number one
// careless handler away from a customer's screen.
type Merchant struct {
	ID          string
	OwnerUserID string
	Name        string
	Type        Type
	// Phone is the shop's public line, which a rider calls on arrival.
	Phone   string
	LogoURL string

	Line1      string
	Line2      string
	SingleLine string
	Lat        float64
	Lng        float64

	AreaCode     string
	AreaName     string
	DistrictCode string
	DivisionCode string

	// IsListed is whether a customer may see this shop at all: approved, and
	// not closed for a holiday.
	IsListed bool
	// IsOpenNow is whether it is taking orders this minute. A listed shop that
	// is shut for the evening is still worth showing — with when it opens.
	IsOpenNow bool
	// OpenStatus is the preformatted line to display, composed by the server so
	// every client shows the same words (2.9).
	OpenStatus string
}

// MerchantContract is the merchant module's public interface.
type MerchantContract interface {
	// Merchant returns one shop, whatever its status. Callers that must not
	// show an unapproved shop check IsListed; order needs to resolve a merchant
	// it already has an order against even after a suspension.
	Merchant(ctx context.Context, merchantID string) (Merchant, error)

	// Listed returns the subset of these ids a customer may see, in the order
	// they were given.
	//
	// The order is preserved because the caller is discovery, handing over the
	// result of a nearest-first radius search: re-sorting here would throw away
	// the distance ordering the query paid for, and a caller that had to
	// re-sort would need the distances this contract deliberately does not
	// carry.
	Listed(ctx context.Context, merchantIDs []string) ([]Merchant, error)

	// IsAcceptingOrders reports whether a shop can take an order right now.
	IsAcceptingOrders(ctx context.Context, merchantID string) (bool, error)

	// OwnedBy returns the ids of the shops an account owns.
	//
	// Added in P11. The order module has to answer "is this shop yours" before
	// it shows a queue or accepts an order on its behalf, and it cannot read
	// the merchant table to find out (2.5). A slice rather than a single id
	// because one owner per shop is today's rule and not a rule of the
	// domain — a chain with three branches is a merchant conversation, not a
	// schema migration for every consumer.
	OwnedBy(ctx context.Context, ownerUserID string) ([]string, error)
}
