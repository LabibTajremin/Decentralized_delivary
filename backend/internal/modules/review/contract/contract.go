// Package contract is the review module's only public surface.
//
// Consumed by: merchant, dispatch (Appendix A). A shop showing its own
// storefront its rating, or a partner app showing a rider their own score,
// reaches it through this — not a review table.
package contract

import "context"

// Rating is one subject's reviews, reduced to what another module needs: a
// mean and how many opinions it rests on. Zero value for a subject nobody
// has reviewed yet — not an error, the way a shop that just opened has no
// rating rather than a broken one.
type Rating struct {
	SubjectID string
	Average   float64
	Count     int
}

// ReviewContract is the review module's public interface.
type ReviewContract interface {
	// MerchantRating returns a merchant's aggregate rating.
	MerchantRating(ctx context.Context, merchantID string) (Rating, error)
	// PartnerRating returns a delivery partner's aggregate rating.
	PartnerRating(ctx context.Context, partnerID string) (Rating, error)
}
