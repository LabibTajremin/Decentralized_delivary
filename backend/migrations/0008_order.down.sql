DROP INDEX IF EXISTS order_events_order_idx;
DROP INDEX IF EXISTS order_lines_order_idx;
DROP INDEX IF EXISTS orders_live_idx;
DROP INDEX IF EXISTS orders_merchant_idx;
DROP INDEX IF EXISTS orders_customer_idx;
DROP INDEX IF EXISTS orders_code_idx;

DROP TABLE IF EXISTS order_idempotency_keys;
DROP TABLE IF EXISTS order_events;
DROP TABLE IF EXISTS order_line_options;
DROP TABLE IF EXISTS order_lines;
DROP TABLE IF EXISTS orders;
