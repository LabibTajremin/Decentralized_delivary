## Phase

<!-- e.g. P00 — Foundation & tooling -->

## What this adds

<!-- The deliverables from the phase file, as a list. -->

## Verification

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `golangci-lint run`
- [ ] Architecture lint (`-count=1`)
- [ ] `scripts/coverage-gate.sh` reports 100.0%
- [ ] `scripts/thin-client-lint.sh`
- [ ] Migrations up and down
- [ ] Flutter analyze / test (skipped until P17)
- [ ] E2E suite

## New dependencies

<!-- Every added dependency is justified here (05-architecture.md 2.9). "None" is a valid answer. -->

## Documentation

- [ ] Technical docs written in the same task as the code
- [ ] User docs for anything user-facing
- [ ] ADR added for any significant decision

## STATE.md

- [ ] Updated and committed
