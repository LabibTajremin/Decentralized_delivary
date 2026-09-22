# 0011 — String tables written by hand, Bengali first

Date: 2026-09-17
Status: Accepted

## Context
The product is Bengali-first: Bengali unless the user explicitly chooses
English, and an unrecognised device locale resolves to Bengali rather than to
English.

The usual Flutter answer is ARB files and `flutter gen-l10n`. That pipeline
earns its keep across hundreds of strings and several translators.

This app has about thirty. Not because the product is small, but because
**almost nothing the user reads is a client string**: every price, status line,
notice, blocker, label on a receipt row and explanation of a rule is composed
by the server (ADR 0004). What is left for the client is the handful of words
that have to exist *before* a server answers, or when one never does — a retry
button, an offline banner, the fallback for an error carrying no message, and
the sign-in flow, which happens before there is a session to compose anything
for.

## Options considered
1. **ARB files and `gen-l10n`.** The ecosystem default, tooling for
   translators, and a generated file that the 100% coverage gate would then
   have to be taught to ignore.
2. **An abstract `GoklayStrings` class with one implementation per language,
   written by hand.**

## Decision
Option 2. `GoklayStrings` is abstract; `GoklayStringsBn` and `GoklayStringsEn`
implement it; a `LocalizationsDelegate` installs the right one. Each app adds
its own table the same way for its own screens.

Adding a string to one of these tables is deliberately a decision worth a
second look: **if it describes anything about an order, a price, a shop or a
rule, it belongs in an API response instead.** The tables staying short is not
an accident, it is the rule holding.

Three things fall out of the design and are tested rather than assumed:

* **Bengali is first in `supportedLocales`**, because Flutter resolves an
  unmatched device locale to the first supported one. A phone set to Hindi
  lands on Bengali.
* **`?lang=en` is sent only for English.** The server is Bengali-first too, so
  the parameter would be noise on every URL otherwise. Changing the language
  changes both the widgets and the parameter — a client that changed only its
  own strings would show a Bengali order status under an English heading.
* **Both tables are tested, and so is the script.** A test asserts every
  string is present in both languages, that the two differ (an untranslated
  string is an English string in disguise), and that every Bengali value
  actually contains characters from the Bengali Unicode block. Layout is
  measured at 320dp in Bengali, because Bengali is the longer of the two
  languages.

## Consequences
- **No generated files, so no coverage exclusions** — which is the same reason
  ADR 0008 gives for every other package it declined.
- **A missing string is a compile error**, not a runtime fallback to a key.
- **Accepted cost: no translator pipeline.** Adding a third language means
  writing a third class rather than handing an ARB file to somebody. At thirty
  strings that is an afternoon; if the tables ever reach the hundreds, this is
  the decision to revisit.
- **Accepted cost: `phoneHint` is identical in both tables**, which the "they
  must differ" test has to know about explicitly. It is a number format,
  `01XXXXXXXXX`, and the exception is stated in the test.
