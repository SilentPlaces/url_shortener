SHELL := /bin/sh

.PHONY: up down migrate run-shortener run-redirector test

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

test:
	cd services/shortener-service && go test ./...
	cd services/redirector-service && go test ./...
