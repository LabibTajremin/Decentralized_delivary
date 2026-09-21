-- P16 review & support: ratings for merchant, partner and item, and the
-- ticket queue a refund decision goes through.
--
-- Two tables, neither referencing another module's schema (2.5): an order,
-- customer, merchant or partner id here is an opaque string handed across a
-- contract boundary, the same convention every other module's migration
-- follows.

CREATE TABLE IF NOT EXISTS reviews (
    id       TEXT PRIMARY KEY,
    order_id TEXT NOT NULL,
    rater_id TEXT NOT NULL,

    subject    TEXT NOT NULL CHECK (subject IN ('merchant', 'partner', 'item')),
    subject_id TEXT NOT NULL,

    rating  SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment TEXT NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- One opinion per person per thing, from one order — not one per tap of
    -- a submit button, and not one per order line if a customer ordered the
    -- same item twice.
    UNIQUE (order_id, rater_id, subject, subject_id)
);

-- A subject's own reviews and rating, both read far more often than a
-- review is written.
CREATE INDEX IF NOT EXISTS reviews_subject_idx
    ON reviews (subject, subject_id, created_at DESC);

CREATE TABLE IF NOT EXISTS support_tickets (
    id        TEXT PRIMARY KEY,
    order_id  TEXT NOT NULL,
    raised_by TEXT NOT NULL,
    subject   TEXT NOT NULL,

    status     TEXT NOT NULL CHECK (status IN ('open', 'resolved')),
    resolution TEXT NOT NULL DEFAULT '' CHECK (resolution IN ('', 'refunded', 'rejected')),
    note       TEXT NOT NULL DEFAULT '',
    agent_id   TEXT NOT NULL DEFAULT '',

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

-- A customer's own tickets, most recent first.
CREATE INDEX IF NOT EXISTS support_tickets_raised_by_idx
    ON support_tickets (raised_by, created_at DESC);

-- The queue an agent works down: open tickets, oldest first.
CREATE INDEX IF NOT EXISTS support_tickets_open_idx
    ON support_tickets (created_at ASC) WHERE status = 'open';
