# Support — customer

`SupportScreen` · `frontend/customer/lib/src/screens/support_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To raise a complaint about an order, and to see what happened to the ones
already raised.

## How you get here

* From **get help** on [the order screen](order.md) — which arrives with that
  order attached, so the form is ready.
* From **support** on [the account screen](account.md) — which shows the list
  only, with no form.

## What you see

* When an order is attached: a **subject** field and **raise a ticket**.
* The customer's own tickets, `GET /v1/me/support/tickets`: the subject, the
  order it is about, when it was raised, and its state.
* A resolved ticket says whether it was **refunded** or **rejected**, with the
  agent's note.

**A ticket is always against an order**, which is why the form appears with
one in hand rather than as a free-standing contact page. P16 checks the caller
was actually on that order.

**A resolved ticket says whether it was refunded, and stops there.** *What*
was refunded is the payment's business, and the payment endpoint states it. An
amount repeated here would be a second figure to disagree with the first.

You do not have to wait for a delivery to complain: "this never arrived" is
exactly the ticket a customer needs to raise about an order still out for
delivery, and the server requires no particular status.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| type a subject | the button becomes live when it is not empty | — |
| **raise a ticket** | `POST /v1/support/tickets` | stays here; the list reloads with the new ticket at the top |
| back | — | wherever you came from |

## States

| State | What you see |
|---|---|
| loading | a spinner |
| tickets | the list, newest first |
| none | "no tickets" |
| open | the "open" label |
| resolved | "resolved", the resolution, and the note |
| submitting | the button is busy |

## When something goes wrong

* **Not your order** — `403` with the server's sentence.
* **No subject** — refused; the button is disabled until there is one, so this
  needs the field to be emptied after typing.
* **The read failed** — error view with a retry; the form stays usable.
* **No connection** — the offline message. A ticket is not queued: it would
  arrive with no way to tell the customer it had.
