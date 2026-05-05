# Data Optimization & Refactoring

**Date:** 2026-05-04  
**Scope:** Tetra Engine Database & Processing Logic  

This document details the optimizations and schema refactoring applied to the local database and backend processing engine. These changes focus strictly on storage efficiency, query speed, and data normalization without affecting the external Input/Output (I/O) API format with Flux WMS.

## 1. Dimensional & Weight Data Type Refactoring

### Previous Design
- Dimensions (`length`, `width`, `height`) and weight were stored as `DECIMAL(10,2)` in PostgreSQL.
- Handled as `float64` in the Golang backend.
- Units were centimeters (cm) and kilograms (kg).

### Optimized Design
- All physical measurements are now stored as `INT` (Integer).
- **Length, Width, Height** are stored in **millimeters (mm)**.
- **Weight and Max Weight** are stored in **grams (g)**.

### Why?
1. **Memory & Storage:** `INTEGER` uses a fixed 4 bytes per row. `DECIMAL` uses variable length space and is much heavier (up to 14 bytes) for both `order_items` and `cartons` tables.
2. **Computational Speed:** Calculating volume (`L × W × H`) and summing weights is significantly faster on the CPU using integers than floating-point math.
3. **Precision:** Eliminates floating point rounding issues.

### How it Works (I/O Transparency)
When fetching from Flux (which sends "35.00" string cm), the engine parses the string as float, multiplies by `10` (for mm) and casts to `int` before storing. For weight, Flux sends grams directly, so it's parsed directly to integer. Carton's `max_weight` is sent in kg, so it's multiplied by `1000`.

## 2. Product Normalization

### Previous Design
- The `order_items` table stored both `sku` and `sku_name` (VARCHAR 255) redundantly for every single item across thousands of orders.

### Optimized Design
- Created a new `products` table: `(sku VARCHAR(50) PRIMARY KEY, name VARCHAR(255))`.
- The `order_items` table now only stores the `sku` as a Foreign Key to `products`.

### Why?
1. **Deduplication:** A single product sold 10,000 times will no longer consume 10,000 copies of its name string in the database.
2. **Scalability:** The `order_items` table size is drastically reduced, enabling faster full-table scans or joins when needed.

### How it Works
During the order detail sync from Flux, the backend groups products and performs a bulk `UPSERT` (Insert on conflict do update) into the `products` table before inserting the `order_items`.

## 3. Carton Foreign Key Relationship

### Previous Design
- `orders.carton_id` was a `VARCHAR(50)` storing the external string code of the carton (e.g., "CB01S"). No strict foreign key constraint existed.

### Optimized Design
- `orders.carton_id` is now a `BIGINT` (in Go `*int64`) referencing `cartons(id)`.

### Why?
1. **Indexing & Query Performance:** Integer-based indexes and foreign keys are significantly faster than string-based matching.

### How it Works
- During recommendation processing, the engine assigns the internal numeric `id` of the carton to the order.
- During the push phase, the engine fetches the `Carton` record using the internal `id`, retrieves its `code`, and maps it back to the Flux external ID format before sending the POST request.

## 4. Status ENUMs

### Previous Design
- `orders.status` and `scheduler_logs.status` used `VARCHAR(20)`.

### Optimized Design
- Converted to Postgres ENUMs (`order_status` and `scheduler_status`).

### Why?
- **Space Efficiency:** ENUMs are stored internally as 4-byte integers rather than raw string bytes.
- **Data Integrity:** The database natively rejects invalid status strings without needing complex triggers or backend validation.

## 5. Operational Tables (Reliability & Distributed Coordination)

### New Tables Added (as of 2026-05-05)

**5.1 `push_outbox` — Reliable Push Delivery**
- **Purpose:** Ensures every recommendation push to Flux is delivered exactly-once, even under transient network failures.
- **Schema:**
  - `id` (BIGSERIAL PRIMARY KEY)
  - `order_id` (BIGINT, FK to orders)
  - `payload` (JSONB): Serialized carton assignment payload
  - `status` (push_outbox_status ENUM): PENDING, RETRY, DELIVERED
  - `retry_count` (INT): Tracks delivery attempts
  - `retry_after` (TIMESTAMP): Next eligible retry time (exponential backoff with jitter)
  - `created_at`, `updated_at` (TIMESTAMP)

- **How It Works:**
  1. When the recommendation job completes, it enqueues all push intents into `push_outbox` in a single transaction (before any network calls).
  2. A dedicated delivery loop claims pending rows and attempts delivery to the Flux API.
  3. On success, the row is marked `DELIVERED` and the order status is atomically updated.
  4. On transient failure, the row is marked `RETRY` with an updated `retry_after` time (exponential backoff).
  5. Stale `RETRY` rows are reattempted periodically, guaranteeing eventual delivery.

- **Why:**
  - **Resilience:** Network failures between Tetra and Flux no longer result in lost recommendations.
  - **Idempotency:** The Flux API can receive duplicate requests without data corruption (idempotent from the DB side).

**5.2 `scheduler_leader` — Single-Leader Coordination**
- **Purpose:** Ensures only one scheduler instance runs jobs in a multi-instance deployment.
- **Schema:**
  - `leader_id` (UUID PRIMARY KEY): Unique instance identifier
  - `job_name` (VARCHAR): Name of the scheduled job
  - `lease_until` (TIMESTAMP): Expiration time of the lease
  - `updated_at` (TIMESTAMP): Last renewal time

- **How It Works:**
  1. Each scheduler instance has a unique `instance_id` (UUID).
  2. On each tick, the scheduler attempts `TryAcquireOrRenew(instanceId, jobName)`:
     - If no record exists → insert and acquire the lease.
     - If the record exists and `lease_until < NOW()` → update to acquire (lease expired).
     - If the record exists and the current holder → renew the lease.
     - Otherwise → skip the job (another instance holds the active lease).
  3. Only the holder of the lease executes the scheduled job for that tick.
  4. Leases auto-expire (default 15 seconds), allowing failover if an instance crashes.

- **Why:**
  - **Safety:** Prevents duplicate job execution in HA setups (multiple instance running same scheduler).
  - **Simplicity:** DB-based coordination avoids external consensus systems (etcd, Consul, etc.).

### Schema Evolution Strategy
All schema changes are reconciled at application startup via the `ReconcileQuery` in `database.go`. If tables or ENUMs are missing, they are created automatically, ensuring zero-downtime deployments across schema versions.
