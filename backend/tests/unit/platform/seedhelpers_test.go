package platform

import (
	"regexp"
	"strings"

	"github.com/rootlogic-lab/delivery/backend/migrations"
)

// seedSQL concatenates every embedded demo-data script.
func seedSQL() (string, error) {
	scripts, err := migrations.Seeds()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, s := range scripts {
		b.WriteString(s.SQL)
	}
	return b.String(), nil
}

var demoMerchantPattern = regexp.MustCompile(`MER-DEMO-\d+`)

// demoMerchantIDs returns every distinct demo merchant id the seed inserts.
func demoMerchantIDs(sql string) []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range demoMerchantPattern.FindAllString(sql, -1) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
