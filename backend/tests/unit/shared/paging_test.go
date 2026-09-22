package shared

import (
	"errors"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/paging"
)

func TestNewPageClampsLimit(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, paging.DefaultLimit},
		{-5, paging.DefaultLimit},
		{10, 10},
		{paging.MaxLimit, paging.MaxLimit},
		{paging.MaxLimit + 1, paging.MaxLimit},
		{100000, paging.MaxLimit},
	}
	for _, c := range cases {
		p, err := paging.NewPage(c.in, 0)
		if err != nil {
			t.Fatalf("NewPage(%d, 0) error: %v", c.in, err)
		}
		if p.Limit != c.want {
			t.Errorf("NewPage(%d).Limit = %d, want %d", c.in, p.Limit, c.want)
		}
	}
}

func TestNewPageRejectsNegativeOffset(t *testing.T) {
	if _, err := paging.NewPage(10, -1); !errors.Is(err, paging.ErrInvalidCursor) {
		t.Errorf("error = %v, want ErrInvalidCursor", err)
	}
}

func TestNewPageKeepsOffset(t *testing.T) {
	p, err := paging.NewPage(10, 40)
	if err != nil {
		t.Fatalf("NewPage error: %v", err)
	}
	if p.Offset != 40 {
		t.Errorf("Offset = %d, want 40", p.Offset)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	for _, offset := range []int{0, 1, 20, 999999} {
		encoded := paging.Cursor{Offset: offset}.Encode()
		got, err := paging.DecodeCursor(encoded)
		if err != nil {
			t.Fatalf("DecodeCursor(%q) error: %v", encoded, err)
		}
		if got.Offset != offset {
			t.Errorf("round trip offset = %d, want %d", got.Offset, offset)
		}
	}
}

func TestCursorIsOpaque(t *testing.T) {
	encoded := paging.Cursor{Offset: 40}.Encode()
	if encoded == "40" || encoded == "o:40" {
		t.Errorf("cursor %q leaks its internal form", encoded)
	}
}

func TestDecodeEmptyCursorStartsAtZero(t *testing.T) {
	got, err := paging.DecodeCursor("")
	if err != nil {
		t.Fatalf("DecodeCursor(\"\") error: %v", err)
	}
	if got.Offset != 0 {
		t.Errorf("Offset = %d, want 0", got.Offset)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	cases := map[string]string{
		"not base64":     "!!!!",
		"wrong prefix":   "eDo0MA",           // "x:40"
		"offset not int": "bzphYmM",          // "o:abc"
		"negative":       "bzotMQ",           // "o:-1"
	}
	for name, input := range cases {
		if _, err := paging.DecodeCursor(input); !errors.Is(err, paging.ErrInvalidCursor) {
			t.Errorf("%s: DecodeCursor(%q) error = %v, want ErrInvalidCursor", name, input, err)
		}
	}
}

func TestNewResultTrimsTheProbeRowAndSignalsMore(t *testing.T) {
	p, _ := paging.NewPage(3, 0)
	// Callers fetch Limit+1 rows; the extra proves more exist.
	fetched := []string{"a", "b", "c", "d"}

	got := paging.NewResult(fetched, p, 10)
	if len(got.Items) != 3 {
		t.Errorf("Items = %v, want the probe row trimmed to 3", got.Items)
	}
	if !got.HasMore {
		t.Error("HasMore = false, want true when a probe row came back")
	}
	if got.NextCursor == "" {
		t.Error("NextCursor must be set when more pages exist")
	}
	next, err := paging.DecodeCursor(got.NextCursor)
	if err != nil {
		t.Fatalf("next cursor did not decode: %v", err)
	}
	if next.Offset != 3 {
		t.Errorf("next offset = %d, want 3", next.Offset)
	}
	if got.Total != 10 {
		t.Errorf("Total = %d, want 10", got.Total)
	}
}

func TestNewResultOnLastPage(t *testing.T) {
	p, _ := paging.NewPage(3, 6)
	got := paging.NewResult([]string{"g", "h"}, p, 8)

	if got.HasMore {
		t.Error("HasMore = true, want false on the last page")
	}
	if got.NextCursor != "" {
		t.Errorf("NextCursor = %q, want empty on the last page", got.NextCursor)
	}
	if len(got.Items) != 2 {
		t.Errorf("Items = %v, want both rows kept", got.Items)
	}
}

func TestNewResultOnEmptySet(t *testing.T) {
	p, _ := paging.NewPage(20, 0)
	got := paging.NewResult([]int{}, p, 0)
	if got.HasMore || got.NextCursor != "" || len(got.Items) != 0 {
		t.Errorf("empty result = %+v, want no items and no next page", got)
	}
}
