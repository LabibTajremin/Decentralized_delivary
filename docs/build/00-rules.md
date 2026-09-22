# Build rules — always loaded

> Split from `CLAUDE_CODE_BUILD_INSTRUCTION.md` v2.0, which remains the single
> authority. These files are its parts, unchanged in meaning.

# Decentralized Delivery Platform — Claude Code Build Instruction

**Version:** 2.0 · September 2026
**Owner:** Rootlogic Lab
**Backend:** Go (payment module isolated for later .NET migration)
**Frontend:** Flutter (cross-platform)
**Design source:** Figma `NlVjn8OuvmLjbm8z8TDVlR`
**Design documentation:** GoKlay Design Audit

This document is the single authoritative instruction for the AI agent building this system. Every rule here is binding. Where this document and any other source disagree, this document wins.

---

# PART 0 — AGENT OPERATING PROTOCOL

Read Part 0 at the start of every session. Never load the entire document in one session.

## 0.1 Session rules

**R1 — Load at most four files per session:** `00-rules.md` (this part), `STATE.md`, `05-architecture.md`, and the one phase file being executed.

**R2 — Define once, reference by ID.** Screens, components, modules, config keys and algorithms all have stable IDs. After definition, refer to them by ID only. Never re-describe something that already exists.

**R3 — Output goes to disk, never to chat.** Report only: file paths, IDs, test results, commit SHA. Never echo file contents back.

**R4 — `STATE.md` is the only memory.** A fresh session with zero conversation history must read `STATE.md` and know exactly what is done, what is in progress, and what is next.

**R5 — One phase per branch. One task at a time.** Never work two phases in parallel.

**R6 — Never mark a task done until its tests pass and coverage holds at 100%.**

## 0.2 STATE.md format

Maintained at repo root. Updated after every task, not every phase.

```markdown
# BUILD STATE
last_updated: 2026-09-13T14:22:00Z
current_phase: P04
current_task: P04.T03
current_branch: phase/04-merchant-catalogue
status: IN_PROGRESS
blocked: false
blocker_reason: ""

## Phase status
P00 DONE   commit a1b2c3d  PR #1  merged
P01 DONE   commit e4f5g6h  PR #2  merged
P02 DONE   commit i7j8k9l  PR #3  merged
P03 DONE   commit m1n2o3p  PR #4  merged
P04 IN_PROGRESS
P05..P19 TODO

## Current phase tasks
P04.T01 DONE  merchant domain entities
P04.T02 DONE  merchant repository interfaces
P04.T03 IN_PROGRESS  catalogue use cases
P04.T04 TODO  merchant HTTP handlers
P04.T05 TODO  100% coverage verification

## Coverage
backend total: 100.0%
last verified: 2026-09-13T14:10:00Z

## Notes for next session
<anything the next session must know>
```

## 0.3 Resume protocol

Every session begins with exactly this sequence:

```bash
cat STATE.md
git status
git branch --show-current
cd backend/tests && go test ./... -coverpkg=../... -cover
```

Then:
- `status: IN_PROGRESS` → continue from `current_task`
- `status: PHASE_COMPLETE` → open the next phase file, create its branch, begin
- `blocked: true` → report the blocker and stop. Do not attempt a workaround.

## 0.4 Token-reset restart

**Claude Code cannot start itself.** There is no built-in trigger that fires when a quota window resets. What this document provides instead is a *resumable* build: `STATE.md` plus the resume protocol means any new session picks up mid-task with no context loss.

To make restart automatic, the operator schedules it externally. Example for Linux/macOS, attempting a resume every hour:

```bash
# crontab -e
0 * * * * cd /path/to/repo && /usr/local/bin/claude -p "Read STATE.md and follow docs/build/00-rules.md section 0.3. Resume the build." >> build.log 2>&1
```

The agent must make this safe by ensuring every session is **idempotent**: re-running a completed task must detect completion from `STATE.md` and exit without duplicating work.

## 0.5 Stop condition

The agent stops when, and only when, all of the following are true:

1. Every phase P00–P19 is `DONE` in `STATE.md`.
2. Every PR is merged with all checks green.
3. Backend coverage is 100.0% against the declared exclusion list.
4. Flutter coverage is 100.0% against the declared exclusion list.
5. The full E2E suite passes.
6. `main` builds clean.

Then write a final summary to `BUILD_COMPLETE.md` and stop. Do not continue to optional work.

---

# PART 3 — TESTING

## 3.1 Tests live in a separate project

**No test file inside the main backend project.** All tests live in `backend/tests/`, which is its own Go module and can be run alone.

```
go.work
├── backend/          module github.com/rootlogic-lab/delivery/backend
└── backend/tests/    module github.com/rootlogic-lab/delivery/backend/tests
```

`backend/tests/go.mod` requires the backend module via a `replace` directive pointing at `../`.

Run standalone:

```bash
cd backend/tests
go test ./... -coverpkg=github.com/rootlogic-lab/delivery/backend/... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1
```

## 3.2 The unexported-code problem — read before Phase 0

This requirement has a real technical consequence the agent must design around rather than discover halfway through.

Go's `-coverpkg` lets an external test module measure coverage of the backend. But an external module **can only call exported identifiers**. Unexported functions are unreachable, so they can never reach 100%.

**Resolution, in priority order:**

1. **Design so there is nothing unexported worth testing separately.** Under clean architecture, every meaningful behaviour sits behind an exported use case or an exported domain method. Unexported helpers should be trivial enough that testing the exported caller covers them fully. This is the intended path and it aligns with the architecture.
2. Where a genuinely complex algorithm must stay unexported, **promote it to its own exported package** under `internal/` — still not importable outside the module boundary in production, but reachable by the test module through the workspace.
3. **Never** add `export_test.go` inside the backend project. That violates the no-test-files rule.

If a piece of logic cannot be covered by rules 1 or 2, that is a design smell. Refactor it rather than lowering the coverage gate.

## 3.3 Coverage gate: 100%

CI fails below 100.0%. Exclusions must be **explicitly listed** in `backend/tests/coverage-exclusions.txt`, with a one-line justification each. Permitted exclusions only:

- Generated code (protobuf, sqlc, mocks)
- `cmd/api/main.go` wiring, covered instead by E2E
- Vendored code

Nothing else. A new exclusion requires an ADR.

## 3.4 Test types

| Type | Location | Scope |
|---|---|---|
| Unit | `tests/unit/` | Domain rules, use cases with faked ports, algorithms |
| Contract | `tests/unit/contracts/` | Every `external/` boundary service against a fake |
| Integration | `tests/integration/` | Repositories against a real database in Docker |
| E2E | `tests/e2e/` | Full API flows against a running stack |

**Every algorithm in the ALG table gets dedicated tests including edge cases**: empty result set, division boundary crossing, maximum radius reached, zero available partners, concurrent assignment of the same order.

## 3.5 Flutter testing

Mirrors the same standard. Unit tests for logic, widget tests for every screen, integration tests for every user flow. 100% coverage gate via `flutter test --coverage` with an equivalent exclusions file for generated code.

---

# PART 4 — GIT WORKFLOW

The agent uses the `git` and `gh` CLIs directly. Every phase follows this cycle without deviation.

## 4.1 Phase cycle

```bash
# 1. Start from updated main
git checkout main
git pull origin main

# 2. Branch for the phase
git checkout -b phase/04-merchant-catalogue

# 3. Work. Commit per task, not per phase.
git add <specific files>
git commit -m "feat(merchant): add catalogue use cases

- Implement CreateCategory, UpdateItem, SetAvailability
- Add MerchantCatalogueRepository port
- Unit tests, coverage 100%

Phase: P04 Task: P04.T03"

# 4. Before opening a PR, verify locally
cd backend/tests && go test ./... -coverpkg=../... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1     # must read 100.0%
cd ../.. && go vet ./... && golangci-lint run

# 5. Push
git push -u origin phase/04-merchant-catalogue

# 6. Open the PR
gh pr create \
  --title "Phase 04 — Merchant registration and catalogue" \
  --body-file .github/pr-body.md

# 7. Wait for checks. ALL must be green.
gh pr checks --watch

# 8. Merge only when green
gh pr merge --squash --delete-branch

# 9. Update STATE.md, commit it to main
git checkout main && git pull origin main
```

## 4.2 Rules

- **A PR is never merged with a failing or pending check.** If a check fails, fix it and push again. Never merge with `--admin`, never bypass.
- Commit messages follow Conventional Commits and always end with the `Phase:` and `Task:` trailers.
- Commit and push **after finishing every phase**, and after every task within a phase.
- One phase per PR. Never combine phases.
- If a phase is abandoned or reworked, record why in `docs/decisions/`.

## 4.3 Required CI checks

`.github/workflows/ci.yml` must run, and all must pass:

1. `go build ./...`
2. `go vet ./...`
3. `golangci-lint run`
4. Architecture lint — layer dependency rules from 2.2
5. `go test` from `backend/tests` with coverage
6. Coverage gate at 100.0%
7. Migration up and down
8. `flutter analyze`
9. Thin-client lint — no money or distance arithmetic, no business thresholds in `frontend/` (2.9)
9b. APK size budget — fail if release APK grows >10% in one PR, or exceeds 20 MB
10. `flutter test --coverage` with gate
11. E2E suite

---

# PART 7 — DEFINITION OF DONE

**A task is done** when code is written, its tests pass, coverage holds at 100%, its documentation exists, and it is committed with a proper message.

**A phase is done** when every task is done, the full suite passes, the architecture lint passes, its PR is green and merged, and `STATE.md` is updated.

**The build is done** per section 0.5.

---
