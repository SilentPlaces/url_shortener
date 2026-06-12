# URL Shortener

Local multi-service URL shortener stack with:

- `shortener-service`: create short aliases and fetch URL metadata
- `redirector-service`: resolve aliases and return HTTP redirects
- Local infrastructure via Docker Compose (`Postgres`, `Redis Stack`, `Redpanda` Kafka-compatible broker)
- Shared Go packages under `pkg/` for HTTP middleware, Prometheus metrics, OpenTelemetry tracing setup, and SSRF-aware URL validation

## Prerequisites

- Go `1.25+`
- Docker + Docker Compose

## Project Structure

- `services/shortener-service`: API for creating and reading short URLs
- `services/redirector-service`: API for redirect resolution
- `pkg/httpx`: shared request logging, request ID, Prometheus middleware, OTel tracing setup
- `pkg/safeurl`: SSRF-aware URL validator (blocks loopback, private, link-local, multicast, metadata endpoints)
- `docker-compose.yml`: local infra (Postgres, Redis Stack, Redpanda)
- `migrate.sh`: applies SQL migrations
- `smoke.sh`: quick end-to-end smoke check
- `go.work`: Go workspace tying every module together for local development

## Quick Start

Start infrastructure:

```bash
make up
make migrate
```

Run services in separate terminals:

```bash
make run-shortener
make run-redirector
```

Run tests:

```bash
make test       # services only
make test-all   # services + shared pkg/* modules
```

## API

### Shortener Service (`http://localhost:8080`)

- `POST /api/v1/urls`
- `GET /api/v1/urls/{alias}`
- `GET /healthz`
- `GET /readyz`
- `GET /metrics` (Prometheus exposition format)

Create URL example:

```bash
curl -sS -X POST http://localhost:8080/api/v1/urls \
  -H "Content-Type: application/json" \
  -d '{"original_url":"https://example.com","custom_alias":"example123"}'
```

### Redirector Service (`http://localhost:8081`)

- `GET /{alias}` returns `307 Temporary Redirect` for active aliases
- `GET /healthz`
- `GET /readyz`
- `GET /metrics` (Prometheus exposition format)

Redirect check example:

```bash
curl -I http://localhost:8081/example123
```

## Observability

Both services expose:

- `GET /metrics` — Prometheus exposition. Notable counters:
  - `*_http_requests_total{method,route,status}` / `*_http_request_duration_seconds`
  - Shortener: `shortener_cache_hits_total`, `shortener_cache_misses_total`, `shortener_bloom_collisions_total`, `shortener_alias_collisions_total`, `shortener_url_created_total{source}`
  - Redirector: `redirector_cache_hits_total`, `redirector_cache_misses_total`
- Structured request logs: `request_id=<hex> method=<m> path=<p> status=<n> duration_ms=<ms>`
- OpenTelemetry trace spans (per-request `<METHOD> <route>` server spans). Disabled by default; set `SHORTENER_TRACING_ENABLED=true` / `REDIRECTOR_TRACING_ENABLED=true` to emit pretty-printed JSON spans to stderr. Swap the stdout exporter in `pkg/httpx/tracing.go` for OTLP / Jaeger / etc. in production.

## SSRF Protection

The shortener validates submitted URLs via `pkg/safeurl` before persisting:

- Scheme must be `http` or `https`
- No userinfo (`user:pass@`)
- Port must be empty / `80` / `443`
- Host must resolve to a public IP — loopback, private (RFC 1918), link-local, multicast, CGNAT, and the AWS/GCP metadata endpoint (`169.254.169.254`) are rejected

Set `SHORTENER_ALLOW_PRIVATE_URLS=true` to bypass the IP check in local dev or tests; do not do this in production.

## Event Publishing

The shortener emits `URLCreated` events (and is wired for `URLClicked` / `URLExpired` / `URLDeactivated`) via `kafka-go`. If `SHORTENER_KAFKA_BROKERS` is unset, the service falls back to a logging publisher so it stays usable without a running broker. Per-URL events use the URL ID as the partition key so consumers see them in order.

## Bloom Filter Rehydration

On startup the shortener can re-warm its Redis bloom filter from Postgres in keyset-paginated batches (`SHORTENER_BLOOM_REHYDRATE_BATCH_SIZE`, default 1000). Disable with `SHORTENER_BLOOM_REHYDRATE_ENABLED=false`. Without rehydration the Postgres unique constraint still catches collisions; rehydration just minimises wasted alias-generation retries.

## Cache Stampede Protection

Both services use `golang.org/x/sync/singleflight` so concurrent lookups for the same alias coalesce into a single DB read on cache miss.

## Environment Configuration

Services use environment variables with defaults for local development.

### Shortener (`SHORTENER_*`)

- `SHORTENER_HTTP_PORT` (default: `8080`)
- `SHORTENER_BASE_URL` (default: `http://localhost:8081`)
- `SHORTENER_POSTGRES_DSN` (default: `postgres://postgres:postgres@localhost:5432/url_shortener?sslmode=disable`)
- `SHORTENER_REDIS_ADDR` (default: `localhost:6379`)
- `SHORTENER_REDIS_PASSWORD` (default: empty)
- `SHORTENER_REDIS_DB` (default: `0`)
- `SHORTENER_KAFKA_BROKERS` (default: `localhost:9092`; empty → log fallback)
- `SHORTENER_KAFKA_TOPIC_PREFIX` (default: `url_shortener`)
- `SHORTENER_CACHE_PREFIX` (default: `shortener:`)
- `SHORTENER_CACHE_TTL_SECONDS` (default: `300`)
- `SHORTENER_BLOOM_KEY` (default: `shortener:aliases`)
- `SHORTENER_BLOOM_EXPECTED_ITEMS` (default: `1000000`)
- `SHORTENER_BLOOM_FALSE_POSITIVE_RATE` (default: `0.01`)
- `SHORTENER_BLOOM_REHYDRATE_ENABLED` (default: `true`)
- `SHORTENER_BLOOM_REHYDRATE_BATCH_SIZE` (default: `1000`)
- `SHORTENER_ID_ALLOCATOR_KEY` (default: `shortener:id`)
- `SHORTENER_ID_ALLOCATOR_BATCH_SIZE` (default: `1024`)
- `SHORTENER_ID_ALLOCATOR_BUFFER_SIZE` (default: `2048`)
- `SHORTENER_REQUEST_TIMEOUT_SECONDS` (default: `10`)
- `SHORTENER_ALLOW_PRIVATE_URLS` (default: `false`)
- `SHORTENER_TRACING_ENABLED` (default: `false`)
- `SHORTENER_TRACING_SAMPLE_RATIO` (default: `1.0`)
- `SHORTENER_SERVICE_NAME` (default: `shortener-service`)

### Redirector (`REDIRECTOR_*`)

- `REDIRECTOR_HTTP_PORT` (default: `8081`)
- `REDIRECTOR_POSTGRES_DSN` (default: `postgres://postgres:postgres@localhost:5432/url_shortener?sslmode=disable`)
- `REDIRECTOR_REDIS_ADDR` (default: `localhost:6379`)
- `REDIRECTOR_REDIS_PASSWORD` (default: empty)
- `REDIRECTOR_REDIS_DB` (default: `0`)
- `REDIRECTOR_CACHE_PREFIX` (default: `redirector:`)
- `REDIRECTOR_CACHE_TTL_SECONDS` (default: `300`)
- `REDIRECTOR_TRACING_ENABLED` (default: `false`)
- `REDIRECTOR_TRACING_SAMPLE_RATIO` (default: `1.0`)
- `REDIRECTOR_SERVICE_NAME` (default: `redirector-service`)

## Common Commands

- `make up`: start local infra
- `make down`: stop local infra
- `make migrate`: apply DB migration
- `make run-shortener`: run shortener service
- `make run-redirector`: run redirector service
- `make test`: run service test suites
- `make test-all`: run service + shared package suites
- `make tidy`: `go mod tidy` every module

## Smoke Test

With infra and both services running:

```bash
chmod +x ./smoke.sh && ./smoke.sh
```

This performs a create URL request, extracts the alias, and checks the redirect headers.

## Known Limitations

- No authentication on either service.
- No rate limiting.
- Tracing exports to stderr by default; replace the exporter for OTLP / Jaeger / Honeycomb in production.
- No automatic recovery from Redis flushes other than bloom rehydration: cache entries simply repopulate on read.
