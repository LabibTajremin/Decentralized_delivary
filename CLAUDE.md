# GoKlay — working agreement

Read this first, then `STATE.md`. Between them they are the whole context: the
conversation that produced the code is gone, and nothing else is remembered.

## The standing order

Build the GoKlay delivery platform, phases **P00 → P20**, from
`docs/build/CLAUDE_CODE_BUILD_INSTRUCTION.md` (v2.0) and the phase files under
`docs/build/phases/`. That document is **binding and authoritative** — where it
and this file disagree, it wins.

The user's instructions, still in force:

- **Do not stop until all 20 phases are finished.** Pausing at a usage limit is
  fine; resume as soon as it resets. "Continue" means: read `STATE.md`, pick up
  `current_task`, keep going.
- **Never merge a pull request, and do not ask for opinions or approval
  mid-phase.** The user merges everything once, at the end. Just do the work.
- **Develop and push only to `claude/goklay-design-system-9z500x`.** Never push
  anywhere else without explicit permission.
- **Do not open a pull request** unless the user asks for one.

## Which model runs which phase

| Phases | Model | Why |
|---|---|---|
| P13 – P16 | **Sonnet 5** | backend modules, built against an established pattern: eleven modules to copy, acceptance criteria that are mechanically checkable, and gates that fail loudly |
| P17 – P20 | **Opus** (the user switches) | the Flutter phases are the least constrained by existing code, and P17 decides a foundation the next three phases are built on — a wrong one there is expensive to undo |

**Hard stop: do not start P17.** When P16 is committed and pushed, run the
handover below, tell the user in one line that P16 is done and P17 needs the
model switched to Opus, and end the turn. Do not begin the Flutter work, do not
scaffold it, do not "just set up the folders". If the user says to carry on with
Sonnet anyway, that is their call — do it, and say once that P17 is the phase
this plan wanted Opus for.

### Handing the build over

The point is that the next session costs almost nothing to start: it reads two
files, not a conversation. Before telling the user to switch:

1. `./scripts/verify.sh` exits 0.
2. Everything is committed and pushed to
   `claude/goklay-design-system-9z500x`; `git status` is clean.
3. `STATE.md` is current: `current_phase`, `current_task`, the phase-status
   table, the finished phase's task list, and anything the tests caught that
   the next session would otherwise rediscover the hard way.
4. `docs/technical/<module>.md` exists for the phase just finished.
5. `./scripts/handover.sh` exits 0 — it checks 1–3 mechanically.

Then the user starts a **new session** on the new model (cheaper than switching
inside one: a switch re-sends the whole conversation, a new session reads
`CLAUDE.md` + `STATE.md` and nothing else) with:

> Read CLAUDE.md and STATE.md, then continue from current_task.

## How a phase goes

One commit per phase, pushed when the phase is done and every gate is green.

1. Read the phase file in `docs/build/phases/`, and `STATE.md` for where the
   last session stopped.
2. Write the tasks into `STATE.md` (`P13.T01 …`) before starting them.
3. Build it: `domain/` → `application/` → `infrastructure/` + `transport/`,
   plus `external/` and `contract/`.
4. Migration, OpenAPI, wiring in `cmd/api/main.go`.
5. Tests: unit (fakes), integration (real Postgres), E2E (the real binary).
6. `./scripts/verify.sh` — every gate green, no exceptions.
7. `docs/technical/<module>.md`, then `STATE.md`, then commit and push.

## The gates — one command

```bash
./scripts/verify.sh
```

It runs every check CI runs, in CI's order, brings the local Postgres and Redis
up first (they do not survive a container restart), keeps going after a failure
so you see all of them, and prints a list at the end. **A phase is not finished
until it exits 0.** Do not commit a phase as done on a partial check, and do not
substitute your own shorter command for it.

`-count=1` is mandatory anywhere you run tests by hand: the architecture guard
tests read files outside their own package, so a cached PASS hides a real
violation. Every script here already passes it.

## Rules that are not negotiable

- **Tests live in a separate Go module** (`backend/tests`), joined by
  `go.work`. Coverage is measured from there against the backend module.
- **Coverage is 100%**, enforced by `scripts/coverage-gate.sh`.
- **Cross-module access goes through the caller's own `external/` package,
  depending only on the target's `contract/`.** Enforced by
  `backend/tests/unit/architecture_test.go`.
- **Thin client (2.9).** Every business rule is in Go. Money crosses the wire as
  a minor-unit integer *and* a preformatted display string; every label,
  notice and status line is composed by the server. Flutter renders, never
  decides.
- **Bengali-first (1.4)**, with `?lang=en` for English. Both languages are
  tested, or half a screen ends up untranslated.
- **API JSON is `snake_case`.**
- **D3, the division ceiling, is the one invariant no setting may disable.**
- **The JWT signing key never lives in the database.**
- Never disable TLS verification or unset `HTTPS_PROXY`.
- The user's email is for git attribution only. Never send it anywhere.

## Practices that have paid for themselves

- **Verify every gate by feeding it a deliberate violation.** A gate nobody has
  seen fail is not known to work.
- **When coverage stalls at 99.x% on a defensive branch that cannot be reached,
  delete the branch** rather than contorting a test to reach it. Say in a
  comment where it would belong if it ever became reachable.
- **Fakes hide real bugs.** Every phase so far has had at least one defect that
  only the integration or E2E test could see — a pointer into a map being
  reallocated, an empty primary key that only collides on the second row, a job
  nothing ever took off the board. Write both.
- E2E tests must not depend on the wall clock: open your own shop 00:00–24:00
  rather than relying on a seeded shop's hours.
- Walk a lifecycle with the parties who really own each step. If a test needs an
  admin to do a shop's job, the test is wrong.

## Commit style

A subject line that says what changed and why it matters, a body that explains
the decisions and anything the tests caught. End every commit with:

```
Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_011sfuvPKfjJTzHEH2rbYhFr
```

(Replace the model name with whichever model is writing the commit.) No model
identifier anywhere else in the repository.
