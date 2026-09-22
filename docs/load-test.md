# Load test — discovery and dispatch

P20's acceptance asks for a load test on discovery and dispatch. This is it:
what is measured, why those two, how to run it, and what it said the last time
it ran.

The tests live in `backend/tests/load/` and run as part of
`./scripts/verify.sh`, so this is a gate rather than a report somebody ran
once. The whole package finishes in about a second at the default scale.

## Why these two and nothing else

Most of this product's cost per request does not grow with the size of the
country. A cart is read by its own id. An order is read by its own id. A menu
is one shop's rows. Those get slower when Postgres gets slower and not
otherwise, and load-testing them would measure the container.

Two paths are different, and they are different because they ask a spatial
question over every row of a growing table:

| Algorithm | Where | What grows |
|---|---|---|
| **ALG-01** merchants within radius | every customer's home screen | shops in the division |
| **ALG-02** radius expansion | every search in a thin area, several times over | shops in the division, × expansion steps |
| **ALG-04** partner assignment | every time any shop says "ready" | riders on shift |

Those are the three the phase list names, and they are the three where a
regression is invisible until the product succeeds enough to expose it.

## What is actually asserted

**The planner's own choice, at scale.** This is the assertion that carries the
weight. `backend/tests/integration/geo_repository_test.go` already proves the
GiST index *can* serve ALG-01 — but it has to `SET LOCAL enable_seqscan = off`
to make Postgres prefer an index over a table of eleven seeded rows. That is a
different claim from the one that matters in production. Here the table holds
two thousand shops, nothing is switched off, and the test asserts that
Postgres chooses `geo_merchants_location_idx` by itself. Dispatch gets the same
assertion against its partial index, `delivery_partners_available_idx`.

Both assertions were verified the way `CLAUDE.md` asks: the index was dropped
and the plan re-read. Without it the plan is
`Seq Scan on geo_merchant_locations (cost=0.00..25225.17)`, which both
assertions catch — the index name disappears and `Seq Scan` appears.

**Latency under concurrency, against a deliberately loose ceiling.** Sixteen
callers at once, twenty-five calls each, p95 asserted under 250 ms for a
single radius query and under a second for a whole seven-step expansion. The
measured p95 is an order of magnitude inside that. The looseness is the point:
this runs on a shared CI container, and a tight bound would go red for reasons
that have nothing to do with the code. What a loose bound still catches is the
change that turns a lookup into a scan.

**The shape of ALG-04's ordering, with no database in it.** The assigner is
documented as O(n log n) to build and O(log n) to take the next candidate. The
way that claim gets broken is not a slow heap — it is a sort moved inside a
loop, or a re-score on every `Next`. So the test builds a pool, then a pool
eight times larger, and requires the cost to grow by less than twenty-five
times. Log-linear growth over an eight-fold increase is about seven; quadratic
is sixty-four. Verified by substituting a quadratic stand-in, which the gate
rejected at 70×.

**That D3 is not something scale can defeat.** The division ceiling is in the
`WHERE` clause, not applied afterwards, so the same centre asked as Chattogram
returns nothing however many Dhaka shops are in the table.

## Running it

```bash
export DATABASE_URL=postgres://delivery@127.0.0.1:5433/delivery?sslmode=disable
cd backend/tests && go test ./load/ -count=1 -v
```

`-v` matters: every test logs its distribution and its query plan, and the
numbers are the output. A passing run with no `-v` tells you only that nothing
broke.

`LOAD_SCALE` raises the fixture size. The assertions are written so a larger
number only makes them stricter — the latency ceilings are fixed and the
scaling bound is a ratio.

```bash
LOAD_SCALE=50000 go test ./load/ -count=1 -v    # a real soak run
```

The default is 2,000 rows per fixture. That is past the point where Postgres
stops treating a table as trivially small — which is the whole question the
plan assertions ask — and still writes in well under a second. A gate that
took four minutes would eventually be commented out.

## The last run

Default scale, 2,000 shops and 2,000 riders, on the development container
(Postgres 16 + PostGIS, same machine as the test).

| Measurement | p50 | p95 | p99 | max | ceiling |
|---|---|---|---|---|---|
| ALG-01, one 3 km radius search, 16 concurrent | 1.4 ms | 6.5 ms | 12.2 ms | 22.8 ms | 250 ms |
| ALG-02, the whole 7-step expansion ladder, 8 concurrent | 9.8 ms | 12.9 ms | 13.7 ms | 16.1 ms | 1 s |
| ALG-04, candidate pool within 5 km, 16 concurrent | 8.5 ms | 11.1 ms | 13.1 ms | 14.1 ms | 250 ms |

ALG-04's ordering, in memory: 2,000 candidates in 1.0 ms, 16,000 in 9.2 ms —
a ratio of 8.8 for an eight-fold increase, against a bound of 25.

The expansion ladder, which is also a check that the steps really widen one
search rather than repeating it:

| Radius | Shops found |
|---|---|
| 1 km | 4 |
| 2 km | 17 |
| 3 km | 33 |
| 5 km | 93 |
| 8 km | 261 |
| 12 km | 611 |
| 25 km | 1,991 |

Both plans at that scale:

```
ALG-01
Limit
  ->  Sort  (Sort Key: st_distance(location, …))
        ->  Bitmap Heap Scan on geo_merchant_locations
              Filter: is_active AND division_code = 'DHA' AND st_dwithin(…)
              ->  Bitmap Index Scan on geo_merchants_location_idx

ALG-04
Limit
  ->  Sort  (Sort Key: st_distance(pin, …))
        ->  Bitmap Heap Scan on delivery_partners
              Recheck Cond: pin IS NOT NULL AND availability = 'available'
              ->  Bitmap Index Scan on delivery_partners_available_idx
```

## What this does not cover, and what would

Being honest about the edges, so the next person does not read more into a
green gate than it says:

* **One process, one machine.** The fixtures and the queries share a container
  with Postgres. The figures are therefore about the *query*, not about a
  deployment: no network hop, no connection-pool saturation across replicas,
  no noisy neighbour. A pre-production soak against the real topology is a
  deploy-time exercise and belongs in `docs/runbook.md`, not in a unit gate.
* **The algorithms, not the HTTP surface.** These call the repositories
  directly. The whole-product path — auth, the handler, JSON, pricing
  composition — is covered by `backend/tests/e2e/regression_test.go`, at one
  order rather than at volume. An HTTP-level throughput test would mostly
  measure Go's HTTP server, which is not the thing that got written here.
* **Reads, not write contention.** Nothing here measures two riders accepting
  the same job at the same moment. That is a correctness property, not a
  throughput one, and dispatch's integration suite holds it: the job board's
  claim is a conditional update, and the test that proves it is the one where
  two partners race.
* **Two thousand rows is a city, not a country.** It is chosen to be past the
  planner's "small table" threshold, not to be Bangladesh. `LOAD_SCALE` is
  there for the day that question matters, and the assertions were written so
  that raising it needs no edits.
