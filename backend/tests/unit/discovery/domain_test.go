package discovery

import (
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/discovery/domain"
)

// policy is the Appendix B default ladder, built the way the use case builds it.
func policy(t *testing.T) domain.Policy {
	t.Helper()
	p, err := domain.NewPolicy(5000, 5000, 4, 5, false, true)
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	return p
}

func TestNewPolicyRejectsImpossibleLadders(t *testing.T) {
	cases := []struct {
		name                        string
		base, step                  float64
		maxExpansions, minMerchants int
		autoExpand, ceiling         bool
		want                        error
	}{
		{"zero radius", 0, 5000, 4, 5, false, true, domain.ErrRadiusNotPositive},
		{"negative radius", -1, 5000, 4, 5, false, true, domain.ErrRadiusNotPositive},
		{"zero step", 5000, 0, 4, 5, false, true, domain.ErrStepNotPositive},
		{"negative expansions", 5000, 5000, -1, 5, false, true, domain.ErrExpansionsNegative},
		{"negative threshold", 5000, 5000, 4, -1, false, true, domain.ErrThresholdNegative},
		{"ceiling disabled", 5000, 5000, 4, 5, false, false, domain.ErrCeilingDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewPolicy(tc.base, tc.step, tc.maxExpansions, tc.minMerchants, tc.autoExpand, tc.ceiling)
			if !errors.Is(err, tc.want) {
				t.Fatalf("NewPolicy = %v, want %v", err, tc.want)
			}
		})
	}
}

// The division ceiling is the one setting the system will not honour being
// turned off (Appendix B). Asserting it here rather than trusting the caller is
// the whole reason NewPolicy takes it as an argument.
func TestDivisionCeilingCannotBeDisabled(t *testing.T) {
	if _, err := domain.NewPolicy(5000, 5000, 4, 5, false, false); !errors.Is(err, domain.ErrCeilingDisabled) {
		t.Fatalf("a policy with the ceiling off was accepted: %v", err)
	}
}

func TestPolicyAccessors(t *testing.T) {
	p, err := domain.NewPolicy(2000, 3000, 2, 7, true, true)
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	if p.BaseRadiusM() != 2000 || p.StepM() != 3000 || p.MaxLevel() != 2 ||
		p.MinMerchants() != 7 || !p.AutoExpand() {
		t.Fatalf("accessors disagree with what was configured: %+v", p)
	}
}

func TestRadiusWalksTheLadder(t *testing.T) {
	p := policy(t)
	for level, want := range []float64{5000, 10000, 15000, 20000, 25000} {
		if got := p.Radius(level); got != want {
			t.Errorf("Radius(%d) = %v, want %v", level, got, want)
		}
	}
	// Total, and clamped at both ends: every caller already holds a level this
	// policy produced, and a search that failed on arithmetic over its own
	// ladder would be worse than the widest allowed radius.
	if got := p.Radius(-1); got != 5000 {
		t.Errorf("Radius(-1) = %v, want the base radius", got)
	}
	if got := p.Radius(99); got != 25000 {
		t.Errorf("Radius(99) = %v, want the ceiling radius", got)
	}
}

func TestClampLevelFoldsAStaleClientIn(t *testing.T) {
	p := policy(t)
	for in, want := range map[int]int{-5: 0, 0: 0, 3: 3, 4: 4, 99: 4} {
		if got := p.ClampLevel(in); got != want {
			t.Errorf("ClampLevel(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestEnoughIsInclusive(t *testing.T) {
	p := policy(t)
	if p.Enough(4) {
		t.Error("four merchants cleared a threshold of five")
	}
	if !p.Enough(5) {
		t.Error("five merchants did not clear a threshold of five")
	}
}

func TestDescribeExpansionAtBase(t *testing.T) {
	p := policy(t)
	e := domain.DescribeExpansion(p, 0, 5000, 12)

	if e.Stage != domain.StageBase {
		t.Errorf("stage = %q, want base", e.Stage)
	}
	if !e.CanExpand || e.NextLevel != 1 || e.NextRadiusM != 10000 {
		t.Errorf("expansion offer is wrong: %+v", e)
	}
	// Twelve shops is plenty. Offering a wider, dearer search here would be
	// suggesting the customer pay more for no reason.
	if e.Offered {
		t.Error("expansion was offered although there were enough shops nearby")
	}
	if e.AtCeiling {
		t.Error("the base radius reported itself as the ceiling")
	}
}

func TestDescribeExpansionOffersWhenThin(t *testing.T) {
	e := domain.DescribeExpansion(policy(t), 1, 10000, 2)
	if e.Stage != domain.StageExpanded {
		t.Errorf("stage = %q, want expanded", e.Stage)
	}
	if !e.Offered {
		t.Error("two shops did not trigger an expansion offer")
	}
	if e.NextRadiusM != 15000 {
		t.Errorf("next radius = %v, want 15000", e.NextRadiusM)
	}
}

// D3: the last rung is terminal, and says so. A client that got CanExpand=true
// here would render a button that could only ever fail.
func TestDescribeExpansionAtTheDivisionCeiling(t *testing.T) {
	e := domain.DescribeExpansion(policy(t), 4, 25000, 0)
	if e.Stage != domain.StageCeiling || !e.AtCeiling {
		t.Errorf("the last rung did not report itself terminal: %+v", e)
	}
	if e.CanExpand || e.Offered || e.NextLevel != 0 || e.NextRadiusM != 0 {
		t.Errorf("the ceiling offered a further expansion: %+v", e)
	}
}

// A ladder with no expansions configured is at the ceiling from the first
// search. Worth its own case because it is the shape an admin gets by setting
// discovery.max_expansions to 0, and "base" and "ceiling" are both defensible
// readings — the terminal one is the honest one.
func TestZeroExpansionsIsImmediatelyTerminal(t *testing.T) {
	p, err := domain.NewPolicy(5000, 5000, 0, 5, false, true)
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	e := domain.DescribeExpansion(p, 0, 5000, 0)
	if e.Stage != domain.StageCeiling || e.CanExpand {
		t.Errorf("a ladder with no rungs offered one: %+v", e)
	}
}

func TestFormatDistance(t *testing.T) {
	cases := []struct {
		metres  float64
		bengali string
		english string
	}{
		{0, "খুব কাছেই", "Very close"},
		{-10, "খুব কাছেই", "Very close"},
		{20, "খুব কাছেই", "Very close"},
		{40, "৫০ মিটার", "50 m"},
		{347, "৩৫০ মিটার", "350 m"},
		{999, "১০০০ মিটার", "1000 m"},
		{1000, "১.০ কিমি", "1.0 km"},
		{1240, "১.২ কিমি", "1.2 km"},
		{23400, "২৩.৪ কিমি", "23.4 km"},
	}
	for _, tc := range cases {
		if got := domain.FormatDistance(tc.metres, ""); got != tc.bengali {
			t.Errorf("FormatDistance(%v, bn) = %q, want %q", tc.metres, got, tc.bengali)
		}
		if got := domain.FormatDistance(tc.metres, "en"); got != tc.english {
			t.Errorf("FormatDistance(%v, en) = %q, want %q", tc.metres, got, tc.english)
		}
	}
}

func TestFormatRadius(t *testing.T) {
	if got := domain.FormatRadius(5000, ""); got != "৫ কিমি" {
		t.Errorf("FormatRadius(5000, bn) = %q", got)
	}
	if got := domain.FormatRadius(5000, "en"); got != "5 km" {
		t.Errorf("FormatRadius(5000, en) = %q", got)
	}
	if got := domain.FormatRadius(7500, ""); got != "৭.৫ কিমি" {
		t.Errorf("FormatRadius(7500, bn) = %q", got)
	}
}

func TestRelevance(t *testing.T) {
	cases := []struct {
		name, query string
		wantScore   float64
		wantMatch   bool
	}{
		{"Star Kabab", "", 0, true},
		{"Star Kabab", "   ", 0, true},
		{"Star Kabab", "star kabab", 1, true},
		{"Star Kabab", "star", 0.9, true},
		{"Star Kabab", "kabab", 0.75, true},
		{"Star Kabab", "aba", 0.5, true},
		{"Star Kabab", "biryani", 0, false},
	}
	for _, tc := range cases {
		score, matched := domain.Relevance(tc.name, tc.query)
		if score != tc.wantScore || matched != tc.wantMatch {
			t.Errorf("Relevance(%q, %q) = %v, %v; want %v, %v",
				tc.name, tc.query, score, matched, tc.wantScore, tc.wantMatch)
		}
	}
}

func TestScoreWeighsTheFourSignals(t *testing.T) {
	near := domain.Candidate{ID: "a", Relevance: 1, DistanceM: 0, Rating: 5, Open: true}
	if got := domain.Score(near, 5000); got != 1 {
		t.Errorf("a perfect candidate scored %v, want 1", got)
	}
	worst := domain.Candidate{ID: "b", Relevance: 0, DistanceM: 5000, Rating: 0, Open: false}
	if got := domain.Score(worst, 5000); got != 0 {
		t.Errorf("the worst candidate scored %v, want 0", got)
	}
	// A zero radius cannot normalise a distance; scoring every candidate the
	// same on that term is better than dividing by zero.
	if got := domain.Score(near, 0); got != 1-0.35 {
		t.Errorf("with a zero radius the proximity term did not drop out: %v", got)
	}
	// Signals out of range are clamped rather than trusted: reviews are another
	// module's number, and a 6-star average must not outrank the arithmetic.
	wild := domain.Candidate{ID: "c", Relevance: 9, DistanceM: -100, Rating: 60, Open: true}
	if got := domain.Score(wild, 5000); got != 1 {
		t.Errorf("out-of-range signals were not clamped: %v", got)
	}
}

func TestRankPutsTheBestFirstAndStaysStable(t *testing.T) {
	candidates := []domain.Candidate{
		{ID: "far-open", Relevance: 1, DistanceM: 4500, Open: true},
		{ID: "near-shut", Relevance: 1, DistanceM: 200, Open: false},
		{ID: "near-open", Relevance: 1, DistanceM: 200, Open: true},
	}
	domain.Rank(candidates, 5000)
	if candidates[0].ID != "near-open" {
		t.Fatalf("ranking put %q first, want near-open", candidates[0].ID)
	}

	// Identical candidates must come back in the same order every time, or a
	// customer paging through results sees one shop twice and misses another.
	tied := []domain.Candidate{
		{ID: "b", Relevance: 1, DistanceM: 100, Open: true},
		{ID: "a", Relevance: 1, DistanceM: 100, Open: true},
	}
	domain.Rank(tied, 5000)
	if tied[0].ID != "a" || tied[1].ID != "b" {
		t.Fatalf("tie-break is not deterministic: %v, %v", tied[0].ID, tied[1].ID)
	}
}
