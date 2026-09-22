# 0003 — PostGIS for radius search, with MySQL as a documented fallback

Date: 2026-09-13
Status: Accepted

## Context
Rule 2.4 requires a swappable database engine. But radius-scoped discovery
(D1–D3) is the product, and its performance rests on `ST_DWithin` with a GiST
index (ALG-01) and `ST_Contains` against division geometry (ALG-03).

## Options considered
1. **Engine-neutral SQL only** — portable, but loses the spatial index and turns
   ALG-01 into a table scan.
2. **PostGIS-only** — fastest, abandons the adapter rule outright.
3. **Engine-neutral port, engine-specific implementations, honest about parity.**

## Decision
Option 3. The port stays neutral — `FindMerchantsWithinRadius` — and
implementations live under `infrastructure/persistence/<engine>/`. PostGIS is
the supported production path. A MySQL implementation may exist, but it is a
fallback, not a peer, and that asymmetry is documented rather than hidden.

## Consequences
- No business logic ever sees an engine-specific type.
- Swapping engines is a config change; swapping to MySQL is also a performance
  decision the operator is told about explicitly.
- The geospatial path is the one place where "any engine" is a claim we decline
  to make.
