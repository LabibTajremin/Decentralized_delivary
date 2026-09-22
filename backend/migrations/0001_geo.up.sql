-- P02 geo schema.
--
-- Every statement is IF NOT EXISTS so this file is safe to run against a fresh
-- database and against one that already has part of the schema. The migrator
-- still records the version in schema_migrations, so a healthy database skips
-- the file entirely; the guards are belt-and-braces for a database that was
-- built by hand before the migrator existed.
--
-- PostGIS is required: ALG-01 (merchants within radius) needs ST_DWithin over a
-- GiST index, and ALG-03 (division boundary) needs ST_Contains. See
-- docs/decisions/0003-postgis-geospatial.md.
-- SCHEMA public for the same reason 0006 pins pg_trgm: an extension created
-- under a caller's own search_path is invisible to every other schema. This
-- one has not bitten because the postgis Docker image pre-installs it into
-- public, which makes the statement a no-op — but that is the image's
-- kindness, not a guarantee.
CREATE EXTENSION IF NOT EXISTS postgis SCHEMA public;

CREATE TABLE IF NOT EXISTS geo_divisions (
    code        TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    boundary    geography(POLYGON, 4326) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS geo_districts (
    code          TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    division_code TEXT NOT NULL REFERENCES geo_divisions(code) ON DELETE RESTRICT,
    boundary      geography(POLYGON, 4326) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS geo_areas (
    code          TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    district_code TEXT NOT NULL REFERENCES geo_districts(code) ON DELETE RESTRICT,
    division_code TEXT NOT NULL REFERENCES geo_divisions(code) ON DELETE RESTRICT,
    centre        geography(POINT, 4326) NOT NULL,
    boundary      geography(POLYGON, 4326) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Merchant locations live here rather than in the merchant module's tables so
-- the spatial index is owned by the module that queries it. The merchant module
-- keeps the merchant record; geo keeps only an id and a point.
CREATE TABLE IF NOT EXISTS geo_merchant_locations (
    merchant_id   TEXT PRIMARY KEY,
    location      geography(POINT, 4326) NOT NULL,
    division_code TEXT NOT NULL REFERENCES geo_divisions(code) ON DELETE RESTRICT,
    area_code     TEXT REFERENCES geo_areas(code) ON DELETE SET NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- GiST indexes turn ST_DWithin and ST_Contains from a scan into a lookup.
-- Without these ALG-01 is O(n) and the product does not work at scale.
CREATE INDEX IF NOT EXISTS geo_divisions_boundary_idx ON geo_divisions USING GIST (boundary);
CREATE INDEX IF NOT EXISTS geo_districts_boundary_idx ON geo_districts USING GIST (boundary);
CREATE INDEX IF NOT EXISTS geo_areas_boundary_idx     ON geo_areas     USING GIST (boundary);
CREATE INDEX IF NOT EXISTS geo_merchants_location_idx ON geo_merchant_locations USING GIST (location);

-- Radius search always filters by division (D3) and by active status, so the
-- composite index lets Postgres narrow before the spatial test.
CREATE INDEX IF NOT EXISTS geo_merchants_division_active_idx
    ON geo_merchant_locations (division_code, is_active);
