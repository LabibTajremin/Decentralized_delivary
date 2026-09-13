# Documentation standard

> Split from `CLAUDE_CODE_BUILD_INSTRUCTION.md` v2.0, which remains the single
> authority. These files are its parts, unchanged in meaning.

# PART 6 — DOCUMENTATION

Technical **and** user documentation must both be delivered. Technical decisions must be reflected in `README.md`.

## 6.1 Technical documentation
One file per source file in `docs/technical/`, mirroring the source tree:

```markdown
# order/application/PlaceOrderUseCase
Layer: Application · Module: Order · Phase: P11
**Responsibility** — one sentence
**Inputs / Outputs** — types
**Dependencies** — ports and external services, by name
**Rules enforced** — business rules, listed
**Algorithms used** — ALG IDs
**Failure modes** — what it returns and when
**Tests** — file path, cases covered
```

Written in the same task as the code. Never retrofitted.

## 6.2 User documentation
`docs/user/`, one file per screen, per role. Purpose, entry point, what the user sees, every action and its destination, all states, error handling.

## 6.3 README.md
Must contain: what the system is, quickstart in under ten minutes, repo layout, and a **Technical Decisions** section recording every significant choice with its reasoning — Go, Flutter, modular monolith, the payment isolation, PostGIS, the separate test module, the adapter pattern, and the 100% coverage policy with its exclusion list.

## 6.4 ADRs
`docs/decisions/NNNN-title.md`, one per significant decision. Context, options considered, decision, consequences.

---
