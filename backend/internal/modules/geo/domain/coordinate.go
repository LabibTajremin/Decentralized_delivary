// Package domain holds the geo module's entities and geometry rules.
//
// It imports nothing else in the backend: radius search, division boundaries
// and distance are the core of this product (D1-D3), so the rules live here in
// pure Go and are exercised without a database. The PostGIS implementation in
// infrastructure/ must agree with these results, which the integration tests
// assert directly.
package domain

import (
	"errors"
	"fmt"
	"math"
)

// Errors returned by coordinate construction.
var (
	// ErrLatitudeOutOfRange is returned for a latitude outside [-90, 90].
	ErrLatitudeOutOfRange = errors.New("latitude out of range")
	// ErrLongitudeOutOfRange is returned for a longitude outside [-180, 180].
	ErrLongitudeOutOfRange = errors.New("longitude out of range")
	// ErrNotFinite is returned for NaN or infinite input, which SQL would
	// otherwise accept and turn into an unqueryable row.
	ErrNotFinite = errors.New("coordinate must be finite")
)

// Coordinate is a validated WGS-84 point.
type Coordinate struct {
	lat float64
	lng float64
}

// NewCoordinate validates and builds a point. Validation happens here rather
// than at the database edge so an impossible location cannot reach a query.
func NewCoordinate(lat, lng float64) (Coordinate, error) {
	if math.IsNaN(lat) || math.IsNaN(lng) || math.IsInf(lat, 0) || math.IsInf(lng, 0) {
		return Coordinate{}, ErrNotFinite
	}
	if lat < -90 || lat > 90 {
		return Coordinate{}, fmt.Errorf("%w: %g", ErrLatitudeOutOfRange, lat)
	}
	if lng < -180 || lng > 180 {
		return Coordinate{}, fmt.Errorf("%w: %g", ErrLongitudeOutOfRange, lng)
	}
	return Coordinate{lat: lat, lng: lng}, nil
}

// MustCoordinate builds a point and panics on invalid input. It is for fixed
// constants such as seed data, never for user input.
func MustCoordinate(lat, lng float64) Coordinate {
	c, err := NewCoordinate(lat, lng)
	if err != nil {
		panic(fmt.Sprintf("geo: invalid constant coordinate (%g, %g): %v", lat, lng, err))
	}
	return c
}

// Lat returns the latitude in degrees.
func (c Coordinate) Lat() float64 { return c.lat }

// Lng returns the longitude in degrees.
func (c Coordinate) Lng() float64 { return c.lng }

// IsZero reports whether the coordinate is the zero value. Null Island is a
// real point, so this is only meaningful for detecting an unset field.
func (c Coordinate) IsZero() bool { return c.lat == 0 && c.lng == 0 }

// String renders the coordinate at roughly one-metre precision.
func (c Coordinate) String() string { return fmt.Sprintf("%.5f,%.5f", c.lat, c.lng) }

// earthRadiusMetres is the mean radius used for haversine.
const earthRadiusMetres = 6371008.8

// DistanceTo returns the great-circle distance to another point.
//
// Haversine is used rather than a projected-plane approximation because radius
// expansion (D2) can reach tens of kilometres, where a flat-earth estimate is
// off by enough to place a merchant on the wrong side of a fee band. It is
// numerically stable for the small distances that dominate here, which the law
// of cosines is not.
func (c Coordinate) DistanceTo(other Coordinate) Distance {
	lat1 := c.lat * math.Pi / 180
	lat2 := other.lat * math.Pi / 180
	dLat := (other.lat - c.lat) * math.Pi / 180
	dLng := (other.lng - c.lng) * math.Pi / 180

	sinLat := math.Sin(dLat / 2)
	sinLng := math.Sin(dLng / 2)
	a := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLng*sinLng
	return Distance(2 * earthRadiusMetres * math.Asin(math.Sqrt(math.Min(1, a))))
}

// Distance is a length in metres. It is a distinct type so a raw float cannot
// be passed where a distance is expected, and so kilometre conversion happens
// in exactly one place.
type Distance float64

// Metres returns the distance in metres.
func (d Distance) Metres() float64 { return float64(d) }

// Kilometres returns the distance in kilometres.
func (d Distance) Kilometres() float64 { return float64(d) / 1000 }

// KilometresFrom builds a Distance from kilometres.
func KilometresFrom(km float64) Distance { return Distance(km * 1000) }

// String renders the distance the way the API reports it.
func (d Distance) String() string { return fmt.Sprintf("%.2f km", d.Kilometres()) }
