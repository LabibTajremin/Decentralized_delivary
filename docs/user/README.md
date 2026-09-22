# User documentation

One file per screen, per role, as §6.2 of the build instruction asks. Each file
says what the screen is for, how you reach it, what is on it, what every
action does and where it takes you, every state it can be in, and what happens
when something fails.

These are written from the screens themselves, not from the design file. Where
the two differ — and they do, in eight places — the difference is recorded in
`docs/design-gaps.md` and repeated in the screen's own file rather than left
for somebody to discover.

## One thing to understand before reading any of it

**The apps render; they do not decide.** Every price, every status line, every
notice, every enabled or disabled button comes from the server already made
(rule 2.9, ADR 0004). So a screen's "states" are almost never states the app
invented: they are what a response said. When a file below says "the button is
live only when `can_submit` is true", that is the whole mechanism — there is no
second rule in the app, and no version of the app that could disagree.

The three consequences that show up on nearly every screen:

* **Money** arrives twice, as a minor-unit integer and as a string already
  formatted for the reader. The app prints the string. It never adds, rounds
  or formats an amount.
* **Language** is Bengali unless the reader chose English, and the choice goes
  to the server on every request as `?lang=en`. Changing it in the app alone
  would leave a Bengali order status under an English heading.
* **Errors** show the server's own sentence whenever it sent one, and a generic
  line only when it did not. So "when something goes wrong" below describes
  *where* the message appears, not what it says.

## Layout

| Folder | Whose screens |
|---|---|
| [`shared/`](shared/) | the four screens all three apps have: splash, sign-in, the code, and the result |
| [`customer/`](customer/) | 24 screens |
| [`merchant/`](merchant/) | 11 screens |
| [`partner/`](partner/) | 7 screens |

## Shared

| Screen | What it is |
|---|---|
| [Splash](shared/splash.md) | the first frame, held while stored tokens are read |
| [Sign in with a mobile number](shared/phone-sign-in.md) | the one sign-in method this product has |
| [Enter the code](shared/otp.md) | the six digits, and the resend countdown |
| [Verified / could not verify](shared/verification-result.md) | one screen, told which it is |

## Customer

| Screen | What it is |
|---|---|
| [Onboarding](customer/onboarding.md) | the three pages before sign-in |
| [Sign-in options](customer/sign-in-options.md) | continue with a phone number |
| [Main shell](customer/main-shell.md) | the four tabs a signed-in customer lives in |
| [Home](customer/home.md) | the shops near the delivery address |
| [Shop](customer/shop.md) | one shop's menu |
| [Item](customer/item.md) | one item, its options, and add-to-cart |
| [Shop reviews](customer/shop-reviews.md) | what other customers said |
| [Cart](customer/cart.md) | what is in the cart, and the bill so far |
| [Review cart](customer/review-cart.md) | choosing where it goes |
| [Place order](customer/place-order.md) | confirming and paying |
| [Order](customer/order.md) | one order: timeline, receipt, what is still possible |
| [Orders](customer/orders.md) | current and past orders |
| [Tracking](customer/tracking.md) | the live delivery stream |
| [Payment](customer/payment.md) | paying for an order placed online |
| [Review](customer/review.md) | rating the shop and the rider |
| [Support](customer/support.md) | tickets, and raising one |
| [Account](customer/account.md) | everything about the customer, not the order |
| [Profile](customer/profile.md) | name and email |
| [Addresses](customer/addresses.md) | the address book |
| [Add address](customer/add-address.md) | saving a delivery address |
| [Security](customer/security.md) | the device list and sign out everywhere |
| [Language](customer/language.md) | Bengali or English |
| [Notifications](customer/notifications.md) | the notification history |
| [Not available yet](customer/placeholder.md) | the eight screens the design draws and this backend has nothing behind |

## Merchant

| Screen | What it is |
|---|---|
| [Register / edit shop](merchant/shop-details.md) | one form for both |
| [Shop](merchant/shop.md) | the shop's record and its approval state |
| [Documents](merchant/document.md) | adding a required paper |
| [Hours](merchant/hours.md) | opening hours, one field per weekday |
| [Holiday](merchant/holiday.md) | closing temporarily, and reopening |
| [Main shell](merchant/main-shell.md) | the three tabs an approved shop lives in |
| [Board](merchant/board.md) | the order board |
| [Order](merchant/order.md) | one order, in kitchen detail |
| [Catalogue](merchant/catalogue.md) | sections, items and bundles |
| [Add item](merchant/item-form.md) | the form, shaped by the shop's type |
| [Account](merchant/account.md) | the owner, as distinct from the shop |

## Partner

| Screen | What it is |
|---|---|
| [Register](partner/register.md) | signing up to carry orders |
| [Main shell](partner/main-shell.md) | four tabs, under the outbox banner |
| [Feed](partner/feed.md) | what to look at right now |
| [Job](partner/job.md) | one delivery — and the screen the offline queue exists for |
| [Jobs](partner/jobs.md) | the rider's own deliveries |
| [Cash](partner/cash.md) | the cash-on-delivery ledger |
| [Account](partner/account.md) | distance choice, position, and the way out |
