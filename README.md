Steps to Run
# Events Ingestion & Metrics API (Go 1.25 + Postgres)

A minimal, production-friendly MVP that ingests user events and exposes aggregate metrics.
Runs locally with **Docker Desktop** (no local Postgres required).

## ✨ Features

- **POST /events** – enqueue single event (async write, 202 Accepted)
- **POST /events/bulk** – enqueue up to 100 events in one request
- **GET /metrics** – totals and optional daily buckets, filterable by `event_name` and `channel`
- Validates payloads; JSONB `metadata` and `tags` supported
- Idempotency via `event_id` or `(event_name,user_id,timestamp)` composite
- Built-in rate limiting for metrics
- OpenAPI file served at `/openapi.yaml`
- One-command up via Docker Compose

## 🧱 Tech

- Go **1.25**
- Postgres **16** (Docker)
- `pgx` driver, no ORM
- Multi-stage Dockerfile, non-root runtime
- Simple in-process queue + batch writer

## 🗂️ Repo structure

- api/openapi.yaml
- cmd/events-api/main.go
- internal/
- config/… # env parsing
- domain/… # Event model + validation
- idempotency/… # idempotency key derivation
- ingest/… # async queue + batch flush
- storage/postgres/… # DB connect, insert, metrics queries
- transport/http/… # handlers, middleware, rate limiting
- migrations/0001_init.sql # schema & indexes
- docker-compose.yml
- Dockerfile

## ⚙️ Requirements

- Docker Desktop (or Docker Engine + docker compose plugin)

## 🚀 Quick start

```bash
# from repo root
docker compose build
docker compose up -d
docker compose ps

curl http://localhost:8080/healthz
curl http://localhost:8080/readyz



Security

This repo contains no secrets. Default credentials are dev-only.

For any shared/staging deployment, set API_KEYS and change the DB password/DSN.

Avoid sending PII in metadata unless you add proper controls (encryption, minimization).

🧪 Troubleshooting

POST returns 202 but metrics show 0: widen your time window; batch flush is async. Check app logs.

No rows inserted: see logs for [ingest] batch insert FAILED; verify DSN and DB health.

Rate limited on metrics: 429 with Retry-After; raise RATE_LIMIT_METRICS_PER_MIN or set to 0 locally.

Port conflict on 5432/8080: edit ports: in docker-compose.yml.


Single POST
curl --location 'http://localhost:8080/events' \
--header 'Content-Type: application/json' \
--data '{
    "event_name": "purchase 11",
    "user_id": "u1",
    "timestamp": 1690000000,
    "channel": "ios"
  }'

Bulk POST
curl --location 'http://localhost:8080/events/bulk' \
--header 'Content-Type: application/json' \
--data '{
    "events": [
        {
            "event_name": "purchase bulk 22",
            "user_id": "u1",
            "timestamp": 1690000000,
            "channel": "ios"
        },
        {
            "event_name": "purchase bulk 33",
            "user_id": "u1",
            "timestamp": 1690000000,
            "channel": "web"
        }
    ]
}'

curl --location 'http://localhost:8080/events/bulk' \
--header 'Content-Type: application/json' \
--data '{
    "events": [
        {
            "event_name": "purchase",
            "user_id": "u100",
            "timestamp": 1700000000,
            "channel": "ios",
            "campaign_id": "cmp-2025-09",
            "tags": [
                "new",
                "promo"
            ],
            "metadata": {
                "order_id": "o-1001",
                "amount": 39.99,
                "currency": "USD",
                "items": [
                    {
                        "sku": "sku-1",
                        "qty": 1,
                        "price": 19.99
                    },
                    {
                        "sku": "sku-2",
                        "qty": 1,
                        "price": 20.00
                    }
                ],
                "coupon": "WELCOME10"
            }
        },
        {
            "event_name": "purchase",
            "user_id": "u101",
            "timestamp": 1700000001,
            "channel": "web",
            "tags": [
                "returning",
                "weekend"
            ],
            "metadata": {
                "order_id": "o-1002",
                "amount": 12.50,
                "currency": "EUR",
                "items": [
                    {
                        "sku": "sku-3",
                        "qty": 2,
                        "price": 6.25
                    }
                ],
                "referrer": "adwords"
            }
        },
        {
            "event_name": "add_to_cart",
            "user_id": "u102",
            "timestamp": 1700000002,
            "channel": "android",
            "campaign_id": "cmp-remarket",
            "tags": [
                "mobile",
                "funnel"
            ],
            "metadata": {
                "cart_value": 75.00,
                "currency": "USD",
                "items": [
                    {
                        "sku": "sku-4",
                        "qty": 3,
                        "price": 25.00
                    }
                ]
            }
        },
        {
            "event_name": "purchase",
            "user_id": "u103",
            "timestamp": 1700000003,
            "channel": "ios",
            "tags": [
                "vip",
                "promo"
            ],
            "metadata": {
                "order_id": "o-1003",
                "amount": 120.00,
                "currency": "USD",
                "payment_method": "apple_pay",
                "shipping": {
                    "method": "express",
                    "fee": 9.99
                }
            }
        },
        {
            "event_name": "signup",
            "user_id": "u104",
            "timestamp": 1700000004,
            "channel": "web",
            "tags": [
                "activation"
            ],
            "metadata": {
                "plan": "pro",
                "utm": {
                    "source": "newsletter",
                    "campaign": "sept-launch"
                }
            }
        }
    ]
}'


GET Requests

curl --location 'http://localhost:8080/metrics?from=1699990000&to=1700010000&group_by=day'
curl --location 'http://localhost:8080/metrics?event_name=purchase&from=1690000000&to=1700010000'
curl --location 'http://localhost:8080/metrics?event_name=purchase&channel=ios&from=1699990000&to=1700010000&group_by=day'
curl --location 'http://localhost:8080/metrics?event_name=purchase&from=1700000000&to=1700000000'