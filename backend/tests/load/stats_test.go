package load

import (
	"context"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// latencies is a set of measured durations.
type latencies []time.Duration

// percentile is the value at p (0..1), nearest-rank. Nearest-rank rather than
// interpolated because the number that matters is a request that really
// happened, not an average of two that did.
func (l latencies) percentile(p float64) time.Duration {
	if len(l) == 0 {
		return 0
	}
	sorted := make(latencies, len(l))
	copy(sorted, l)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	i := int(p * float64(len(sorted)-1))
	return sorted[i]
}

// report logs the distribution. Every load test prints one, because a gate
// that only says "under the ceiling" tells the next session nothing about how
// much headroom it had.
func (l latencies) report(t *testing.T, what string) {
	t.Helper()
	t.Logf("%s: n=%d  p50=%v  p95=%v  p99=%v  max=%v",
		what, len(l), l.percentile(0.50), l.percentile(0.95),
		l.percentile(0.99), l.percentile(1))
}

// measure runs one operation `iterations` times across `workers` goroutines
// and returns every duration.
//
// Concurrent rather than serial: a serial loop measures the query, and what a
// server does is answer several at once over one pool. A lock contention or a
// connection starvation that only shows up under concurrency is the kind of
// thing this suite exists to find.
func measure(t *testing.T, workers, iterations int, op func(ctx context.Context) error) latencies {
	t.Helper()
	ctx := context.Background()

	var mu sync.Mutex
	out := make(latencies, 0, workers*iterations)
	var failures []string

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make(latencies, 0, iterations)
			var localErrs []string
			for i := 0; i < iterations; i++ {
				start := time.Now()
				err := op(ctx)
				local = append(local, time.Since(start))
				if err != nil {
					localErrs = append(localErrs, err.Error())
				}
			}
			mu.Lock()
			out = append(out, local...)
			failures = append(failures, localErrs...)
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(failures) > 0 {
		t.Fatalf("%d of %d calls failed; first: %s",
			len(failures), len(out), failures[0])
	}
	return out
}

// explain returns the planner's text plan for a query, with the planner left
// entirely alone. Nothing is switched off: the point is what Postgres chooses
// on its own once the table is big enough for the choice to matter.
func explain(t *testing.T, sql string, args ...any) string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "EXPLAIN (FORMAT TEXT) "+sql, args...)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		plan.WriteString(line)
		plan.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read plan: %v", err)
	}
	if plan.Len() == 0 {
		t.Fatal("EXPLAIN returned no plan")
	}
	return plan.String()
}

// analyse updates the statistics the planner reads. Without it the planner is
// working from the estimates of an empty table, and the plan this suite
// asserts on would be the plan for a table that no longer exists.
func analyse(t *testing.T, table string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "ANALYZE "+table); err != nil {
		t.Fatalf("analyze %s: %v", table, err)
	}
}
