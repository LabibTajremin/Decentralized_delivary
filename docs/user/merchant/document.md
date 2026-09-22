# Documents — merchant

`DocumentScreen` · `frontend/merchant/lib/src/screens/document_screen.dart`
· no Figma frame (`docs/design-gaps.md`)

## Purpose

To file one of the documents this shop's type requires.

## How you get here

From **add a document** on [the shop screen](shop.md).

## What you see

* A picker of document **kinds** — and it offers only
  `merchant.missing_documents`, the server's list of what is still wanted,
  rather than a local subtraction of uploaded from required.
* **Document number.**
* **File location.**
* **Add.**

**There is no file picker.** This build has no camera or storage plugin, so
the form takes the location of a file that has already been uploaded.
Recorded in `docs/design-gaps.md` with the rest of the platform work P19 did
not invent.

## Actions, and where they go

| Action | What happens | Where you go |
|---|---|---|
| choose a kind | — | — |
| **add** | `PUT /v1/merchants/me/documents` | back to [the shop screen](shop.md), reloaded — the kind just filed is gone from "still needed" |

The call is a `PUT` per kind, so re-filing a document replaces it rather than
adding a second row of the same type.

## States

| State | What you see |
|---|---|
| nothing chosen, or a field empty | the button is disabled |
| complete | live |
| adding | busy |
| refused | the server's message, with the fields kept |

## When something goes wrong

* **That kind is not wanted** — refused. It should not be offerable, so this
  means the list changed under you; going back and returning refreshes it.
* **The shop is past the point where documents can change** — refused with the
  reason.
* **No connection** — the offline message.
