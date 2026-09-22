# Catalogue — merchant

`CatalogueScreen` · `frontend/merchant/lib/src/screens/catalogue_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To manage everything the shop sells: its sections, its items, and its bundles.

## How you get here

The second tab of the [merchant shell](main-shell.md).

## What you see

Tabs — **sections**, **items**, and **bundles** *only where the server says
bundles exist* — over:

* **Sections:** a row each, with **show** / **hide** and a field to add one.
* **Items:** a row each with the server's formatted price, whether it is
  orderable and why not, **show** / **hide**, and — where this kind of shop
  has one — a **shelf count**.
* **Bundles:** the same, where they exist.
* **Add an item.**

**The screen's shape comes from `GET .../catalogue/capabilities`.** A grocery
gets a shelf count; a restaurant does not. A pharmacy gets a prescription
switch; nothing else does. The bundles tab appears only where the server says
bundles exist. **The app never branches on the shop's type itself**, which is
what keeps the per-type schema rules in one place.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **add a section** | `POST .../catalogue/categories` | stays here; the list reloads |
| **show** / **hide** a section | `.../categories/{id}/active` | stays here. Hiding takes its items with it |
| **show** / **hide** an item | `.../items/{id}/active` | stays here |
| set a shelf count | `.../items/{id}/stock` | stays here; whether the item is orderable is then the server's answer |
| **show** / **hide** a bundle | `.../combos/{id}/active` | stays here |
| **add an item** | — | [the item form](item-form.md), which is handed the capabilities |

## States

| State | What you see |
|---|---|
| loading capabilities | a spinner — nothing can be drawn until the shape is known |
| a catalogue | the tabs and rows |
| nothing yet | "nothing in the catalogue yet", with the add buttons |
| hidden | the row is marked hidden; customers do not see it |
| not orderable | the row says so, with the server's reason |
| a change in flight | the rows are not tappable |
| served from cache | the saved-information banner |

## When something goes wrong

* **A section with items cannot be deleted** — refused with the reason;
  hiding it is the way to take it off the menu.
* **A field this kind of shop does not have** — refused rather than silently
  dropped, which is why the screen only ever offers what will be accepted.
* **The read failed** — error view with a retry.
* **No connection** — the cached catalogue under the banner; writes need the
  network, because whether an item is orderable is the server's answer and
  not a local one.
