-- Tetra Engine — Optimized Schema
-- Creates all tables per architecture-design.md v1.1 with optimized data types

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'order_status') THEN
        CREATE TYPE order_status AS ENUM ('SYNCED', 'PENDING', 'RECOMMENDED', 'PUSHED', 'NO RECOMMENDATION');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'scheduler_status') THEN
        CREATE TYPE scheduler_status AS ENUM ('RUNNING', 'SUCCESS', 'FAILED');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'push_outbox_status') THEN
        CREATE TYPE push_outbox_status AS ENUM ('PENDING', 'RETRY', 'DELIVERED');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS cartons (
    id          SERIAL          PRIMARY KEY,
    flux_id     INT             NOT NULL DEFAULT 0,
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
CREATE INDEX IF NOT EXISTS idx_cartons_code ON cartons (code);
CREATE INDEX IF NOT EXISTS idx_cartons_is_active ON cartons (is_active);

CREATE TABLE IF NOT EXISTS orders (
    id              BIGSERIAL       PRIMARY KEY,
    flux_id         INT             NOT NULL DEFAULT 0,
    code            VARCHAR(50)     NOT NULL UNIQUE,
    warehouse_id    VARCHAR(50),
    status          order_status    NOT NULL DEFAULT 'SYNCED',
    carton_id       INT             REFERENCES cartons(id),
    flux_created_at TIMESTAMPTZ,
    synced_at       TIMESTAMPTZ,
    recommended_at  TIMESTAMPTZ,
    pushed_at       TIMESTAMPTZ,
    reason          TEXT,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);
CREATE INDEX IF NOT EXISTS idx_orders_code ON orders (code);
CREATE INDEX IF NOT EXISTS idx_orders_carton_id ON orders (carton_id);

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
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items (order_id);
CREATE INDEX IF NOT EXISTS idx_order_items_sku ON order_items (sku);

CREATE TABLE IF NOT EXISTS scheduler_logs (
    id                  BIGSERIAL           PRIMARY KEY,
    scheduler_name      VARCHAR(50)         NOT NULL,
    status              scheduler_status    NOT NULL,
    records_processed   INT,
    error_message       TEXT,
    started_at          TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    finished_at         TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_scheduler_logs_name ON scheduler_logs (scheduler_name);
CREATE INDEX IF NOT EXISTS idx_scheduler_logs_started_at ON scheduler_logs (started_at);

CREATE TABLE IF NOT EXISTS push_outbox (
    id              BIGSERIAL           PRIMARY KEY,
    order_id        BIGINT              NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    flux_order_id   INT                 NOT NULL,
    flux_carton_id  VARCHAR(50),
    created_by      VARCHAR(255)        NOT NULL,
    status          push_outbox_status  NOT NULL DEFAULT 'PENDING',
    attempt_count   INT                 NOT NULL DEFAULT 0,
    last_error      TEXT,
    next_attempt_at TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ         NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ         NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_push_outbox_status_next_attempt_at ON push_outbox (status, next_attempt_at);

CREATE TABLE IF NOT EXISTS scheduler_leader (
    id          SMALLINT    PRIMARY KEY CHECK (id = 1),
    leader_id   VARCHAR(255) NOT NULL,
    lease_until TIMESTAMPTZ  NOT NULL,
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
