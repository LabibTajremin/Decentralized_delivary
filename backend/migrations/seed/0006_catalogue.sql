-- P07 demo catalogue.
--
-- Menus for the fourteen shops the merchant seed (0005) approves and the geo
-- seed (0001) places on the map. Without these a demo opens a shop and finds
-- nothing to buy, which is the opposite of what a demo is for.
--
-- Each shop type gets a menu of its own shape, because that is the phase's
-- acceptance criterion made visible: a restaurant has variants and add-ons and
-- no shelf count, a grocery sells by unit and counts stock, a pharmacy carries
-- a generic name, a strength, and a prescription flag on the items that need
-- one.
--
-- `migrate seed` refuses to run when APP_ENV=production, which is what keeps
-- this out of a real database.

-- MER-DEMO-0001 (restaurant)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0001-1', 'MER-DEMO-0001', 'বিরিয়ানি ও কাচ্চি', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0001-11', 'MER-DEMO-0001', 'CAT-0001-1', 'restaurant', 'কাচ্চি বিরিয়ানি', 'Slow-cooked mutton kacchi with aromatic rice.',
        '/static/demo/merchants/MER-DEMO-0001-cover.png',
        35000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        40, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0001-12', 'MER-DEMO-0001', 'CAT-0001-1', 'restaurant', 'মোরগ পোলাও', 'Chicken pulao cooked the Old Dhaka way.',
        '/static/demo/merchants/MER-DEMO-0001-cover.png',
        28000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        30, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0001-13', 'MER-DEMO-0001', 'CAT-0001-1', 'restaurant', 'তেহারি', 'Beef tehari with mustard oil.',
        '/static/demo/merchants/MER-DEMO-0001-cover.png',
        22000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0001-2', 'MER-DEMO-0001', 'কাবাব', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0001-21', 'MER-DEMO-0001', 'CAT-0001-2', 'restaurant', 'চিকেন টিক্কা', 'Charcoal-grilled chicken tikka.',
        '/static/demo/merchants/MER-DEMO-0001-cover.png',
        18000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        20, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0001-22', 'MER-DEMO-0001', 'CAT-0001-2', 'restaurant', 'বিফ শিক কাবাব', 'Minced beef seekh kabab.',
        '/static/demo/merchants/MER-DEMO-0001-cover.png',
        24000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0001-3', 'MER-DEMO-0001', 'পানীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0001-31', 'MER-DEMO-0001', 'CAT-0001-3', 'restaurant', 'বোরহানি', 'Spiced yoghurt drink.',
        '/static/demo/merchants/MER-DEMO-0001-cover.png',
        6000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0001-32', 'MER-DEMO-0001', 'CAT-0001-3', 'restaurant', 'লাচ্ছি', 'Sweet yoghurt lassi.',
        '/static/demo/merchants/MER-DEMO-0001-cover.png',
        8000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0002 (restaurant)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0002-1', 'MER-DEMO-0002', 'বিরিয়ানি ও কাচ্চি', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0002-11', 'MER-DEMO-0002', 'CAT-0002-1', 'restaurant', 'কাচ্চি বিরিয়ানি', 'Slow-cooked mutton kacchi with aromatic rice.',
        '/static/demo/merchants/MER-DEMO-0002-cover.png',
        35000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        40, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0002-12', 'MER-DEMO-0002', 'CAT-0002-1', 'restaurant', 'মোরগ পোলাও', 'Chicken pulao cooked the Old Dhaka way.',
        '/static/demo/merchants/MER-DEMO-0002-cover.png',
        28000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        30, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0002-13', 'MER-DEMO-0002', 'CAT-0002-1', 'restaurant', 'তেহারি', 'Beef tehari with mustard oil.',
        '/static/demo/merchants/MER-DEMO-0002-cover.png',
        22000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0002-2', 'MER-DEMO-0002', 'কাবাব', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0002-21', 'MER-DEMO-0002', 'CAT-0002-2', 'restaurant', 'চিকেন টিক্কা', 'Charcoal-grilled chicken tikka.',
        '/static/demo/merchants/MER-DEMO-0002-cover.png',
        18000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        20, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0002-22', 'MER-DEMO-0002', 'CAT-0002-2', 'restaurant', 'বিফ শিক কাবাব', 'Minced beef seekh kabab.',
        '/static/demo/merchants/MER-DEMO-0002-cover.png',
        24000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0002-3', 'MER-DEMO-0002', 'পানীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0002-31', 'MER-DEMO-0002', 'CAT-0002-3', 'restaurant', 'বোরহানি', 'Spiced yoghurt drink.',
        '/static/demo/merchants/MER-DEMO-0002-cover.png',
        6000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0002-32', 'MER-DEMO-0002', 'CAT-0002-3', 'restaurant', 'লাচ্ছি', 'Sweet yoghurt lassi.',
        '/static/demo/merchants/MER-DEMO-0002-cover.png',
        8000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0003 (restaurant)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0003-1', 'MER-DEMO-0003', 'বিরিয়ানি ও কাচ্চি', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0003-11', 'MER-DEMO-0003', 'CAT-0003-1', 'restaurant', 'কাচ্চি বিরিয়ানি', 'Slow-cooked mutton kacchi with aromatic rice.',
        '/static/demo/merchants/MER-DEMO-0003-cover.png',
        35000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        40, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0003-12', 'MER-DEMO-0003', 'CAT-0003-1', 'restaurant', 'মোরগ পোলাও', 'Chicken pulao cooked the Old Dhaka way.',
        '/static/demo/merchants/MER-DEMO-0003-cover.png',
        28000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        30, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0003-13', 'MER-DEMO-0003', 'CAT-0003-1', 'restaurant', 'তেহারি', 'Beef tehari with mustard oil.',
        '/static/demo/merchants/MER-DEMO-0003-cover.png',
        22000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0003-2', 'MER-DEMO-0003', 'কাবাব', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0003-21', 'MER-DEMO-0003', 'CAT-0003-2', 'restaurant', 'চিকেন টিক্কা', 'Charcoal-grilled chicken tikka.',
        '/static/demo/merchants/MER-DEMO-0003-cover.png',
        18000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        20, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0003-22', 'MER-DEMO-0003', 'CAT-0003-2', 'restaurant', 'বিফ শিক কাবাব', 'Minced beef seekh kabab.',
        '/static/demo/merchants/MER-DEMO-0003-cover.png',
        24000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0003-3', 'MER-DEMO-0003', 'পানীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0003-31', 'MER-DEMO-0003', 'CAT-0003-3', 'restaurant', 'বোরহানি', 'Spiced yoghurt drink.',
        '/static/demo/merchants/MER-DEMO-0003-cover.png',
        6000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0003-32', 'MER-DEMO-0003', 'CAT-0003-3', 'restaurant', 'লাচ্ছি', 'Sweet yoghurt lassi.',
        '/static/demo/merchants/MER-DEMO-0003-cover.png',
        8000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0004 (restaurant)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0004-1', 'MER-DEMO-0004', 'বিরিয়ানি ও কাচ্চি', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0004-11', 'MER-DEMO-0004', 'CAT-0004-1', 'restaurant', 'কাচ্চি বিরিয়ানি', 'Slow-cooked mutton kacchi with aromatic rice.',
        '/static/demo/merchants/MER-DEMO-0004-cover.png',
        35000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        40, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0004-12', 'MER-DEMO-0004', 'CAT-0004-1', 'restaurant', 'মোরগ পোলাও', 'Chicken pulao cooked the Old Dhaka way.',
        '/static/demo/merchants/MER-DEMO-0004-cover.png',
        28000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        30, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0004-13', 'MER-DEMO-0004', 'CAT-0004-1', 'restaurant', 'তেহারি', 'Beef tehari with mustard oil.',
        '/static/demo/merchants/MER-DEMO-0004-cover.png',
        22000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0004-2', 'MER-DEMO-0004', 'কাবাব', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0004-21', 'MER-DEMO-0004', 'CAT-0004-2', 'restaurant', 'চিকেন টিক্কা', 'Charcoal-grilled chicken tikka.',
        '/static/demo/merchants/MER-DEMO-0004-cover.png',
        18000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        20, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0004-22', 'MER-DEMO-0004', 'CAT-0004-2', 'restaurant', 'বিফ শিক কাবাব', 'Minced beef seekh kabab.',
        '/static/demo/merchants/MER-DEMO-0004-cover.png',
        24000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0004-3', 'MER-DEMO-0004', 'পানীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0004-31', 'MER-DEMO-0004', 'CAT-0004-3', 'restaurant', 'বোরহানি', 'Spiced yoghurt drink.',
        '/static/demo/merchants/MER-DEMO-0004-cover.png',
        6000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0004-32', 'MER-DEMO-0004', 'CAT-0004-3', 'restaurant', 'লাচ্ছি', 'Sweet yoghurt lassi.',
        '/static/demo/merchants/MER-DEMO-0004-cover.png',
        8000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0005 (grocery)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0005-1', 'MER-DEMO-0005', 'চাল ও ডাল', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0005-11', 'MER-DEMO-0005', 'CAT-0005-1', 'grocery', 'মিনিকেট চাল', '',
        '/static/demo/merchants/MER-DEMO-0005-cover.png',
        7500, TRUE, 60, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0005-12', 'MER-DEMO-0005', 'CAT-0005-1', 'grocery', 'নাজিরশাইল চাল', '',
        '/static/demo/merchants/MER-DEMO-0005-cover.png',
        8200, TRUE, 45, 'kg', '1 kg', 'Teer',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0005-13', 'MER-DEMO-0005', 'CAT-0005-1', 'grocery', 'মসুর ডাল', '',
        '/static/demo/merchants/MER-DEMO-0005-cover.png',
        14000, TRUE, 30, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0005-2', 'MER-DEMO-0005', 'তেল ও মসলা', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0005-21', 'MER-DEMO-0005', 'CAT-0005-2', 'grocery', 'সয়াবিন তেল', '',
        '/static/demo/merchants/MER-DEMO-0005-cover.png',
        18000, TRUE, 25, 'litre', '1 litre', 'Rupchanda',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0005-22', 'MER-DEMO-0005', 'CAT-0005-2', 'grocery', 'সরিষার তেল', '',
        '/static/demo/merchants/MER-DEMO-0005-cover.png',
        24000, TRUE, 18, 'litre', '500 ml', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0005-23', 'MER-DEMO-0005', 'CAT-0005-2', 'grocery', 'হলুদ গুঁড়া', '',
        '/static/demo/merchants/MER-DEMO-0005-cover.png',
        5500, TRUE, 40, 'pack', '200 g', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0005-3', 'MER-DEMO-0005', 'নিত্যপ্রয়োজনীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0005-31', 'MER-DEMO-0005', 'CAT-0005-3', 'grocery', 'চিনি', '',
        '/static/demo/merchants/MER-DEMO-0005-cover.png',
        13500, TRUE, 35, 'kg', '1 kg', 'Fresh',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0005-32', 'MER-DEMO-0005', 'CAT-0005-3', 'grocery', 'লবণ', '',
        '/static/demo/merchants/MER-DEMO-0005-cover.png',
        4200, TRUE, 50, 'kg', '1 kg', 'ACI',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0006 (grocery)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0006-1', 'MER-DEMO-0006', 'চাল ও ডাল', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0006-11', 'MER-DEMO-0006', 'CAT-0006-1', 'grocery', 'মিনিকেট চাল', '',
        '/static/demo/merchants/MER-DEMO-0006-cover.png',
        7500, TRUE, 60, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0006-12', 'MER-DEMO-0006', 'CAT-0006-1', 'grocery', 'নাজিরশাইল চাল', '',
        '/static/demo/merchants/MER-DEMO-0006-cover.png',
        8200, TRUE, 45, 'kg', '1 kg', 'Teer',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0006-13', 'MER-DEMO-0006', 'CAT-0006-1', 'grocery', 'মসুর ডাল', '',
        '/static/demo/merchants/MER-DEMO-0006-cover.png',
        14000, TRUE, 30, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0006-2', 'MER-DEMO-0006', 'তেল ও মসলা', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0006-21', 'MER-DEMO-0006', 'CAT-0006-2', 'grocery', 'সয়াবিন তেল', '',
        '/static/demo/merchants/MER-DEMO-0006-cover.png',
        18000, TRUE, 25, 'litre', '1 litre', 'Rupchanda',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0006-22', 'MER-DEMO-0006', 'CAT-0006-2', 'grocery', 'সরিষার তেল', '',
        '/static/demo/merchants/MER-DEMO-0006-cover.png',
        24000, TRUE, 18, 'litre', '500 ml', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0006-23', 'MER-DEMO-0006', 'CAT-0006-2', 'grocery', 'হলুদ গুঁড়া', '',
        '/static/demo/merchants/MER-DEMO-0006-cover.png',
        5500, TRUE, 40, 'pack', '200 g', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0006-3', 'MER-DEMO-0006', 'নিত্যপ্রয়োজনীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0006-31', 'MER-DEMO-0006', 'CAT-0006-3', 'grocery', 'চিনি', '',
        '/static/demo/merchants/MER-DEMO-0006-cover.png',
        13500, TRUE, 35, 'kg', '1 kg', 'Fresh',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0006-32', 'MER-DEMO-0006', 'CAT-0006-3', 'grocery', 'লবণ', '',
        '/static/demo/merchants/MER-DEMO-0006-cover.png',
        4200, TRUE, 50, 'kg', '1 kg', 'ACI',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0007 (pharmacy)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0007-1', 'MER-DEMO-0007', 'ব্যথানাশক', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0007-11', 'MER-DEMO-0007', 'CAT-0007-1', 'pharmacy', 'নাপা ৫০০ মিগ্রা', '',
        '/static/demo/merchants/MER-DEMO-0007-cover.png',
        1200, TRUE, 80, 'strip', '10 tablets', 'Beximco',
        'Paracetamol', '500mg', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0007-12', 'MER-DEMO-0007', 'CAT-0007-1', 'pharmacy', 'এইস প্লাস', '',
        '/static/demo/merchants/MER-DEMO-0007-cover.png',
        2500, TRUE, 60, 'strip', '10 tablets', 'Square',
        'Paracetamol + Caffeine', '500mg', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0007-2', 'MER-DEMO-0007', 'অ্যান্টিবায়োটিক', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0007-21', 'MER-DEMO-0007', 'CAT-0007-2', 'pharmacy', 'মোনাস ১০', '',
        '/static/demo/merchants/MER-DEMO-0007-cover.png',
        9000, TRUE, 25, 'strip', '10 tablets', 'Acme',
        'Montelukast', '10mg', TRUE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0007-22', 'MER-DEMO-0007', 'CAT-0007-2', 'pharmacy', 'সেফ-৩ ২০০', '',
        '/static/demo/merchants/MER-DEMO-0007-cover.png',
        35000, TRUE, 15, 'strip', '10 capsules', 'Square',
        'Cefixime', '200mg', TRUE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0007-3', 'MER-DEMO-0007', 'স্বাস্থ্য সামগ্রী', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0007-31', 'MER-DEMO-0007', 'CAT-0007-3', 'pharmacy', 'হ্যান্ড স্যানিটাইজার', '',
        '/static/demo/merchants/MER-DEMO-0007-cover.png',
        8000, TRUE, 40, 'bottle', '100 ml', 'Savlon',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0008 (grocery)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0008-1', 'MER-DEMO-0008', 'চাল ও ডাল', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0008-11', 'MER-DEMO-0008', 'CAT-0008-1', 'grocery', 'মিনিকেট চাল', '',
        '/static/demo/merchants/MER-DEMO-0008-cover.png',
        7500, TRUE, 60, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0008-12', 'MER-DEMO-0008', 'CAT-0008-1', 'grocery', 'নাজিরশাইল চাল', '',
        '/static/demo/merchants/MER-DEMO-0008-cover.png',
        8200, TRUE, 45, 'kg', '1 kg', 'Teer',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0008-13', 'MER-DEMO-0008', 'CAT-0008-1', 'grocery', 'মসুর ডাল', '',
        '/static/demo/merchants/MER-DEMO-0008-cover.png',
        14000, TRUE, 30, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0008-2', 'MER-DEMO-0008', 'তেল ও মসলা', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0008-21', 'MER-DEMO-0008', 'CAT-0008-2', 'grocery', 'সয়াবিন তেল', '',
        '/static/demo/merchants/MER-DEMO-0008-cover.png',
        18000, TRUE, 25, 'litre', '1 litre', 'Rupchanda',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0008-22', 'MER-DEMO-0008', 'CAT-0008-2', 'grocery', 'সরিষার তেল', '',
        '/static/demo/merchants/MER-DEMO-0008-cover.png',
        24000, TRUE, 18, 'litre', '500 ml', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0008-23', 'MER-DEMO-0008', 'CAT-0008-2', 'grocery', 'হলুদ গুঁড়া', '',
        '/static/demo/merchants/MER-DEMO-0008-cover.png',
        5500, TRUE, 40, 'pack', '200 g', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0008-3', 'MER-DEMO-0008', 'নিত্যপ্রয়োজনীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0008-31', 'MER-DEMO-0008', 'CAT-0008-3', 'grocery', 'চিনি', '',
        '/static/demo/merchants/MER-DEMO-0008-cover.png',
        13500, TRUE, 35, 'kg', '1 kg', 'Fresh',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0008-32', 'MER-DEMO-0008', 'CAT-0008-3', 'grocery', 'লবণ', '',
        '/static/demo/merchants/MER-DEMO-0008-cover.png',
        4200, TRUE, 50, 'kg', '1 kg', 'ACI',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0009 (restaurant)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0009-1', 'MER-DEMO-0009', 'বিরিয়ানি ও কাচ্চি', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0009-11', 'MER-DEMO-0009', 'CAT-0009-1', 'restaurant', 'কাচ্চি বিরিয়ানি', 'Slow-cooked mutton kacchi with aromatic rice.',
        '/static/demo/merchants/MER-DEMO-0009-cover.png',
        35000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        40, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0009-12', 'MER-DEMO-0009', 'CAT-0009-1', 'restaurant', 'মোরগ পোলাও', 'Chicken pulao cooked the Old Dhaka way.',
        '/static/demo/merchants/MER-DEMO-0009-cover.png',
        28000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        30, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0009-13', 'MER-DEMO-0009', 'CAT-0009-1', 'restaurant', 'তেহারি', 'Beef tehari with mustard oil.',
        '/static/demo/merchants/MER-DEMO-0009-cover.png',
        22000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0009-2', 'MER-DEMO-0009', 'কাবাব', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0009-21', 'MER-DEMO-0009', 'CAT-0009-2', 'restaurant', 'চিকেন টিক্কা', 'Charcoal-grilled chicken tikka.',
        '/static/demo/merchants/MER-DEMO-0009-cover.png',
        18000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        20, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0009-22', 'MER-DEMO-0009', 'CAT-0009-2', 'restaurant', 'বিফ শিক কাবাব', 'Minced beef seekh kabab.',
        '/static/demo/merchants/MER-DEMO-0009-cover.png',
        24000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0009-3', 'MER-DEMO-0009', 'পানীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0009-31', 'MER-DEMO-0009', 'CAT-0009-3', 'restaurant', 'বোরহানি', 'Spiced yoghurt drink.',
        '/static/demo/merchants/MER-DEMO-0009-cover.png',
        6000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0009-32', 'MER-DEMO-0009', 'CAT-0009-3', 'restaurant', 'লাচ্ছি', 'Sweet yoghurt lassi.',
        '/static/demo/merchants/MER-DEMO-0009-cover.png',
        8000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0010 (grocery)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0010-1', 'MER-DEMO-0010', 'চাল ও ডাল', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0010-11', 'MER-DEMO-0010', 'CAT-0010-1', 'grocery', 'মিনিকেট চাল', '',
        '/static/demo/merchants/MER-DEMO-0010-cover.png',
        7500, TRUE, 60, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0010-12', 'MER-DEMO-0010', 'CAT-0010-1', 'grocery', 'নাজিরশাইল চাল', '',
        '/static/demo/merchants/MER-DEMO-0010-cover.png',
        8200, TRUE, 45, 'kg', '1 kg', 'Teer',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0010-13', 'MER-DEMO-0010', 'CAT-0010-1', 'grocery', 'মসুর ডাল', '',
        '/static/demo/merchants/MER-DEMO-0010-cover.png',
        14000, TRUE, 30, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0010-2', 'MER-DEMO-0010', 'তেল ও মসলা', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0010-21', 'MER-DEMO-0010', 'CAT-0010-2', 'grocery', 'সয়াবিন তেল', '',
        '/static/demo/merchants/MER-DEMO-0010-cover.png',
        18000, TRUE, 25, 'litre', '1 litre', 'Rupchanda',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0010-22', 'MER-DEMO-0010', 'CAT-0010-2', 'grocery', 'সরিষার তেল', '',
        '/static/demo/merchants/MER-DEMO-0010-cover.png',
        24000, TRUE, 18, 'litre', '500 ml', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0010-23', 'MER-DEMO-0010', 'CAT-0010-2', 'grocery', 'হলুদ গুঁড়া', '',
        '/static/demo/merchants/MER-DEMO-0010-cover.png',
        5500, TRUE, 40, 'pack', '200 g', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0010-3', 'MER-DEMO-0010', 'নিত্যপ্রয়োজনীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0010-31', 'MER-DEMO-0010', 'CAT-0010-3', 'grocery', 'চিনি', '',
        '/static/demo/merchants/MER-DEMO-0010-cover.png',
        13500, TRUE, 35, 'kg', '1 kg', 'Fresh',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0010-32', 'MER-DEMO-0010', 'CAT-0010-3', 'grocery', 'লবণ', '',
        '/static/demo/merchants/MER-DEMO-0010-cover.png',
        4200, TRUE, 50, 'kg', '1 kg', 'ACI',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0011 (grocery)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0011-1', 'MER-DEMO-0011', 'চাল ও ডাল', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0011-11', 'MER-DEMO-0011', 'CAT-0011-1', 'grocery', 'মিনিকেট চাল', '',
        '/static/demo/merchants/MER-DEMO-0011-cover.png',
        7500, TRUE, 60, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0011-12', 'MER-DEMO-0011', 'CAT-0011-1', 'grocery', 'নাজিরশাইল চাল', '',
        '/static/demo/merchants/MER-DEMO-0011-cover.png',
        8200, TRUE, 45, 'kg', '1 kg', 'Teer',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0011-13', 'MER-DEMO-0011', 'CAT-0011-1', 'grocery', 'মসুর ডাল', '',
        '/static/demo/merchants/MER-DEMO-0011-cover.png',
        14000, TRUE, 30, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0011-2', 'MER-DEMO-0011', 'তেল ও মসলা', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0011-21', 'MER-DEMO-0011', 'CAT-0011-2', 'grocery', 'সয়াবিন তেল', '',
        '/static/demo/merchants/MER-DEMO-0011-cover.png',
        18000, TRUE, 25, 'litre', '1 litre', 'Rupchanda',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0011-22', 'MER-DEMO-0011', 'CAT-0011-2', 'grocery', 'সরিষার তেল', '',
        '/static/demo/merchants/MER-DEMO-0011-cover.png',
        24000, TRUE, 18, 'litre', '500 ml', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0011-23', 'MER-DEMO-0011', 'CAT-0011-2', 'grocery', 'হলুদ গুঁড়া', '',
        '/static/demo/merchants/MER-DEMO-0011-cover.png',
        5500, TRUE, 40, 'pack', '200 g', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0011-3', 'MER-DEMO-0011', 'নিত্যপ্রয়োজনীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0011-31', 'MER-DEMO-0011', 'CAT-0011-3', 'grocery', 'চিনি', '',
        '/static/demo/merchants/MER-DEMO-0011-cover.png',
        13500, TRUE, 35, 'kg', '1 kg', 'Fresh',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0011-32', 'MER-DEMO-0011', 'CAT-0011-3', 'grocery', 'লবণ', '',
        '/static/demo/merchants/MER-DEMO-0011-cover.png',
        4200, TRUE, 50, 'kg', '1 kg', 'ACI',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0012 (grocery)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0012-1', 'MER-DEMO-0012', 'চাল ও ডাল', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0012-11', 'MER-DEMO-0012', 'CAT-0012-1', 'grocery', 'মিনিকেট চাল', '',
        '/static/demo/merchants/MER-DEMO-0012-cover.png',
        7500, TRUE, 60, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0012-12', 'MER-DEMO-0012', 'CAT-0012-1', 'grocery', 'নাজিরশাইল চাল', '',
        '/static/demo/merchants/MER-DEMO-0012-cover.png',
        8200, TRUE, 45, 'kg', '1 kg', 'Teer',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0012-13', 'MER-DEMO-0012', 'CAT-0012-1', 'grocery', 'মসুর ডাল', '',
        '/static/demo/merchants/MER-DEMO-0012-cover.png',
        14000, TRUE, 30, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0012-2', 'MER-DEMO-0012', 'তেল ও মসলা', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0012-21', 'MER-DEMO-0012', 'CAT-0012-2', 'grocery', 'সয়াবিন তেল', '',
        '/static/demo/merchants/MER-DEMO-0012-cover.png',
        18000, TRUE, 25, 'litre', '1 litre', 'Rupchanda',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0012-22', 'MER-DEMO-0012', 'CAT-0012-2', 'grocery', 'সরিষার তেল', '',
        '/static/demo/merchants/MER-DEMO-0012-cover.png',
        24000, TRUE, 18, 'litre', '500 ml', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0012-23', 'MER-DEMO-0012', 'CAT-0012-2', 'grocery', 'হলুদ গুঁড়া', '',
        '/static/demo/merchants/MER-DEMO-0012-cover.png',
        5500, TRUE, 40, 'pack', '200 g', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0012-3', 'MER-DEMO-0012', 'নিত্যপ্রয়োজনীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0012-31', 'MER-DEMO-0012', 'CAT-0012-3', 'grocery', 'চিনি', '',
        '/static/demo/merchants/MER-DEMO-0012-cover.png',
        13500, TRUE, 35, 'kg', '1 kg', 'Fresh',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0012-32', 'MER-DEMO-0012', 'CAT-0012-3', 'grocery', 'লবণ', '',
        '/static/demo/merchants/MER-DEMO-0012-cover.png',
        4200, TRUE, 50, 'kg', '1 kg', 'ACI',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0013 (restaurant)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0013-1', 'MER-DEMO-0013', 'বিরিয়ানি ও কাচ্চি', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0013-11', 'MER-DEMO-0013', 'CAT-0013-1', 'restaurant', 'কাচ্চি বিরিয়ানি', 'Slow-cooked mutton kacchi with aromatic rice.',
        '/static/demo/merchants/MER-DEMO-0013-cover.png',
        35000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        40, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0013-12', 'MER-DEMO-0013', 'CAT-0013-1', 'restaurant', 'মোরগ পোলাও', 'Chicken pulao cooked the Old Dhaka way.',
        '/static/demo/merchants/MER-DEMO-0013-cover.png',
        28000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        30, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0013-13', 'MER-DEMO-0013', 'CAT-0013-1', 'restaurant', 'তেহারি', 'Beef tehari with mustard oil.',
        '/static/demo/merchants/MER-DEMO-0013-cover.png',
        22000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0013-2', 'MER-DEMO-0013', 'কাবাব', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0013-21', 'MER-DEMO-0013', 'CAT-0013-2', 'restaurant', 'চিকেন টিক্কা', 'Charcoal-grilled chicken tikka.',
        '/static/demo/merchants/MER-DEMO-0013-cover.png',
        18000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        20, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0013-22', 'MER-DEMO-0013', 'CAT-0013-2', 'restaurant', 'বিফ শিক কাবাব', 'Minced beef seekh kabab.',
        '/static/demo/merchants/MER-DEMO-0013-cover.png',
        24000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        25, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0013-3', 'MER-DEMO-0013', 'পানীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0013-31', 'MER-DEMO-0013', 'CAT-0013-3', 'restaurant', 'বোরহানি', 'Spiced yoghurt drink.',
        '/static/demo/merchants/MER-DEMO-0013-cover.png',
        6000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0013-32', 'MER-DEMO-0013', 'CAT-0013-3', 'restaurant', 'লাচ্ছি', 'Sweet yoghurt lassi.',
        '/static/demo/merchants/MER-DEMO-0013-cover.png',
        8000, FALSE, 0, '', '', '',
        '', '', FALSE, FALSE,
        5, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0014 (grocery)

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0014-1', 'MER-DEMO-0014', 'চাল ও ডাল', 1, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0014-11', 'MER-DEMO-0014', 'CAT-0014-1', 'grocery', 'মিনিকেট চাল', '',
        '/static/demo/merchants/MER-DEMO-0014-cover.png',
        7500, TRUE, 60, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0014-12', 'MER-DEMO-0014', 'CAT-0014-1', 'grocery', 'নাজিরশাইল চাল', '',
        '/static/demo/merchants/MER-DEMO-0014-cover.png',
        8200, TRUE, 45, 'kg', '1 kg', 'Teer',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0014-13', 'MER-DEMO-0014', 'CAT-0014-1', 'grocery', 'মসুর ডাল', '',
        '/static/demo/merchants/MER-DEMO-0014-cover.png',
        14000, TRUE, 30, 'kg', '1 kg', 'Pran',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0014-2', 'MER-DEMO-0014', 'তেল ও মসলা', 2, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0014-21', 'MER-DEMO-0014', 'CAT-0014-2', 'grocery', 'সয়াবিন তেল', '',
        '/static/demo/merchants/MER-DEMO-0014-cover.png',
        18000, TRUE, 25, 'litre', '1 litre', 'Rupchanda',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0014-22', 'MER-DEMO-0014', 'CAT-0014-2', 'grocery', 'সরিষার তেল', '',
        '/static/demo/merchants/MER-DEMO-0014-cover.png',
        24000, TRUE, 18, 'litre', '500 ml', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0014-23', 'MER-DEMO-0014', 'CAT-0014-2', 'grocery', 'হলুদ গুঁড়া', '',
        '/static/demo/merchants/MER-DEMO-0014-cover.png',
        5500, TRUE, 40, 'pack', '200 g', 'Radhuni',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 3)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_categories (id, merchant_id, name, sort_order, active)
VALUES ('CAT-0014-3', 'MER-DEMO-0014', 'নিত্যপ্রয়োজনীয়', 3, TRUE)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order,
        active = EXCLUDED.active, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0014-31', 'MER-DEMO-0014', 'CAT-0014-3', 'grocery', 'চিনি', '',
        '/static/demo/merchants/MER-DEMO-0014-cover.png',
        13500, TRUE, 35, 'kg', '1 kg', 'Fresh',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_items (
    id, merchant_id, category_id, merchant_type, name, description, image_url,
    price_minor, stock_tracked, stock_quantity, unit, pack_size, brand,
    generic_name, strength, requires_prescription, is_vegetarian,
    preparation_minutes, active, availability, sort_order)
VALUES ('ITM-0014-32', 'MER-DEMO-0014', 'CAT-0014-3', 'grocery', 'লবণ', '',
        '/static/demo/merchants/MER-DEMO-0014-cover.png',
        4200, TRUE, 50, 'kg', '1 kg', 'ACI',
        '', '', FALSE, FALSE,
        0, TRUE, '{}'::jsonb, 2)
ON CONFLICT (id) DO UPDATE
    SET category_id = EXCLUDED.category_id, name = EXCLUDED.name,
        description = EXCLUDED.description, price_minor = EXCLUDED.price_minor,
        stock_tracked = EXCLUDED.stock_tracked, stock_quantity = EXCLUDED.stock_quantity,
        unit = EXCLUDED.unit, pack_size = EXCLUDED.pack_size, brand = EXCLUDED.brand,
        generic_name = EXCLUDED.generic_name, strength = EXCLUDED.strength,
        requires_prescription = EXCLUDED.requires_prescription,
        preparation_minutes = EXCLUDED.preparation_minutes,
        sort_order = EXCLUDED.sort_order, updated_at = now();

-- MER-DEMO-0001 shows the full restaurant shape: a required size, optional
-- extras, and a combo priced below its parts.

INSERT INTO catalogue_variant_groups (id, item_id, name, required, min_choices, max_choices, sort_order)
VALUES ('VGR-0001-SIZE', 'ITM-0001-11', 'সাইজ', TRUE, 1, 1, 1)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, required = EXCLUDED.required,
        min_choices = EXCLUDED.min_choices, max_choices = EXCLUDED.max_choices;

INSERT INTO catalogue_variant_options (id, group_id, name, price_delta_minor, available, sort_order)
VALUES ('VOP-0001-HALF', 'VGR-0001-SIZE', 'হাফ', -10000, TRUE, 0),
       ('VOP-0001-FULL', 'VGR-0001-SIZE', 'ফুল', 0, TRUE, 1)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, price_delta_minor = EXCLUDED.price_delta_minor,
        available = EXCLUDED.available, sort_order = EXCLUDED.sort_order;

INSERT INTO catalogue_addon_groups (id, item_id, name, min_choices, max_choices, sort_order)
VALUES ('AGR-0001-EXTRA', 'ITM-0001-11', 'এক্সট্রা', 0, 2, 1)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, min_choices = EXCLUDED.min_choices,
        max_choices = EXCLUDED.max_choices;

INSERT INTO catalogue_addon_options (id, group_id, name, price_minor, available, sort_order)
VALUES ('AOP-0001-EGG', 'AGR-0001-EXTRA', 'ডিম', 3000, TRUE, 0),
       ('AOP-0001-SALAD', 'AGR-0001-EXTRA', 'সালাদ', 2000, TRUE, 1)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, price_minor = EXCLUDED.price_minor,
        available = EXCLUDED.available, sort_order = EXCLUDED.sort_order;

-- Kacchi (৳350) plus a borhani (৳60) is ৳410 apart, ৳380 together.
INSERT INTO catalogue_combos (
    id, merchant_id, merchant_type, name, description, image_url,
    price_minor, active, availability, sort_order)
VALUES ('CMB-0001-MEAL', 'MER-DEMO-0001', 'restaurant', 'কাচ্চি মিল',
        'কাচ্চি বিরিয়ানি ও বোরহানি একসাথে।',
        '/static/demo/merchants/MER-DEMO-0001-cover.png',
        38000, TRUE, '{}'::jsonb, 1)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, description = EXCLUDED.description,
        price_minor = EXCLUDED.price_minor, active = EXCLUDED.active,
        sort_order = EXCLUDED.sort_order, updated_at = now();

INSERT INTO catalogue_combo_lines (combo_id, item_id, quantity)
VALUES ('CMB-0001-MEAL', 'ITM-0001-11', 1),
       ('CMB-0001-MEAL', 'ITM-0001-31', 1)
ON CONFLICT (combo_id, item_id) DO UPDATE SET quantity = EXCLUDED.quantity;
