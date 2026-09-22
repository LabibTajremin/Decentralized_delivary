# 0002 — Tests live in a separate Go module

Date: 2026-09-13
Status: Accepted

## Context
The build instruction requires that no test file sits inside the backend
project, and that coverage reaches 100%. Those two requirements collide: Go's
`-coverpkg` lets an external module measure coverage, but an external module can
only call **exported** identifiers. Unexported functions are unreachable and can
never be covered.

## Options considered
1. **`export_test.go` inside the backend** — forbidden by the rules.
2. **Lower the coverage gate** — abandons the requirement.
3. **Design so nothing unexported needs separate testing**, and promote any
   genuinely complex unexported algorithm into its own exported package under
   `internal/`.

## Decision
Option 3. Under clean architecture every meaningful behaviour already sits
behind an exported use case or domain method; unexported helpers are trivial
enough that testing the exported caller covers them. Where that is not true, the
logic is promoted to an exported package under `internal/` — still private to
the module boundary in production, reachable by the test module through the
workspace.

## Consequences
- Coverage below 100% signals a design smell, not a testing gap.
- `internal/` stays private outside the module, so promotion leaks nothing.
- A new coverage exclusion requires its own ADR.
