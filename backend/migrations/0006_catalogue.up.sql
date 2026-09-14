-- P07 catalogue: what a shop sells.
--
-- The acceptance criterion for the phase is that the per-merchant-type schema
-- differences hold. They are enforced in the domain (domain.Capabilities) rather
-- than by three sets of tables, because every consumer — cart, order, discovery
-- — handles an item the same way and only the owner's screen cares about the
-- difference. What this schema does is give each type's fields a column and let
-- the unused ones stay empty, which is cheap, and keep the CHECK constraints
-- that are true for every type.

CREATE TABLE IF NOT EXISTS catalogue_categories (
    id          TEXT PRIMARY KEY,
    merchant_id TEXT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    -- The owner's own ordering, applied by the server so every client shows the
    -- menu the same way round.
    sort_order  INTEGER NOT NULL DEFAULT 0,
    -- Hidden rather than deleted: an owner who hides a seasonal section in
    -- February expects it back in December.
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS catalogue_items (
    id            TEXT PRIMARY KEY,
    merchant_id   TEXT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    -- RESTRICT rather than CASCADE: deleting a section that still holds food
    -- would silently delete the food. The use case refuses instead and tells
    -- the owner to empty it first.
    category_id   TEXT NOT NULL REFERENCES catalogue_categories(id) ON DELETE RESTRICT,
    merchant_type TEXT NOT NULL CHECK (merchant_type IN ('restaurant', 'grocery', 'pharmacy')),

    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    image_url     TEXT NOT NULL DEFAULT '',

    -- Money as minor units in a bigint, never a float. A float64 cannot hold
    -- 0.1, and a price that drifts is a price a customer sees two of.
    price_minor   BIGINT NOT NULL CHECK (price_minor >= 0),
    currency      TEXT NOT NULL DEFAULT 'BDT',

    -- Stock. tracked is not "quantity > 0": a kitchen has no count at all, and
    -- storing that as zero would read as sold out.
    stock_tracked  BOOLEAN NOT NULL DEFAULT FALSE,
    stock_quantity INTEGER NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),

    -- Grocery and pharmacy.
    unit          TEXT NOT NULL DEFAULT '',
    pack_size     TEXT NOT NULL DEFAULT '',
    brand         TEXT NOT NULL DEFAULT '',

    -- Pharmacy.
    generic_name          TEXT NOT NULL DEFAULT '',
    strength              TEXT NOT NULL DEFAULT '',
    requires_prescription BOOLEAN NOT NULL DEFAULT FALSE,

    -- Restaurant.
    is_vegetarian       BOOLEAN NOT NULL DEFAULT FALSE,
    preparation_minutes INTEGER NOT NULL DEFAULT 0 CHECK (preparation_minutes >= 0),

    -- The owner's own switch, distinct from being out of stock and from being
    -- outside an availability window. Three reasons an item is not orderable,
    -- and a customer is owed the right one.
    active        BOOLEAN NOT NULL DEFAULT TRUE,

    -- When the item may be ordered, as {"1": ["07:00-11:00"]} keyed by weekday
    -- with Sunday as 0. An empty object means always, which is what almost
    -- every item is.
    availability  JSONB NOT NULL DEFAULT '{}'::jsonb,

    sort_order    INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Only a pharmacy may require a prescription. Enforced in the domain and
    -- again here: the domain is where the message an owner reads comes from,
    -- and this is what stops a direct write or a future import from creating a
    -- restaurant dish that needs one.
    CONSTRAINT catalogue_items_prescription_is_pharmacy
        CHECK (NOT requires_prescription OR merchant_type = 'pharmacy')
);

-- Groups of mutually-exclusive choices: a size, a strength, a pack.
CREATE TABLE IF NOT EXISTS catalogue_variant_groups (
    id          TEXT PRIMARY KEY,
    item_id     TEXT NOT NULL REFERENCES catalogue_items(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    required    BOOLEAN NOT NULL DEFAULT FALSE,
    min_choices INTEGER NOT NULL DEFAULT 0 CHECK (min_choices >= 0),
    max_choices INTEGER NOT NULL DEFAULT 1 CHECK (max_choices >= 1),
    sort_order  INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT catalogue_variant_groups_range CHECK (min_choices <= max_choices)
);

CREATE TABLE IF NOT EXISTS catalogue_variant_options (
    id          TEXT PRIMARY KEY,
    group_id    TEXT NOT NULL REFERENCES catalogue_variant_groups(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    -- Signed: a small size legitimately costs less, and modelling that as a
    -- separate item would double every menu.
    price_delta_minor BIGINT NOT NULL DEFAULT 0,
    available   BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order  INTEGER NOT NULL DEFAULT 0
);

-- Extras bought alongside. Restaurants only, which the domain enforces; the
-- tables are shared because the shape is identical.
CREATE TABLE IF NOT EXISTS catalogue_addon_groups (
    id          TEXT PRIMARY KEY,
    item_id     TEXT NOT NULL REFERENCES catalogue_items(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    min_choices INTEGER NOT NULL DEFAULT 0 CHECK (min_choices >= 0),
    max_choices INTEGER NOT NULL DEFAULT 1 CHECK (max_choices >= 1),
    sort_order  INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT catalogue_addon_groups_range CHECK (min_choices <= max_choices)
);

CREATE TABLE IF NOT EXISTS catalogue_addon_options (
    id          TEXT PRIMARY KEY,
    group_id    TEXT NOT NULL REFERENCES catalogue_addon_groups(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    -- A price, not a delta: an extra is a thing with a price, and expressing
    -- "extra cheese, ৳30" as an adjustment to the pizza makes a receipt
    -- impossible to read.
    price_minor BIGINT NOT NULL DEFAULT 0 CHECK (price_minor >= 0),
    available   BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS catalogue_combos (
    id            TEXT PRIMARY KEY,
    merchant_id   TEXT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    merchant_type TEXT NOT NULL CHECK (merchant_type IN ('restaurant', 'grocery', 'pharmacy')),
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    image_url     TEXT NOT NULL DEFAULT '',
    -- Stated, not derived from the members: a combo exists to be cheaper than
    -- its parts, and computing it from them would move the discount every time
    -- a member's price changed — including upward.
    price_minor   BIGINT NOT NULL CHECK (price_minor >= 0),
    currency      TEXT NOT NULL DEFAULT 'BDT',
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    availability  JSONB NOT NULL DEFAULT '{}'::jsonb,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS catalogue_combo_lines (
    combo_id TEXT NOT NULL REFERENCES catalogue_combos(id) ON DELETE CASCADE,
    -- RESTRICT: deleting an item that a combo still sells would leave the
    -- bundle resolving to nothing at checkout, which the customer discovers
    -- and the owner does not.
    item_id  TEXT NOT NULL REFERENCES catalogue_items(id) ON DELETE RESTRICT,
    quantity INTEGER NOT NULL CHECK (quantity >= 1),
    -- One line per item: "two of these" is the quantity, not a second row.
    PRIMARY KEY (combo_id, item_id)
);

-- A menu is always read per shop, in the owner's order.
CREATE INDEX IF NOT EXISTS catalogue_categories_merchant_idx
    ON catalogue_categories (merchant_id, sort_order, id);
CREATE INDEX IF NOT EXISTS catalogue_items_merchant_idx
    ON catalogue_items (merchant_id, sort_order, id);

-- Browsing one section, and counting what is in it before deleting it.
CREATE INDEX IF NOT EXISTS catalogue_items_category_idx
    ON catalogue_items (merchant_id, category_id);

-- Searching a grocery's aisles by name. Trigram rather than a prefix index
-- because a customer types "chal" for "Miniket Chal" as often as not.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS catalogue_items_name_trgm_idx
    ON catalogue_items USING GIN (lower(name) gin_trgm_ops);

CREATE INDEX IF NOT EXISTS catalogue_variant_groups_item_idx
    ON catalogue_variant_groups (item_id, sort_order);
CREATE INDEX IF NOT EXISTS catalogue_variant_options_group_idx
    ON catalogue_variant_options (group_id, sort_order);
CREATE INDEX IF NOT EXISTS catalogue_addon_groups_item_idx
    ON catalogue_addon_groups (item_id, sort_order);
CREATE INDEX IF NOT EXISTS catalogue_addon_options_group_idx
    ON catalogue_addon_options (group_id, sort_order);
CREATE INDEX IF NOT EXISTS catalogue_combos_merchant_idx
    ON catalogue_combos (merchant_id, sort_order, id);
