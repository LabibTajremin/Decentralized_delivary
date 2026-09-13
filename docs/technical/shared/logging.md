# shared/logging
Layer: Shared · Module: — · Phase: P01

**Responsibility** — structured JSON logging with sensitive values redacted centrally.

**Inputs / Outputs** — `New(io.Writer, Level) *slog.Logger`; `Into(ctx, logger)` / `From(ctx)`.

**Dependencies** — `log/slog`.

**Rules enforced**
- Redaction happens in the handler, not at the call site. `phone`, `otp`, `token`, `access_token`, `refresh_token`, `authorization`, `password`, `secret` and `api_key` are replaced with `[REDACTED]`, case-insensitively, including inside `slog.Group`. Call-site discipline is exactly what fails under pressure, so it is not relied on.
- An unrecognised level falls back to info, so a typo in config cannot silence production logs.
- `From` on a context with no logger returns a discarding logger, so no call site needs a nil check.

**Algorithms used** — none.

**Failure modes** — none; logging never returns an error to the caller.

**Tests** — `backend/tests/unit/shared/logging_test.go`. Asserts each redacted key, case-insensitivity, redaction inside a group, level filtering including the unknown-level fallback, and context round-trip with a nil logger.
