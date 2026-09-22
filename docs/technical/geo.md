# Geo module

The foundation of the product. Every other module's correctness depends on this
one being right: pricing bands off its distances, discovery scopes off its
radius search, dispatch assigns off its geometry, and the config module resolves
per-area values off its administrative hierarchy.

## What it owns

- The administrative hierarchy: eight divisions, their districts, and the areas
  inside them.
- Merchant *locations* — an id and a point. Nothing else about a merchant. The
  name, menu and opening hours belong to the merchant module; geo keeps only
  what it needs to index spatially, because the spatial index should be owned by
  the module that queries it.
- The single definition of distance for the whole system.

## Why distance lives here

Pricing, dispatch and the client all need the distance between two points. If
each computed its own, a customer could be quoted a fee for 4.2 km, a rider paid
for 4.4 km, and the app show 4.3 km — three answers for one journey, with no way
to say which is wrong. So there is one implementation, exposed through the
contract, and everything calls it.

That decision has a consequence in the SQL. PostGIS defaults to the WGS84
spheroid; `domain.Coordinate.DistanceTo` uses a sphere. The two differ by about
0.4% — roughly 1.7 m over 440 m. Rather than live with two answers, the queries
pass `use_spheroid = false` so PostGIS uses the same sphere the Go code does, and
`TestPostGISAgreesWithDomainGeometry` asserts they agree to within a metre.

Over the 25 km maximum radius the sphere is off by tens of metres in absolute
terms. That is well inside the kilometre granularity of the fee bands (ALG-05),
so agreement between components is worth more here than absolute accuracy
against the true shape of the earth.

## The division ceiling (D3)

A radius search never crosses a division boundary. This is the one invariant no
admin setting and no auto-tuner may disable, and it is enforced in the `WHERE`
clause rather than by filtering results afterwards:

```sql
WHERE is_active
  AND division_code = $3
  AND ST_DWithin(location, ..., $4, false)
```

A post-filter would be one forgotten call away from leaking a Chattogram
merchant into a Dhaka customer's results. In the query, a caller cannot forget
it, because there is no code path that reaches the rows without it.

`TestRadiusSearchNeverCrossesADivision` asserts this end to end, at the maximum
radius the transport allows.

## Algorithms

| ID | What | Where |
|---|---|---|
| ALG-01 | Merchants within radius, nearest first | `ST_DWithin` over a GiST index, `MerchantsWithinRadius` |
| ALG-03 | Which division contains a point | `ST_Contains` over a GiST index, `DivisionContaining` |

Both depend on the spatial index. Without it ALG-01 is a full scan and the
product does not work at national scale — which is why ADR 0003 records PostGIS
as a deliberate, non-portable dependency rather than an incidental choice.

`AreaContaining` orders by `ST_Area` ascending so the *smallest* containing area
wins. Areas may abut or nest, and config resolution wants the most specific one:
a neighbourhood, not the upazila enclosing it.

## Layers

```
transport/http  →  contract  ←  other modules (via their own external/)
                      ↑
                 application  →  ports  ←  infrastructure/persistence/postgres
                      ↓
                   domain
```

Transport depends on `contract.GeoContract`, the same interface other modules
use. There is one public surface to keep correct, not two, and extracting geo
into its own service later means changing the implementations behind that
interface rather than every caller.

`contract` uses plain `float64` pairs rather than `domain.Coordinate`. A
consumer therefore never links geo's internals, and the shape survives a move to
HTTP or gRPC unchanged.

## Errors

| Code | Kind | When |
|---|---|---|
| `invalid_coordinate` | invalid (400) | Latitude or longitude out of range |
| `outside_service_area` | not found (404) | A valid coordinate outside every division |
| `geo_lookup_failed` | unavailable (503) | The database could not answer |

The first two are deliberately different statuses. A rise in
`invalid_coordinate` means a client is sending nonsense and is our bug; a rise
in `outside_service_area` is ordinary traffic from people in places we have not
reached yet. Collapsing both into 400 would hide the first inside the second.

## Geometry data

`backend/migrations/seed/0001_geo.sql` carries demo geometry: eight
non-overlapping rectangles partitioning Bangladesh's bounding box, chosen so
each divisional capital falls inside its own division. It is enough to exercise
ALG-03 and D3 honestly, and it is not survey data. Official boundaries replace it
without any code change — the schema and the queries are already correct for
real polygons.

Seed data is never applied by `migrate up`; see ADR 0006.

## Testing

- **Unit** — the domain's haversine and ray-casting against known values, and
  the repository's failure branches through a stubbed `Querier`.
- **Integration** — real PostGIS, each test in a transaction that rolls back.
  Includes the assertion that the database and the Go domain agree on distance,
  and that the radius search actually uses the spatial index rather than
  silently falling back to a scan.
- **E2E** — the real binary against a real database, including D3.
