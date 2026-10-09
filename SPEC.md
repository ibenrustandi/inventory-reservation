# SPEC — High-Throughput Inventory Reservation System & Mini Dashboard

This is the technical specification the implementation and tests are built from. It was written before the code.

**How to read it.** "Brief" means the *Fullstack Take Home Assignment* document. Where a section implements a requirement from the Brief, the exact statement is quoted with its section number. Anything marked **Decision** is not stated in the Brief; it is a choice made here to remove an ambiguity.

## 1. Scope

> Brief §1: "design a resilient, high-concurrency backend service using Go, Gin Framework, and a real PostgreSQL database (80% weighting) and connect it to a clean, functional frontend application built strictly using React and Vite (20% weighting)."

In scope: three REST endpoints, automatic expiry of reservations, a React dashboard, Docker Compose setup, unit and stress tests.

Out of scope (**Decision**): authentication, payments, multi-item orders, stock administration, deleting reservations.

## 2. Glossary

| Term | Meaning |
|---|---|
| `total_stock` | Physical units on hand, including units held by pending reservations. |
| `reserved_stock` | Units held by pending reservations. |
| `available_stock` | `total_stock − reserved_stock`. Never stored. |
| Reservation | A hold of a quantity of one item for one user, with a deadline. |
| Due | A pending reservation whose deadline has passed but whose stock has not been released yet. |

## 3. Decisions on points the Brief leaves open

| ID | Decision | Brief statement it interprets |
|---|---|---|
| D-1 | Confirm decreases `total_stock` and `reserved_stock` by the reservation quantity, so `available_stock` does not change on confirm. | §2 (2): "completing the order process and decrementing physical inventory." |
| D-2 | A reservation is expired the instant database time reaches `expires_at`, whether or not its stock has been released. Only database time is used. | §2 (1): "must automatically expire after 5 minutes unless confirmed." |
| D-3 | Confirming an already-confirmed reservation returns `200` with the original `confirmed_at`, so a client retry is safe. Stock is decremented once only. | §2 (2): "design the JSON response structure you deem best for client integration." |
| D-4 | One item exists, `item_4021`, seeded with `total_stock` 100. The API still takes `item_id` from the request and the database decides whether it exists. The dashboard uses `item_4021` as a constant. | §2 (3): "Handle negative/edge cases where the item is not found"; §3 (2): "input a User ID and Quantity". |
| D-5 | `quantity` is a JSON integer from 1 to 1000. `user_id`, `item_id` and `reservation_id` match `^[A-Za-z0-9_-]{1,64}$`. | §2 (1): "invalid input parameters". |
| D-6 | A user may hold several reservations at once; there is no per-user limit. | Not addressed in the Brief. |
| D-7 | `user_id` is trusted as sent; there is no authentication. | Not addressed in the Brief. |
| D-8 | The reservation lifetime (default 5 minutes) and the sweep interval (default 1 second) are configuration values, so tests do not wait. | §4 (5): "Include unit and stress tests." |

## 4. Reservation lifecycle

```
PENDING ──confirm before deadline──▶ CONFIRMED   (terminal)
PENDING ──deadline passes──────────▶ EXPIRED     (terminal)
```

A reservation changes state exactly once. A confirmed reservation never expires; an expired reservation can never be confirmed.

## 5. Data model

> Brief §4 (1): "Database Store: Real PostgreSQL instance. You must include SQL migration files or schema initialization scripts."

```sql
CREATE TABLE inv_item (
  item_id             TEXT PRIMARY KEY,
  item_stock          INT NOT NULL CHECK (item_stock >= 0),
  item_stock_reserved INT NOT NULL DEFAULT 0 CHECK (item_stock_reserved >= 0),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (item_stock_reserved <= item_stock)
);

CREATE TABLE inv_reservation (
  reservation_id           TEXT PRIMARY KEY,
  user_id                  TEXT NOT NULL,
  item_id                  TEXT NOT NULL REFERENCES inv_item(item_id),
  reservation_quantity     INT  NOT NULL CHECK (reservation_quantity > 0),
  reservation_status       TEXT NOT NULL CHECK (reservation_status IN ('PENDING','CONFIRMED','EXPIRED')),
  reservation_created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  reservation_expires_at   TIMESTAMPTZ NOT NULL,
  reservation_confirmed_at TIMESTAMPTZ,
  reservation_expired_at   TIMESTAMPTZ
);
CREATE INDEX inv_reservation_pending_expiry_idx
  ON inv_reservation (reservation_expires_at) WHERE reservation_status = 'PENDING';
```

Mapping to the API: `item_stock` → `total_stock`, `item_stock_reserved` → `reserved_stock`.

Naming rules (**Decision**): tables are prefixed `inv_` and singular; a table's own columns are prefixed with the table name without `inv_`; reference columns (`item_id`, `user_id`) keep the name of what they refer to; `updated_at` is unprefixed.

`item_stock` alone is never the availability check. Available is always `item_stock - item_stock_reserved`.

## 6. Invariants

> Brief §4 (2): "Strict PostgreSQL transaction handling and concurrency control. Over-selling or negative stock allocations under heavy concurrent load are strictly unacceptable."

These hold at every committed state, under any concurrency:

- **INV-1** `0 ≤ reserved_stock ≤ total_stock` for every item.
- **INV-2** `reserved_stock` equals the sum of quantities of that item's `PENDING` reservations.
- **INV-3** `total_stock` plus the sum of quantities of that item's `CONFIRMED` reservations equals the item's initial stock.
- **INV-4** A reservation is confirmed at most once, and never at or after its deadline.
- **INV-5** A reservation's stock is released at most once.

INV-1 is also enforced by `CHECK` constraints, so a logic error fails the statement instead of over-selling.

## 7. API

> Brief §2: "For each endpoint, implement both successful and negative error scenarios, designing the JSON response structure that best fits API best practices".

Common rules (**Decision**):
- Requests and responses are JSON. Timestamps are RFC 3339 in UTC.
- Success bodies are exactly those given in the Brief.
- Every error uses one envelope:

```json
{
  "status": "error",
  "error": {
    "code": "INSUFFICIENT_STOCK",
    "message": "Only 1 unit available.",
    "details": { "available_stock": 1, "requested": 2 }
  },
  "request_id": "req_…"
}
```

- `code` is stable and machine-readable; `message` is for people; `details` is optional and depends on the code.
- `400 INVALID_REQUEST` carries `details.fields`, a list of `{ "field", "reason" }`.
- `500 INTERNAL_ERROR` never exposes internal details.

### 7.1 Reserve stock — `POST /api/v1/inventory/reserve`

> Brief §2 (1): "Atomically reserve inventory in PostgreSQL for a user if sufficient available stock exists. Successfully reserved stock must automatically expire after 5 minutes unless confirmed."

Request: `{ "user_id": "usr_9981", "item_id": "item_4021", "quantity": 2 }`

Behaviour:
1. Validate the body (D-5).
2. In one atomic database operation: if `available_stock ≥ quantity`, increase `reserved_stock` by `quantity` and create a `PENDING` reservation with `expires_at = database now + 5 minutes`.
3. If stock was insufficient, release that item's due reservations and try step 2 once more (§8, layer 2).
4. Reservation IDs are `res_` followed by a random suffix and are not guessable from one another.

Success — `201 Created` (**Decision**: the Brief gives the body, not the status):

```json
{ "status": "success", "reservation_id": "res_883291", "item_id": "item_4021", "quantity": 2, "expires_at": "2026-07-20T16:35:00Z" }
```

> Brief §2 (1): "Handle and implement negative error scenarios (e.g., insufficient inventory, invalid input parameters)".

| Case | HTTP | Code |
|---|---|---|
| Malformed JSON; missing field; wrong type; `quantity` not an integer in 1..1000; ID fails the pattern | 400 | `INVALID_REQUEST` |
| `item_id` does not exist | 404 | `ITEM_NOT_FOUND` |
| `available_stock < quantity` after releasing due holds | 409 | `INSUFFICIENT_STOCK` (`details.available_stock`, `details.requested`) |

### 7.2 Confirm reservation — `POST /api/v1/inventory/confirm`

> Brief §2 (2): "Permanently commit an active reservation in PostgreSQL, completing the order process and decrementing physical inventory."

Request: `{ "reservation_id": "res_883291" }`

Behaviour, in one transaction: if the reservation is `PENDING` and database time is before `expires_at`, set it to `CONFIRMED` with `confirmed_at = database now`, and decrease the item's `total_stock` and `reserved_stock` by its quantity (D-1).

Success — `200 OK`:

```json
{ "status": "success", "reservation_id": "res_883291", "confirmed_at": "2026-07-20T16:32:00Z" }
```

> Brief §2 (2): "Handle and implement negative error scenarios (e.g., expired reservations, non-existent reservation IDs)".

| Case | HTTP | Code |
|---|---|---|
| Malformed JSON; missing or invalid `reservation_id` | 400 | `INVALID_REQUEST` |
| Reservation does not exist | 404 | `RESERVATION_NOT_FOUND` |
| Reservation is `EXPIRED`, or `PENDING` with its deadline passed | 410 | `RESERVATION_EXPIRED` (`details.expires_at`) |
| Reservation is already `CONFIRMED` | 200 | — same success body, original `confirmed_at` (D-3) |

### 7.3 Check inventory status — `GET /api/v1/inventory/stock?item_id=item_4021`

> Brief §2 (3): "Query PostgreSQL for real-time stock breakdowns for an item. Handle negative/edge cases where the item is not found or parameters are invalid".

Success — `200 OK`:

```json
{ "item_id": "item_4021", "total_stock": 100, "reserved_stock": 15, "available_stock": 85 }
```

| Case | HTTP | Code |
|---|---|---|
| `item_id` missing, empty, or fails the pattern | 400 | `INVALID_REQUEST` |
| `item_id` does not exist | 404 | `ITEM_NOT_FOUND` |

Known limit: `reserved_stock` here can include due reservations for up to one sweep interval (default 1 second). This affects the displayed number only; reserve is not affected (§8).

### 7.4 Operational endpoints (**Decision**)

`GET /healthz` returns `200` while the process is running. `GET /readyz` returns `200` only when the database answers.

## 8. Expiry

> Brief §2 (1): "Successfully reserved stock must automatically expire after 5 minutes unless confirmed."
>
> Brief §4 (3): "Expired reservations must be automatically returned to available stock upon expiration."

A reservation is expired the instant database time reaches `expires_at` (D-2). This is enforced in three layers:

1. **Confirm checks the deadline itself.** A confirm on a reservation past its deadline is rejected with `410 RESERVATION_EXPIRED`, whether or not its stock has been released yet.
2. **Reserve releases due holds inline before saying sold out.** When a reserve finds too little available stock, the service releases that item's due reservations and tries the reserve once more. Only if that also fails does it return `409 INSUFFICIENT_STOCK`. It retries once and never loops. A successful first attempt costs no extra query.
3. **A background sweeper releases everything else** on a fixed interval, including holds on items nobody is currently buying.

Releasing a reservation means, atomically: set it to `EXPIRED` with `expired_at`, and decrease the item's `reserved_stock` by its quantity.

The sweep interval must not be visible to buyers: a buyer does not receive `INSUFFICIENT_STOCK` because of holds that are past their deadline but not yet swept.

Known limit: if the sweeper is releasing a due reservation at the same instant, the inline release skips that row rather than wait for it, and the request can get a `409` a few milliseconds before the stock is freed.

## 9. Frontend

> Brief §3: "The frontend mini-dashboard must be built strictly using React initialized with Vite (JavaScript or TypeScript)."

| ID | Requirement | Brief statement |
|---|---|---|
| FE-1 | Show `total_stock`, `reserved_stock` and `available_stock` for `item_4021`, refreshed automatically every 2 seconds and by a refresh button. | §3 (1): "Display current total, reserved, and available stock for an item with a manual refresh button or automated polling." |
| FE-2 | A form with User ID and Quantity that calls the reserve endpoint. It validates by the rules in D-5 and is disabled while a request is in flight. | §3 (2): "A clean form allowing a user to input a User ID and Quantity to request a reservation from the Gin backend." |
| FE-3 | After a successful reserve, show the reservation ID, item, quantity and a `mm:ss` countdown to `expires_at`. | §3 (3): "Show active reservation details alongside a live 5-minute countdown timer." |
| FE-4 | A "Confirm Purchase" button that calls the confirm endpoint and shows the confirmed state. | §3 (3): "Provide a "Confirm Purchase" button to execute order confirmation." |
| FE-5 | Every error code in §7 is rendered as a readable message in the page. When the countdown reaches zero, or the server answers `410`, the reservation is shown as expired and confirm is disabled. The server is the authority on expiry. | §3 (3): "Gracefully handle and render negative backend response states (e.g., failed reservations, expired sessions) directly on the React UI." |
| FE-6 | **Decision:** the countdown is computed from `expires_at`, not decremented locally; the active reservation survives a page reload; inputs are labelled, errors and countdown are announced to assistive technology, and the layout works at phone width. | Not addressed in the Brief. |

## 10. Operational requirements

| ID | Requirement | Brief statement |
|---|---|---|
| OP-1 | `docker compose up` starts PostgreSQL, applies the schema and seed, and starts the backend and the frontend, with no manual steps. | §4 (1): "provide a working docker-compose.yaml file that spins up the PostgreSQL container alongside the backend (and optionally the frontend) so the entire system can be run with a single command". |
| OP-2 | PostgreSQL is the only data store, in the application and in the tests that exercise storage. | §2: "in-memory databases, SQLite, or mock data stores are strictly disallowed". |
| OP-3 | On `SIGINT` or `SIGTERM` the server stops accepting requests, lets in-flight requests finish within 15 seconds, stops the sweeper after its current pass, then closes the database pool last. | §4 (4): "The Gin server must cleanly intercept system signals and shut down gracefully without corrupting state or dropping active database connections." |
| OP-4 | `go test -race -v ./...` passes with no skipped tests on a machine that has Go and Docker. | §4 (5): "Include unit and stress tests. The Go codebase will be validated using standard race detection: go test -race -v ./..." |

## 11. Acceptance criteria

Each criterion names the test that proves it.

### Reserve

| ID | Given / When / Then | Test |
|---|---|---|
| AC-RES-01 | Given stock 100 and none reserved, when a user reserves 2, then `201` with a `res_` ID and `expires_at` 5 minutes ahead, and `reserved_stock` is 2. | `TestReserve_Success` |
| AC-RES-02 | Given available stock 1, when a user reserves 2, then `409 INSUFFICIENT_STOCK` with `available_stock` 1, and nothing changes. | `TestReserve_InsufficientStock` |
| AC-RES-03 | When the item does not exist, then `404 ITEM_NOT_FOUND`. | `TestReserve_ItemNotFound` |
| AC-RES-04 | When the body is malformed, a field is missing, or `quantity` is 0, negative, fractional, a string, or above 1000, then `400 INVALID_REQUEST` naming the field. | `TestReserve_InvalidRequest` |

### Confirm

| ID | Given / When / Then | Test |
|---|---|---|
| AC-CON-01 | Given a pending reservation of 2 before its deadline, when confirmed, then `200` with `confirmed_at`; `total_stock` and `reserved_stock` each drop by 2; `available_stock` is unchanged. | `TestConfirm_Success` |
| AC-CON-02 | Given a confirmed reservation, when confirmed again, then `200` with the same `confirmed_at`, and stock does not change. | `TestConfirm_RepeatIsIdempotent` |
| AC-CON-03 | Given a reservation past its deadline that has not been released, when confirmed, then `410 RESERVATION_EXPIRED`. | `TestConfirm_PastDeadlineNotYetReleased` |
| AC-CON-04 | Given a released (`EXPIRED`) reservation, when confirmed, then `410 RESERVATION_EXPIRED`. | `TestConfirm_Expired` |
| AC-CON-05 | When the reservation does not exist, then `404 RESERVATION_NOT_FOUND`. | `TestConfirm_NotFound` |
| AC-CON-06 | When `reservation_id` is missing or invalid, then `400 INVALID_REQUEST`. | `TestConfirm_InvalidRequest` |

### Stock

| ID | Given / When / Then | Test |
|---|---|---|
| AC-STK-01 | Given stock 100 with 15 reserved, when queried, then `200` with 100 / 15 / 85. | `TestStock_Success` |
| AC-STK-02 | When the item does not exist, then `404 ITEM_NOT_FOUND`. | `TestStock_ItemNotFound` |
| AC-STK-03 | When `item_id` is missing or invalid, then `400 INVALID_REQUEST`. | `TestStock_InvalidRequest` |

### Expiry

| ID | Given / When / Then | Test |
|---|---|---|
| AC-EXP-01 | Given an item whose only free stock is held by reservations past their deadline, when a user reserves, then the reserve succeeds without waiting for the sweeper. The released reservations end `EXPIRED`, each released exactly once. | `TestReserve_ReleasesDueHoldsInline` |
| AC-EXP-02 | Given an item fully held by reservations that are not yet due, when a user reserves, then `409 INSUFFICIENT_STOCK` and those reservations stay `PENDING`. | `TestReserve_DoesNotReleaseActiveHolds` |
| AC-EXP-03 | Given a due reservation, when one sweeper pass runs, then it is `EXPIRED` and its quantity is back in `available_stock`. | `TestSweepOnce_ReturnsStock` |
| AC-EXP-04 | Given the sweeper running on its interval, when a reservation becomes due, then its stock is returned without any request being made. | `TestSweeperRun_ReleasesEventually` |

### Concurrency

| ID | Given / When / Then | Test |
|---|---|---|
| AC-CC-01 | Given stock 100, when 1000 concurrent requests each reserve 1, then exactly 100 succeed, 900 get `INSUFFICIENT_STOCK`, and `reserved_stock` is 100. | `TestStress_NoOversell` |
| AC-CC-02 | Given limited stock, when concurrent requests reserve random quantities 1–5, then the reserved total never exceeds stock and INV-1, INV-2 hold. | `TestStress_MixedQuantities` |
| AC-CC-03 | Given one pending reservation, when 50 concurrent requests confirm it, then all get `200` with the same `confirmed_at` and stock is decremented once. | `TestStress_DoubleConfirm` |
| AC-CC-04 | Given due reservations, when confirms and sweeper passes run concurrently, then each reservation ends in exactly one terminal state and INV-1…INV-5 hold. | `TestStress_ConfirmVsSweeper` |
| AC-CC-05 | Given an item whose holds have just become due, when many concurrent reserves run while sweeper passes run, then stock never goes negative, each reservation is released exactly once, and INV-1, INV-2 hold. | `TestStress_InlineReleaseVsSweeper` |
| AC-CC-06 | Given several items, when reserve, confirm and expiry run concurrently, then INV-1…INV-5 hold for every item at the end. | `TestStress_FullChurn` |

### Operations

| ID | Given / When / Then | Test |
|---|---|---|
| AC-OP-01 | Given a request in flight, when the process receives `SIGTERM`, then the request completes, the sweeper stops, and the pool closes last. | `TestShutdown_Graceful` |
| AC-OP-02 | Given a cancelled context, when the sweeper loop is running, then it returns promptly. | `TestSweeperRun_StopsOnCancel` |

Tests never use a fixed sleep. A reservation is made due by moving its `reservation_expires_at` into the past in SQL.
