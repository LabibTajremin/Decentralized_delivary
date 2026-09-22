package geo

import (
	"errors"
	"math"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
)

func TestNewCoordinateAcceptsValidPoints(t *testing.T) {
	cases := map[string]struct{ lat, lng float64 }{
		"Dhaka":       {23.8103, 90.4125},
		"north pole":  {90, 0},
		"south pole":  {-90, 0},
		"date line +": {0, 180},
		"date line -": {0, -180},
		"null island": {0, 0},
	}
	for name, c := range cases {
		got, err := domain.NewCoordinate(c.lat, c.lng)
		if err != nil {
			t.Errorf("%s: NewCoordinate(%g, %g) error: %v", name, c.lat, c.lng, err)
			continue
		}
		if got.Lat() != c.lat || got.Lng() != c.lng {
			t.Errorf("%s: got (%g, %g), want (%g, %g)", name, got.Lat(), got.Lng(), c.lat, c.lng)
		}
	}
}

func TestNewCoordinateRejectsOutOfRange(t *testing.T) {
	cases := []struct {
		name     string
		lat, lng float64
		want     error
	}{
		{"lat too high", 90.1, 0, domain.ErrLatitudeOutOfRange},
		{"lat too low", -90.1, 0, domain.ErrLatitudeOutOfRange},
		{"lng too high", 0, 180.1, domain.ErrLongitudeOutOfRange},
		{"lng too low", 0, -180.1, domain.ErrLongitudeOutOfRange},
		{"lat NaN", math.NaN(), 0, domain.ErrNotFinite},
		{"lng NaN", 0, math.NaN(), domain.ErrNotFinite},
		{"lat +Inf", math.Inf(1), 0, domain.ErrNotFinite},
		{"lng -Inf", 0, math.Inf(-1), domain.ErrNotFinite},
	}
	for _, c := range cases {
		if _, err := domain.NewCoordinate(c.lat, c.lng); !errors.Is(err, c.want) {
			t.Errorf("%s: error = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestMustCoordinate(t *testing.T) {
	if got := domain.MustCoordinate(23.8103, 90.4125); got.Lat() != 23.8103 {
		t.Errorf("MustCoordinate lat = %g, want 23.8103", got.Lat())
	}
	defer func() {
		if recover() == nil {
			t.Error("MustCoordinate must panic on invalid input")
		}
	}()
	domain.MustCoordinate(91, 0)
}

func TestIsZeroAndString(t *testing.T) {
	if !domain.MustCoordinate(0, 0).IsZero() {
		t.Error("(0,0) should report IsZero")
	}
	if domain.MustCoordinate(23.8, 90.4).IsZero() {
		t.Error("a real point should not report IsZero")
	}
	if got, want := domain.MustCoordinate(23.81034, 90.41252).String(), "23.81034,90.41252"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestDistanceToKnownPlaces(t *testing.T) {
	dhaka := domain.MustCoordinate(23.8103, 90.4125)
	chattogram := domain.MustCoordinate(22.3569, 91.7832)
	sylhet := domain.MustCoordinate(24.8949, 91.8687)

	cases := []struct {
		name   string
		a, b   domain.Coordinate
		wantKm float64
		tolKm  float64
	}{
		// Great-circle, not road distance: Dhaka-Sylhet is ~240 km by road but
		// ~191 km in a straight line, which is what a radius test must use.
		{"Dhaka to Chattogram", dhaka, chattogram, 214, 4},
		{"Dhaka to Sylhet", dhaka, sylhet, 191, 4},
		{"same point", dhaka, dhaka, 0, 0.0001},
	}
	for _, c := range cases {
		got := c.a.DistanceTo(c.b).Kilometres()
		if math.Abs(got-c.wantKm) > c.tolKm {
			t.Errorf("%s = %.1f km, want %.0f km (±%.0f)", c.name, got, c.wantKm, c.tolKm)
		}
	}
}

func TestDistanceIsSymmetric(t *testing.T) {
	a := domain.MustCoordinate(23.8103, 90.4125)
	b := domain.MustCoordinate(22.3569, 91.7832)
	if math.Abs(a.DistanceTo(b).Metres()-b.DistanceTo(a).Metres()) > 1e-6 {
		t.Error("distance must be symmetric")
	}
}

func TestDistanceIsStableAtVeryShortRange(t *testing.T) {
	// Two points ~11 m apart. The law of cosines loses precision here;
	// haversine must not.
	a := domain.MustCoordinate(23.810300, 90.412500)
	b := domain.MustCoordinate(23.810400, 90.412500)
	got := a.DistanceTo(b).Metres()
	if got < 5 || got > 20 {
		t.Errorf("short distance = %.3f m, want roughly 11 m", got)
	}
}

func TestDistanceAcrossAntipodes(t *testing.T) {
	// Half the circumference, ~20,015 km. Guards the asin clamp.
	got := domain.MustCoordinate(0, 0).DistanceTo(domain.MustCoordinate(0, 180)).Kilometres()
	if math.Abs(got-20015) > 50 {
		t.Errorf("antipodal distance = %.0f km, want ~20015 km", got)
	}
}

func TestDistanceUnits(t *testing.T) {
	d := domain.KilometresFrom(5)
	if d.Metres() != 5000 {
		t.Errorf("Metres() = %g, want 5000", d.Metres())
	}
	if d.Kilometres() != 5 {
		t.Errorf("Kilometres() = %g, want 5", d.Kilometres())
	}
	if got, want := d.String(), "5.00 km"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
