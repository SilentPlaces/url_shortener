SHELL := /bin/sh

.PHONY: up down migrate run-shortener run-redirector test test-all tidy

up:
	docker compose up -d

down:
	docker compose down

migrate:
	chmod +x ./migrate.sh && ./migrate.sh

run-shortener:
	cd services/shortener-service && go run ./cmd

run-redirector:
	cd services/redirector-service && go run ./cmd

# test runs the service-level suites (used by CI for fast feedback).
test:
	cd services/shortener-service && go test ./...
	cd services/redirector-service && go test ./...

# test-all also runs the shared packages.
test-all:
	cd pkg/httpx && go test ./...
	cd pkg/safeurl && go test ./...
	cd services/shortener-service && go test ./...
	cd services/redirector-service && go test ./...

# tidy keeps every module's go.sum in sync after dependency changes.
tidy:
	cd pkg/httpx && go mod tidy
	cd pkg/safeurl && go mod tidy
	cd services/shortener-service && go mod tidy
	cd services/redirector-service && go mod tidy
