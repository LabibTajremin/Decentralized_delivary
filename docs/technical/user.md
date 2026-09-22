# User profile and addresses

What a person tells us about themselves, and where they want deliveries taken.

## Why this is separate from identity

Identity (P04) owns authentication and keeps only `(id, phone, role)`. Everything
else lives here.

The split is not tidiness. A phone number is a credential — the rate-limit key,
the OTP key, the account identity — and a home address is the thing a stalker
wants. Keeping them in separate tables owned by separate modules means a bug or
a breach that reaches one does not hand over the other, and the two can be
pulled apart into separate services later without untangling a shared row.

`ports.UserDirectory` in identity is the seam: identity creates an account on
first sign-in and knows nothing further about the person.

## An addition to Appendix A

The contract registry lists `user` only as a *consumer* of geo, with no contract
of its own. It needs one: order must know where to deliver, and dispatch must
know who to hand the parcel to.

The alternative — letting order read the user tables — breaks the rule that a
module never reaches into another's storage, and would make extracting either
one later a rewrite rather than a deployment change. So `UserContract` exists,
noted here as a deliberate addition rather than an oversight.

## Addresses are placed before they are saved

This is the phase's acceptance criterion, and the reason is entirely downstream.
Config resolves per area (`area → district → division → global`), pricing bands
off the area, and dispatch scopes off it. An address with no area is an order
that reaches checkout and cannot be priced.

So an address that geo cannot place is refused at save time, with geo's own
wording — one message rather than two for the same situation.

The placement is **stored, not recomputed**. Re-deriving it per read would put a
spatial query on the hot path of every order for a value that changes only when
the pin moves.

It is re-resolved on **every edit**, not only when the pin changes. A customer
correcting a street name has usually nudged the pin too, and an address whose
stored area no longer matches its location prices the wrong zone — silently, and
in the customer's favour or ours at random.

## The map pin

Validated in the domain rather than deferred to geo: an address stored with a
bad pin becomes a delivery that fails at dispatch, hours later, in front of
somebody who cannot fix it.

`(0, 0)` is refused specifically. It is in the Gulf of Guinea and is almost
always an uninitialised variable rather than a place someone lives.

## Exactly one default

A customer who has any addresses has exactly one default, always.

The rule lives in three places, deliberately:

1. **`domain.ChooseDefault`** decides who inherits. It is a product decision —
   the newest remaining address wins, because someone deleting their default is
   usually moving on from it — and burying that in a trigger is how it gets
   answered differently by the next person to touch the schema.
2. **The repository writes the whole list in one transaction.** "Exactly one
   default" is a property of the set, not of a row. Writing rows individually
   leaves a window where two are default or none is, and a concurrent read in
   that window returns a wrong delivery address.
3. **A partial unique index** enforces it in the database. Declarative, and
   unlike a trigger it cannot be disabled by somebody doing a bulk fix.

The transaction clears the flag before setting it, because the index would
otherwise reject the intermediate state.

The first address a customer saves becomes their default whether they asked or
not: a customer with one address and no default gets asked to choose between one
option at checkout.

## The composed address line

`single_line` is built by the server. Every surface that shows an address — the
app, a receipt, the rider's screen, an SMS — shows the same string, because a
client that joins the parts itself will eventually join them differently, and a
customer comparing two of them should not see two addresses (2.9).

It is composed once, in `application.ToContractAddress`, and both transport and
the contract go through it. The address a consuming module receives and the
address the app renders are therefore assembled by the same code and cannot
drift apart.

## Partial updates

Profile update fields are pointers, so "leave this alone" and "clear this" are
different requests. Without that distinction a client updating only the name
would have to send the email back, and one that forgot would silently erase it.

## The caller's own data, always

Every route acts on the signed-in user. The user id comes from the verified
token and never from the request — an endpoint that takes one from the client is
an endpoint that will be asked for somebody else's home address.

The repository carries the same rule: the user id is in the `WHERE` clause, not
checked after the fact. A query that *can* return another user's address is one
careless call away from a handler that forgets to compare.

Another customer's address is a **404**, not a 403. A 403 would confirm the
address exists.

## Small decisions worth stating

- **Names are limited in characters, not bytes.** A Bengali name is three bytes
  per character; a byte limit would give it a third of the room an English name
  gets.
- **The display name is never empty.** The server picks the fallback, translated,
  so two clients cannot invent two different greetings.
- **Email validation is deliberately loose.** The only way to know an address
  works is to send to it, and a strict pattern turns away real customers over an
  apostrophe. This is a receipt field, not a login.
- **The address book is capped at twenty, and the cap refuses rather than
  evicting.** An address is something the customer typed; quietly losing one is
  worse than saying the book is full.
- **A missing profile is not an error.** Identity creates the account; the
  customer may never open the profile screen. Returning 404 would make every
  client handle "signed in but no profile" as a special case, and some would
  handle it wrong.
- **A stored language outside the supported set falls back** rather than failing
  the read. Schema drift should not lock someone out of their own profile.

## Testing

- **Unit** — the domain rules and every use-case branch, over an in-memory
  repository that applies the same one-default rule the real transaction does.
- **Integration** — real PostGIS: the pin survives the round trip to within a
  metre, the partial unique index really refuses a second default, and one
  user's address is unreachable from another's account.
- **E2E** — the real binary: an address placed against the seeded Dhanmondi
  geometry, an address in Paris refused, and one customer unable to read, edit
  or delete another's address book.
