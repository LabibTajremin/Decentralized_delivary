// Package codes makes the short reference a customer reads out on the phone.
package codes

import (
	"crypto/rand"
	"math/big"
	"strings"
)

// alphabet is deliberately not the whole of A–Z0–9.
//
// Gone are I, O, 0 and 1, which people confuse when reading a code aloud to a
// call centre, and 5/S. What is left is unambiguous over a bad phone line in a
// second language, which is the only situation this code exists for.
const alphabet = "2346789ABCDEFGHJKLMNPQRTUVWXYZ"

// length is how many characters. Thirty symbols to the sixth power is a little
// over seven hundred million, which is enough that guessing one is pointless
// and short enough to say in one breath.
const length = 6

// Generator makes order codes.
type Generator struct {
	// random is the source, injectable so a test can prove the shape of the
	// output rather than that it varies.
	//
	// It returns no error, which is a claim worth defending: the only way
	// crypto/rand.Int fails is a non-positive bound or an unreadable reader,
	// and the bound here is a constant. Since Go 1.24 an unreadable reader is
	// a panic rather than an error, because a process with no entropy cannot
	// safely continue — so an error return would be a branch that exists only
	// to be untestable.
	random func(upper int) int
}

// New builds a generator over crypto/rand.
//
// Cryptographic randomness rather than math/rand, because the code appears in
// support conversations and a predictable sequence would let somebody ask about
// an order that is not theirs.
func New() *Generator { return &Generator{random: randomIndex} }

// NewWith builds a generator over a supplied source, for tests.
func NewWith(random func(upper int) int) *Generator {
	return &Generator{random: random}
}

// randomIndex draws one index from crypto/rand.
func randomIndex(upper int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(upper))) //nolint:errcheck // see the note on Generator.random
	return int(n.Int64())
}

// Code returns a short, unambiguous reference.
//
// Uniqueness is the database's job, not this function's: the unique index on
// orders.code is what actually guarantees a code is free, and a generator that
// tried to guarantee it would need a round trip per attempt.
func (g *Generator) Code() string {
	var b strings.Builder
	b.Grow(length)
	for i := 0; i < length; i++ {
		b.WriteByte(alphabet[g.random(len(alphabet))])
	}
	return b.String()
}
