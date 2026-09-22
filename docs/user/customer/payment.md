# Payment — customer

`PaymentScreen` · `frontend/customer/lib/src/screens/payment_screen.dart`
· no Figma frame for this state (`docs/design-gaps.md`)

## Purpose

To pay for an order placed with **pay online**, and to see where that payment
has got to.

## How you get here

From **pay now** on [the order screen](order.md), which appears only while an
online order is still waiting for payment.

## What you see

* The amount, as the server formatted it.
* The payment's status and the server's own line for it — pending, captured,
  failed or refunded.
* **Check payment**, to re-read it.
* A payment link, *only* if the gateway supplied one.

**Why there is usually no link.** The one gateway this product ships is the
manual one. It records the attempt and returns no `redirect_url` — a human
confirms the payment through the webhook. So the screen shows the state and a
way to re-read it, and shows a link only when a gateway actually gives one. It
launches nothing: there is no browser plugin in this build, and inventing a
redirect for a gateway that did not give one sends the customer to a blank
page.

Until a real gateway adapter exists, this screen honestly reports a pending
payment rather than pretending to collect money. `docs/runbook.md` §0 treats
that as a release blocker.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| opening the screen | `POST /v1/payments/checkout` | — |
| **check payment** | `GET /v1/payments/{orderId}` | — |
| back, after it is captured | — | [the order](order.md), now placed |

**Starting a checkout twice is safe.** `POST /v1/payments/checkout` resumes
the attempt the order already has rather than starting a second one, which is
what makes a customer who closed the page and came back an ordinary case
rather than a double charge.

## States

| State | What you see |
|---|---|
| starting the checkout | a spinner |
| pending | the status line, and "no payment page was provided" where a link would be |
| captured | the captured line. The order has left `pending_payment` and the shop can start |
| failed | the server's reason. Retrying starts a fresh attempt — a previous failure does not block one |
| refunded | the refunded line, with the reason |

## When something goes wrong

* **The order is not waiting for payment** — a cash order, or one already
  paid, answers `409` and its sentence is shown.
* **Not your order** — `404`, the same answer an order id that never existed
  gets.
* **The gateway could not be reached** — `503` with the server's message, and
  the check button to try again.
* **No connection** — the offline message. Nothing is queued: a payment
  replayed later is the one action that must never be guessed at.
