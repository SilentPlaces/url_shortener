#!/usr/bin/env sh
set -eu

echo "Applying migrations..."
docker compose exec -T postgres psql \
  -U postgres \
  -d url_shortener \
  -v ON_ERROR_STOP=1 \
  -f /workspace/001_create_urls_table.sql

echo "Migrations applied."
