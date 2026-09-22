-- P03 config schema. Implements D5.
--
-- Two tables: what the settings currently are, and how they got that way.
-- The audit log is not derived from the overrides table — an override that is
-- deleted would take its history with it, and "why did this area go back to
-- the default" is exactly the question the log has to answer.
--
-- Every statement is IF NOT EXISTS so this file converges on a database that
-- already has part of the schema.

CREATE TABLE IF NOT EXISTS config_overrides (
    config_key   TEXT NOT NULL,
    scope_level  TEXT NOT NULL CHECK (scope_level IN ('area', 'district', 'division', 'global')),
    -- Empty rather than NULL for the global scope, so the primary key works
    -- without a partial index and a duplicate global row is impossible.
    scope_code   TEXT NOT NULL DEFAULT '',
    value        TEXT NOT NULL,
    -- A pinned variable is one an admin has decided; the auto-tuner must leave
    -- it alone.
    pinned       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (config_key, scope_level, scope_code),
    CONSTRAINT config_overrides_global_has_no_code
        CHECK ((scope_level = 'global') = (scope_code = ''))
);

-- Resolution reads every scope that could apply to one placement in a single
-- query, so the index covers the way that query filters.
CREATE INDEX IF NOT EXISTS config_overrides_scope_idx
    ON config_overrides (scope_level, scope_code);

CREATE TABLE IF NOT EXISTS config_changes (
    id           TEXT PRIMARY KEY,
    config_key   TEXT NOT NULL,
    scope_level  TEXT NOT NULL CHECK (scope_level IN ('area', 'district', 'division', 'global')),
    scope_code   TEXT NOT NULL DEFAULT '',
    old_value    TEXT NOT NULL,
    new_value    TEXT NOT NULL,
    actor_kind   TEXT NOT NULL CHECK (actor_kind IN ('admin', 'auto_tuner')),
    -- Empty for the auto-tuner, which is not a person.
    actor_id     TEXT NOT NULL DEFAULT '',
    reason       TEXT NOT NULL,
    changed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The audit log is read newest-first, usually filtered to one key or one scope.
CREATE INDEX IF NOT EXISTS config_changes_recent_idx
    ON config_changes (changed_at DESC);
CREATE INDEX IF NOT EXISTS config_changes_key_idx
    ON config_changes (config_key, changed_at DESC);
CREATE INDEX IF NOT EXISTS config_changes_scope_idx
    ON config_changes (scope_level, scope_code, changed_at DESC);
