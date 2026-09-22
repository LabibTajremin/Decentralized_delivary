# Discovery

> P08. Implements `D1` local visibility, `D2` radius expansion with a cost, and
> `D3` the division ceiling. Algorithms `ALG-01`, `ALG-02` and `ALG-07`.

Discovery is the decentralization model as an API. It answers one question —
*what can I order from, standing here* — and everything it returns is a decision
the server has already made.

## It owns no storage

There is no `discovery` table and no migration for this phase. Every fact it
uses belongs to another module, reached through `external/` (05-architecture.md
2.5):

| From | What it asks | Why not itself |
|---|---|---|
| `geo` | which division this point is in; which merchants are inside a circle; how far two points are | PostGIS owns the geometry, and D3 belongs in the query predicate rather than a filter afterwards |
| `merchant` | which of those ids a customer may actually see | approval and suspension are merchant's decisions; a `WHERE` clause in a spatial query must never be the thing deciding them |
| `config` | the radius ladder in force *here* | Appendix B resolves area → district → division → global, so two customers a hundred metres apart can have different ladders |

That is what makes discovery a policy module rather than another table. It can
be tested end to end with four fakes and no database.

## The ladder

`domain.Policy` is the resolved expansion ladder for one place, built from six
configuration values:

```
radius(level) = discovery.base_radius + discovery.expansion_step × level
level ∈ [0, discovery.max_expansions]
```

Linear and stepwise, not geometric (`ALG-02`). A customer who taps "search
wider" twice should get a radius they can predict, and a doubling ladder
reaches the division boundary in three taps from a rural start.

`Policy.Radius` is **total**: it clamps rather than returning an error. Every
caller already holds a level the policy produced or clamped, so an error return
would be a branch no test could reach honestly.

### The one invariant

`NewPolicy` takes `divisionCeiling` as an argument and then **refuses to build a
policy when it is false**. Appendix B marks `discovery.division_ceiling` as
never disableable, and the only way to keep that true is to have somewhere that
says so out loud. A tuner bug that wrote `false` stops search in that area with
`division_ceiling_disabled` rather than silently widening every search in the
country to the whole of Bangladesh.

The bound itself is applied by geo, inside the query predicate — see
`MerchantsWithinRadiusUseCase`. Discovery never filters results by division
afterwards, because a filter is something a caller can forget.

## Settling on a level

`discovery.auto_expand` is off by default, and that default is the interesting
case: expansion costs the customer money, so it is their decision. The requested
level is clamped and searched, in one query.

With auto-expand on, `SearchUseCase.settle` walks the ladder with *counting*
queries until `discovery.min_merchants` is reached or the ceiling stops it —
`O(s·log n)` for `s` steps. Counts rather than fetches, because nobody reads the
rows of a search that is about to be widened.

Either way exactly one fetch runs, at the settled radius.

## Ranking

`ALG-07`, in `domain/ranking.go`, as a weighted score over four signals:

| Signal | Weight | Note |
|---|---|---|
| relevance | 0.40 | substring match on the shop name; a browse with no query scores every candidate the same, which leaves distance in charge |
| proximity | 0.35 | linear, normalised against *this search's* radius |
| rating | 0.15 | reviews are P16; until then every candidate is unrated and the term contributes nothing |
| open now | 0.10 | a closed shop is worth showing — with when it opens — but not above an open one |

Ties break on merchant id. That is not cosmetic: without it a customer paging
through results sees one shop twice and misses another.

## Everything the client receives is final

Per 2.9, the response carries no parts for the app to combine:

- `distance` — "১.২ কিমি", rounded to fifty metres below a kilometre because a
  shop "৩৪৭ মিটার" away is false precision over a straight line nobody walks.
- `delivery` — both the minor-unit integer and the rendered string, plus
  `expanded` so the app can say *why* the fee is higher.
- `openStatus` — a sentence, composed by merchant.
- `expansion.offered` — the answer to "should I show the widen button", not an
  input to it.
- `notice` — the one line above the list. Four genuinely different messages, not
  one template: "there is nothing here and nothing we can do about it" has to
  read as a final answer, or the customer keeps tapping a button that will never
  help.

All of it Bengali-first (1.4).

## The delivery fee is provisional

`infrastructure/fees` implements `ALG-05` from configuration and satisfies
`ports.DeliveryQuoter`. It exists because discovery is P08 and pricing is P10,
and D2's whole point is that the customer sees expansion cost more *before* they
expand.

When pricing lands it owns `ALG-05`, this package is deleted, and
`external/pricing` satisfies the same port. No use case changes.

## Contract

`DiscoveryContract.Reach` is the single-merchant form, consumed by cart
(Appendix A). A cart is built against one delivery address and then the customer
changes it at checkout — at which point the shop they chose may be in another
division, and the order must be refused before a rider is dispatched rather than
after.

It distinguishes two failures, because they are different facts:

- `outside_division` — permanent. D3 makes it unreachable at any radius, so no
  expansion level is offered against it.
- `beyond_max_radius` — inside the division, past the widest rung.

## Deferred

Cross-merchant **item** search ("who near me sells paracetamol") is not here. A
correct version needs a materialised search index rather than a menu read per
nearby shop, which would be twenty round trips on a 2G connection. It belongs
with the search work in P20.
