// Package paging provides cursor pagination.
//
// Every list endpoint is paginated (05-architecture.md 2.9): the client must
// never receive a list long enough to need slicing on device. Cursors are
// opaque to the client so the server can change ordering without breaking it.
package paging

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

const (
	// DefaultLimit is used when the caller asks for none.
	DefaultLimit = 20
	// MaxLimit caps a page. A low-end device should never be handed more.
	MaxLimit = 100
)

// ErrInvalidCursor is returned when a cursor is malformed or truncated.
var ErrInvalidCursor = errors.New("invalid cursor")

// Page is a validated pagination request.
type Page struct {
	Limit  int
	Offset int
}

// NewPage clamps a requested limit into range and rejects a negative offset.
// A limit of zero or less selects DefaultLimit; anything above MaxLimit is
// clamped rather than rejected, so a client bug degrades instead of failing.
func NewPage(limit, offset int) (Page, error) {
	if offset < 0 {
		return Page{}, ErrInvalidCursor
	}
	switch {
	case limit <= 0:
		limit = DefaultLimit
	case limit > MaxLimit:
		limit = MaxLimit
	}
	return Page{Limit: limit, Offset: offset}, nil
}

// Cursor encodes a position in a result set.
type Cursor struct {
	Offset int
}

// Encode renders a cursor as an opaque string.
func (c Cursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString([]byte("o:" + strconv.Itoa(c.Offset)))
}

// DecodeCursor parses a cursor produced by Encode. An empty string decodes to
// the start of the result set, so a first request needs no special case.
func DecodeCursor(s string) (Cursor, error) {
	if s == "" {
		return Cursor{Offset: 0}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	text := string(raw)
	if !strings.HasPrefix(text, "o:") {
		return Cursor{}, ErrInvalidCursor
	}
	offset, err := strconv.Atoi(strings.TrimPrefix(text, "o:"))
	if err != nil || offset < 0 {
		return Cursor{}, ErrInvalidCursor
	}
	return Cursor{Offset: offset}, nil
}

// Result is one page of items plus what the client needs to ask for the next.
type Result[T any] struct {
	Items      []T
	NextCursor string
	HasMore    bool
	Total      int
}

// NewResult builds a page result. Callers fetch Limit+1 rows and pass them in:
// the extra row is what proves more exist without a second COUNT query. The
// extra row is trimmed before the result is returned.
func NewResult[T any](items []T, p Page, total int) Result[T] {
	hasMore := len(items) > p.Limit
	if hasMore {
		items = items[:p.Limit]
	}
	out := Result[T]{Items: items, HasMore: hasMore, Total: total}
	if hasMore {
		out.NextCursor = Cursor{Offset: p.Offset + p.Limit}.Encode()
	}
	return out
}
