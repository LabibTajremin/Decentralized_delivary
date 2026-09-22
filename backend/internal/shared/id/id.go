// Package id generates identifiers.
//
// IDs are time-ordered so that primary keys cluster on insert rather than
// scattering across the index the way random UUIDs do. The encoding is
// Crockford base32: case-insensitive, no ambiguous characters, safe to read
// aloud down a phone line — which support staff in this product actually do.
package id

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/clock"
)

const (
	// alphabet is Crockford base32: no I, L, O or U.
	alphabet   = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	timeChars  = 10 // 48 bits of millisecond timestamp
	randChars  = 16 // 80 bits of randomness
	totalChars = timeChars + randChars
)

// ErrInvalid is returned when a string is not a well-formed ID.
var ErrInvalid = errors.New("invalid id")

// Generator produces identifiers.
type Generator interface {
	New(prefix string) string
}

// Gen is the default Generator. Entropy and clock are injectable so tests get
// deterministic output without weakening production randomness.
type Gen struct {
	clock   clock.Clock
	entropy io.Reader
	mu      sync.Mutex
}

// NewGen builds a Generator. Passing a nil clock or reader selects the
// production defaults.
func NewGen(c clock.Clock, entropy io.Reader) *Gen {
	if c == nil {
		c = clock.System{}
	}
	if entropy == nil {
		entropy = rand.Reader
	}
	return &Gen{clock: c, entropy: entropy}
}

// New returns an identifier, optionally prefixed as "prefix_XXXX" so an ID is
// self-describing in a log line. It panics only if the entropy source fails,
// which in production means the OS RNG is broken and nothing else can be trusted.
func (g *Gen) New(prefix string) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	ms := g.clock.Now().UnixMilli()
	var b strings.Builder
	b.Grow(len(prefix) + 1 + totalChars)
	if prefix != "" {
		b.WriteString(prefix)
		b.WriteByte('_')
	}

	// Timestamp, most significant character first.
	for i := timeChars - 1; i >= 0; i-- {
		b.WriteByte(alphabet[(ms>>(5*i))&31])
	}

	buf := make([]byte, randChars)
	if _, err := io.ReadFull(g.entropy, buf); err != nil {
		panic(fmt.Sprintf("id: entropy source failed: %v", err))
	}
	for _, v := range buf {
		b.WriteByte(alphabet[v&31])
	}
	return b.String()
}

// Parse validates an identifier and returns the time it encodes. It accepts an
// optional prefix and is case-insensitive.
func Parse(s string) (time.Time, error) {
	if i := strings.LastIndex(s, "_"); i >= 0 {
		s = s[i+1:]
	}
	if len(s) != totalChars {
		return time.Time{}, ErrInvalid
	}
	s = strings.ToUpper(s)

	var ms int64
	for i := 0; i < timeChars; i++ {
		v := strings.IndexByte(alphabet, s[i])
		if v < 0 {
			return time.Time{}, ErrInvalid
		}
		ms = ms<<5 | int64(v)
	}
	for i := timeChars; i < totalChars; i++ {
		if strings.IndexByte(alphabet, s[i]) < 0 {
			return time.Time{}, ErrInvalid
		}
	}
	return time.UnixMilli(ms).UTC(), nil
}
