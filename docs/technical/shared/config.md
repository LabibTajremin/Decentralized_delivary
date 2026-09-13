# shared/config
Layer: Shared · Module: — · Phase: P01

**Responsibility** — load **process** configuration from the environment.

**Inputs / Outputs** — `Load(Lookup) (Config, error)`; `NewLoader` with `String`, `Required`, `Int`, `Bool`, `Duration`, `Err`.

**Dependencies** — standard library only.

**Rules enforced**
- This is bootstrap configuration only: bind address, database URL, Redis URL, log level. **Business variables — radius, fees, COD limits — are not here.** They live in the config module, resolve per area, and are tunable at runtime by an admin (D5, Appendix B). Putting one here would make it need a redeploy to change.
- One pass reports every problem, sorted, rather than failing on the first. An operator fixing a misconfigured deployment sees the whole list at once.
- A whitespace-only value counts as missing, because an empty environment variable in a compose file is a common and silent mistake.
- A malformed but present value records an error *and* returns the default, so the loader can keep going and collect the rest.
- `Lookup` is injectable, so tests never mutate the real process environment.

**Algorithms used** — none.

**Failure modes** — returns a single joined error listing every missing required key and every unparseable value.

**Tests** — `backend/tests/unit/shared/config_test.go`. Covers defaults, overrides, multi-error reporting, blank-as-missing, every parse-failure branch, sorted output, and the nil-lookup path against a real environment variable.
