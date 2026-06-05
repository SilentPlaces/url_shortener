# URL Shortener

Local multi-service URL shortener stack with:

- `shortener-service`: create short aliases and fetch URL metadata
- `redirector-service`: resolve aliases and return HTTP redirects
- Local infrastructure via Docker Compose (`Postgres`, `Redis Stack`, `Redpanda` Kafka-compatible broker)

## Prerequisites

- Go `1.25+`
- Docker + Docker Compose

## Project Structure

- `services/shortener-service`: API for creating and reading short URLs
- `services/redirector-service`: API for redirect resolution
- `docker-compose.yml`: local infra (Postgres, Redis Stack, Redpanda)
- `migrate.sh`: applies SQL migrations
- `smoke.sh`: quick end-to-end smoke check

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
make test
```

## API

### Shortener Service (`http://localhost:8080`)

- `POST /api/v1/urls`
- `GET /api/v1/urls/{alias}`
- `GET /healthz`
- `GET /readyz`

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

Redirect check example:

```bash
curl -I http://localhost:8081/example123
```

## Environment Configuration

Services use environment variables with defaults for local development.

### Shortener (`SHORTENER_*`)

- `SHORTENER_HTTP_PORT` (default: `8080`)
- `SHORTENER_BASE_URL` (default: `http://localhost:8081`)
- `SHORTENER_POSTGRES_DSN` (default: `postgres://postgres:postgres@localhost:5432/url_shortener?sslmode=disable`)
- `SHORTENER_REDIS_ADDR` (default: `localhost:6379`)
- `SHORTENER_REDIS_PASSWORD` (default: empty)
- `SHORTENER_REDIS_DB` (default: `0`)
- `SHORTENER_KAFKA_BROKERS` (default: `localhost:9092`)
- `SHORTENER_KAFKA_TOPIC_PREFIX` (default: `url_shortener`)
- `SHORTENER_CACHE_PREFIX` (default: `shortener:`)
- `SHORTENER_CACHE_TTL_SECONDS` (default: `300`)
- `SHORTENER_BLOOM_KEY` (default: `shortener:aliases`)
- `SHORTENER_BLOOM_EXPECTED_ITEMS` (default: `1000000`)
- `SHORTENER_BLOOM_FALSE_POSITIVE_RATE` (default: `0.01`)
- `SHORTENER_ID_ALLOCATOR_KEY` (default: `shortener:id`)
- `SHORTENER_ID_ALLOCATOR_BATCH_SIZE` (default: `1024`)
- `SHORTENER_ID_ALLOCATOR_BUFFER_SIZE` (default: `2048`)
- `SHORTENER_REQUEST_TIMEOUT_SECONDS` (default: `10`)

### Redirector (`REDIRECTOR_*`)

- `REDIRECTOR_HTTP_PORT` (default: `8081`)
- `REDIRECTOR_POSTGRES_DSN` (default: `postgres://postgres:postgres@localhost:5432/url_shortener?sslmode=disable`)
- `REDIRECTOR_REDIS_ADDR` (default: `localhost:6379`)
- `REDIRECTOR_REDIS_PASSWORD` (default: empty)
- `REDIRECTOR_REDIS_DB` (default: `0`)
- `REDIRECTOR_CACHE_PREFIX` (default: `redirector:`)
- `REDIRECTOR_CACHE_TTL_SECONDS` (default: `300`)

## Common Commands

- `make up`: start local infra
- `make down`: stop local infra
- `make migrate`: apply DB migration
- `make run-shortener`: run shortener service
- `make run-redirector`: run redirector service
- `make test`: run both service test suites

## Smoke Test

With infra and both services running:

```bash
chmod +x ./smoke.sh && ./smoke.sh
```

This performs a create URL request, extracts the alias, and checks the redirect headers.

## Production Caveats

This is a learning project. Notable simplifications:

- `KafkaProducer` currently logs events instead of writing to Kafka. Wire it to a real client (`segmentio/kafka-go` or `franz-go`) before relying on event delivery.
- No authentication or rate limiting on the shortener API.
- No SSRF protection: any HTTP/HTTPS URL is accepted, including private and link-local addresses.
- The bloom filter is not rehydrated from the database on startup; after a Redis flush, false negatives may briefly occur (Postgres unique constraint still catches collisions).
- Cache TTL is short (default 300s) and entries are invalidated on `expires_at`/`is_active` mismatch, but there is no pub/sub invalidation across instances.
