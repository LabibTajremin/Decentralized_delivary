package domain

import "sort"

// ALG-07 — search ranking. A weighted score over four signals, then a sort.
// O(n log n) on the candidates one radius query returned, which is bounded.
//
// The weights are here, together, as named constants rather than scattered
// through a comparison function: ranking is the thing most likely to be argued
// about, and an argument about whether distance should matter more than rating
// should be a one-line change with a test, not an archaeology exercise.
const (
	weightRelevance    = 0.40
	weightDistance     = 0.35
	weightRating       = 0.15
	weightAvailability = 0.10
)

// Candidate is one merchant as the ranker sees it.
type Candidate struct {
	// ID identifies the merchant. The ranker does not care what it is beyond
	// keeping the sort stable and reproducible.
	ID string
	// Relevance is how well the merchant matched the query text, 0 to 1. A
	// browse with no query gives every candidate the same value, which makes
	// the term drop out and leaves distance in charge — which is what a
	// customer opening the app expects to see.
	Relevance float64
	// DistanceM is metres from the customer.
	DistanceM float64
	// Rating is the merchant's average review score out of five, or 0 when it
	// has none. Reviews are P16; until then every candidate is unrated, the
	// term contributes nothing, and nothing here needs to change when they
	// arrive.
	Rating float64
	// Open is whether the shop is taking orders right now. A closed shop is
	// still worth showing — with when it opens — but not above an open one.
	Open bool
}

// Rank sorts candidates best-first, in place.
//
// radiusM normalises the distance term: "far" means far *for this search*, so
// the same shop ranks differently in a 5 km browse and a 25 km expanded one.
// The tie-break on ID is not cosmetic — two shops with identical scores must
// come back in the same order on every request, or a customer paging through
// results sees the same shop twice and misses another.
func Rank(candidates []Candidate, radiusM float64) {
	scores := make(map[string]float64, len(candidates))
	for _, c := range candidates {
		scores[c.ID] = Score(c, radiusM)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		si, sj := scores[candidates[i].ID], scores[candidates[j].ID]
		if si != sj {
			return si > sj
		}
		return candidates[i].ID < candidates[j].ID
	})
}

// Score is one candidate's weighted score, 0 to 1.
func Score(c Candidate, radiusM float64) float64 {
	return weightRelevance*clamp01(c.Relevance) +
		weightDistance*proximity(c.DistanceM, radiusM) +
		weightRating*clamp01(c.Rating/5) +
		weightAvailability*boolScore(c.Open)
}

// proximity turns a distance into a 0-to-1 score, nearest scoring highest.
//
// Linear rather than inverse-square: a customer choosing between a shop 1 km
// away and one 3 km away is making a mild preference, not a decision that
// should bury the second one.
func proximity(distanceM, radiusM float64) float64 {
	if radiusM <= 0 {
		return 0
	}
	return clamp01(1 - distanceM/radiusM)
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func boolScore(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
