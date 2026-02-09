#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

echo "migration-smoke: status"
go run ./cmd/migrate status --ci --env-file /dev/null

echo "migration-smoke: plan"
go run ./cmd/migrate plan --ci --env-file /dev/null

echo "migration-smoke: up"
go run ./cmd/migrate up --ci --env-file /dev/null

echo "migration-smoke: completed"
