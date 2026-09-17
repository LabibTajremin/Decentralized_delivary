-- P13 payment: taking money through a gateway, and tracking the cash a rider
-- is carrying.
--
-- Two tables, one for each kind of money this platform ever touches. Neither
-- references order_id as a foreign key: payment is the isolated module (2.6)
-- and must not depend on order's schema, only on order ids as opaque strings
-- handed across the contract boundary.

CREATE TABLE IF NOT EXISTS payments (
    id          TEXT PRIMARY KEY,
    order_id    TEXT NOT NULL,
    customer_id TEXT NOT NULL,

    -- Which adapter took this attempt — "manual" until a real provider is
    -- chosen. Free text rather than an enum: a second gateway is a config
    -- change, not a migration.
    gateway     TEXT NOT NULL,
    -- The identifier this module minted and handed to the gateway as its own
    -- merchant reference — how a webhook is matched back before the gateway
    -- has assigned anything of its own.
    reference   TEXT NOT NULL,
    -- The gateway's own transaction id, empty until it exists.
    gateway_ref TEXT NOT NULL DEFAULT '',

    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency     TEXT NOT NULL,

    status TEXT NOT NULL CHECK (status IN ('pending', 'captured', 'failed', 'refunded')),
    failure_reason TEXT NOT NULL DEFAULT '',
    refund_reason  TEXT NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS payments_reference_idx ON payments (reference);
-- Checkout's idempotency: at most one open attempt per order at a time, so two
-- requests racing to pay the same order cannot both start a checkout.
CREATE UNIQUE INDEX IF NOT EXISTS payments_order_pending_idx
    ON payments (order_id) WHERE status = 'pending';
-- "What is the latest attempt for this order" — the read every screen and
-- every refund makes.
CREATE INDEX IF NOT EXISTS payments_order_idx ON payments (order_id, created_at DESC);

CREATE TABLE IF NOT EXISTS cash_collections (
    id         TEXT PRIMARY KEY,
    -- One collection per order: a delivery happens once, so its cash is
    -- recorded once.
    order_id   TEXT NOT NULL,
    partner_id TEXT NOT NULL,

    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency     TEXT NOT NULL,

    status TEXT NOT NULL CHECK (status IN ('held', 'remitted')),
    remittance_ref TEXT NOT NULL DEFAULT '',

    collected_at TIMESTAMPTZ NOT NULL,
    remitted_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS cash_collections_order_idx ON cash_collections (order_id);
-- A partner's own ledger: held first (what a "please remit" screen lists),
-- oldest first within a status.
CREATE INDEX IF NOT EXISTS cash_collections_partner_idx
    ON cash_collections (partner_id, status, collected_at);
