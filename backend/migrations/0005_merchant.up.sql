-- P06 merchant: registration, documents and the approval workflow.
--
-- D1 lives in what this schema does *not* constrain: nothing here limits how
-- many shops a division may hold, how close two shops may be, or which areas
-- accept registrations. A shop anywhere in Bangladesh gets a row on the same
-- terms as one in Dhaka. Whether a customer ever sees it is decided by the
-- status column here and by the division-bounded radius search in geo (D3).

CREATE TABLE IF NOT EXISTS merchants (
    id             TEXT PRIMARY KEY,

    -- One shop per account. A unique constraint rather than a check in the use
    -- case: two registrations racing would both pass an application-level
    -- check, and the loser must be told it lost rather than creating a second
    -- shop nobody can administer.
    owner_user_id  TEXT NOT NULL UNIQUE,

    name           TEXT NOT NULL,
    type           TEXT NOT NULL CHECK (type IN ('restaurant', 'grocery', 'pharmacy')),
    status         TEXT NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft', 'pending_review', 'approved', 'rejected', 'suspended')),

    -- The shop's public line, which a rider calls on arrival. Stored in E.164
    -- so one number has one spelling, whatever the owner typed.
    phone          TEXT NOT NULL,
    email          TEXT NOT NULL DEFAULT '',
    logo_url       TEXT NOT NULL DEFAULT '',

    line1          TEXT NOT NULL,
    line2          TEXT NOT NULL DEFAULT '',

    -- Where the shop is. Geo keeps the searchable copy and the GiST index over
    -- it; this column is the merchant module's own record of the pin its owner
    -- dropped, so re-publishing to geo after a change never needs to ask geo
    -- what it currently believes.
    pin            geography(POINT, 4326) NOT NULL,

    -- The administrative placement, resolved from the pin at write time.
    area_code      TEXT NOT NULL DEFAULT '',
    area_name      TEXT NOT NULL DEFAULT '',
    district_code  TEXT NOT NULL DEFAULT '',
    division_code  TEXT NOT NULL DEFAULT '',

    -- Opening hours, as {"0": ["09:00-22:00"], ...} keyed by weekday with
    -- Sunday as 0. JSONB rather than a row per window: the schedule is read as
    -- a whole every time and written as a whole every time, and seven joins to
    -- answer "are you open" is a cost paid on every listing.
    hours          JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Holiday mode: the owner closing temporarily, as distinct from a
    -- suspension an admin imposes. holiday_until NULL means indefinitely.
    holiday_active BOOLEAN NOT NULL DEFAULT FALSE,
    holiday_until  TIMESTAMPTZ,
    holiday_reason TEXT NOT NULL DEFAULT '',

    -- The admin's last word: why a shop was rejected or suspended. The owner
    -- reads it, so "not visible" is never unexplained.
    review_note    TEXT NOT NULL DEFAULT '',

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS merchant_documents (
    merchant_id  TEXT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    kind         TEXT NOT NULL
                      CHECK (kind IN ('trade_licence', 'national_id', 'food_licence', 'drug_licence')),
    number       TEXT NOT NULL,
    -- Where the scan is stored, not the scan itself. Keeping bytes out of the
    -- row means moving to object storage later touches one column.
    file_url     TEXT NOT NULL,
    uploaded_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- One current document per kind: re-uploading a licence corrects the last
    -- one, and keeping both would leave a reviewer choosing between two numbers
    -- with nothing to say which is current.
    PRIMARY KEY (merchant_id, kind)
);

CREATE TABLE IF NOT EXISTS merchant_status_events (
    id            TEXT PRIMARY KEY,
    merchant_id   TEXT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    from_status   TEXT NOT NULL,
    to_status     TEXT NOT NULL,
    -- Who decided. An approval with no name attached is not an audit trail.
    actor_user_id TEXT NOT NULL,
    note          TEXT NOT NULL DEFAULT '',
    at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The admin queue: "everything waiting for review", newest first.
CREATE INDEX IF NOT EXISTS merchants_status_idx ON merchants (status, created_at DESC);

-- An operator works a region (D3), so the listing filters by division first.
CREATE INDEX IF NOT EXISTS merchants_division_status_idx ON merchants (division_code, status);

-- Discovery resolves a page of ids from the radius search; catalogue and order
-- resolve one at a time by primary key, which needs no index of its own.
CREATE INDEX IF NOT EXISTS merchants_type_status_idx ON merchants (type, status);

-- A merchant's history is always read newest first.
CREATE INDEX IF NOT EXISTS merchant_status_events_merchant_idx
    ON merchant_status_events (merchant_id, at DESC);
