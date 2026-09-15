package domain

import "strings"

// Relevance scores how well a name answers a query, and whether it answers it
// at all.
//
// Substring matching rather than a stemmer or an edit distance. The names being
// matched are shop and product names in Bengali and English, often transliterated
// inconsistently by the shopkeepers who typed them; a stemmer tuned for English
// would do worse than nothing on "বিরিয়ানি" and a fuzzy distance would match
// "চাল" to "চাউল" and also to "ছাল". Postgres does the heavy version of this with
// pg_trgm on the catalogue side; what this decides is how to *order* what came
// back.
//
// The returned bool is the filter and the float is the ordering. Callers need
// both and deriving one from the other — "matched means score above zero" —
// would make an exact match that scored 0 disappear.
func Relevance(name, query string) (float64, bool) {
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		// No query is not "nothing matched": a browse matches everything, with
		// nothing to say about relative relevance, which leaves distance in
		// charge of the ranking.
		return 0, true
	}
	n := strings.ToLower(strings.TrimSpace(name))

	switch {
	case n == q:
		return 1, true
	case strings.HasPrefix(n, q):
		return 0.9, true
	case hasWordPrefix(n, q):
		return 0.75, true
	case strings.Contains(n, q):
		return 0.5, true
	default:
		return 0, false
	}
}

// hasWordPrefix reports whether any word in n starts with q. "chicken" should
// find "Spicy Chicken Roll" as readily as it finds "Chicken Roll".
func hasWordPrefix(n, q string) bool {
	for _, word := range strings.Fields(n) {
		if strings.HasPrefix(word, q) {
			return true
		}
	}
	return false
}
