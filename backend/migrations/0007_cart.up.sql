-- P09 cart: what a customer has chosen, from one shop, for one address.
--
-- The single-merchant rule is a column, not a constraint: a cart names its shop
-- and every line belongs to the cart, so a line from another shop has nowhere
-- to go. That is stronger than a CHECK, because there is no shape the data can
-- take that breaks it.

CREATE TABLE IF NOT EXISTS carts (
    id          TEXT PRIMARY KEY,
    -- No foreign key to user_profiles. A profile row is only written when a
    -- customer sets a name, and most never do — a customer who signs in and
    -- immediately fills a cart has no profile row, and a foreign key here
    -- would refuse them a cart for having skipped an optional screen.
    user_id     TEXT NOT NULL,
    merchant_id TEXT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,

    -- The delivery point the cart is held against. Nullable in effect — an
    -- empty address_id means none has been chosen — because a customer fills a
    -- cart before they think about where it is going.
    --
    -- Plain columns rather than a geography point: the cart runs no spatial
    -- query of its own. It hands these coordinates to discovery, which owns the
    -- geometry, and storing them as a PostGIS type here would suggest otherwise.
    address_id  TEXT NOT NULL DEFAULT '',
    lat         DOUBLE PRECISION NOT NULL DEFAULT 0,
    lng         DOUBLE PRECISION NOT NULL DEFAULT 0,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One cart per customer. Two open carts is how a customer ends up ordering from
-- a shop they had forgotten they were in the middle of.
CREATE UNIQUE INDEX IF NOT EXISTS carts_one_per_user ON carts (user_id);
CREATE INDEX IF NOT EXISTS carts_merchant_idx ON carts (merchant_id);

CREATE TABLE IF NOT EXISTS cart_lines (
    id       TEXT PRIMARY KEY,
    cart_id  TEXT NOT NULL REFERENCES carts(id) ON DELETE CASCADE,

    kind     TEXT NOT NULL CHECK (kind IN ('item', 'combo')),
    -- No foreign key to catalogue_items. A line deliberately survives the item
    -- being deleted: the cart's job on the next read is to tell the customer
    -- "the shop has removed this", which it cannot do if the row vanished with
    -- the item.
    target_id TEXT NOT NULL,

    name      TEXT NOT NULL,
    -- The price when it went in, in poisha. Money is never a float (2.9).
    -- A snapshot on purpose: revalidation compares it against what the shop
    -- charges now and says the price changed, rather than quietly repricing.
    unit_price_minor BIGINT NOT NULL CHECK (unit_price_minor >= 0),
    quantity  INTEGER NOT NULL CHECK (quantity > 0 AND quantity <= 20),
    note      TEXT NOT NULL DEFAULT '',
    -- The order the customer added things in, so the cart screen does not
    -- reshuffle itself between reads.
    position  INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT cart_lines_target_present CHECK (target_id <> '')
);

CREATE INDEX IF NOT EXISTS cart_lines_cart_idx ON cart_lines (cart_id, position);

CREATE TABLE IF NOT EXISTS cart_line_options (
    line_id     TEXT NOT NULL REFERENCES cart_lines(id) ON DELETE CASCADE,
    group_id    TEXT NOT NULL,
    option_id   TEXT NOT NULL,
    name        TEXT NOT NULL,
    -- A delta for a variant and a price for an add-on. Which it means is
    -- decided by the group it came from, which is catalogue's business; the
    -- cart only needs to add it up.
    price_minor BIGINT NOT NULL,
    position    INTEGER NOT NULL DEFAULT 0,

    PRIMARY KEY (line_id, group_id, option_id)
);
