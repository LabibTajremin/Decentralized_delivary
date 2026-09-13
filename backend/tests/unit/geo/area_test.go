package geo

import (
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
)

// box builds a rectangular boundary between two corners.
func box(t *testing.T, minLat, minLng, maxLat, maxLng float64) domain.Polygon {
	t.Helper()
	p, err := domain.NewPolygon([]domain.Coordinate{
		domain.MustCoordinate(minLat, minLng),
		domain.MustCoordinate(minLat, maxLng),
		domain.MustCoordinate(maxLat, maxLng),
		domain.MustCoordinate(maxLat, minLng),
	})
	if err != nil {
		t.Fatalf("build box: %v", err)
	}
	return p
}

func TestDivisionCodeValidity(t *testing.T) {
	if got := len(domain.AllDivisionCodes()); got != 8 {
		t.Errorf("AllDivisionCodes() = %d codes, want 8 divisions", got)
	}
	for _, code := range domain.AllDivisionCodes() {
		if !code.IsValid() {
			t.Errorf("%s should be valid", code)
		}
		if code.String() != string(code) {
			t.Errorf("String() = %q, want %q", code.String(), string(code))
		}
	}
	for _, bad := range []domain.DivisionCode{"", "XXX", "dha"} {
		if bad.IsValid() {
			t.Errorf("%q should not be valid", bad)
		}
	}
}

func TestNewDivision(t *testing.T) {
	boundary := box(t, 23, 90, 25, 91)

	d, err := domain.NewDivision(domain.DivisionDhaka, "Dhaka", boundary)
	if err != nil {
		t.Fatalf("NewDivision error: %v", err)
	}
	if d.Code != domain.DivisionDhaka || d.Name != "Dhaka" {
		t.Errorf("division = %+v, want Dhaka", d)
	}

	if _, err := domain.NewDivision("NOPE", "X", boundary); !errors.Is(err, domain.ErrEmptyCode) {
		t.Errorf("invalid code error = %v, want ErrEmptyCode", err)
	}
	if _, err := domain.NewDivision(domain.DivisionDhaka, "", boundary); !errors.Is(err, domain.ErrEmptyName) {
		t.Errorf("empty name error = %v, want ErrEmptyName", err)
	}
	if _, err := domain.NewDivision(domain.DivisionDhaka, "Dhaka", domain.Polygon{}); !errors.Is(err, domain.ErrDegeneratePolygon) {
		t.Errorf("empty boundary error = %v, want ErrDegeneratePolygon", err)
	}
}

func TestDivisionContains(t *testing.T) {
	d, err := domain.NewDivision(domain.DivisionDhaka, "Dhaka", box(t, 23, 90, 25, 91))
	if err != nil {
		t.Fatalf("NewDivision: %v", err)
	}
	if !d.Contains(domain.MustCoordinate(23.81, 90.41)) {
		t.Error("Dhaka city should be inside the Dhaka division box")
	}
	if d.Contains(domain.MustCoordinate(22.35, 91.78)) {
		t.Error("Chattogram should not be inside the Dhaka division box")
	}
}

func TestNewDistrict(t *testing.T) {
	boundary := box(t, 23.6, 90.3, 24.0, 90.6)

	d, err := domain.NewDistrict("DHK", "Dhaka", domain.DivisionDhaka, boundary)
	if err != nil {
		t.Fatalf("NewDistrict error: %v", err)
	}
	if d.Division != domain.DivisionDhaka {
		t.Errorf("Division = %v, want DHA", d.Division)
	}
	if !d.Contains(domain.MustCoordinate(23.81, 90.41)) {
		t.Error("district should contain its own city")
	}

	cases := map[string]struct {
		code, name string
		division   domain.DivisionCode
		boundary   domain.Polygon
		want       error
	}{
		"no code":        {"", "Dhaka", domain.DivisionDhaka, boundary, domain.ErrEmptyCode},
		"no name":        {"DHK", "", domain.DivisionDhaka, boundary, domain.ErrEmptyName},
		"bad division":   {"DHK", "Dhaka", "NOPE", boundary, domain.ErrEmptyCode},
		"empty boundary": {"DHK", "Dhaka", domain.DivisionDhaka, domain.Polygon{}, domain.ErrDegeneratePolygon},
	}
	for name, c := range cases {
		if _, err := domain.NewDistrict(c.code, c.name, c.division, c.boundary); !errors.Is(err, c.want) {
			t.Errorf("%s: error = %v, want %v", name, err, c.want)
		}
	}
}

func TestNewArea(t *testing.T) {
	centre := domain.MustCoordinate(23.7461, 90.3742)

	a, err := domain.NewArea("DHN", "Dhanmondi", "DHK", domain.DivisionDhaka, centre)
	if err != nil {
		t.Fatalf("NewArea error: %v", err)
	}
	if a.Name != "Dhanmondi" || a.District != "DHK" || a.Division != domain.DivisionDhaka {
		t.Errorf("area = %+v", a)
	}

	cases := map[string]struct {
		code, name, district string
		division             domain.DivisionCode
		want                 error
	}{
		"no code":      {"", "Dhanmondi", "DHK", domain.DivisionDhaka, domain.ErrEmptyCode},
		"no name":      {"DHN", "", "DHK", domain.DivisionDhaka, domain.ErrEmptyName},
		"no district":  {"DHN", "Dhanmondi", "", domain.DivisionDhaka, domain.ErrEmptyCode},
		"bad division": {"DHN", "Dhanmondi", "DHK", "NOPE", domain.ErrEmptyCode},
	}
	for name, c := range cases {
		if _, err := domain.NewArea(c.code, c.name, c.district, c.division, centre); !errors.Is(err, c.want) {
			t.Errorf("%s: error = %v, want %v", name, err, c.want)
		}
	}
}

// twoDivisions returns adjacent, non-overlapping Dhaka and Chattogram boxes.
func twoDivisions(t *testing.T) []domain.Division {
	t.Helper()
	dhaka, err := domain.NewDivision(domain.DivisionDhaka, "Dhaka", box(t, 23, 89.5, 25, 91))
	if err != nil {
		t.Fatalf("dhaka: %v", err)
	}
	ctg, err := domain.NewDivision(domain.DivisionChattogram, "Chattogram", box(t, 21, 91.001, 23, 92.5))
	if err != nil {
		t.Fatalf("ctg: %v", err)
	}
	return []domain.Division{dhaka, ctg}
}

func TestDivisionOf(t *testing.T) {
	divisions := twoDivisions(t)

	got, err := domain.DivisionOf(divisions, domain.MustCoordinate(23.81, 90.41))
	if err != nil {
		t.Fatalf("DivisionOf error: %v", err)
	}
	if got.Code != domain.DivisionDhaka {
		t.Errorf("Dhaka city resolved to %v, want DHA", got.Code)
	}

	got, err = domain.DivisionOf(divisions, domain.MustCoordinate(22.35, 91.78))
	if err != nil {
		t.Fatalf("DivisionOf error: %v", err)
	}
	if got.Code != domain.DivisionChattogram {
		t.Errorf("Chattogram resolved to %v, want CTG", got.Code)
	}
}

func TestDivisionOfRejectsPointOutsideEveryDivision(t *testing.T) {
	// Far out in the Bay of Bengal.
	_, err := domain.DivisionOf(twoDivisions(t), domain.MustCoordinate(15, 88))
	if !errors.Is(err, domain.ErrUnknownDivision) {
		t.Errorf("error = %v, want ErrUnknownDivision", err)
	}
}

// TestSameDivisionIsTheD3Invariant guards the one rule that can never be
// disabled: a search may not cross a division boundary.
func TestSameDivisionIsTheD3Invariant(t *testing.T) {
	divisions := twoDivisions(t)
	dhakaCity := domain.MustCoordinate(23.81, 90.41)
	dhakaEdge := domain.MustCoordinate(23.10, 90.90)
	chattogram := domain.MustCoordinate(22.35, 91.78)

	same, err := domain.SameDivision(divisions, dhakaCity, dhakaEdge)
	if err != nil {
		t.Fatalf("SameDivision error: %v", err)
	}
	if !same {
		t.Error("two points inside Dhaka must be in the same division")
	}

	same, err = domain.SameDivision(divisions, dhakaCity, chattogram)
	if err != nil {
		t.Fatalf("SameDivision error: %v", err)
	}
	if same {
		t.Error("Dhaka and Chattogram must never be in the same division")
	}
}

func TestSameDivisionReportsUnknownPoints(t *testing.T) {
	divisions := twoDivisions(t)
	sea := domain.MustCoordinate(15, 88)
	dhaka := domain.MustCoordinate(23.81, 90.41)

	if _, err := domain.SameDivision(divisions, sea, dhaka); !errors.Is(err, domain.ErrUnknownDivision) {
		t.Errorf("unknown first point: error = %v, want ErrUnknownDivision", err)
	}
	if _, err := domain.SameDivision(divisions, dhaka, sea); !errors.Is(err, domain.ErrUnknownDivision) {
		t.Errorf("unknown second point: error = %v, want ErrUnknownDivision", err)
	}
}
