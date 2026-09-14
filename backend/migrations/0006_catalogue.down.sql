DROP INDEX IF EXISTS catalogue_combos_merchant_idx;
DROP INDEX IF EXISTS catalogue_addon_options_group_idx;
DROP INDEX IF EXISTS catalogue_addon_groups_item_idx;
DROP INDEX IF EXISTS catalogue_variant_options_group_idx;
DROP INDEX IF EXISTS catalogue_variant_groups_item_idx;
DROP INDEX IF EXISTS catalogue_items_name_trgm_idx;
DROP INDEX IF EXISTS catalogue_items_category_idx;
DROP INDEX IF EXISTS catalogue_items_merchant_idx;
DROP INDEX IF EXISTS catalogue_categories_merchant_idx;

DROP TABLE IF EXISTS catalogue_combo_lines;
DROP TABLE IF EXISTS catalogue_combos;
DROP TABLE IF EXISTS catalogue_addon_options;
DROP TABLE IF EXISTS catalogue_addon_groups;
DROP TABLE IF EXISTS catalogue_variant_options;
DROP TABLE IF EXISTS catalogue_variant_groups;
DROP TABLE IF EXISTS catalogue_items;
DROP TABLE IF EXISTS catalogue_categories;

-- pg_trgm is left installed. It is a database-wide extension that another
-- migration may also rely on, and dropping it on a rollback of this one would
-- take those indexes with it.
