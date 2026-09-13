# 0001 — Modular monolith that can be split into services

Date: 2026-09-13
Status: Accepted

## Context
The platform has fourteen modules and one team. Microservices from day one would
buy independent deployment at the cost of distributed transactions, network
failure handling and operational overhead, before the product has traffic to
justify any of it.

## Options considered
1. **Microservices from the start** — correct end state, wrong starting cost.
2. **Unstructured monolith** — fastest now, unsplittable later.
3. **Modular monolith with enforced boundaries** — one deployable, module walls
   held up by a lint that fails the build.

## Decision
Option 3. Each module owns `domain/`, `application/`, `infrastructure/`,
`transport/` and `external/`. Cross-module calls go only through the caller's
own `external/` service against the target's public contract.

## Consequences
- Extraction to a service means rewriting one `external/` file, not the module.
- The boundary is enforced by `backend/tests/unit/architecture_test.go`, which
  runs as its own CI check, so it cannot erode quietly.
- Developers must write an `external/` service even for a trivial call. That is
  the cost we accept for the split option staying open.
