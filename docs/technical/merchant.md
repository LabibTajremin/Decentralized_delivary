# Merchant

A shop: who owns it, where it is, what papers it has filed, and whether the
public may see it.

Two rules shape everything in this module.

**D1 — nationwide registration, local visibility.** A merchant may register from
anywhere in Bangladesh. Nothing in registration asks how many shops the division
already holds, how far the nearest customer is, or whether we currently deliver
there.

**Approval gates visibility.** A shop that exists and a shop customers can see
are deliberately two different things.

## What "visible" means, exactly

Three separate questions, answered in three places, and conflating any two of
them is a bug:

| Question | Where it is answered | What it decides |
|---|---|---|
| Is this shop in the search index at all? | `is_active` on geo's row | Whether a radius query can ever return it |
| May a customer see it? | `Merchant.IsListed` | Approved, and not on holiday |
| Is it taking orders right now? | `Merchant.IsOpenAt` | Listed, and inside an opening window |

`is_active` tracks **approval**, not existence. A shop in draft, under review,
rejected or suspended keeps its point in the spatial index with the flag off. So
approving it is a flag change rather than a re-registration, and a suspension
takes effect on the next search rather than after an index rebuild.

**Holiday mode never touches the index.** An approved shop stays active there and
is filtered out by `IsListed`. That is why a holiday costs one row write instead
of a spatial update, and why a holiday with an end date expires on its own: a
sweeper job that fails would leave every shop that closed for Eid invisible, and
nobody would notice until the merchants called.

Being shut for the evening is a fourth thing again, and it does *not* hide the
shop. A customer scrolling at midnight should find the restaurant and see when it
opens.

## The approval workflow

```
draft ──submit──▶ pending_review ──approve──▶ approved ──suspend──▶ suspended
                        │                        ▲                      │
                        └──reject──▶ rejected    └──────reinstate───────┘
                                        │
                                        └──submit──▶ pending_review
```

The whole table lives in one place, `domain.transitions`. A table rather than
`if`s spread across the use cases: the set of legal moves is a product decision,
and the only way to keep it reviewable is to keep it somewhere an admin route
cannot quietly invent a new one. `TestTheApprovalWorkflowAllowsExactlyTheseMoves`
pins every cell.

Two properties worth naming:

- **Rejection is not terminal.** The usual cause is an unreadable licence photo,
  and the owner can fix that in a minute — if we tell them why. The reason is
  mandatory, and the owner reads it.
- **Suspension is distinct from rejection** precisely so a reinstatement does not
  need a fresh review.

Details are **frozen while a shop is under review**. An admin should be deciding
on the set of details they were shown; an owner who edits mid-review either
invalidates the decision or gets approved for a shop nobody checked. Opening
hours and holiday mode are deliberately *outside* that freeze — a shop waiting on
an admin still has to be able to say it is shut for Eid.

## Documents

Required documents are per shop type:

| Type | Documents |
|---|---|
| restaurant | trade licence, national ID, food licence |
| grocery | trade licence, national ID |
| pharmacy | trade licence, national ID, **drug licence** |

Per type rather than one list for everyone: asking a grocer for a drug licence
they cannot have blocks a perfectly legal registration, and not asking a pharmacy
for one makes us the distribution channel for whoever skipped it.

The list is **served**, at `GET /v1/merchants/registration-requirements`, not
hard-coded in the app. A Flutter build that decided for itself which licences a
pharmacy needs would be a second copy of a legal requirement, updated on a
different schedule (2.9).

Uploading a document a shop does not need is **refused**, not stored and ignored:
an owner who uploads a drug licence to a grocery has misunderstood something, and
accepting it silently leaves them waiting for an approval that was never blocked
on it.

Re-uploading replaces the previous document of that kind. Keeping both would
leave a reviewer choosing between two numbers with nothing to say which is
current. A save writes the *whole* document set and prunes what is absent, so a
row left behind from before a type change is never counted as supplied.

## Why geo gained `ResolveDivision`

This is the one place where D1 forced a change outside the module.

`ResolveArea` requires both a division and a mapped area — the right rule for a
delivery address, because config resolution, pricing and dispatch all key off the
area, so an address we cannot place that precisely is an order that reaches
checkout and cannot be priced.

Applying the same rule to registration broke D1. A shop in a rural upazila we
have not drawn an area for sits inside a division and outside every area, and was
being refused — which makes whether a merchant may join depend on how finely we
have mapped their district. The E2E test
`TestAShopInARuralUpazilaRegistersOnTheSameTerms` is the one that caught it.

So geo now offers both, with the difference stated on the contract:

- `ResolveArea` — division **and** area required. Addresses.
- `ResolveDivision` — division required, area best-effort. Registration.

The division is never optional either way: it is the D3 ceiling, and a point in
no division is not in Bangladesh. A shop there would get a row no search could
ever return, which looks to its owner like a successful registration and an empty
order book.

## Who writes what

Geo owns the spatial index over merchant locations; merchant owns the merchant
record. The merchant module publishes through `GeoContract.PlaceMerchant` rather
than inserting into `geo_merchant_locations`, which keeps the rule that a module
never touches another's storage — and keeps the division code derived by the
module that owns the boundaries. A division code supplied by a caller is a
division code that can be wrong, and the division decides who can ever see the
shop.

Ordering is deliberate in both directions:

- **Registration** stores the record, then publishes. The reverse would leave a
  point in the index with no shop behind it — a result discovery would return and
  nothing could resolve.
- **Withdrawal** removes from the index, then deletes the record. Same reason,
  mirrored: at worst this leaves a withdrawn shop the owner can delete again.

A moderation decision writes the status and the index flag together, and a failed
index update is **reported**, not logged and swallowed. The two may disagree for
the moment between the writes; what must not happen is that they disagree
silently and permanently.

## Withdrawal

An owner may withdraw a shop from `draft` or `rejected` only. An approved shop is
withdrawn by suspending it, which is an admin's decision and leaves a record:
orders, reviews and payouts point at a merchant id, and letting an owner delete
one because a customer complained would take the evidence with it.

## Opening hours

Minutes from local midnight, not `time.Time`: an opening hour is a recurring
rule, not an instant, and storing an instant ties the schedule to the date it was
entered on.

- **One timezone**, a fixed `+06`. Bangladesh has had no daylight saving since the
  2009 experiment was abandoned, and a fixed zone means opening hours do not
  depend on whether the container image ships tzdata. If the country adopts DST
  again, `domain.bangladesh` is the single line to change.
- **Windows are half-open**, `[open, close)`, so two adjacent windows never both
  claim the same minute.
- **Windows may not wrap past midnight.** A restaurant trading until 2am enters
  `22:00-24:00` and, the next day, `00:00-02:00`. A wrapping window makes every
  "is it open" and "when does it close" answer ambiguous for the hour either side
  of midnight.
- **At most three windows a day** — breakfast, lunch and dinner. More is a
  data-entry accident.
- **A schedule that never opens is refused.** It makes a shop permanently
  invisible in a way that looks like a bug to its owner. Closing indefinitely is
  what holiday mode is for, and it says so on the listing.

A new shop gets 09:00–22:00 every day. A shop registered with no hours would be
approved and still invisible, with nothing to tell its owner why.

Storage is JSONB keyed by weekday number, Sunday as `"0"`. Read as a whole and
written as a whole every time, so seven joins to answer "are you open" is a cost
paid on every listing for nothing. The encode/decode pair lives in the domain
because the schedule is a value object with an invariant — sorted,
non-overlapping — and letting a decoder build one field by field would let it
build an invalid one. Decoding re-checks every invariant, so a row written before
a rule tightened fails loudly rather than becoming an invalid schedule in memory.

## The contract, and what it leaves out

`MerchantContract` carries what discovery, catalogue and order need: identity,
type, the public phone, the placement, and the three visibility booleans plus a
preformatted `open_status` string.

It deliberately does **not** carry documents, the owner's contact details, the
review note or the status history. Those exist for the owner and for an admin. A
contract that handed a trade licence number to discovery would put it one
careless handler away from a customer's screen.

`Listed` preserves the order it was given. The caller is discovery, handing over
the result of a nearest-first radius search; re-sorting here would throw away the
ordering the query paid for, and a caller that had to re-sort would need the
distances this contract deliberately does not carry. A missing id is skipped
rather than failing the batch: discovery holds ids from a spatial index that can
be a moment behind a withdrawal, and failing the page would turn one stale row
into an empty search result.

## Thin client

`open_status` is composed by the server — "Open until 22:00", "খোলা আছে — বন্ধ হবে
22:00", "On holiday" — and rendered verbatim. The client never decides for itself
whether a shop is open by reading the hours. Two clients doing that arithmetic
will eventually disagree, and the customer comparing two screens sees two
answers.

It is Bengali-first, because the audience is (1.4). `?lang=en` opts out; anything
else gets Bengali.

The address is likewise composed server-side into `single_line`, and the phone is
normalised to one stored spelling (`+8801712345678`) whatever the owner typed.

## Roles

Registration is open to **any signed-in account**. The merchant role is something
an approved owner gets, not a prerequisite for applying — requiring it would mean
an account had to become a merchant before it could ask to be one.

The owner routes act on the shop the token owns; there is no merchant id for a
caller to substitute. The approval queue sits behind the admin role, because it
carries trade licence numbers and the owner's NID.

## Audit

Every status change writes a `merchant_status_events` row with who decided and
why. An approval with no name attached is not an audit trail.

The history is recorded separately from `Save` rather than derived from it:
deriving it from writes would record the changes that happened to go through
`Save` rather than the decisions somebody made. Statuses are read back as
written, not re-parsed — history is a record of what happened, and a status we
have since retired must stay readable rather than failing the whole page.
