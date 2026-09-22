package domain

import (
	"errors"
	"math"
)

// ErrDegeneratePolygon is returned when a ring has fewer than three distinct
// vertices and therefore encloses no area.
var ErrDegeneratePolygon = errors.New("polygon needs at least three vertices")

// Polygon is a closed ring of coordinates.
//
// Division and district boundaries are stored as PostGIS geometry and queried
// with ST_Contains in production (ALG-03). This pure implementation exists so
// the boundary rule — the one hard invariant of the system, D3 — is testable
// without a database, and so the integration tests can assert that PostGIS and
// this code agree.
type Polygon struct {
	vertices []Coordinate
	min, max Coordinate
}

// NewPolygon builds a polygon from a ring. A repeated closing vertex is
// accepted and ignored; the ring is treated as implicitly closed.
func NewPolygon(vertices []Coordinate) (Polygon, error) {
	ring := make([]Coordinate, len(vertices))
	copy(ring, vertices)

	// Drop an explicit closing vertex so the edge walk does not count it twice.
	if n := len(ring); n > 1 && ring[0] == ring[n-1] {
		ring = ring[:n-1]
	}
	if len(ring) < 3 {
		return Polygon{}, ErrDegeneratePolygon
	}

	minLat, minLng := math.Inf(1), math.Inf(1)
	maxLat, maxLng := math.Inf(-1), math.Inf(-1)
	for _, v := range ring {
		minLat = math.Min(minLat, v.lat)
		minLng = math.Min(minLng, v.lng)
		maxLat = math.Max(maxLat, v.lat)
		maxLng = math.Max(maxLng, v.lng)
	}
	return Polygon{
		vertices: ring,
		min:      Coordinate{lat: minLat, lng: minLng},
		max:      Coordinate{lat: maxLat, lng: maxLng},
	}, nil
}

// Vertices returns a copy of the ring, so a caller cannot reshape the polygon.
func (p Polygon) Vertices() []Coordinate {
	out := make([]Coordinate, len(p.vertices))
	copy(out, p.vertices)
	return out
}

// BoundingBox returns the south-west and north-east corners.
func (p Polygon) BoundingBox() (southWest, northEast Coordinate) { return p.min, p.max }

// Contains reports whether a point lies inside the polygon.
//
// Ray casting: count how many edges a ray cast east from the point crosses. An
// odd count means inside. O(n) in the number of vertices, which is why the
// bounding-box rejection runs first — most candidate points are nowhere near a
// given division, and that test is O(1).
//
// A point exactly on an edge is treated as inside, so a merchant sitting on a
// division border is served rather than silently excluded.
func (p Polygon) Contains(c Coordinate) bool {
	if c.lat < p.min.lat || c.lat > p.max.lat || c.lng < p.min.lng || c.lng > p.max.lng {
		return false
	}

	inside := false
	n := len(p.vertices)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		vi, vj := p.vertices[i], p.vertices[j]
		if onSegment(vi, vj, c) {
			return true
		}
		// Does the edge straddle the ray's latitude?
		if (vi.lat > c.lat) == (vj.lat > c.lat) {
			continue
		}
		// Longitude where the edge crosses that latitude.
		crossLng := vi.lng + (c.lat-vi.lat)/(vj.lat-vi.lat)*(vj.lng-vi.lng)
		if c.lng < crossLng {
			inside = !inside
		}
	}
	return inside
}

// epsilon absorbs float noise. At Bangladesh's latitude 1e-9 degrees is well
// under a millimetre, so it cannot swallow a real gap between two places.
const epsilon = 1e-9

// onSegment reports whether c lies on the segment a-b.
func onSegment(a, b, c Coordinate) bool {
	cross := (c.lat-a.lat)*(b.lng-a.lng) - (c.lng-a.lng)*(b.lat-a.lat)
	if math.Abs(cross) > epsilon {
		return false
	}
	return c.lat >= math.Min(a.lat, b.lat)-epsilon && c.lat <= math.Max(a.lat, b.lat)+epsilon &&
		c.lng >= math.Min(a.lng, b.lng)-epsilon && c.lng <= math.Max(a.lng, b.lng)+epsilon
}
