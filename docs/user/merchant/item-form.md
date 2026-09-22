# Add an item — merchant

`ItemFormScreen` · `frontend/merchant/lib/src/screens/item_form_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To add an item to the catalogue, with exactly the fields this kind of shop
has.

## How you get here

From **add an item** on [the catalogue screen](catalogue.md), which hands it
the shop's capabilities and its list of sections.

## What you see

* **Section** — chosen from the shop's own sections.
* **Name**, **description**.
* **Price, in poisha.**
* Then, only where the capabilities say so: **unit of sale**, **pack size**,
  **brand**, **requires a prescription**.
* **Add.**

**Which fields appear is the capabilities response, not the shop's type.** A
unit picker only where `requires_unit`; a prescription switch only where
`prescriptions`. A field this kind of shop does not have is *refused* by the
server rather than dropped, so an owner never silently loses something they
typed — which is why the form shows only what will be accepted.

**The price is typed in minor units, and that is deliberate.** Money crosses
this wire as an integer of poisha and nothing else. A decimal field would mean
parsing one here — an arithmetic step, on money, in the client, which rule 2.9
forbids. Everything the owner reads back afterwards is the server's formatted
string.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| **add** | `POST .../catalogue/items` | back to [the catalogue](catalogue.md), reloaded with the new item |

## States

| State | What you see |
|---|---|
| a required field empty | the button is disabled |
| complete | live |
| adding | busy |
| refused | the server's message above the button, with everything typed kept |

## When something goes wrong

* **The price is not a whole number** — refused. It is poisha, and half a
  poisha does not exist.
* **A field this shop does not have** — refused; the form should not have
  offered it, so this means the capabilities changed under you.
* **No such section** — refused; go back and reload.
* **No connection** — the offline message; nothing typed is lost.
