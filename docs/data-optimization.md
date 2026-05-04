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
