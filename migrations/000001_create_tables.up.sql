-- Tetra Engine — Optimized Schema
-- Creates all tables per architecture-design.md v1.1 with optimized data types

CREATE TYPE order_status AS ENUM ('SYNCED', 'PENDING', 'RECOMMENDED', 'PUSHED', 'NO RECOMMENDATION');
CREATE TYPE scheduler_status AS ENUM ('RUNNING', 'SUCCESS', 'FAILED');

CREATE TABLE IF NOT EXISTS cartons (
    id          SERIAL          PRIMARY KEY,
    code        VARCHAR(50)     NOT NULL UNIQUE,
    length      INT             NOT NULL, -- Stored in millimeters (mm)
    width       INT             NOT NULL, -- Stored in millimeters (mm)
    height      INT             NOT NULL, -- Stored in millimeters (mm)
    max_weight  INT             NOT NULL, -- Stored in grams (g)
    is_active   BOOLEAN         NOT NULL DEFAULT TRUE,
    synced_at   TIMESTAMPTZ,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cartons_code ON cartons (code);
CREATE INDEX idx_cartons_is_active ON cartons (is_active);

CREATE TABLE IF NOT EXISTS orders (
    id              BIGSERIAL       PRIMARY KEY,
    code            VARCHAR(50)     NOT NULL UNIQUE,
    warehouse_id    VARCHAR(50),
    status          order_status    NOT NULL DEFAULT 'SYNCED',
    carton_id       INT             REFERENCES cartons(id),
    flux_created_at TIMESTAMPTZ,
    synced_at       TIMESTAMPTZ,
    recommended_at  TIMESTAMPTZ,
    pushed_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_orders_status ON orders (status);
CREATE INDEX idx_orders_code ON orders (code);
CREATE INDEX idx_orders_carton_id ON orders (carton_id);

CREATE TABLE IF NOT EXISTS products (
    sku         VARCHAR(50)     PRIMARY KEY,
    name        VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS order_items (
    id          BIGSERIAL       PRIMARY KEY,
    order_id    BIGINT          NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sku         VARCHAR(50)     NOT NULL REFERENCES products(sku),
    qty         SMALLINT        NOT NULL,
    length      INT             NOT NULL, -- Stored in millimeters (mm)
    width       INT             NOT NULL, -- Stored in millimeters (mm)
    height      INT             NOT NULL, -- Stored in millimeters (mm)
    weight      INT             NOT NULL, -- Stored in grams (g)
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_order_items_order_id ON order_items (order_id);
CREATE INDEX idx_order_items_sku ON order_items (sku);

CREATE TABLE IF NOT EXISTS scheduler_logs (
    id                  BIGSERIAL           PRIMARY KEY,
    scheduler_name      VARCHAR(50)         NOT NULL,
    status              scheduler_status    NOT NULL,
    records_processed   INT,
    error_message       TEXT,
    started_at          TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    finished_at         TIMESTAMPTZ
);
CREATE INDEX idx_scheduler_logs_name ON scheduler_logs (scheduler_name);
CREATE INDEX idx_scheduler_logs_started_at ON scheduler_logs (started_at);
