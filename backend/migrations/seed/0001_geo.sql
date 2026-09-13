-- Demo data for local development and demonstrations.
--
-- This file is NOT a migration. It is never applied by `migrate up`, because
-- seeding a production database with fake merchants would be a serious
-- incident. It runs only via `migrate seed`, which refuses to run when
-- APP_ENV=production.
--
-- Every statement is an upsert keyed on the primary key, so re-seeding an
-- already-seeded database updates rows in place rather than failing or
-- duplicating. That makes it safe to run after every schema change.
--
-- The boundaries below are a deliberate simplification: eight non-overlapping
-- rectangles that partition Bangladesh's bounding box, chosen so that each real
-- divisional capital falls inside its own division. They are the right shape
-- for exercising ALG-03 (division containment) and the D3 ceiling; they are not
-- survey data, and P02's geometry import replaces them with official boundaries.
-- Nothing in the system reads these as authoritative — they are demo rows like
-- any other.

BEGIN;

-- Divisions -----------------------------------------------------------------

INSERT INTO geo_divisions (code, name, boundary) VALUES
    ('DHA', 'Dhaka', ST_GeogFromText('SRID=4326;POLYGON((89.700000 23.400000, 90.600000 23.400000, 90.600000 24.400000, 89.700000 24.400000, 89.700000 23.400000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_divisions (code, name, boundary) VALUES
    ('CTG', 'Chattogram', ST_GeogFromText('SRID=4326;POLYGON((90.600000 20.500000, 92.700000 20.500000, 92.700000 24.400000, 90.600000 24.400000, 90.600000 20.500000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_divisions (code, name, boundary) VALUES
    ('KHU', 'Khulna', ST_GeogFromText('SRID=4326;POLYGON((88.000000 20.500000, 89.700000 20.500000, 89.700000 23.400000, 88.000000 23.400000, 88.000000 20.500000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_divisions (code, name, boundary) VALUES
    ('RAJ', 'Rajshahi', ST_GeogFromText('SRID=4326;POLYGON((88.000000 23.400000, 89.700000 23.400000, 89.700000 25.300000, 88.000000 25.300000, 88.000000 23.400000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_divisions (code, name, boundary) VALUES
    ('BAR', 'Barishal', ST_GeogFromText('SRID=4326;POLYGON((89.700000 20.500000, 90.600000 20.500000, 90.600000 23.400000, 89.700000 23.400000, 89.700000 20.500000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_divisions (code, name, boundary) VALUES
    ('SYL', 'Sylhet', ST_GeogFromText('SRID=4326;POLYGON((90.600000 24.400000, 92.700000 24.400000, 92.700000 26.700000, 90.600000 26.700000, 90.600000 24.400000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_divisions (code, name, boundary) VALUES
    ('RAN', 'Rangpur', ST_GeogFromText('SRID=4326;POLYGON((88.000000 25.300000, 90.600000 25.300000, 90.600000 26.700000, 88.000000 26.700000, 88.000000 25.300000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_divisions (code, name, boundary) VALUES
    ('MYM', 'Mymensingh', ST_GeogFromText('SRID=4326;POLYGON((89.700000 24.400000, 90.600000 24.400000, 90.600000 25.300000, 89.700000 25.300000, 89.700000 24.400000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, boundary = EXCLUDED.boundary, updated_at = now();


-- Districts -----------------------------------------------------------------
-- One district per division for the demo. A district's boundary is its
-- division's, which keeps district lookups consistent with division lookups
-- until the real district geometry is imported.

INSERT INTO geo_districts (code, name, division_code, boundary) VALUES
    ('DHK', 'Dhaka', 'DHA', ST_GeogFromText('SRID=4326;POLYGON((89.700000 23.400000, 90.600000 23.400000, 90.600000 24.400000, 89.700000 24.400000, 89.700000 23.400000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, division_code = EXCLUDED.division_code,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_districts (code, name, division_code, boundary) VALUES
    ('CTB', 'Chattogram', 'CTG', ST_GeogFromText('SRID=4326;POLYGON((90.600000 20.500000, 92.700000 20.500000, 92.700000 24.400000, 90.600000 24.400000, 90.600000 20.500000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, division_code = EXCLUDED.division_code,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_districts (code, name, division_code, boundary) VALUES
    ('KHL', 'Khulna', 'KHU', ST_GeogFromText('SRID=4326;POLYGON((88.000000 20.500000, 89.700000 20.500000, 89.700000 23.400000, 88.000000 23.400000, 88.000000 20.500000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, division_code = EXCLUDED.division_code,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_districts (code, name, division_code, boundary) VALUES
    ('RJS', 'Rajshahi', 'RAJ', ST_GeogFromText('SRID=4326;POLYGON((88.000000 23.400000, 89.700000 23.400000, 89.700000 25.300000, 88.000000 25.300000, 88.000000 23.400000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, division_code = EXCLUDED.division_code,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_districts (code, name, division_code, boundary) VALUES
    ('BRS', 'Barishal', 'BAR', ST_GeogFromText('SRID=4326;POLYGON((89.700000 20.500000, 90.600000 20.500000, 90.600000 23.400000, 89.700000 23.400000, 89.700000 20.500000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, division_code = EXCLUDED.division_code,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_districts (code, name, division_code, boundary) VALUES
    ('SYH', 'Sylhet', 'SYL', ST_GeogFromText('SRID=4326;POLYGON((90.600000 24.400000, 92.700000 24.400000, 92.700000 26.700000, 90.600000 26.700000, 90.600000 24.400000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, division_code = EXCLUDED.division_code,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_districts (code, name, division_code, boundary) VALUES
    ('RNP', 'Rangpur', 'RAN', ST_GeogFromText('SRID=4326;POLYGON((88.000000 25.300000, 90.600000 25.300000, 90.600000 26.700000, 88.000000 26.700000, 88.000000 25.300000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, division_code = EXCLUDED.division_code,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_districts (code, name, division_code, boundary) VALUES
    ('MYS', 'Mymensingh', 'MYM', ST_GeogFromText('SRID=4326;POLYGON((89.700000 24.400000, 90.600000 24.400000, 90.600000 25.300000, 89.700000 25.300000, 89.700000 24.400000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, division_code = EXCLUDED.division_code,
        boundary = EXCLUDED.boundary, updated_at = now();


-- Areas ---------------------------------------------------------------------
-- Each area is a 0.012-degree square (roughly 1.3 km) centred on the real
-- neighbourhood. They are small and disjoint, so AreaContaining returns exactly
-- one area and the smallest-area tie-break is never needed for demo data.

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('DHK-DHM', 'Dhanmondi', 'DHK', 'DHA',
     ST_GeogFromText('SRID=4326;POINT(90.374200 23.746100)'),
     ST_GeogFromText('SRID=4326;POLYGON((90.368200 23.740100, 90.380200 23.740100, 90.380200 23.752100, 90.368200 23.752100, 90.368200 23.740100))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('DHK-GUL', 'Gulshan', 'DHK', 'DHA',
     ST_GeogFromText('SRID=4326;POINT(90.415200 23.792500)'),
     ST_GeogFromText('SRID=4326;POLYGON((90.409200 23.786500, 90.421200 23.786500, 90.421200 23.798500, 90.409200 23.798500, 90.409200 23.786500))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('DHK-MIR', 'Mirpur', 'DHK', 'DHA',
     ST_GeogFromText('SRID=4326;POINT(90.365400 23.822300)'),
     ST_GeogFromText('SRID=4326;POLYGON((90.359400 23.816300, 90.371400 23.816300, 90.371400 23.828300, 90.359400 23.828300, 90.359400 23.816300))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('DHK-UTT', 'Uttara', 'DHK', 'DHA',
     ST_GeogFromText('SRID=4326;POINT(90.379500 23.875900)'),
     ST_GeogFromText('SRID=4326;POLYGON((90.373500 23.869900, 90.385500 23.869900, 90.385500 23.881900, 90.373500 23.881900, 90.373500 23.869900))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('DHK-MTJ', 'Motijheel', 'DHK', 'DHA',
     ST_GeogFromText('SRID=4326;POINT(90.417200 23.733000)'),
     ST_GeogFromText('SRID=4326;POLYGON((90.411200 23.727000, 90.423200 23.727000, 90.423200 23.739000, 90.411200 23.739000, 90.411200 23.727000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('DHK-MPR', 'Mohammadpur', 'DHK', 'DHA',
     ST_GeogFromText('SRID=4326;POINT(90.358000 23.760100)'),
     ST_GeogFromText('SRID=4326;POLYGON((90.352000 23.754100, 90.364000 23.754100, 90.364000 23.766100, 90.352000 23.766100, 90.352000 23.754100))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('CTB-AGR', 'Agrabad', 'CTB', 'CTG',
     ST_GeogFromText('SRID=4326;POINT(91.812300 22.328000)'),
     ST_GeogFromText('SRID=4326;POLYGON((91.806300 22.322000, 91.818300 22.322000, 91.818300 22.334000, 91.806300 22.334000, 91.806300 22.322000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('CTB-KHU', 'Khulshi', 'CTB', 'CTG',
     ST_GeogFromText('SRID=4326;POINT(91.810000 22.362000)'),
     ST_GeogFromText('SRID=4326;POLYGON((91.804000 22.356000, 91.816000 22.356000, 91.816000 22.368000, 91.804000 22.368000, 91.804000 22.356000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('SYH-ZND', 'Zindabazar', 'SYH', 'SYL',
     ST_GeogFromText('SRID=4326;POINT(91.868700 24.894900)'),
     ST_GeogFromText('SRID=4326;POLYGON((91.862700 24.888900, 91.874700 24.888900, 91.874700 24.900900, 91.862700 24.900900, 91.862700 24.888900))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('KHL-SDR', 'Khulna Sadar', 'KHL', 'KHU',
     ST_GeogFromText('SRID=4326;POINT(89.540300 22.845600)'),
     ST_GeogFromText('SRID=4326;POLYGON((89.534300 22.839600, 89.546300 22.839600, 89.546300 22.851600, 89.534300 22.851600, 89.534300 22.839600))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('RJS-BLA', 'Boalia', 'RJS', 'RAJ',
     ST_GeogFromText('SRID=4326;POINT(88.604200 24.374500)'),
     ST_GeogFromText('SRID=4326;POLYGON((88.598200 24.368500, 88.610200 24.368500, 88.610200 24.380500, 88.598200 24.380500, 88.598200 24.368500))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('BRS-SDR', 'Barishal Sadar', 'BRS', 'BAR',
     ST_GeogFromText('SRID=4326;POINT(90.353500 22.701000)'),
     ST_GeogFromText('SRID=4326;POLYGON((90.347500 22.695000, 90.359500 22.695000, 90.359500 22.707000, 90.347500 22.707000, 90.347500 22.695000))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('RNP-SDR', 'Rangpur Sadar', 'RNP', 'RAN',
     ST_GeogFromText('SRID=4326;POINT(89.275200 25.743900)'),
     ST_GeogFromText('SRID=4326;POLYGON((89.269200 25.737900, 89.281200 25.737900, 89.281200 25.749900, 89.269200 25.749900, 89.269200 25.737900))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();

INSERT INTO geo_areas (code, name, district_code, division_code, centre, boundary) VALUES
    ('MYS-SDR', 'Mymensingh Sadar', 'MYS', 'MYM',
     ST_GeogFromText('SRID=4326;POINT(90.420300 24.747100)'),
     ST_GeogFromText('SRID=4326;POLYGON((90.414300 24.741100, 90.426300 24.741100, 90.426300 24.753100, 90.414300 24.753100, 90.414300 24.741100))'))
ON CONFLICT (code) DO UPDATE
    SET name = EXCLUDED.name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, centre = EXCLUDED.centre,
        boundary = EXCLUDED.boundary, updated_at = now();


-- Merchant locations --------------------------------------------------------
-- Clustered a few hundred metres apart inside each area, so a 1 km radius
-- search returns some of them and a 5 km search returns more. That gradient is
-- what makes stepwise expansion (D2) visible in a demo.

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0001', ST_GeogFromText('SRID=4326;POINT(90.373800 23.745500)'), 'DHA', 'DHK-DHM', TRUE)  -- Kacchi Bhai Dhanmondi
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0002', ST_GeogFromText('SRID=4326;POINT(90.376100 23.748200)'), 'DHA', 'DHK-DHM', TRUE)  -- Sultan's Dine Dhanmondi
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0003', ST_GeogFromText('SRID=4326;POINT(90.414800 23.793100)'), 'DHA', 'DHK-GUL', TRUE)  -- Star Kabab Gulshan
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0004', ST_GeogFromText('SRID=4326;POINT(90.416600 23.791000)'), 'DHA', 'DHK-GUL', TRUE)  -- Cheez Gulshan
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0005', ST_GeogFromText('SRID=4326;POINT(90.366100 23.823000)'), 'DHA', 'DHK-MIR', TRUE)  -- Shwapno Mirpur
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0006', ST_GeogFromText('SRID=4326;POINT(90.364000 23.821400)'), 'DHA', 'DHK-MIR', TRUE)  -- Agora Mirpur
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0007', ST_GeogFromText('SRID=4326;POINT(90.380100 23.876300)'), 'DHA', 'DHK-UTT', TRUE)  -- Lazz Pharma Uttara
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0008', ST_GeogFromText('SRID=4326;POINT(90.378800 23.875000)'), 'DHA', 'DHK-UTT', TRUE)  -- Unimart Uttara
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0009', ST_GeogFromText('SRID=4326;POINT(90.416800 23.733500)'), 'DHA', 'DHK-MTJ', TRUE)  -- Nanna Biryani Motijheel
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0010', ST_GeogFromText('SRID=4326;POINT(90.358600 23.759700)'), 'DHA', 'DHK-MPR', TRUE)  -- Meena Bazar Mohammadpur
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0011', ST_GeogFromText('SRID=4326;POINT(91.811800 22.328400)'), 'CTG', 'CTB-AGR', TRUE)  -- Bhai Bhai Store Agrabad
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0012', ST_GeogFromText('SRID=4326;POINT(91.810500 22.361700)'), 'CTG', 'CTB-KHU', TRUE)  -- Khulshi Mart
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0013', ST_GeogFromText('SRID=4326;POINT(91.868100 24.894500)'), 'SYL', 'SYH-ZND', TRUE)  -- Panshi Restaurant
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO geo_merchant_locations (merchant_id, location, division_code, area_code, is_active) VALUES
    ('MER-DEMO-0014', ST_GeogFromText('SRID=4326;POINT(89.539800 22.846000)'), 'KHU', 'KHL-SDR', TRUE)  -- Khulna Grocers
ON CONFLICT (merchant_id) DO UPDATE
    SET location = EXCLUDED.location, division_code = EXCLUDED.division_code,
        area_code = EXCLUDED.area_code, is_active = EXCLUDED.is_active, updated_at = now();


COMMIT;
