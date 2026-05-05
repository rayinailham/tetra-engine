-- Rollback initial schema
DROP TABLE IF EXISTS scheduler_leader;
DROP TABLE IF EXISTS push_outbox;
DROP TABLE IF EXISTS scheduler_logs;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS cartons;
DROP TYPE IF EXISTS push_outbox_status;
DROP TYPE IF EXISTS scheduler_status;
DROP TYPE IF EXISTS order_status;
