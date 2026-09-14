-- P04 identity schema.
--
-- Only the account record lives in Postgres. Sessions, refresh tokens and
-- one-time codes live in Redis, where a TTL expires them automatically and no
-- sweeper job can fail to run (ADR 0005).
--
-- The profile — name, addresses, preferences — belongs to the user module
-- (P05). Identity keeps the minimum needed to answer "who is this phone
-- number, and what may they do", so the two can be separated later without
-- untangling a shared table.

CREATE TABLE IF NOT EXISTS identity_accounts (
    id          TEXT PRIMARY KEY,
    -- E.164, normalised by the domain before it reaches here. Two spellings of
    -- one number must not become two accounts.
    phone       TEXT NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('customer', 'partner', 'merchant', 'admin')),
    -- A suspended account keeps its history and its sessions can be revoked,
    -- which deleting the row would make impossible.
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- One account per (phone, role), not per phone. The same person may be a
    -- customer and a delivery partner, and those are different accounts with
    -- different permissions — but they must not be able to hold two customer
    -- accounts on one number and dodge a per-account limit.
    UNIQUE (phone, role)
);

CREATE INDEX IF NOT EXISTS identity_accounts_phone_idx
    ON identity_accounts (phone);
