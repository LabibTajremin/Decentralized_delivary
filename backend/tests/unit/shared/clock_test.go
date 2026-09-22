package shared

import (
	"sync"
	"testing"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
)

func TestSystemClockReturnsUTC(t *testing.T) {
	now := clock.System{}.Now()
	if now.Location() != time.UTC {
		t.Errorf("System.Now() location = %v, want UTC", now.Location())
	}
	if time.Since(now) > time.Minute {
		t.Errorf("System.Now() = %v, which is not close to now", now)
	}
}

func TestFixedClockIsPinned(t *testing.T) {
	base := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	c := clock.NewFixed(base)
	if !c.Now().Equal(base) {
		t.Errorf("Now() = %v, want %v", c.Now(), base)
	}
	if !c.Now().Equal(c.Now()) {
		t.Error("a fixed clock must not move on its own")
	}
}

func TestFixedClockNormalisesToUTC(t *testing.T) {
	loc := time.FixedZone("BST", 6*3600)
	c := clock.NewFixed(time.Date(2026, 9, 13, 16, 0, 0, 0, loc))
	if c.Now().Location() != time.UTC {
		t.Errorf("NewFixed location = %v, want UTC", c.Now().Location())
	}
	if got, want := c.Now().Hour(), 10; got != want {
		t.Errorf("hour = %d, want %d after UTC conversion", got, want)
	}
}

func TestFixedAdvanceAndSet(t *testing.T) {
	base := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	c := clock.NewFixed(base)

	got := c.Advance(90 * time.Minute)
	want := base.Add(90 * time.Minute)
	if !got.Equal(want) {
		t.Errorf("Advance returned %v, want %v", got, want)
	}
	if !c.Now().Equal(want) {
		t.Errorf("Now() after Advance = %v, want %v", c.Now(), want)
	}

	reset := time.Date(2030, 1, 1, 0, 0, 0, 0, time.FixedZone("X", 3600))
	c.Set(reset)
	if !c.Now().Equal(reset.UTC()) {
		t.Errorf("Now() after Set = %v, want %v", c.Now(), reset.UTC())
	}
	if c.Now().Location() != time.UTC {
		t.Error("Set must normalise to UTC")
	}
}

func TestFixedClockIsConcurrencySafe(t *testing.T) {
	c := clock.NewFixed(time.Unix(0, 0))
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); c.Advance(time.Second) }()
		go func() { defer wg.Done(); _ = c.Now() }()
	}
	wg.Wait()
	if got := c.Now().Unix(); got != 50 {
		t.Errorf("after 50 one-second advances, Unix() = %d, want 50", got)
	}
}
