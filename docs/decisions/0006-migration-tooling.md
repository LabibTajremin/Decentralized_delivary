# 0006 — Migration tooling: an in-repo versioned migrator

- Status: accepted
- Date: 2026-09-13
- Deciders: backend

## Context

The schema has to reach three kinds of database and behave correctly in each:

1. An empty database on a developer's machine or in CI — create everything.
2. A database that is already partly migrated — apply only what is missing.
3. A database that is fully up to date — do nothing, on every single deploy.

The deployment model is a single binary plus a connection string. Whatever
applies migrations has to be available wherever that binary runs, without a
second toolchain being installed on the host.

We also need demo data. Demo data is not schema, and the two must not be able
to travel together: a seed that can ride along with a migration is a seed that
will eventually run in production.

## Options considered

**golang-migrate as a CLI.** Mature and well understood, but it means a second
binary on every host and in every CI image, and its version has to be kept in
step with the repo by hand. The failure mode is a host that migrates with a
different tool version than the one the migrations were written against.

**goose / atlas as a library.** Both are capable; atlas in particular does
schema diffing we do not want, because a diffing tool decides what to run and
we want that decided by a file someone reviewed.

**A migrator in this repo, embedded in the binaries.** Roughly 200 lines: read
numbered `.up.sql`/`.down.sql` pairs, compare against a `schema_migrations`
table, apply what is missing.

## Decision

We write the migrator, in `internal/platform/migrate`, and embed the SQL with
`go:embed` from the `migrations` package.

What that buys, specifically:

- **The binary carries its migrations.** There is no "the image shipped without
  the sql directory" failure, and the migrations that run are exactly the ones
  built and tested together.
- **The rules are ours and are tested.** Every branch — a dirty version, a
  failure partway, a missing `.down.sql` — is asserted in
  `tests/unit/platform`, and the integration suite runs the migrator against
  real PostGIS rather than hand-rolling its own schema setup.
- **Seeding is a separate verb.** `migrate seed` reads from a different
  embedded FS than `migrate up`, and refuses outright when `APP_ENV=production`.
  A seed cannot be applied by a deploy that only runs `up`.

Two properties are enforced in the SQL rather than assumed:

- Every statement is `IF NOT EXISTS`, so a database built by hand before the
  migrator existed converges instead of failing on "relation already exists".
- Every migration has a `.down.sql`. A missing one is an error at load time,
  not a surprise during a rollback.

## Consequences

We own the code, including its bugs. That cost is bounded — the package is
small and fully covered — and it is paid once, against a tool that must behave
correctly on exactly the three cases above.

`cmd/migrate` is added to `tests/coverage-exclusions.txt`. It parses arguments,
opens a connection and calls into `internal/platform/migrate`; it contains no
decision that a test could meaningfully assert that is not already asserted
against the package it calls. This is the second exclusion in the repo, after
`cmd/api`, and it is the same justification: wiring, not rules.

A dirty version stops the migrator rather than being cleaned up automatically.
Recovering from a half-applied migration needs a human to look at the schema;
guessing would risk data, and the marker is there precisely so the guess is not
made.
