package domain

// TuningSignal is what ALG-09 reacts to for one area.
type TuningSignal struct {
	// MerchantsNearby is how many merchants discovery currently finds within
	// the area's own effective search radius.
	MerchantsNearby int
	// OrderFailureRate is the fraction, 0 to 1, of this area's recent
	// deliveries that ended in "failed".
	OrderFailureRate float64
}

// maxAcceptableFailureRate is ALG-09's own judgement of "failing often enough
// that a tighter radius might be why" — a property of the algorithm, not a
// business rule an operator tunes area by area, so it is not in Appendix B.
const maxAcceptableFailureRate = 0.15

// oversupplyFactor is how far above the discovery threshold merchant density
// has to sit before ALG-09 will narrow a radius — comfortably above, not
// merely at, so a reading that just crossed the line does not immediately
// reverse the next widen.
const oversupplyFactor = 2

// tuningStep is how much one pass moves a radius, as a fraction of its
// current value. Gradual by design: a single noisy reading must not swing a
// live radius from one extreme to the other.
const tuningStep = 0.10

// Tune decides ALG-09's next value for one radius-shaped, auto-tunable
// variable, clamped to def's own bounds.
//
// Widens when supply is thin (fewer merchants nearby than the area's own
// discovery.min_merchants) or deliveries are failing more than the algorithm
// accepts — either reads as "the served area is too small." Narrows only
// when density is comfortably above that threshold and delivery success is
// fine, so a stable area does not keep shrinking pass after pass toward its
// floor. Everything in between is left alone: changed reports false, and
// next echoes current.
func Tune(def Definition, current Value, minMerchants int, signal TuningSignal) (next Value, reason string, changed bool) {
	metres, _ := current.Int()
	minMetres, _ := def.Min.Int()
	maxMetres, _ := def.Max.Int()

	thin := minMerchants > 0 && signal.MerchantsNearby < minMerchants
	failing := signal.OrderFailureRate > maxAcceptableFailureRate
	oversupplied := minMerchants > 0 && signal.MerchantsNearby >= minMerchants*oversupplyFactor && !failing

	var target int64
	switch {
	case thin && failing:
		target = clamp(metres+step(metres), minMetres, maxMetres)
		reason = "widened: fewer merchants nearby than this area's own threshold, and deliveries are failing more than accepted"
	case thin:
		target = clamp(metres+step(metres), minMetres, maxMetres)
		reason = "widened: fewer merchants nearby than this area's own threshold"
	case failing:
		target = clamp(metres+step(metres), minMetres, maxMetres)
		reason = "widened: delivery failures above the accepted rate"
	case oversupplied:
		target = clamp(metres-step(metres), minMetres, maxMetres)
		reason = "narrowed: merchant density comfortably above this area's threshold and delivery success is within range"
	default:
		return current, "", false
	}

	if target == metres {
		return current, "", false
	}
	next, _ = Distance(target)
	return next, reason, true
}

func step(metres int64) int64 {
	moved := int64(float64(metres) * tuningStep)
	if moved == 0 {
		// A step that would round to nothing still has to move the value by
		// something, or a radius small enough would never budge at all.
		moved = 1
	}
	return moved
}

func clamp(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
