-- Demo accounts: the sign-ins a published demonstration is shown with.
--
-- Everything before this file gives the demo a world — fourteen approved
-- shops, their menus, the real administrative geography. What it does not
-- give is a way in: identity_accounts is empty, so the first person to sign
-- in creates a brand-new account with no shop, no deliveries and no history,
-- and sees an empty app.
--
-- These rows are the way in. Seven accounts, one or two per role, each
-- already owning whatever that role needs to have something to do:
--
--   Customer  a saved address, so Home has somewhere to search from
--   Merchant  one of the seeded shops, approved, stocked and open all day
--   Partner   a registered rider with a position, off shift
--   Admin     the approval queue, the config surface and the dispatch sweep
--
-- They are ordinary accounts. Nothing here grants a privilege the same person
-- could not get by signing up, and nothing skips a step: a demo visitor
-- requests a code and verifies it exactly as a real user would. What makes
-- the demo work is DEMO_MODE showing them the code, not anything in this file.
--
-- `migrate seed` refuses to run when APP_ENV=production, which is what keeps
-- these numbers out of a real deployment. See docs/demo.md.

-- ---------------------------------------------------------------- accounts
--
-- One account per (phone, role), which is the unique key EnsureUser conflicts
-- on: signing in with one of these numbers in the matching app returns the id
-- below rather than creating a new account. That is the whole mechanism — the
-- same number in a different app is a different account, exactly as it is for
-- a real person who both orders food and delivers it.

INSERT INTO identity_accounts (id, phone, role) VALUES
    ('USR-DEMO-C001', '+8801700000001', 'customer'),
    ('USR-DEMO-C002', '+8801700000002', 'customer'),
    -- The merchant ids are not new: 0005_merchant.sql already made these two
    -- users the owners of two of its shops. Giving them a phone number is all
    -- that was missing.
    ('USR-DEMO-0001', '+8801700000011', 'merchant'),
    ('USR-DEMO-0005', '+8801700000012', 'merchant'),
    ('USR-DEMO-P001', '+8801700000021', 'partner'),
    ('USR-DEMO-P002', '+8801700000022', 'partner'),
    ('USR-DEMO-A001', '+8801700000031', 'admin')
ON CONFLICT (phone, role) DO UPDATE
    SET is_active = TRUE, updated_at = now();

-- ---------------------------------------------------------------- profiles

INSERT INTO user_profiles (user_id, name, email, language) VALUES
    ('USR-DEMO-C001', 'আয়েশা রহমান', 'ayesha@example.com', 'bn'),
    ('USR-DEMO-C002', 'তানভীর হাসান', 'tanvir@example.com', 'bn'),
    ('USR-DEMO-0001', 'কাচ্চি ভাই ধানমন্ডি', '', 'bn'),
    ('USR-DEMO-0005', 'স্বপ্ন মিরপুর', '', 'bn'),
    ('USR-DEMO-P001', 'করিম উদ্দিন', '', 'bn'),
    ('USR-DEMO-P002', 'রফিকুল ইসলাম', '', 'bn'),
    ('USR-DEMO-A001', 'ডেমো অ্যাডমিন', '', 'bn')
ON CONFLICT (user_id) DO UPDATE
    SET name = EXCLUDED.name, email = EXCLUDED.email,
        language = EXCLUDED.language, updated_at = now();

-- --------------------------------------------------------------- addresses
--
-- Two customers in two different areas, on purpose. Dhanmondi reaches the
-- demo restaurant and Mirpur reaches the demo grocery, so the two halves of
-- the catalogue — a menu with options, and shelves with stock counts — are
-- both one sign-in away. It also means a demo can show the same search
-- returning a different answer from a different address, which is the whole
-- idea behind radius-scoped discovery.
--
-- The area, district and division are written here because the seed cannot
-- call P02 to resolve them. They are the real codes from 0001_geo.sql, and
-- the coordinates really are inside those areas' boundaries.

INSERT INTO user_addresses (
    id, user_id, label, recipient_name, recipient_phone,
    line1, line2, instructions, pin,
    area_code, area_name, district_code, division_code, is_default)
VALUES
    ('ADR-DEMO-0001', 'USR-DEMO-C001', 'বাসা', 'আয়েশা রহমান', '+8801700000001',
     'House 32, Road 27', 'Dhanmondi', 'নীল গেট',
     ST_GeogFromText('SRID=4326;POINT(90.373800 23.745500)'),
     'DHK-DHM', 'Dhanmondi', 'DHK', 'DHA', TRUE),
    ('ADR-DEMO-0002', 'USR-DEMO-C002', 'বাসা', 'তানভীর হাসান', '+8801700000002',
     'Block C, Road 4', 'Mirpur', 'তিন তলা',
     ST_GeogFromText('SRID=4326;POINT(90.365400 23.822300)'),
     'DHK-MIR', 'Mirpur', 'DHK', 'DHA', TRUE)
ON CONFLICT (id) DO UPDATE
    SET label = EXCLUDED.label, recipient_name = EXCLUDED.recipient_name,
        recipient_phone = EXCLUDED.recipient_phone, line1 = EXCLUDED.line1,
        line2 = EXCLUDED.line2, instructions = EXCLUDED.instructions,
        pin = EXCLUDED.pin, area_code = EXCLUDED.area_code,
        area_name = EXCLUDED.area_name, district_code = EXCLUDED.district_code,
        division_code = EXCLUDED.division_code, updated_at = now();

-- ---------------------------------------------------------------- partners
--
-- Registered, positioned, and **off shift**. Going on shift is the first
-- thing a rider does and the thing a demo most needs to show — dispatch's
-- first offer round asks only partners who are already available, so a rider
-- who has not gone on shift is a rider with an empty feed and a notice
-- explaining why. Seeding them online would hide the step that matters.
--
-- Two riders, in the two areas the two customers order from, so a delivery
-- can be walked in either.
--
-- The acceptance counters start at zero, which the server reads as 100%: a
-- rider who has never been offered anything has never declined anything.

INSERT INTO delivery_partners (
    id, user_id, name, phone, vehicle, availability, preference, pin,
    carrying, offered, accepted)
VALUES
    ('PTR-DEMO-0001', 'USR-DEMO-P001', 'করিম উদ্দিন', '+8801700000021',
     'motorcycle', 'offline', 'any',
     ST_GeogFromText('SRID=4326;POINT(90.374200 23.746100)'), 0, 0, 0),
    ('PTR-DEMO-0002', 'USR-DEMO-P002', 'রফিকুল ইসলাম', '+8801700000022',
     'bicycle', 'offline', 'short',
     ST_GeogFromText('SRID=4326;POINT(90.365400 23.822300)'), 0, 0, 0)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name, phone = EXCLUDED.phone,
        vehicle = EXCLUDED.vehicle, preference = EXCLUDED.preference,
        pin = EXCLUDED.pin, updated_at = now();

-- ------------------------------------------------------- always-open demo
--
-- The other twelve seeded shops keep realistic 09:00–22:00 hours, which is
-- right: a demo that showed every shop open at four in the morning would be
-- teaching something false about how the product behaves.
--
-- These two are the exception, because they are the two a visitor signs in as
-- and orders from. A demo that works in the afternoon and fails overnight is
-- a demo that fails in front of whoever is nine time zones away, and "come
-- back at nine" is not an answer.

UPDATE merchants
SET hours = '{"0":["00:00-24:00"],"1":["00:00-24:00"],"2":["00:00-24:00"],
              "3":["00:00-24:00"],"4":["00:00-24:00"],"5":["00:00-24:00"],
              "6":["00:00-24:00"]}'::jsonb,
    updated_at = now()
WHERE id IN ('MER-DEMO-0001', 'MER-DEMO-0005');
