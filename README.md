# QMart — Quick-Commerce Backend (Go)

A production-shaped quick-commerce / e-commerce backend written in **native Go**
(standard library `net/http` only — no web framework). It models a real
Q-Commerce flow: catalog, per-store inventory, cart, order checkout with
inventory reservation, payments, role-based access control, and a Swagger UI.

Built to demonstrate the skills from a Backend Engineer (Golang) role:
scalable REST APIs, SQL discipline, concurrency, clean layering, Docker,
PostgreSQL, and API documentation.

## Stack & key decisions

| Concern            | Choice                                                                 |
| ------------------ | ---------------------------------------------------------------------- |
| Language           | Go (stdlib `net/http` with method + wildcard routing)                    |
| Database           | PostgreSQL via `pgx` (`database/sql` stdlib driver)                     |
| Migrations         | Embedded SQL migrations applied by a homegrown runner                   |
| Auth / RBAC        | HMAC-SHA256 signed tokens (JWT-shaped) + role claims (`admin`/`customer`) |
| API docs           | Swagger UI served at `/swagger/`, generated from annotations (`swag`)   |
| Logging            | `log/slog` structured logging                                           |
| Containerization   | Multi-stage `Dockerfile` + `docker-compose` (Postgres + API + seed)     |
| Tests              | Store integration tests boot Postgres via `dockertest`                  |
| Money              | Integer cents (`int64`) — no float rounding                            |

## Run it with Docker

```powershell
cd qmart
docker compose up --build
```

This starts PostgreSQL, seeds demo data, and serves the API on `:8080`.
Swagger UI: http://localhost:8080/swagger/index.html

## Run it locally (against your own Postgres)

```bash
export QMART_DSN="postgres://qmart:qmart@localhost:5432/qmart?sslmode=disable"
go run ./cmd/seed      # load demo catalog + inventory
go run ./cmd/api       # serve on :8080
```

> PowerShell: `$env:QMART_DSN = "postgres://qmart:qmart@localhost:5432/qmart?sslmode=disable"`

## Exercise the API

The examples below use `curl` (available on macOS, Linux, WSL, and Windows).
`sed` is used only to pull the token out of the JSON response.

```bash
# health
curl -s http://localhost:8080/healthz

# get a customer token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"email":"buyer@example.com","name":"Buyer"}' \
  | sed -E 's/.*"token":"([^"]+)".*/\1/')

# get an admin token (email listed in QMART_ADMIN_EMAILS)
ADMIN=$(curl -s -X POST http://localhost:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","name":"Admin"}' \
  | sed -E 's/.*"token":"([^"]+)".*/\1/')

# browse catalog (public)
curl -s "http://localhost:8080/api/v1/products?category=dairy"

# add to cart -> checkout -> pay (customer)
curl -s -X POST http://localhost:8080/api/v1/cart/items \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"product_id":1,"quantity":2}'

curl -s -X POST http://localhost:8080/api/v1/orders \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"store_id":"store-bali-01","delivery_fee_cents":500}'

curl -s -X POST http://localhost:8080/api/v1/orders/1/payments \
  -H "Authorization: Bearer $TOKEN"

curl -s -X POST http://localhost:8080/api/v1/payments/webhook \
  -H "Content-Type: application/json" \
  -d '{"order_id":1,"status":"paid"}'

# admin-only: advance order status
curl -s -X POST http://localhost:8080/api/v1/orders/1/status \
  -H "Authorization: Bearer $ADMIN" -H "Content-Type: application/json" \
  -d '{"status":"confirmed"}'
```

> Windows PowerShell equivalent: swap `curl -s ...` for
> `Invoke-RestMethod -Uri ... -Method Post -Headers $h -ContentType application/json -Body '...'`.
> Easiest of all: use the interactive Swagger UI at
> http://localhost:8080/swagger/index.html

## Authentication & roles

`POST /api/v1/auth/token` issues a signed bearer token for a demo user. The
role is assigned at issuance: emails in `QMART_ADMIN_EMAILS` (comma-separated)
get `admin`, everyone else `customer`.

- `customer` — browse catalog, manage their cart, checkout, pay.
- `admin` — additionally create/update/delete products, adjust inventory, and
  advance order status.

Endpoints are guarded by `auth.RequireRole(...)`; unauthorized access returns
`403 Forbidden`.

## API surface

Full interactive docs: http://localhost:8080/swagger/index.html

| Method | Path                          | Auth    | Description                          |
| ------ | ----------------------------- | ------- | ------------------------------------ |
| GET    | `/healthz`                    | no      | Liveness                             |
| POST   | `/api/v1/auth/token`          | no      | Issue a bearer token                 |
| GET    | `/api/v1/products`            | no      | List catalog (category/q/cursor)     |
| GET    | `/api/v1/products/{id}`       | no      | Product detail                       |
| POST   | `/api/v1/products`            | admin   | Create product                       |
| PUT    | `/api/v1/products/{id}`       | admin   | Update product                       |
| DELETE | `/api/v1/products/{id}`       | admin   | Delete product                       |
| GET    | `/api/v1/inventory/{id}`      | auth    | Per-store stock for a product        |
| POST   | `/api/v1/inventory`           | admin   | Adjust on-hand / reserved stock      |
| GET    | `/api/v1/cart`                | auth    | Current cart + totals                |
| POST   | `/api/v1/cart/items`          | auth    | Add item (upsert)                    |
| PUT    | `/api/v1/cart/items/{id}`     | auth    | Update quantity (0 removes)          |
| DELETE | `/api/v1/cart/items/{id}`     | auth    | Remove item                          |
| POST   | `/api/v1/orders`              | auth    | Checkout cart → order (reserve stock)|
| GET    | `/api/v1/orders`              | auth    | List orders (cursor)                 |
| GET    | `/api/v1/orders/{id}`         | auth    | Order + items                        |
| POST   | `/api/v1/orders/{id}/status`  | admin   | Advance lifecycle                    |
| POST   | `/api/v1/orders/{id}/payments`| auth    | Create (mock) payment                |
| POST   | `/api/v1/payments/webhook`    | no      | Provider callback (simplified)       |

## Project structure

```
qmart/
├── cmd/
│   ├── api/main.go          # entrypoint: config, store, graceful shutdown
│   └── seed/main.go         # demo data loader
├── docs/                    # generated Swagger (swag init)
├── internal/
│   ├── auth/                # HMAC token sign/verify + RBAC middleware
│   ├── config/              # env-based config with safe defaults
│   ├── handler/             # HTTP handlers (thin) + swagger annotations
│   ├── httpx/               # JSON encode/decode + error envelope helpers
│   ├── middleware/          # request id, access log, panic recovery
│   ├── model/               # domain types + custom timestamp scanner
│   ├── server/              # route wiring + middleware stack
│   └── store/               # DB access: migrations + one file per aggregate
│       └── migrations/      # embedded SQL migrations
├── Dockerfile
└── docker-compose.yml
```

## Tests

```powershell
go test ./...
```

Store integration tests boot a throwaway PostgreSQL container via
[`dockertest`](https://github.com/ory/dockertest) and are skipped automatically
when no Docker daemon is reachable. Unit tests (`internal/auth`) run without
Docker. `go vet ./...` and `gofmt -l .` should both be clean.

To regenerate the Swagger docs after changing annotations:

```powershell
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/api/main.go -o docs --parseInternal
```

## Design highlights

- **Inventory accounting** — three states, all atomic:
  - *checkout* reserves stock (`reserved += qty`, available = `quantity - reserved`),
  - *payment* settles it (`quantity -= qty`, `reserved -= qty`),
  - *cancel* releases it (`reserved -= qty`).
  The `UPDATE ... WHERE quantity - reserved >= $n` guard makes overselling
  impossible without an explicit lock.
- **No N+1** — cart/order reads join line items in one query; checkout
  snapshots prices inside a single transaction.
- **Keyset pagination** — list endpoints use `id` cursors, not `OFFSET`, so
  deep pages stay fast.
- **RBAC** — role is carried in the signed token and enforced by middleware,
  not spread across handlers.
- **Graceful shutdown** — signal-aware `http.Server.Shutdown` with a timeout.
- **Defense in depth** — request-id, panic recovery, structured logs, body size
  limits, `DisallowUnknownFields` decoding.

## Notes & next steps (deliberately out of scope)

- The payment webhook is simplified: a real integration verifies provider
  signatures (e.g. Xendit's) before mutating state.
- Auth is passwordless for the demo; swap `auth.Token` for a real credential
  exchange / OIDC in production.
- Product delete is a hard delete; order history keeps references, so prefer
  soft-delete (`is_active=false`) once a catalog has order history.
