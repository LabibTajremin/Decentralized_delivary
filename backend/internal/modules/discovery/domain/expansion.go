package domain

// Stage is where a search sits on the ladder, as a primitive the client can
// branch its layout on without re-deriving it (2.9).
type Stage string

// The three stages a search can be in.
const (
	// StageBase is the first, unexpanded search.
	StageBase Stage = "base"
	// StageExpanded is any search the customer has widened.
	StageExpanded Stage = "expanded"
	// StageCeiling is the widest search the ladder allows. D3 makes this a
	// terminal state rather than a step that happens to be last: the division
	// boundary is the end of what the customer can ever see, and saying so is
	// better than a button that does nothing.
	StageCeiling Stage = "ceiling"
)

// Expansion is the whole expansion decision for one search.
//
// The server decides all of it. A client that worked out whether to show the
// "search wider" button would be deciding visibility, which 2.9 forbids
// outright, and would need a copy of three configuration values to do it.
type Expansion struct {
	// Level is the expansion level this result was searched at, 0 being base.
	Level int
	// RadiusM is the radius actually searched.
	RadiusM float64
	// Stage is base, expanded or ceiling.
	Stage Stage
	// CanExpand is whether a wider search exists.
	CanExpand bool
	// NextLevel and NextRadiusM describe that wider search. Both are zero when
	// CanExpand is false.
	NextLevel   int
	NextRadiusM float64
	// Offered is whether the customer should be *prompted* to expand, as
	// opposed to merely being allowed to. D2 offers expansion when there is
	// nothing nearby; a customer looking at forty restaurants does not need a
	// button suggesting they look further afield and pay more for it.
	Offered bool
	// AtCeiling is whether this is the widest search the division allows.
	AtCeiling bool
	// Found is how many merchants this search matched.
	Found int
}

// DescribeExpansion computes the expansion state for a completed search.
//
// level must already be clamped by ClampLevel; radiusM must be the radius that
// level resolves to. Both are passed in rather than recomputed so this function
// describes what actually happened rather than what should have.
func DescribeExpansion(p Policy, level int, radiusM float64, found int) Expansion {
	e := Expansion{
		Level:     level,
		RadiusM:   radiusM,
		Found:     found,
		AtCeiling: level >= p.MaxLevel(),
	}

	switch {
	case e.AtCeiling:
		e.Stage = StageCeiling
	case level == 0:
		e.Stage = StageBase
	default:
		e.Stage = StageExpanded
	}

	if !e.AtCeiling {
		e.CanExpand = true
		e.NextLevel = level + 1
		e.NextRadiusM = p.Radius(e.NextLevel)
		e.Offered = !p.Enough(found)
	}

	return e
}
