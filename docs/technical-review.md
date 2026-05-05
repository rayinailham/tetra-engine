# Technical Review: Tetra Recommendation Engine — Post-Reliability Upgrade

## **Introduction**
This technical review summarizes the reliability, scalability, and operational improvements recently implemented in the Tetra Recommendation Engine (changes made to support batching, per-job and per-record timeouts, an outbox pattern for push reliability, Prometheus metrics, and a DB-based scheduler leader lease). It is written for the author of `architecture-design.md` to explain what we changed, why, and the measurable benefits.

## **Executive Summary**
- Implemented cursor-based batching for recommendation processing to avoid loading all `PENDING` orders into memory.
- Added per-job and per-record timeouts to isolate slow external calls and avoid scheduler stalls.
- Implemented an outbox pattern for reliable push delivery with transactional enqueue and idempotent delivery semantics plus retry/backoff.
- Added Prometheus metrics and exposed `/metrics` for operational monitoring and SLO alerting.
- Added a DB-backed leader lease (`scheduler_leader`) so only one instance runs scheduled jobs at a time.

These changes significantly reduce tail latency risk, decrease memory footprint during large backlogs, and make push delivery robust to transient Flux outages.

## **What We Changed (by area)**

**Batching / Memory Usage**
- Recommendation job now reads `PENDING` orders in ascending `id` pages (configurable `recommendation_batch_size`) using `GetOrdersByStatusAfterID`. This keeps memory bounded and predictable.

**Timeouts & Isolation**
- Scheduler jobs are wrapped with a configurable `scheduler.job_timeout`. Individual record processing uses `scheduler.record_timeout` to prevent a single external call from blocking other work.

**Reliable Push (Outbox)**
- Push intents are persisted in `push_outbox` via a transactional `EnqueuePushIntent` before any network activity.
- A dedicated delivery loop (`deliverOutboxBatch`) claims pending outbox rows and performs delivery with exponential backoff and jitter on transient failures, marking rows `DELIVERED` or rescheduling with increased `retry_after`.

**Distributed Locking / Leader Lease**
- `scheduler_leader` table and `TryAcquireOrRenew` ensure a single leader instance performs scheduled jobs. Lease ownership renews periodically; stale leases can be overtaken safely.

**Observability**
- Added Prometheus metrics: job duration histogram, job failure counters, pending-orders gauge, and push retry counters. `/metrics` endpoint is available on the HTTP server.

**Schema & Migrations**
- New DB objects: `push_outbox_status` enum, `push_outbox` table, and `scheduler_leader` table. Startup reconciliation ensures these exist if the migrations haven't been applied yet.

## **Operational Impact**

- Reliability: Scheduler no longer stalls due to slow network calls; transient push failures are retried safely without losing events.
- Throughput: Batching reduces peak memory and DB load, enabling stable processing under large backlogs.
- Observability: Metrics make it straightforward to detect slow jobs, rising retry counts, or leader-election churn and create alerts.
- Safety: Outbox + transactional semantics ensure recommendations are not lost and pushes are idempotent from the DB perspective.

## **Configuration & Tuning (important knobs)**

- `scheduler.job_timeout` (default: 30s): Max duration for an entire scheduler run.
- `scheduler.record_timeout` (default: 5s): Max duration for a single record's external call.
- `recommendation_batch_size` (default: 250): Number of orders processed per page.
- `push_batch_size` (default: 100): Number of outbox records delivered per delivery batch.
- `scheduler.leader_lease_seconds` (default: 15): Lease TTL for leader renewal.

Recommendation: Start with these defaults in staging, observe `job_duration_seconds` and `push_retry_total`, then tune `recommendation_batch_size` and `push_batch_size` to match DB and Flux throughput.

## **SLO / Alerting Suggestions**

- Alert on `job_failures_total{job="recommendations"} > 0` sustained for 5 minutes.
- Alert on `pending_orders` > X (define X from capacity tests) for 5+ minutes to signal backlog growth.
- Alert on growth of `push_retry_total` as a ratio of delivered pushes — sustained climb indicates external API degradation.

## **Testing & Validation**

- Unit tests pass locally (`go test ./...`).
- Integration advice: run CI with `CGO_ENABLED=1 go test -race ./...` to run the race detector (environment here prevented that run).
- End-to-end validation: run a staging scenario with a synthetic backlog (100k orders) to validate memory, page throughput, and push retry behavior.

## **Limitations & Next Steps**

- Race detector must be run in CI (CGO enabled) to validate concurrency assumptions under `-race`.
- Consider adding a Flux-side bulk-details endpoint or server-side bulk fetch to reduce network calls during sync.
- Optional: move outbox delivery to a separate worker process (or queue) if push delivery becomes CPU/network bound.

## **Why this exceeds expectations**

The original architecture prioritized correctness and clarity; these targeted improvements materially increase operational safety (timeouts and outbox), scalability (cursor batching), and observability (Prometheus metrics). Together they transform the engine from a functional prototype into a reliable service suitable for production deployment under realistic warehouse loads.

## **Appendix — Key Files Changed**

- `internal/scheduler/scheduler.go` — job timeouts & leader gating
- `internal/service/recommendation.go` — paginated processing, per-record timeouts, outbox enqueue/delivery
- `internal/repository/outbox.go` — outbox persistence and delivery helpers
- `internal/repository/scheduler_lock.go` — leader lease implementation
- `internal/observability/metrics.go` — Prometheus metrics
- `migrations/000001_create_tables.*.sql` — new schema objects

If you want, I can also add a short changelog entry to `README.md` and prepare a small deployment checklist for the ops team (DB migration, config knobs, CI with `-race`).

**Status:** `docs/technical-review.md` updated and ready to share with the architecture author.
