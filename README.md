# Tetra Recommendation Engine

Package recommendation engine for **AnterAja** warehouse operations. Tetra automatically syncs orders from **Flux WMS**, calculates the optimal carton for each order based on volume and weight, and pushes recommendations back to Flux.

## Architecture

Tetra operates as a scheduler-based integration engine with 4 periodic jobs:

1. **Order Retrieval** (every 15 min) — Syncs orders from Flux WMS to local database
2. **Carton Master Sync** (every 30 min) — Syncs carton catalog from Flux
3. **Carton Recommendation** (every 5 min) — Calculates optimal carton for pending orders
4. **Carton Push** (every 5 min) — Pushes recommendations back to Flux

### Order Status Lifecycle

```
SYNCED → PENDING → RECOMMENDED → PUSHED
                                   ↗
                          ERROR ←─┘ (if no suitable carton found)
```

## Tech Stack

- **Go 1.24+** — Modern, high-performance backend
- **uber-go/fx** — Compile-time dependency injection & lifecycle management
- **spf13/viper** — Layered configuration (env, file, defaults)
- **sqlx + lib/pq** — Optimized PostgreSQL data access
- **samber/oops** — Structured error handling with full stack traces
- **log/slog + tint** — Structured logging with colorized, human-friendly terminal output
- **Vue.js 3 + Vite** — High-end glassmorphism monitoring dashboard

## Prerequisites

- **Go 1.24+**
- **PostgreSQL 14+**
- **golang-migrate CLI** (optional, for manual schema management; migrations are automatic on startup)

## Features

- **Zero-Config Sync** — 4-stage idempotent pipeline for seamless Flux WMS integration.
- **Volumetric Intelligence** — Optimized recommendation algorithm using precise integer arithmetic (mm/g).
- **Auto-Schema Management** — Database schema is automatically applied and reconciled on startup.
- **Observability First** — Comprehensive scheduler logs and structured error context for rapid debugging.
* **Premium Monitoring Dashboard** — Real-time visualization of warehouse throughput and engine health.

## Quick Start

```bash
# 1. Clone and install dependencies
go mod tidy

# 2. Set up PostgreSQL database
createdb tetra

# 3. Copy and edit configuration
cp .env.example .env
# Edit .env with your database credentials (DB_USER, DB_PASSWORD, etc.)

# 4. Run the application (Migrations are automatic!)
go run ./cmd/tetra
```

### Running the Frontend Dashboard

```bash
cd web
npm install
npm run dev
```
The monitoring dashboard will be available at `http://localhost:5173`.

## API Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check (DB connectivity & engine status) |
| `GET` | `/metrics` | Prometheus metrics endpoint for scheduler and queue SLO monitoring |
| `POST` | `/api/internal/trigger/{job_name}` | Manually trigger a scheduler job |
| `GET` | `/api/dashboard/stats` | High-level KPI statistics for the dashboard |
| `GET` | `/api/dashboard/orders` | Recent orders list with advanced filtering |
| `GET` | `/api/dashboard/cartons` | Available cartons master data |
| `GET` | `/api/dashboard/logs` | Comprehensive scheduler execution history |
| `GET` | `/api/dashboard/engine/status` | Real-time engine running status |
| `POST` | `/api/dashboard/engine/start` | Start the scheduler engine |
| `POST` | `/api/dashboard/engine/stop` | Stop the scheduler engine |
| `POST` | `/api/dashboard/engine/reset` | Emergency stop and database reset |
| `GET` | `/api/dashboard/settings` | Retrieve engine scheduler intervals |
| `POST` | `/api/dashboard/settings` | Dynamically update scheduler intervals |

**Valid job names:** `order_retrieval`, `carton_sync`, `carton_recommendation`, `carton_push`

## Scheduler Reliability Settings

Configure these environment variables in `.env` to tune safety and throughput:

```dotenv
SCHEDULER_JOB_TIMEOUT=2m
SCHEDULER_RECORD_TIMEOUT=10s
SCHEDULER_RECOMMENDATION_BATCH_SIZE=500
SCHEDULER_PUSH_BATCH_SIZE=500
SCHEDULER_LEADER_LEASE_DURATION=30s
```

What these settings control:
1. `SCHEDULER_RECOMMENDATION_BATCH_SIZE`: stable cursor pagination for recommendation runs.
2. `SCHEDULER_JOB_TIMEOUT`: max runtime for one scheduler cycle per job.
3. `SCHEDULER_RECORD_TIMEOUT`: per-order processing timeout to isolate slow records.
4. `SCHEDULER_PUSH_BATCH_SIZE`: outbox delivery page size per push cycle.
5. `SCHEDULER_LEADER_LEASE_DURATION`: DB lease window used for distributed leader election.

## SLO Metrics and Alert Baseline

Exposed metrics:
1. `job_duration_seconds`
2. `job_failures_total`
3. `pending_orders`
4. `push_retry_total`

Suggested starter alert rules:

```yaml
groups:
  - name: tetra-engine-alerts
    rules:
      - alert: TetraSchedulerJobLatencyHigh
        expr: histogram_quantile(0.95, sum(rate(job_duration_seconds_bucket[10m])) by (le, job)) > 120
        for: 10m
      - alert: TetraSchedulerFailuresSpike
        expr: increase(job_failures_total[15m]) > 5
        for: 5m
      - alert: TetraPendingBacklogGrowing
        expr: pending_orders > 5000
        for: 15m
      - alert: TetraPushRetriesHigh
        expr: increase(push_retry_total[15m]) > 50
        for: 10m
```

## Project Structure

```
cmd/tetra/          — Application entry point (dependency wiring)
internal/
  client/           — Flux WMS API client (resilient HTTP)
  config/           — Configuration layering (viper)
  database/         — Database setup & auto-migrations
  domain/           — Domain models & business logic constants
  repository/       — High-performance data access (sqlx)
  scheduler/        — Periodic job orchestration
  server/           — REST API & Dashboard backend
  service/          — Core recommendation engine logic
migrations/         — SQL migration scripts
web/                — Vue.js 3 monitoring dashboard
```

## Testing

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Recommendation Algorithm

For each pending order:
1. Calculate total **volume** = Σ (length × width × height × qty) per item (in mm³)
2. Calculate total **weight** = Σ (weight × qty) per item (in grams)
3. Select the **smallest carton** where:
   - Carton volume ≥ total item volume
   - Carton max_weight ≥ total item weight
4. If no suitable carton exists, mark order as `ERROR` (`NO RECOMMENDATION`)
5. Pushes results back to Flux API using numeric IDs for idempotency.

## Performance Optimization

The Tetra Engine features a highly optimized data processing layer:
1. **Integer Precision:** Dimensions from Flux (cm/kg) are normalized to integers (mm/g) before storage. This avoids floating-point inaccuracies and significantly speeds up volumetric calculations.
2. **Parallel Sync Pipeline:** Uses high-concurrency patterns (**errgroup + semaphores**) to fetch order details from Flux in parallel, drastically reducing total sync time.
3. **Bulk DB Operations:** Eliminates N+1 query problems by using batch retrieval for order items and idempotent `ON CONFLICT` upserts.
4. **Schema Reconciliation:** The engine automatically reconciles database columns on startup, ensuring the latest optimized types are always in use.
5. **Resilient Sync:** Every API and DB operation is wrapped in context-aware timeouts and atomic transactions.

For full technical details, see [`docs/technical-review.md`](docs/technical-review.md).

