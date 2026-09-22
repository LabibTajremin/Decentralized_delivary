-- P06 demo merchants.
--
-- One shop per point already seeded into geo_merchant_locations by 0001, with
-- the same ids, so the radius search and the merchant record agree. Every shop
-- is approved and fully documented: a demo whose merchants all sit in draft
-- shows an empty map, which is the opposite of what a demo is for.
--
-- The document numbers are obviously fake and the scans point at generated
-- placeholder images. `migrate seed` refuses to run when APP_ENV=production,
-- which is what keeps them out of a real database.

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0001', 'USR-DEMO-0001', 'কাচ্চি ভাই ধানমন্ডি', 'restaurant', 'approved',
    '+8801711000001', '', '/static/demo/merchants/MER-DEMO-0001-logo.png',
    'House 32, Road 27, Dhanmondi', '', ST_GeogFromText('SRID=4326;POINT(90.373800 23.745500)'),
    'DHK-DHM', 'Dhanmondi', 'DHK', 'DHA',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0001', 'trade_licence', 'DEMO-0001-1', '/static/demo/merchants/MER-DEMO-0001-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0001', 'national_id', 'DEMO-0001-2', '/static/demo/merchants/MER-DEMO-0001-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0001', 'food_licence', 'DEMO-0001-3', '/static/demo/merchants/MER-DEMO-0001-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0002', 'USR-DEMO-0002', 'সুলতান''স ডাইন ধানমন্ডি', 'restaurant', 'approved',
    '+8801711000002', '', '/static/demo/merchants/MER-DEMO-0002-logo.png',
    'Road 2, Dhanmondi', '', ST_GeogFromText('SRID=4326;POINT(90.376100 23.748200)'),
    'DHK-DHM', 'Dhanmondi', 'DHK', 'DHA',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0002', 'trade_licence', 'DEMO-0002-1', '/static/demo/merchants/MER-DEMO-0002-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0002', 'national_id', 'DEMO-0002-2', '/static/demo/merchants/MER-DEMO-0002-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0002', 'food_licence', 'DEMO-0002-3', '/static/demo/merchants/MER-DEMO-0002-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0003', 'USR-DEMO-0003', 'স্টার কাবাব গুলশান', 'restaurant', 'approved',
    '+8801711000003', '', '/static/demo/merchants/MER-DEMO-0003-logo.png',
    'Gulshan Avenue, Gulshan 1', '', ST_GeogFromText('SRID=4326;POINT(90.414800 23.793100)'),
    'DHK-GUL', 'Gulshan', 'DHK', 'DHA',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0003', 'trade_licence', 'DEMO-0003-1', '/static/demo/merchants/MER-DEMO-0003-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0003', 'national_id', 'DEMO-0003-2', '/static/demo/merchants/MER-DEMO-0003-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0003', 'food_licence', 'DEMO-0003-3', '/static/demo/merchants/MER-DEMO-0003-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0004', 'USR-DEMO-0004', 'চিজ গুলশান', 'restaurant', 'approved',
    '+8801711000004', '', '/static/demo/merchants/MER-DEMO-0004-logo.png',
    'Road 11, Gulshan 1', '', ST_GeogFromText('SRID=4326;POINT(90.416600 23.791000)'),
    'DHK-GUL', 'Gulshan', 'DHK', 'DHA',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0004', 'trade_licence', 'DEMO-0004-1', '/static/demo/merchants/MER-DEMO-0004-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0004', 'national_id', 'DEMO-0004-2', '/static/demo/merchants/MER-DEMO-0004-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0004', 'food_licence', 'DEMO-0004-3', '/static/demo/merchants/MER-DEMO-0004-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0005', 'USR-DEMO-0005', 'স্বপ্ন মিরপুর', 'grocery', 'approved',
    '+8801711000005', '', '/static/demo/merchants/MER-DEMO-0005-logo.png',
    'Mirpur 10 Circle', '', ST_GeogFromText('SRID=4326;POINT(90.366100 23.823000)'),
    'DHK-MIR', 'Mirpur', 'DHK', 'DHA',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0005', 'trade_licence', 'DEMO-0005-1', '/static/demo/merchants/MER-DEMO-0005-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0005', 'national_id', 'DEMO-0005-2', '/static/demo/merchants/MER-DEMO-0005-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0006', 'USR-DEMO-0006', 'আগোরা মিরপুর', 'grocery', 'approved',
    '+8801711000006', '', '/static/demo/merchants/MER-DEMO-0006-logo.png',
    'Mirpur 6, Block C', '', ST_GeogFromText('SRID=4326;POINT(90.364000 23.821400)'),
    'DHK-MIR', 'Mirpur', 'DHK', 'DHA',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0006', 'trade_licence', 'DEMO-0006-1', '/static/demo/merchants/MER-DEMO-0006-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0006', 'national_id', 'DEMO-0006-2', '/static/demo/merchants/MER-DEMO-0006-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0007', 'USR-DEMO-0007', 'লাজ ফার্মা উত্তরা', 'pharmacy', 'approved',
    '+8801711000007', '', '/static/demo/merchants/MER-DEMO-0007-logo.png',
    'Sector 7, Uttara', '', ST_GeogFromText('SRID=4326;POINT(90.380100 23.876300)'),
    'DHK-UTT', 'Uttara', 'DHK', 'DHA',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0007', 'trade_licence', 'DEMO-0007-1', '/static/demo/merchants/MER-DEMO-0007-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0007', 'national_id', 'DEMO-0007-2', '/static/demo/merchants/MER-DEMO-0007-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0007', 'drug_licence', 'DEMO-0007-3', '/static/demo/merchants/MER-DEMO-0007-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0008', 'USR-DEMO-0008', 'ইউনিমার্ট উত্তরা', 'grocery', 'approved',
    '+8801711000008', '', '/static/demo/merchants/MER-DEMO-0008-logo.png',
    'Sector 4, Uttara', '', ST_GeogFromText('SRID=4326;POINT(90.378800 23.875000)'),
    'DHK-UTT', 'Uttara', 'DHK', 'DHA',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0008', 'trade_licence', 'DEMO-0008-1', '/static/demo/merchants/MER-DEMO-0008-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0008', 'national_id', 'DEMO-0008-2', '/static/demo/merchants/MER-DEMO-0008-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0009', 'USR-DEMO-0009', 'নান্না বিরিয়ানি মতিঝিল', 'restaurant', 'approved',
    '+8801711000009', '', '/static/demo/merchants/MER-DEMO-0009-logo.png',
    'Dilkusha, Motijheel', '', ST_GeogFromText('SRID=4326;POINT(90.416800 23.733500)'),
    'DHK-MTJ', 'Motijheel', 'DHK', 'DHA',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0009', 'trade_licence', 'DEMO-0009-1', '/static/demo/merchants/MER-DEMO-0009-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0009', 'national_id', 'DEMO-0009-2', '/static/demo/merchants/MER-DEMO-0009-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0009', 'food_licence', 'DEMO-0009-3', '/static/demo/merchants/MER-DEMO-0009-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0010', 'USR-DEMO-0010', 'মীনা বাজার মোহাম্মদপুর', 'grocery', 'approved',
    '+8801711000010', '', '/static/demo/merchants/MER-DEMO-0010-logo.png',
    'Ring Road, Mohammadpur', '', ST_GeogFromText('SRID=4326;POINT(90.358600 23.759700)'),
    'DHK-MPR', 'Mohammadpur', 'DHK', 'DHA',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0010', 'trade_licence', 'DEMO-0010-1', '/static/demo/merchants/MER-DEMO-0010-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0010', 'national_id', 'DEMO-0010-2', '/static/demo/merchants/MER-DEMO-0010-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0011', 'USR-DEMO-0011', 'ভাই ভাই স্টোর আগ্রাবাদ', 'grocery', 'approved',
    '+8801711000011', '', '/static/demo/merchants/MER-DEMO-0011-logo.png',
    'Agrabad Commercial Area', '', ST_GeogFromText('SRID=4326;POINT(91.811800 22.328400)'),
    'CTB-AGR', 'Agrabad', 'CTB', 'CTG',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0011', 'trade_licence', 'DEMO-0011-1', '/static/demo/merchants/MER-DEMO-0011-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0011', 'national_id', 'DEMO-0011-2', '/static/demo/merchants/MER-DEMO-0011-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0012', 'USR-DEMO-0012', 'খুলশী মার্ট', 'grocery', 'approved',
    '+8801711000012', '', '/static/demo/merchants/MER-DEMO-0012-logo.png',
    'Zakir Hossain Road, Khulshi', '', ST_GeogFromText('SRID=4326;POINT(91.810500 22.361700)'),
    'CTB-KHU', 'Khulshi', 'CTB', 'CTG',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0012', 'trade_licence', 'DEMO-0012-1', '/static/demo/merchants/MER-DEMO-0012-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0012', 'national_id', 'DEMO-0012-2', '/static/demo/merchants/MER-DEMO-0012-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0013', 'USR-DEMO-0013', 'পানসী রেস্টুরেন্ট', 'restaurant', 'approved',
    '+8801711000013', '', '/static/demo/merchants/MER-DEMO-0013-logo.png',
    'Jallarpar Road, Zindabazar', '', ST_GeogFromText('SRID=4326;POINT(91.868100 24.894500)'),
    'SYH-ZND', 'Zindabazar', 'SYH', 'SYL',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0013', 'trade_licence', 'DEMO-0013-1', '/static/demo/merchants/MER-DEMO-0013-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0013', 'national_id', 'DEMO-0013-2', '/static/demo/merchants/MER-DEMO-0013-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0013', 'food_licence', 'DEMO-0013-3', '/static/demo/merchants/MER-DEMO-0013-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchants (
    id, owner_user_id, name, type, status, phone, email, logo_url,
    line1, line2, pin, area_code, area_name, district_code, division_code,
    hours, holiday_active, review_note)
VALUES (
    'MER-DEMO-0014', 'USR-DEMO-0014', 'খুলনা গ্রোসার্স', 'grocery', 'approved',
    '+8801711000014', '', '/static/demo/merchants/MER-DEMO-0014-logo.png',
    'Shibbari More, Khulna Sadar', '', ST_GeogFromText('SRID=4326;POINT(89.539800 22.846000)'),
    'KHL-SDR', 'Khulna Sadar', 'KHL', 'KHU',
    '{"0":["09:00-22:00"],"1":["09:00-22:00"],"2":["09:00-22:00"],"3":["09:00-22:00"],"4":["09:00-22:00"],"5":["09:00-22:00"],"6":["09:00-22:00"]}'::jsonb,
    FALSE, '')
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, type = EXCLUDED.type, status = EXCLUDED.status,
        phone = EXCLUDED.phone, logo_url = EXCLUDED.logo_url,
        line1 = EXCLUDED.line1, pin = EXCLUDED.pin,
        area_code = EXCLUDED.area_code, area_name = EXCLUDED.area_name,
        district_code = EXCLUDED.district_code, division_code = EXCLUDED.division_code,
        hours = EXCLUDED.hours, updated_at = now();

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0014', 'trade_licence', 'DEMO-0014-1', '/static/demo/merchants/MER-DEMO-0014-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

INSERT INTO merchant_documents (merchant_id, kind, number, file_url)
VALUES ('MER-DEMO-0014', 'national_id', 'DEMO-0014-2', '/static/demo/merchants/MER-DEMO-0014-logo.png')
ON CONFLICT (merchant_id, kind) DO UPDATE
    SET number = EXCLUDED.number, file_url = EXCLUDED.file_url;

-- One decision per shop, so the admin console's history view has something to
-- show and the audit trail is not an empty table in a demo.
INSERT INTO merchant_status_events (id, merchant_id, from_status, to_status, actor_user_id, note)
SELECT 'MSE-DEMO-' || substring(id from 10), id, 'pending_review', 'approved', 'USR-DEMO-ADMIN', ''
FROM merchants WHERE id LIKE 'MER-DEMO-%'
ON CONFLICT (id) DO NOTHING;
