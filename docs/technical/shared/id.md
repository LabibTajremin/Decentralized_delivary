# shared/id
Layer: Shared · Module: — · Phase: P01

**Responsibility** — generate time-ordered, human-readable identifiers.

**Inputs / Outputs** — `Generator` interface; `NewGen(clock, entropy) *Gen`; `New(prefix) string`; `Parse(string) (time.Time, error)`.

**Dependencies** — `shared/clock`, `crypto/rand`.

**Rules enforced**
- 48 bits of millisecond timestamp first, then 80 bits of randomness, so IDs sort by creation time and primary keys cluster on insert instead of scattering across the index the way random UUIDs do.
- Crockford base32: no `I`, `L`, `O` or `U`. Support staff in this product read order IDs aloud over the phone, so ambiguous characters are a real cost.
- An optional prefix (`ord_`, `mer_`) makes an ID self-describing in a log line.
- Clock and entropy are injectable, so tests are deterministic without weakening production randomness.

**Algorithms used** — none from the ALG table.

**Failure modes** — `New` panics if the entropy source fails; in production that means the OS RNG is broken and nothing else can be trusted either. `Parse` returns `ErrInvalid` for wrong length or an out-of-alphabet character.

**Tests** — `backend/tests/unit/shared/id_test.go`. Covers time ordering across an advancing clock, prefix handling, case-insensitive parsing, every malformed-input branch, 100-goroutine uniqueness, and the entropy-failure panic.
