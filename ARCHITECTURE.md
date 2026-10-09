# Architecture

Behaviour, API contract and acceptance criteria are in [SPEC.md](SPEC.md). This document defends the design.

## 1. Architectural Design & Database Locking Strategy

**Schema.** `inv_item` holds physical stock (`item_stock`) and a running counter of held units (`item_stock_reserved`); available stock is the difference and is never stored. `inv_reservation` is the ledger: one row per hold, moving once from `PENDING` to `CONFIRMED` or `EXPIRED`. The counter is derivable from the ledger, and is stored anyway because it is the lock. A ledger-only design oversells: two transactions each `SUM` the rows, neither sees the other's uncommitted insert, both pass. `CHECK (0 <= item_stock_reserved <= item_stock)` is the backstop, so a logic bug fails a statement instead of selling stock that does not exist.

**Reserve is one statement.** A conditional `UPDATE inv_item … WHERE item_stock - item_stock_reserved >= $qty` feeds an `INSERT` of the reservation through a CTE. At `READ COMMITTED` the update takes the row lock and re-evaluates its `WHERE` against the latest committed row, so two buyers can never both take the last unit, and the lock is held for one round trip. Rejected: `SELECT … FOR UPDATE` (extra round trips while holding the hottest row), optimistic versioning and `SERIALIZABLE` (retry storms at exactly the moment of peak contention).

**Confirm is one transaction**: reservation row, then item row. The deadline is in the `WHERE`, checked by the database clock, so an overdue hold can never be confirmed. A repeated confirm returns the original result, so a client retry is safe.

**Expiry has three layers**, so correctness never depends on a timer: (1) confirm checks the deadline itself; (2) a reserve that finds too little stock first releases that item's overdue holds, then tries once more, so buyers never see "sold out" because of dead holds; (3) a background sweeper releases everything else. All three share one release statement using `FOR UPDATE SKIP LOCKED`, which makes double release impossible.

**Deadlocks.** Reserve touches only the item row; confirm and release both go reservation rows → item row; sweeps are serialised by an advisory lock. No lock cycle exists. A transaction aborted with `40P01`/`40001` is still retried once.

**Time.** Every deadline comparison uses PostgreSQL `now()`, never the Go clock, so instance clock skew cannot matter.

**Gin.** Routes live under `/api/v1/inventory` in one file. Middleware order is request ID → logger → error handler → recovery → timeout. Handlers only parse, call a use case and write; every error leaves through one envelope, and one map assigns HTTP status to error codes. Layers: `delivery → usecase → domain`, `repository → domain`.

**Evidence.** Stress tests under `-race` run 1000 buyers against 100 units, confirms racing the sweeper, and mixed churn, then assert five invariants from SQL. Deliberately breaking the reserve guard, the confirm deadline or the release filter makes them fail.

## 2. Distributed Scaling & Failure Modes

**Already safe at 10 instances.** No state lives in process memory; the sweeper's advisory lock lets one instance sweep per tick; migrations take a lock at start-up; the database is the only clock; `/ready` lets a load balancer drop an instance that has lost its database, and it recovers without a restart.

**What breaks, and the redesign:**

| Failure | Change |
|---|---|
| 10 pools exceed `max_connections` | PgBouncer or RDS Proxy; size pools per instance |
| One item row serialises every reserve for that item, so throughput is capped by a single row lock | Atomic decrement in Redis as an admission gate with PostgreSQL as system of record; or stock split across bucket rows; or a queue with one consumer per item |
| Stock polling from every browser hits the primary | Read replica or a 1-second cache; push (SSE) instead of polling |
| A client retry after a lost response creates a second reservation | `Idempotency-Key` on reserve, stored with the reservation |
| A sweeper in every API pod | A separate single-purpose deployment or scheduled job |
| No protection against one client flooding reserve | Rate limiting at the edge or in Redis, never in process memory |

Target shape on AWS: stateless containers on ECS or EKS behind an ALB, RDS PostgreSQL with RDS Proxy, ElastiCache for the gate and cache.

## 3. Engineering Trade-offs & AI Transparency

**Cut for time.** Backend: authentication, idempotency keys, rate limiting, metrics and tracing, CI. Frontend: push updates (it polls every 2 seconds), automated component tests (logic is unit-tested, screens were checked by a scripted browser walkthrough), and tracking more than one reservation at a time.

**Known limits.** The stock endpoint can show held units up to one sweep interval after they expire; buyers are unaffected because reserve releases them itself. A reserve can lose a millisecond race with the sweeper and get one early "sold out". A device clock that is wrong shows a wrong countdown, but the server still decides what can be confirmed.

**Layout over idiom.** The backend follows a widely used Go clean-architecture template (`delivery`, `usecase`, `repository`, `domain`, tests under `test/`, one migration per table) for recognisability and consistency. I know this costs a little Go idiom: tests are not beside the code, interfaces are declared next to their implementations, and names such as `exception` and `Impl` come from other ecosystems. I departed from the template once: errors carry no HTTP status, so the inner layers import neither Gin nor `net/http`.

**AI use.** I built this with Claude Code, Anthropic's coding agent, which wrote the specification, code and tests under my direction. The decisions were mine, among them the schema naming, the layered layout, making inline release on reserve mandatory, and TanStack Query on the frontend.

*A flawed suggestion.* The first sweeper loop was `select { case <-ctx.Done(): return; case <-ticker.C: sweep() }`, with each sweep given a context that survives cancellation so a sweep in progress can finish. When a sweep finished after shutdown had begun, both cases were ready, and Go's `select` picks at random: about half the time it started another sweep during shutdown. One test run was green. Running the suite repeatedly with `-race -count=5` failed every time, because the test double was called twice. The fix checks `ctx.Err()` before starting a sweep; `TestSweeperRun_FinishesCurrentSweep` covers it, verified over 60 runs. The lesson I took: a single passing run proves little for concurrent code.
