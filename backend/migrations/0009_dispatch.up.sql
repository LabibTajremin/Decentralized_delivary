-- P12 dispatch: getting an order from a counter to a door.
--
-- Two tables and one rule each. A partner is a person who carries orders, and
-- their location is the thing that decides what they are shown (D4). A job is
-- one delivery, and it belongs to at most one partner at a time — which is the
-- phase's third acceptance criterion, enforced by a unique index and a
-- compare-and-set rather than by a check somebody could skip.

CREATE TABLE IF NOT EXISTS delivery_partners (
    id      TEXT PRIMARY KEY,
    -- One partner record per account. A second would give somebody two feeds
    -- and two concurrent-job budgets.
    user_id TEXT NOT NULL,

    name    TEXT NOT NULL,
    phone   TEXT NOT NULL,
    -- Free text, not an enum: the list of vehicles people deliver on in
    -- Bangladesh would be wrong within a month.
    vehicle TEXT NOT NULL DEFAULT '',

    availability TEXT NOT NULL DEFAULT 'offline'
        CHECK (availability IN ('offline', 'available', 'busy')),
    -- D4's distance choice. 'any' is the default because a partner who has not
    -- chosen has not chosen to exclude anything.
    preference   TEXT NOT NULL DEFAULT 'any'
        CHECK (preference IN ('short', 'long', 'any')),

    -- Where they last reported being. A geography point rather than plain
    -- columns, because unlike the cart this module really does run a spatial
    -- query over it: the candidate pool for an assignment round is "the
    -- partners within N metres of this shop".
    pin geography(Point, 4326),

    carrying INTEGER NOT NULL DEFAULT 0 CHECK (carrying >= 0),
    -- The acceptance rate's two halves, kept as counters so concurrent offers
    -- cannot lose each other in a read-modify-write.
    offered  INTEGER NOT NULL DEFAULT 0 CHECK (offered >= 0),
    accepted INTEGER NOT NULL DEFAULT 0 CHECK (accepted >= 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS delivery_partners_user_idx ON delivery_partners (user_id);
-- The candidate-pool query: partners who are working, near a point. GiST over
-- the geography, partial on the availability, because a sweep never looks at
-- an offline rider.
CREATE INDEX IF NOT EXISTS delivery_partners_available_idx
    ON delivery_partners USING GIST (pin)
    WHERE availability = 'available';

CREATE TABLE IF NOT EXISTS delivery_jobs (
    id       TEXT PRIMARY KEY,
    -- One job per order, by a unique index rather than a check: an order that
    -- reaches `ready` twice — a retry, a re-delivered event — must not become
    -- two jobs for two riders.
    order_id TEXT NOT NULL,
    -- The order's short reference, so a rider and a shopkeeper can say the
    -- same six characters to each other.
    code     TEXT NOT NULL DEFAULT '',

    status TEXT NOT NULL CHECK (status IN (
        'waiting', 'offered', 'assigned', 'collected',
        'delivered', 'failed', 'cancelled')),
    -- Who holds it. NULL while waiting, which is what makes "nobody has this"
    -- a state the database can express rather than a convention.
    partner_id TEXT REFERENCES delivery_partners(id) ON DELETE SET NULL,
    -- The last partner who had it and did not take it. The next round skips
    -- them: a rider who has already declined should not be asked the same
    -- question again while somebody else is standing by. Plain text, not a
    -- reference, because it is a hint for one round and must survive a partner
    -- record being removed.
    passed_by  TEXT NOT NULL DEFAULT '',

    -- Both ends, copied off the order. The order froze them at placement for
    -- its own reasons; copying again here means a rider's screen needs no
    -- second module to render.
    pickup_name        TEXT NOT NULL DEFAULT '',
    pickup_phone       TEXT NOT NULL DEFAULT '',
    pickup_single      TEXT NOT NULL DEFAULT '',
    pickup_pin         geography(Point, 4326),
    destination_name   TEXT NOT NULL DEFAULT '',
    destination_phone  TEXT NOT NULL DEFAULT '',
    destination_single TEXT NOT NULL DEFAULT '',
    destination_pin    geography(Point, 4326),

    distance_m DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (distance_m >= 0),
    -- D4's classification, computed once against the area's ceilings at the
    -- moment the job was created. Stored rather than derived so a retuned
    -- ceiling does not silently move live jobs between riders' feeds.
    band TEXT NOT NULL CHECK (band IN ('short', 'long', 'beyond')),

    -- Where the job is, in administrative terms. Copied off the order because
    -- the settings that govern a job are per-area (D2) and the sweeper that
    -- re-offers it minutes later has only the job to go on.
    area_code     TEXT NOT NULL DEFAULT '',
    district_code TEXT NOT NULL DEFAULT '',
    division_code TEXT NOT NULL DEFAULT '',

    reason   TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    offered_at       TIMESTAMPTZ,
    offer_expires_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- A job that is offered or assigned has somebody on it; one that is
    -- waiting does not. Expressed here so no code path can leave a job in a
    -- state that contradicts itself.
    CONSTRAINT delivery_jobs_holder CHECK (
        (status IN ('waiting', 'cancelled') AND partner_id IS NULL)
        OR (status NOT IN ('waiting') AND partner_id IS NOT NULL)
        OR (status = 'cancelled')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS delivery_jobs_order_idx ON delivery_jobs (order_id);
-- A partner's own work, newest first.
CREATE INDEX IF NOT EXISTS delivery_jobs_partner_idx ON delivery_jobs (partner_id, created_at DESC);
-- The feed query: waiting jobs near a point (ALG-08).
CREATE INDEX IF NOT EXISTS delivery_jobs_waiting_idx
    ON delivery_jobs USING GIST (pickup_pin)
    WHERE status = 'waiting';
-- The sweeper's second pass: jobs on the board, oldest first.
CREATE INDEX IF NOT EXISTS delivery_jobs_waiting_age_idx
    ON delivery_jobs (created_at)
    WHERE status = 'waiting';
-- The sweeper's query: offers that have run out.
CREATE INDEX IF NOT EXISTS delivery_jobs_expiring_idx
    ON delivery_jobs (offer_expires_at)
    WHERE status = 'offered';
