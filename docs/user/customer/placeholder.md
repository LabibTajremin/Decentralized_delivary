# Not available yet — customer

`PlaceholderScreenView` ·
`frontend/customer/lib/src/screens/placeholder_screen.dart`
· Figma: Offers, Offer Popup, Promos, Add Promos, Apply Voucher, Referral,
Business Profile, Digital Payment, Payment Methods, GoKlay Pay, Safety,
Permissions

## Purpose

To say, honestly and in one line, that a screen the design draws has nothing
behind it yet.

## How you get here

From the rows on [the account screen](account.md): Offers, Promos, Referral,
Payment methods, GoKlay Safety, Permissions.

## What you see

The screen's title, and a sentence: "this is not available yet." In Bengali,
or English if that is the chosen language.

**Why a blank rather than something plausible.** A wallet balance, a made-up
referral code or an offer nobody will honour is worse than nothing, because
somebody eventually believes it and complains when it turns out not to be
real. `docs/design-gaps.md` lists every one of these screens with the reason
the backend has nothing for it.

**Two of them have something true to add, so they add it:**

* **Payment methods** states the two methods P13 really has — cash on
  delivery, and pay online — instead of an empty card list.
* **Permissions** says what the app will ask the handset for and why, which is
  true regardless of whether there is an endpoint behind it.

## Actions, and where they go

| Action | Where you go |
|---|---|
| back | [the account screen](account.md) |

Nothing else. There is deliberately no "notify me", because nothing would.

## States

One per variant, and each one is the same shape: a title and a sentence.

## When something goes wrong

Nothing can. No request is made — which is the point.
