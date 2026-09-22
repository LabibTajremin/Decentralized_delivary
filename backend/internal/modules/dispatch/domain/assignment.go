package domain

import "container/heap"

// ALG-04 — delivery partner assignment. A min-heap scored on distance, current
// load and acceptance rate. O(n log n) to order the candidates, O(log n) to take
// the next one.
//
// A heap rather than a sort, because the caller almost never wants the whole
// order. Assignment offers to the best candidate, waits for
// dispatch.assignment_timeout, and only then wants the second — by which time
// the pool has changed and the rest of a sorted slice would be stale anyway.
//
// The weights are here, together, as named constants. Assignment is the thing
// most likely to be argued about, and an argument about whether load should
// matter more than acceptance rate should be a one-line change with a test.
const (
	weightProximity  = 0.55
	weightLoad       = 0.25
	weightAcceptance = 0.20
)

// Candidate is one partner as the assigner sees them.
type Candidate struct {
	PartnerID string
	// DistanceM is how far the partner is from the *pickup*, not from the
	// customer. The rider has to reach the counter before anything else can
	// happen, and a rider who is already outside the shop is worth more than
	// one who is nearer the customer but forty minutes from the food.
	DistanceM float64
	// Carrying and MaxConcurrent are the load term. A partner at their limit
	// is not a candidate at all, so this only separates the ones who can.
	Carrying      int
	MaxConcurrent int
	// AcceptanceRate is the share of offers they have taken, 0 to 1. A partner
	// who declines everything makes every customer wait for the next round.
	AcceptanceRate float64
}

// Score is one candidate's score, 0 to 1, lower being better.
//
// Lower-is-better so the heap is a min-heap and the arithmetic reads the way it
// means: the distance term is the distance, not one minus it.
func Score(c Candidate, radiusM float64) float64 {
	return weightProximity*clamp01(proximity(c.DistanceM, radiusM)) +
		weightLoad*clamp01(load(c.Carrying, c.MaxConcurrent)) +
		weightAcceptance*clamp01(1-c.AcceptanceRate)
}

// proximity is how far away, normalised against the search radius.
//
// Linear. A rider 1 km from the shop and one 3 km away are a mild preference,
// not a decision that should bury the second — they may be the only two working.
func proximity(distanceM, radiusM float64) float64 {
	if radiusM <= 0 {
		return 0
	}
	return distanceM / radiusM
}

// load is how full a partner already is.
func load(carrying, maxConcurrent int) float64 {
	if maxConcurrent <= 0 {
		return 0
	}
	return float64(carrying) / float64(maxConcurrent)
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

// scored is a candidate with its score, for the heap.
type scored struct {
	candidate Candidate
	score     float64
}

// candidateHeap is a min-heap of scored candidates.
type candidateHeap []scored

func (h candidateHeap) Len() int { return len(h) }

// Less breaks ties on the partner id, so two equally-scored partners are
// offered in the same order every time. Without it, two dispatch rounds a
// second apart could offer the same job to two different people.
func (h candidateHeap) Less(i, j int) bool {
	if h[i].score != h[j].score {
		return h[i].score < h[j].score
	}
	return h[i].candidate.PartnerID < h[j].candidate.PartnerID
}

func (h candidateHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *candidateHeap) Push(x any) { *h = append(*h, x.(scored)) }

func (h *candidateHeap) Pop() any {
	old := *h
	n := len(old)
	last := old[n-1]
	*h = old[:n-1]
	return last
}

// Assigner orders candidates best-first and hands them out one at a time.
type Assigner struct {
	heap    candidateHeap
	radiusM float64
}

// NewAssigner builds the heap. O(n log n).
//
// Pushed one at a time rather than filled and heapified, so a candidate added
// here goes in by exactly the path a candidate added later would — there is one
// way into this heap, and it is tested every time a round runs.
func NewAssigner(candidates []Candidate, radiusM float64) *Assigner {
	h := make(candidateHeap, 0, len(candidates))
	for _, c := range candidates {
		heap.Push(&h, scored{candidate: c, score: Score(c, radiusM)})
	}
	return &Assigner{heap: h, radiusM: radiusM}
}

// Next takes the best remaining candidate. O(log n). The bool is false when
// there is nobody left.
func (a *Assigner) Next() (Candidate, bool) {
	if a.heap.Len() == 0 {
		return Candidate{}, false
	}
	return heap.Pop(&a.heap).(scored).candidate, true
}

// Remaining is how many candidates are left, so a caller can say "no partners
// nearby" rather than "we tried".
func (a *Assigner) Remaining() int { return a.heap.Len() }
