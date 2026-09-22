-- P11 order: the agreement between a customer and a shop.
--
-- Everything on an order is a copy. The lines copy the menu, the destination
-- copies the address book, the pickup copies the shop. None of it refers to the
-- live record, because an order is a record of an agreement and an agreement
-- that changed afterwards was not an agreement: a shop renaming an item must
-- not rename it on last week's receipt, and a customer editing an address must
-- not redirect a rider who is already on the road.

CREATE TABLE IF NOT EXISTS orders (
    id          TEXT PRIMARY KEY,
    -- The short reference a customer reads out on the phone. Unique so it can
    -- be searched on by support without ambiguity.
    code        TEXT NOT NULL,
    customer_id TEXT NOT NULL,
    -- The only live reference on the order, and it is a soft one: the shop must
    -- still be resolvable afterwards for tracking, support and reviews, so
    -- there is no foreign key that a merchant deletion could cascade through.
    merchant_id TEXT NOT NULL,

    status  TEXT NOT NULL CHECK (status IN (
        'pending_payment', 'placed', 'accepted', 'preparing', 'ready',
        'picked_up', 'delivered', 'cancelled', 'rejected', 'failed')),
    payment_method TEXT NOT NULL CHECK (payment_method IN ('cash', 'online')),

    -- Money in poisha, never a float (2.9). Frozen: a receipt that changed when
    -- an admin retuned the area's delivery rate is a receipt nobody could
    -- reconcile.
    subtotal_minor            BIGINT NOT NULL CHECK (subtotal_minor >= 0),
    delivery_minor            BIGINT NOT NULL CHECK (delivery_minor >= 0),
    expansion_surcharge_minor BIGINT NOT NULL DEFAULT 0 CHECK (expansion_surcharge_minor >= 0),
    total_minor               BIGINT NOT NULL CHECK (total_minor >= 0),
    free_delivery             BOOLEAN NOT NULL DEFAULT FALSE,
    -- Whether this delivery crossed the base radius (D2), and which rung. Kept
    -- so an admin looking at a division can see how much of its traffic needed
    -- expansion — which is the signal the auto-tuner acts on in P15.
    expanded        BOOLEAN NOT NULL DEFAULT FALSE,
    expansion_level INTEGER NOT NULL DEFAULT 0 CHECK (expansion_level >= 0),
    distance_m      DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (distance_m >= 0),

    -- The destination, copied.
    address_id       TEXT NOT NULL,
    address_label    TEXT NOT NULL DEFAULT '',
    recipient_name   TEXT NOT NULL DEFAULT '',
    recipient_phone  TEXT NOT NULL DEFAULT '',
    address_line1    TEXT NOT NULL DEFAULT '',
    address_line2    TEXT NOT NULL DEFAULT '',
    address_single   TEXT NOT NULL DEFAULT '',
    address_lat      DOUBLE PRECISION NOT NULL DEFAULT 0,
    address_lng      DOUBLE PRECISION NOT NULL DEFAULT 0,
    address_area     TEXT NOT NULL DEFAULT '',
    address_area_name TEXT NOT NULL DEFAULT '',
    address_directions TEXT NOT NULL DEFAULT '',

    -- The pickup, copied.
    pickup_name   TEXT NOT NULL DEFAULT '',
    pickup_phone  TEXT NOT NULL DEFAULT '',
    pickup_single TEXT NOT NULL DEFAULT '',
    pickup_lat    DOUBLE PRECISION NOT NULL DEFAULT 0,
    pickup_lng    DOUBLE PRECISION NOT NULL DEFAULT 0,

    placed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS orders_code_idx ON orders (code);
-- The two listings the product actually asks for: a customer's own orders and
-- a shop's queue, both newest first.
CREATE INDEX IF NOT EXISTS orders_customer_idx ON orders (customer_id, placed_at DESC);
CREATE INDEX IF NOT EXISTS orders_merchant_idx ON orders (merchant_id, placed_at DESC);
-- The live queue, which is what a shop's screen and a dispatch sweep both read.
CREATE INDEX IF NOT EXISTS orders_live_idx ON orders (status, placed_at DESC)
    WHERE status NOT IN ('delivered', 'cancelled', 'rejected', 'failed');

CREATE TABLE IF NOT EXISTS order_lines (
    id       TEXT PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,

    kind      TEXT NOT NULL CHECK (kind IN ('item', 'combo')),
    -- What it was, so a reorder button has something to point at. No foreign
    -- key: the item may be deleted, and the order still happened.
    target_id TEXT NOT NULL,
    name      TEXT NOT NULL,
    unit_price_minor BIGINT NOT NULL CHECK (unit_price_minor >= 0),
    quantity  INTEGER NOT NULL CHECK (quantity > 0),
    note      TEXT NOT NULL DEFAULT '',
    position  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS order_lines_order_idx ON order_lines (order_id, position);

CREATE TABLE IF NOT EXISTS order_line_options (
    line_id     TEXT NOT NULL REFERENCES order_lines(id) ON DELETE CASCADE,
    group_id    TEXT NOT NULL,
    option_id   TEXT NOT NULL,
    name        TEXT NOT NULL,
    price_minor BIGINT NOT NULL,
    position    INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY (line_id, group_id, option_id)
);

-- The history, kept rather than derived. "When did the shop accept it" is a
-- question the customer asks, tracking answers (P14) and support settles
-- arguments with (P16); a status column alone cannot answer it.
CREATE TABLE IF NOT EXISTS order_events (
    id       TEXT PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status   TEXT NOT NULL,
    actor    TEXT NOT NULL CHECK (actor IN ('customer', 'merchant', 'partner', 'admin', 'system')),
    actor_id TEXT NOT NULL DEFAULT '',
    reason   TEXT NOT NULL DEFAULT '',
    at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS order_events_order_idx ON order_events (order_id, at);

-- Idempotency. A customer on a village 2G connection taps "Place order", sees
-- nothing happen, and taps again. Without this they are charged twice and the
-- shop cooks twice.
--
-- Scoped to the customer as well as the key: two customers using the same
-- client library may generate the same key, and a global unique index would
-- hand one of them the other's order.
CREATE TABLE IF NOT EXISTS order_idempotency_keys (
    customer_id TEXT NOT NULL,
    key         TEXT NOT NULL,
    order_id    TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (customer_id, key)
);
