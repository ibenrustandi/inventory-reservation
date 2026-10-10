# Inventory Reservation

A flash-sale inventory service that never oversells, with a small dashboard.

- **Backend:** Go, Gin, PostgreSQL
- **Frontend:** React, Vite, TypeScript

Documents: [SPEC.md](SPEC.md) (what the system does and how it is verified) · [ARCHITECTURE.md](ARCHITECTURE.md) (why it is built this way)

## Run everything

Requires Docker with Compose.

```bash
docker compose up --build
```

| What | URL |
|---|---|
| Dashboard | http://localhost:5173 |
| API | http://localhost:8080/api/v1/inventory |
| PostgreSQL | `localhost:5432`, user, password and database all `inventory` |

The database schema and one item, `item_4021` with 100 units, are created on start-up.

**If a port is already in use**, override it:

```bash
FRONTEND_PORT=3000 BACKEND_PORT=8081 POSTGRES_PORT=5433 docker compose up --build
```

**To watch a reservation expire without waiting five minutes:**

```bash
RESERVATION_TTL=30s docker compose up --build
```

**To reset the data** to the seeded 100 units:

```bash
docker compose down -v
```

## Run the parts separately

Requires Go 1.26+ and Node.js 24+.

**1. PostgreSQL**

```bash
docker compose up -d postgres
```

**2. Gin backend**

```bash
cd backend
DATABASE_URL='postgres://inventory:inventory@localhost:5432/inventory?sslmode=disable' go run ./cmd/web
```

It applies the migrations, then listens on http://localhost:8080.

**3. React + Vite frontend**

```bash
cd frontend
npm install
npm run dev
```

Open http://localhost:5173. The dev server forwards `/api` to `http://localhost:8080`; set `API_PROXY_TARGET` to point it elsewhere.

## Configuration

Backend settings are environment variables.

| Variable | Default | Meaning |
|---|---|---|
| `DATABASE_URL` | required | PostgreSQL connection string |
| `DB_MAX_CONNS` | `20` | Largest number of PostgreSQL connections this instance opens |
| `PORT` | `8080` | HTTP port |
| `RESERVATION_TTL` | `5m` | How long a reservation holds stock |
| `SWEEP_INTERVAL` | `1s` | How often expired reservations are released in the background |
| `REQUEST_TIMEOUT` | `10s` | Longest one request may run before it is cancelled and answered with `503`; must be below `SHUTDOWN_TIMEOUT` |
| `SHUTDOWN_TIMEOUT` | `15s` | How long in-flight requests get to finish on shutdown |

## Tests

**Backend** — requires Docker to be running; the integration and stress tests start their own PostgreSQL container.

```bash
cd backend
go test -race -v ./...
```

To use a database that is already running instead, set `TEST_DATABASE_URL`.

| Folder | Contents |
|---|---|
| `backend/test/unit` | Handlers and use cases with test doubles; no database |
| `backend/test/integration` | Every endpoint, expiry, graceful shutdown and the concurrency stress suite, against real PostgreSQL |

**Frontend**

```bash
cd frontend
npm run lint
npm run test
npm run build
```

## API

Base path: `/api/v1/inventory`. Full contract: [SPEC.md §7](SPEC.md).

**Check stock**

```bash
curl 'http://localhost:8080/api/v1/inventory/stock?item_id=item_4021'
```

```json
{"item_id":"item_4021","total_stock":100,"reserved_stock":0,"available_stock":100}
```

**Reserve stock** — `201 Created`

```bash
curl -X POST http://localhost:8080/api/v1/inventory/reserve \
  -d '{"user_id":"usr_9981","item_id":"item_4021","quantity":2}'
```

```json
{"status":"success","reservation_id":"res_…","item_id":"item_4021","quantity":2,"expires_at":"2026-07-20T16:35:00Z"}
```

**Confirm a reservation** — `200 OK`

```bash
curl -X POST http://localhost:8080/api/v1/inventory/confirm \
  -d '{"reservation_id":"res_…"}'
```

```json
{"status":"success","reservation_id":"res_…","confirmed_at":"2026-07-20T16:32:00Z"}
```

**Errors** all use one envelope:

```json
{
  "status": "error",
  "error": {
    "code": "INSUFFICIENT_STOCK",
    "message": "Not enough stock is available.",
    "details": { "available_stock": 0, "requested": 1 }
  },
  "request_id": "req_…"
}
```

| HTTP | Code | When |
|---|---|---|
| 400 | `INVALID_REQUEST` | Malformed JSON, or a missing or invalid field (`details.fields` lists them) |
| 404 | `ITEM_NOT_FOUND` | The item does not exist |
| 404 | `RESERVATION_NOT_FOUND` | The reservation does not exist |
| 409 | `INSUFFICIENT_STOCK` | Not enough available stock |
| 410 | `RESERVATION_EXPIRED` | The reservation passed its deadline |
| 404 | `NOT_FOUND` | Unknown route |
| 405 | `METHOD_NOT_ALLOWED` | Wrong method for a route |
| 500 | `INTERNAL_ERROR` | Unexpected failure; details are logged, never returned |
| 503 | `SERVICE_UNAVAILABLE` | The request ran out of time (`REQUEST_TIMEOUT`); sent with `Retry-After` |

Health: `GET /health` (process is up) and `GET /ready` (database answers).

## Project layout

```
backend/
  cmd/web/                  entry point, dependency wiring, graceful shutdown
  config/                   environment settings
  db/migrations/            SQL migrations, applied in order on start-up
  internal/
    delivery/http/          handler, middleware, route
    usecase/                business logic
    repository/             PostgreSQL queries
    domain/                 core types
    exception/              errors returned to clients
    model/                  request and response structures
    infrastructure/         Gin, PostgreSQL pool, migrations, logger, HTTP server
    sweeper/                background release of expired reservations
  test/                     unit and integration tests
frontend/
  src/api/                  typed fetch client
  src/components/           StockPanel, ReserveForm, ReservationCard
  src/hooks/                data and timer hooks (TanStack Query)
  src/lib/                  validation, countdown, error messages, storage
docker-compose.yaml
```
