#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

echo "migration-smoke: up"
go run ./cmd/migrate up

echo "migration-smoke: down one step"
go run ./cmd/migrate down --steps 1

echo "migration-smoke: up again"
go run ./cmd/migrate up

echo "migration-smoke: completed"
