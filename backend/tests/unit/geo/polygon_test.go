package geo

import (
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/geo/domain"
)

// square is a 2x2 degree box from (0,0) to (2,2).
func square(t *testing.T) domain.Polygon {
	t.Helper()
	p, err := domain.NewPolygon([]domain.Coordinate{
		domain.MustCoordinate(0, 0),
		domain.MustCoordinate(0, 2),
		domain.MustCoordinate(2, 2),
		domain.MustCoordinate(2, 0),
	})
	if err != nil {
		t.Fatalf("build square: %v", err)
	}
	return p
}

func TestNewPolygonRejectsDegenerateRings(t *testing.T) {
	cases := map[string][]domain.Coordinate{
		"empty":     {},
		"one point": {domain.MustCoordinate(0, 0)},
		"two points": {
			domain.MustCoordinate(0, 0),
			domain.MustCoordinate(1, 1),
		},
		"closing vertex leaves only two": {
			domain.MustCoordinate(0, 0),
			domain.MustCoordinate(1, 1),
			domain.MustCoordinate(0, 0),
		},
	}
	for name, ring := range cases {
		if _, err := domain.NewPolygon(ring); !errors.Is(err, domain.ErrDegeneratePolygon) {
			t.Errorf("%s: error = %v, want ErrDegeneratePolygon", name, err)
		}
	}
}

func TestNewPolygonAcceptsExplicitlyClosedRing(t *testing.T) {
	closed, err := domain.NewPolygon([]domain.Coordinate{
		domain.MustCoordinate(0, 0),
		domain.MustCoordinate(0, 2),
		domain.MustCoordinate(2, 2),
		domain.MustCoordinate(2, 0),
		domain.MustCoordinate(0, 0), // repeated closing vertex
	})
	if err != nil {
		t.Fatalf("closed ring rejected: %v", err)
	}
	if got := len(closed.Vertices()); got != 4 {
		t.Errorf("vertices = %d, want the closing vertex dropped to 4", got)
	}
	if !closed.Contains(domain.MustCoordinate(1, 1)) {
		t.Error("a closed ring must behave like the open one")
	}
}

func TestVerticesReturnsCopy(t *testing.T) {
	p := square(t)
	v := p.Vertices()
	v[0] = domain.MustCoordinate(50, 50)
	if p.Vertices()[0] != domain.MustCoordinate(0, 0) {
		t.Error("Vertices() must return a copy the caller cannot use to reshape the polygon")
	}
}

func TestBoundingBox(t *testing.T) {
	sw, ne := square(t).BoundingBox()
	if sw != domain.MustCoordinate(0, 0) || ne != domain.MustCoordinate(2, 2) {
		t.Errorf("bounding box = (%v, %v), want ((0,0), (2,2))", sw, ne)
	}
}

func TestContainsInsideAndOutside(t *testing.T) {
	p := square(t)
	cases := map[string]struct {
		point domain.Coordinate
		want  bool
	}{
		"centre":              {domain.MustCoordinate(1, 1), true},
		"well outside north":  {domain.MustCoordinate(3, 1), false},
		"well outside south":  {domain.MustCoordinate(-1, 1), false},
		"well outside east":   {domain.MustCoordinate(1, 3), false},
		"well outside west":   {domain.MustCoordinate(1, -1), false},
		"just inside":         {domain.MustCoordinate(0.0001, 1), true},
		"just outside":        {domain.MustCoordinate(-0.0001, 1), false},
		"corner vertex":       {domain.MustCoordinate(0, 0), true},
		"opposite corner":     {domain.MustCoordinate(2, 2), true},
		"midpoint of an edge": {domain.MustCoordinate(0, 1), true},
		"edge on the right":   {domain.MustCoordinate(1, 2), true},
	}
	for name, c := range cases {
		if got := p.Contains(c.point); got != c.want {
			t.Errorf("%s: Contains(%v) = %v, want %v", name, c.point, got, c.want)
		}
	}
}

func TestContainsConcavePolygon(t *testing.T) {
	// An L shape: the notch must read as outside even though it is inside the
	// bounding box. This is where a bounding-box-only check would be wrong.
	l, err := domain.NewPolygon([]domain.Coordinate{
		domain.MustCoordinate(0, 0),
		domain.MustCoordinate(0, 3),
		domain.MustCoordinate(1, 3),
		domain.MustCoordinate(1, 1),
		domain.MustCoordinate(3, 1),
		domain.MustCoordinate(3, 0),
	})
	if err != nil {
		t.Fatalf("build L: %v", err)
	}
	if l.Contains(domain.MustCoordinate(2, 2)) {
		t.Error("the notch of an L must be outside")
	}
	if !l.Contains(domain.MustCoordinate(0.5, 2)) {
		t.Error("the tall arm of an L must be inside")
	}
	if !l.Contains(domain.MustCoordinate(2, 0.5)) {
		t.Error("the wide arm of an L must be inside")
	}
}

func TestContainsIgnoresWindingOrder(t *testing.T) {
	clockwise, err := domain.NewPolygon([]domain.Coordinate{
		domain.MustCoordinate(0, 0),
		domain.MustCoordinate(2, 0),
		domain.MustCoordinate(2, 2),
		domain.MustCoordinate(0, 2),
	})
	if err != nil {
		t.Fatalf("build clockwise: %v", err)
	}
	if !clockwise.Contains(domain.MustCoordinate(1, 1)) {
		t.Error("ray casting must not depend on winding order")
	}
}

func TestContainsAtVertexLatitude(t *testing.T) {
	// A point level with a vertex is the classic ray-casting double-count bug.
	tri, err := domain.NewPolygon([]domain.Coordinate{
		domain.MustCoordinate(0, 0),
		domain.MustCoordinate(2, 1),
		domain.MustCoordinate(0, 2),
	})
	if err != nil {
		t.Fatalf("build triangle: %v", err)
	}
	if tri.Contains(domain.MustCoordinate(2, 5)) {
		t.Error("a point east of the apex must be outside")
	}
	if !tri.Contains(domain.MustCoordinate(0.5, 1)) {
		t.Error("a point inside the triangle must be inside")
	}
}
