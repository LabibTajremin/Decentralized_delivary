package domain

import "errors"

// Errors returned when building the administrative hierarchy.
var (
	// ErrEmptyName is returned when a division, district or area has no name.
	ErrEmptyName = errors.New("name is required")
	// ErrEmptyCode is returned when a code is missing.
	ErrEmptyCode = errors.New("code is required")
	// ErrUnknownDivision is returned when a point falls outside every division.
	ErrUnknownDivision = errors.New("point is outside every division")
)

// DivisionCode identifies one of Bangladesh's eight divisions.
//
// Division is the hard geographic boundary of the whole system: radius
// expansion can never cross it (D3), and that rule cannot be disabled by an
// admin or the auto-tuner.
type DivisionCode string

// The eight divisions of Bangladesh.
const (
	DivisionDhaka      DivisionCode = "DHA"
	DivisionChattogram DivisionCode = "CTG"
	DivisionKhulna     DivisionCode = "KHU"
	DivisionRajshahi   DivisionCode = "RAJ"
	DivisionBarishal   DivisionCode = "BAR"
	DivisionSylhet     DivisionCode = "SYL"
	DivisionRangpur    DivisionCode = "RAN"
	DivisionMymensingh DivisionCode = "MYM"
)

// AllDivisionCodes lists every division code in a fixed order.
func AllDivisionCodes() []DivisionCode {
	return []DivisionCode{
		DivisionDhaka, DivisionChattogram, DivisionKhulna, DivisionRajshahi,
		DivisionBarishal, DivisionSylhet, DivisionRangpur, DivisionMymensingh,
	}
}

// IsValid reports whether the code is one of the eight divisions.
func (d DivisionCode) IsValid() bool {
	for _, known := range AllDivisionCodes() {
		if d == known {
			return true
		}
	}
	return false
}

// String returns the code as text.
func (d DivisionCode) String() string { return string(d) }

// Division is a top-level administrative region with its boundary.
type Division struct {
	Code     DivisionCode
	Name     string
	Boundary Polygon
}

// NewDivision validates and builds a division.
func NewDivision(code DivisionCode, name string, boundary Polygon) (Division, error) {
	if !code.IsValid() {
		return Division{}, ErrEmptyCode
	}
	if name == "" {
		return Division{}, ErrEmptyName
	}
	if len(boundary.vertices) == 0 {
		return Division{}, ErrDegeneratePolygon
	}
	return Division{Code: code, Name: name, Boundary: boundary}, nil
}

// Contains reports whether a point lies inside the division.
func (d Division) Contains(c Coordinate) bool { return d.Boundary.Contains(c) }

// District is a second-level region belonging to exactly one division.
type District struct {
	Code     string
	Name     string
	Division DivisionCode
	Boundary Polygon
}

// NewDistrict validates and builds a district.
func NewDistrict(code, name string, division DivisionCode, boundary Polygon) (District, error) {
	if code == "" {
		return District{}, ErrEmptyCode
	}
	if name == "" {
		return District{}, ErrEmptyName
	}
	if !division.IsValid() {
		return District{}, ErrEmptyCode
	}
	if len(boundary.vertices) == 0 {
		return District{}, ErrDegeneratePolygon
	}
	return District{Code: code, Name: name, Division: division, Boundary: boundary}, nil
}

// Contains reports whether a point lies inside the district.
func (d District) Contains(c Coordinate) bool { return d.Boundary.Contains(c) }

// Area is the finest unit an admin can tune config against — a neighbourhood
// or upazila. Appendix B resolves values area -> district -> division -> global,
// so this is where "Dhaka needs a small radius, a rural upazila needs a large
// one" is actually expressed.
type Area struct {
	Code     string
	Name     string
	District string
	Division DivisionCode
	Centre   Coordinate
}

// NewArea validates and builds an area.
func NewArea(code, name, district string, division DivisionCode, centre Coordinate) (Area, error) {
	if code == "" {
		return Area{}, ErrEmptyCode
	}
	if name == "" {
		return Area{}, ErrEmptyName
	}
	if district == "" {
		return Area{}, ErrEmptyCode
	}
	if !division.IsValid() {
		return Area{}, ErrEmptyCode
	}
	return Area{Code: code, Name: name, District: district, Division: division, Centre: centre}, nil
}

// DivisionOf returns the division containing a point, implementing ALG-03.
//
// Divisions do not overlap, so the first containing division is the answer and
// the scan stops there. With eight divisions a linear scan is faster than any
// index; PostGIS handles the same question with ST_Contains against a GiST
// index where the candidate set is large.
func DivisionOf(divisions []Division, c Coordinate) (Division, error) {
	for _, d := range divisions {
		if d.Contains(c) {
			return d, nil
		}
	}
	return Division{}, ErrUnknownDivision
}

// SameDivision reports whether two points fall in the same division. This is
// the D3 check: a search may expand only while this stays true.
func SameDivision(divisions []Division, a, b Coordinate) (bool, error) {
	da, err := DivisionOf(divisions, a)
	if err != nil {
		return false, err
	}
	db, err := DivisionOf(divisions, b)
	if err != nil {
		return false, err
	}
	return da.Code == db.Code, nil
}
