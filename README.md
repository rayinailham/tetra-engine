# Tetra Recommendation Engine

Package recommendation engine for **Anter Aja** warehouse operations. Tetra automatically syncs orders from **Flux WMS**, calculates the optimal carton for each order based on volume and weight, and pushes recommendations back to Flux.

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

- **Go 1.24+** with modern standard library features
- **uber-go/fx** — Dependency injection & lifecycle management
- **spf13/viper** — Configuration from environment variables
- **sqlx + lib/pq** — PostgreSQL database access
- **samber/oops** — Structured error handling with stack traces
- **log/slog** — Structured logging (text/JSON)
- **Vue.js 3 + Vite** — High-end glassmorphism monitoring dashboard

## Prerequisites

- Go 1.24+
- PostgreSQL 14+
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI (for migrations)

## Features

- **Automated Data Sync** — 4-stage idempotent pipeline for Flux WMS integration.
- **Volumetric Recommendation** — Optimized algorithm using integer arithmetic (mm/g).
- **Auto-Migration** — Database schema is automatically applied on startup.
- **Real-time Monitoring** — Dashboard with smooth robust polling for live updates.
- **High-End UI** — Premium glassmorphism dashboard built with Vue.js 3.

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
| `GET` | `/health` | Health check (DB connectivity) |
| `POST` | `/api/internal/trigger/{job_name}` | Manually trigger a scheduler job |
| `GET` | `/api/dashboard/stats` | High-level KPI statistics for the dashboard |
| `GET` | `/api/dashboard/orders` | Recent orders list with sorting support |
| `GET` | `/api/dashboard/cartons` | Available cartons list |
| `GET` | `/api/dashboard/logs` | Recent scheduler logs with sorting support |
| `GET` | `/api/dashboard/engine/status` | Engine running status |
| `POST` | `/api/dashboard/engine/start` | Start the scheduler engine |
| `POST` | `/api/dashboard/engine/stop` | Stop the scheduler engine |
| `POST` | `/api/dashboard/engine/reset` | Stop engine and completely reset the database |
| `GET` | `/api/dashboard/settings` | Get current engine scheduler intervals |
| `POST` | `/api/dashboard/settings` | Update engine scheduler intervals |

**Valid job names:** `order_retrieval`, `carton_sync`, `carton_recommendation`, `carton_push`

## Project Structure

```
cmd/tetra/          — Application entry point (fx wiring)
internal/
  config/           — Configuration loading (viper)
  domain/           — Domain models (orders, cartons, etc.)
  database/         — Database connection setup
  repository/       — Data access layer (sqlx)
  client/           — Flux WMS HTTP client
  service/          — Business logic (recommendation engine)
  scheduler/        — Background job management
  server/           — HTTP server (health + triggers)
migrations/         — SQL migration files
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

## Data Refactoring & Optimization

The Tetra Engine features a highly optimized database schema and data processing layer:
1. **Integer Arithmetic:** All floating-point dimensions from Flux (cm/kg) are converted to integers (mm/g) before storage. This reduces memory footprint and drastically speeds up the recommendation engine's volumetric calculations.
2. **SKU Normalization:** Product names are extracted from `order_items` into a normalized `products` table, preventing massive string duplication across thousands of order items.
3. **Database ENUMs:** Order and scheduler statuses are stored as Postgres ENUMs for optimal space efficiency and strict data integrity.

For full details, see [`docs/data-optimization.md`](docs/data-optimization.md).
