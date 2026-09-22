package shared

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/id"
)

// failingReader always errors, standing in for a broken OS entropy source.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("no entropy") }

func TestNewHasPrefixAndFixedLength(t *testing.T) {
	g := id.NewGen(clock.NewFixed(time.UnixMilli(1_757_000_000_000)), bytes.NewReader(bytes.Repeat([]byte{7}, 64)))
	got := g.New("ord")

	if !strings.HasPrefix(got, "ord_") {
		t.Errorf("New(%q) = %q, want an ord_ prefix", "ord", got)
	}
	if body := strings.TrimPrefix(got, "ord_"); len(body) != 26 {
		t.Errorf("body length = %d, want 26", len(body))
	}
}

func TestNewWithoutPrefix(t *testing.T) {
	g := id.NewGen(clock.NewFixed(time.UnixMilli(0)), bytes.NewReader(bytes.Repeat([]byte{0}, 64)))
	got := g.New("")
	if strings.Contains(got, "_") {
		t.Errorf("New(\"\") = %q, want no separator", got)
	}
	if len(got) != 26 {
		t.Errorf("length = %d, want 26", len(got))
	}
}

func TestNewIsTimeOrdered(t *testing.T) {
	c := clock.NewFixed(time.UnixMilli(1_757_000_000_000))
	g := id.NewGen(c, bytes.NewReader(bytes.Repeat([]byte{0}, 4096)))

	first := g.New("")
	c.Advance(time.Second)
	second := g.New("")

	if !(first < second) {
		t.Errorf("ids must sort by time: %q should be < %q", first, second)
	}
}

func TestNewUsesDefaultsForNilArguments(t *testing.T) {
	g := id.NewGen(nil, nil)
	a, b := g.New("x"), g.New("x")
	if a == b {
		t.Error("two ids from the real entropy source must differ")
	}
	if _, err := id.Parse(a); err != nil {
		t.Errorf("Parse(%q) failed: %v", a, err)
	}
}

func TestNewIsConcurrencySafe(t *testing.T) {
	g := id.NewGen(nil, nil)
	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v := g.New("c")
			mu.Lock()
			defer mu.Unlock()
			seen[v] = true
		}()
	}
	wg.Wait()
	if len(seen) != 100 {
		t.Errorf("generated %d unique ids from 100 goroutines, want 100", len(seen))
	}
}

func TestNewPanicsWhenEntropyFails(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("New must panic when the entropy source fails")
		}
	}()
	id.NewGen(clock.NewFixed(time.UnixMilli(0)), failingReader{}).New("x")
}

func TestParseRecoversTheTimestamp(t *testing.T) {
	want := time.UnixMilli(1_757_000_123_000).UTC()
	g := id.NewGen(clock.NewFixed(want), bytes.NewReader(bytes.Repeat([]byte{3}, 64)))

	got, err := id.Parse(g.New("ord"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("Parse = %v, want %v", got, want)
	}
}

func TestParseIsCaseInsensitive(t *testing.T) {
	g := id.NewGen(clock.NewFixed(time.UnixMilli(1_757_000_000_000)), bytes.NewReader(bytes.Repeat([]byte{9}, 64)))
	raw := g.New("")
	upper, err := id.Parse(raw)
	if err != nil {
		t.Fatalf("Parse(upper) failed: %v", err)
	}
	lower, err := id.Parse(strings.ToLower(raw))
	if err != nil {
		t.Fatalf("Parse(lower) failed: %v", err)
	}
	if !upper.Equal(lower) {
		t.Errorf("case must not change the parse: %v vs %v", upper, lower)
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	cases := map[string]string{
		"too short":              "ABC",
		"too long":               strings.Repeat("A", 27),
		"bad char in timestamp":  "U" + strings.Repeat("A", 25),
		"bad char in randomness": strings.Repeat("A", 25) + "U",
		"empty":                  "",
	}
	for name, input := range cases {
		if _, err := id.Parse(input); !errors.Is(err, id.ErrInvalid) {
			t.Errorf("%s: Parse(%q) error = %v, want ErrInvalid", name, input, err)
		}
	}
}
