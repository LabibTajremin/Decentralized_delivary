DROP INDEX IF EXISTS delivery_jobs_expiring_idx;
DROP INDEX IF EXISTS delivery_jobs_waiting_idx;
DROP INDEX IF EXISTS delivery_jobs_partner_idx;
DROP INDEX IF EXISTS delivery_jobs_order_idx;
DROP INDEX IF EXISTS delivery_partners_available_idx;
DROP INDEX IF EXISTS delivery_partners_user_idx;

DROP TABLE IF EXISTS delivery_jobs;
DROP TABLE IF EXISTS delivery_partners;
