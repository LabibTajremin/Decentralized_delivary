// Package domain holds the rules of local visibility: how far a customer can
// see, how that distance grows when there is nothing nearby, and where it
// stops. It imports nothing outside itself (05-architecture.md 2.2).
//
// This is D1, D2 and D3 expressed as arithmetic. The geography — which
// merchants are actually inside a circle, and which division a point falls in —
// belongs to geo; what this package decides is which circle to ask about.
package domain

import "errors"

// The rules this package refuses to bend.
var (
	ErrRadiusNotPositive  = errors.New("base radius must be greater than zero")
	ErrStepNotPositive    = errors.New("expansion step must be greater than zero")
	ErrExpansionsNegative = errors.New("maximum expansions cannot be negative")
	ErrThresholdNegative  = errors.New("merchant threshold cannot be negative")
	ErrCeilingDisabled    = errors.New("the division ceiling can never be disabled")
)

// Policy is the resolved expansion ladder for one place.
//
// Every field comes from configuration (Appendix B), resolved area → district →
// division → global, which is why two customers a hundred metres apart can have
// different ladders: a Dhaka street needs a 2 km radius and a haor upazila
// needs twenty.
type Policy struct {
	baseRadiusM   float64
	stepM         float64
	maxExpansions int
	minMerchants  int
	autoExpand    bool
}

// NewPolicy builds a policy from resolved configuration.
//
// divisionCeiling is taken as an argument and then required to be true rather
// than simply assumed. Appendix B marks `discovery.division_ceiling` as "never
// disableable", and the only way to keep that true is to have somewhere that
// refuses the false case out loud — otherwise a tuner bug that writes `false`
// silently widens every search in the country to the whole of Bangladesh.
func NewPolicy(baseRadiusM, stepM float64, maxExpansions, minMerchants int, autoExpand, divisionCeiling bool) (Policy, error) {
	switch {
	case baseRadiusM <= 0:
		return Policy{}, ErrRadiusNotPositive
	case stepM <= 0:
		return Policy{}, ErrStepNotPositive
	case maxExpansions < 0:
		return Policy{}, ErrExpansionsNegative
	case minMerchants < 0:
		return Policy{}, ErrThresholdNegative
	case !divisionCeiling:
		return Policy{}, ErrCeilingDisabled
	}
	return Policy{
		baseRadiusM:   baseRadiusM,
		stepM:         stepM,
		maxExpansions: maxExpansions,
		minMerchants:  minMerchants,
		autoExpand:    autoExpand,
	}, nil
}

// BaseRadiusM is the radius a customer searches with before expanding.
func (p Policy) BaseRadiusM() float64 { return p.baseRadiusM }

// StepM is how much one expansion adds.
func (p Policy) StepM() float64 { return p.stepM }

// MaxLevel is the last expansion level the ladder offers. Level 0 is the base
// radius, so a policy allowing four expansions has a maximum level of four.
func (p Policy) MaxLevel() int { return p.maxExpansions }

// MinMerchants is how many shops count as "enough nearby".
func (p Policy) MinMerchants() int { return p.minMerchants }

// AutoExpand reports whether a thin result expands without asking.
func (p Policy) AutoExpand() bool { return p.autoExpand }

// Radius returns the radius for an expansion level.
//
// ALG-02 is stepwise and linear, not geometric: a customer who taps "search
// wider" twice should get a radius they can predict, and a doubling ladder
// reaches the division boundary in three taps from a rural start.
//
// Total, and clamping rather than returning an error. Every caller already
// holds a level this policy produced or clamped, so an error return would be a
// branch no test could reach honestly — and a search that failed because of
// arithmetic on its own ladder would be a worse outcome than the widest
// allowed radius.
func (p Policy) Radius(level int) float64 {
	return p.baseRadiusM + p.stepM*float64(p.ClampLevel(level))
}

// ClampLevel folds a requested level into the ladder.
//
// A client may ask for any level — it is holding a number the server gave it,
// and configuration may have changed underneath it since. Clamping rather than
// refusing means a stale app shows the widest search it is allowed instead of
// an error the customer cannot act on.
func (p Policy) ClampLevel(level int) int {
	if level < 0 {
		return 0
	}
	if level > p.maxExpansions {
		return p.maxExpansions
	}
	return level
}

// Enough reports whether a merchant count clears the expansion threshold.
func (p Policy) Enough(count int) bool { return count >= p.minMerchants }
