-- P05 user profile and addresses.
--
-- Identity (0003) keeps only what authentication needs: an id, a phone and a
-- role. Everything a person tells us about themselves is here, so a breach of
-- one does not hand over the other, and so the two can be separated later
-- without untangling a shared table.

CREATE TABLE IF NOT EXISTS user_profiles (
    user_id     TEXT PRIMARY KEY,
    -- Nullable in spirit, empty in practice: a customer can order without
    -- giving a name, and demanding one at sign-up costs more sign-ups than the
    -- name is worth.
    name        TEXT NOT NULL DEFAULT '',
    email       TEXT NOT NULL DEFAULT '',
    language    TEXT NOT NULL DEFAULT 'bn' CHECK (language IN ('bn', 'en')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_addresses (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL,
    label            TEXT NOT NULL DEFAULT '',
    -- The recipient may not be the account holder: people send food to their
    -- parents, and the rider needs whoever will open the door.
    recipient_name   TEXT NOT NULL,
    recipient_phone  TEXT NOT NULL,
    line1            TEXT NOT NULL,
    line2            TEXT NOT NULL DEFAULT '',
    instructions     TEXT NOT NULL DEFAULT '',

    -- Where the rider actually goes.
    pin              geography(POINT, 4326) NOT NULL,

    -- The administrative placement, resolved from the pin at write time.
    -- Stored rather than re-derived because config resolution, pricing and
    -- dispatch all key off it, and a spatial query per request would put
    -- geometry on the hot path of every order.
    area_code        TEXT NOT NULL,
    area_name        TEXT NOT NULL DEFAULT '',
    district_code    TEXT NOT NULL,
    division_code    TEXT NOT NULL,

    is_default       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Exactly one default per user, enforced by the database as well as the domain.
-- A partial unique index rather than a trigger: it states the rule declaratively
-- and cannot be bypassed by a direct write, which a trigger someone disables
-- can.
CREATE UNIQUE INDEX IF NOT EXISTS user_addresses_one_default_idx
    ON user_addresses (user_id) WHERE is_default;

-- The address book is always read per user, oldest first.
CREATE INDEX IF NOT EXISTS user_addresses_user_idx
    ON user_addresses (user_id, created_at);

-- Dispatch and discovery ask "which addresses are in this area".
CREATE INDEX IF NOT EXISTS user_addresses_area_idx
    ON user_addresses (area_code);
