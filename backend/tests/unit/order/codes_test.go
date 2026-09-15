package order

import (
	"strings"
	"testing"

	"github.com/rootlogic-lab/delivery/backend/internal/modules/order/infrastructure/codes"
)

// The short reference a customer reads out on the phone. Its whole job is to
// survive a bad line in a second language.

func TestACodeHasNoConfusableCharacters(t *testing.T) {
	generator := codes.New()
	// Enough draws to make a missing exclusion very unlikely to hide.
	for i := 0; i < 500; i++ {
		code := generator.Code()
		if len(code) != 6 {
			t.Fatalf("code %q is %d characters, want 6", code, len(code))
		}
		// I, O, 0, 1 and S/5 are what people get wrong when reading a code
		// aloud. None of them may appear.
		if strings.ContainsAny(code, "IO01S5") {
			t.Fatalf("code %q contains a confusable character", code)
		}
		for _, c := range code {
			if !strings.ContainsRune("2346789ABCDEFGHJKLMNPQRTUVWXYZ", c) {
				t.Fatalf("code %q contains %q, which is outside the alphabet", code, c)
			}
		}
	}
}

func TestCodesVary(t *testing.T) {
	generator := codes.New()
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		seen[generator.Code()] = true
	}
	// Not a uniqueness guarantee — the unique index on the column is that —
	// but a generator returning one value would be a real bug and this is what
	// catches it.
	if len(seen) < 190 {
		t.Fatalf("200 draws produced only %d distinct codes", len(seen))
	}
}

func TestTheSourceDecidesTheCharacters(t *testing.T) {
	// A source that always returns 0 gives the first character of the alphabet
	// six times, which pins the mapping rather than the randomness.
	if got := codes.NewWith(func(int) int { return 0 }).Code(); got != "222222" {
		t.Fatalf("code = %q, want 222222", got)
	}
	// And one that walks the alphabet pins the indexing.
	i := 0
	walking := codes.NewWith(func(upper int) int {
		i++
		return (i - 1) % upper
	})
	if got := walking.Code(); got != "234678" {
		t.Fatalf("code = %q, want 234678", got)
	}
}
