package domain

import "fmt"

// Pin is where the shop actually is.
type Pin struct {
	Lat float64
	Lng float64
}

// NewPin validates a map location.
func NewPin(lat, lng float64) (Pin, error) {
	if lat < -90 || lat > 90 {
		return Pin{}, fmt.Errorf("%w: latitude %v", ErrInvalidPin, lat)
	}
	if lng < -180 || lng > 180 {
		return Pin{}, fmt.Errorf("%w: longitude %v", ErrInvalidPin, lng)
	}
	// A pin at exactly (0,0) is in the Gulf of Guinea and is almost always an
	// uninitialised variable rather than a shop.
	if lat == 0 && lng == 0 {
		return Pin{}, fmt.Errorf("%w: the pin was not set", ErrInvalidPin)
	}
	return Pin{Lat: lat, Lng: lng}, nil
}

// Placement is where a shop sits administratively.
//
// Resolved from the pin by the geo module and stored rather than recomputed:
// the division decides who can ever see this shop (D3), and putting a spatial
// query on the read path of every listing would make discovery pay for it on
// every request.
type Placement struct {
	AreaCode     string
	AreaName     string
	DistrictCode string
	DivisionCode string
}

// IsPlaced reports whether the shop has been located.
func (p Placement) IsPlaced() bool { return p.DivisionCode != "" }
