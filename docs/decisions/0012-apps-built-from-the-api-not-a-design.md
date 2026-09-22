# 0012 — The merchant and partner apps are built from the API, not from a design

Date: 2026-09-19
Status: Accepted

## Context
The binding build instruction says the agent does not invent a screen that is
not in Figma: it records the gap in `docs/design-gaps.md` and continues against
a documented placeholder.

P19's job was the merchant and partner apps. The Figma file
(`NlVjn8OuvmLjbm8z8TDVlR`) was searched exhaustively — 10,456 named nodes — and
**it contains no merchant screens and no partner screens at all.** Not
incomplete ones: none. The file is a customer app.

So the instruction's rule, taken literally, would mean not building two of the
three apps.

## Options considered
1. **Do not build them.** Faithful to the letter, and leaves the product with
   no way for a shop to accept an order or a rider to mark a delivery — which
   makes the customer app a demo rather than a product.
2. **Invent the visual design.** Two apps' worth of new layout, new
   components, new colours, chosen by an agent with no brief. Every one of
   those choices would be wrong in a way nobody could check, and the design
   review would be a rewrite.
3. **Take the layout from the API and the look from `goklay_core`.**

## Decision
Option 3, recorded in `docs/design-gaps.md` before either app was started.

Concretely:

* **The layout of every merchant and partner screen comes from the endpoint
  behind it.** The order board's buttons are `next_actions`. The catalogue
  screen's shape is the capabilities response. The item form's fields are the
  capabilities response. The feed's empty state is `reason` and `notice`. Not
  one of those is a layout decision an agent made — each is a rendering of
  what the server said, which is what ADR 0004 asks of a screen anyway.
* **The look comes entirely from `goklay_core`**, whose tokens, theme,
  typography, spacing and components *were* extracted from the Figma file in
  P17. So these apps are drawn in the design's own visual language even though
  the design never drew them.
* **Nothing visual is invented.** No new component, no new colour, no new
  spacing value. If a screen needs something `goklay_core` does not have, that
  is a gap to record rather than a thing to design.

## Consequences
- **The product works end to end**, and the whole-product E2E regression can
  walk an order from a shop's registration to a customer's review.
- **A designer picking this up later has a working app to react to**, with
  every screen's layout traceable to a response rather than to taste. That is
  a much better starting point than an empty Figma page, and a much more
  honest one than a design an agent guessed at.
- **Accepted cost: these two apps will not match a future design.** When the
  merchant and partner screens are drawn, the layouts here will change. What
  will not change is the data each screen needs, because that came from the
  API.
- **`docs/design-gaps.md` is now load-bearing.** It records this decision, the
  eight customer screens the design draws with no backend behind them, the four
  backend surfaces the design never drew, and the three platform capabilities
  deliberately left absent (no map, no file picker, no background location).
  Anyone who thinks something is missing should read it before concluding
  anything was forgotten.
