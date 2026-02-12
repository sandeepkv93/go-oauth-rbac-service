#!/usr/bin/env bash
set -euo pipefail

IMAGE="${IMAGE:-secure-observable-api:dev}"
CLUSTER_NAME="${KIND_CLUSTER_NAME:-secure-observable}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found" >&2
  exit 1
fi

if ! command -v kind >/dev/null 2>&1; then
  echo "kind not found" >&2
  exit 1
fi

docker build -t "${IMAGE}" .
kind load docker-image "${IMAGE}" --name "${CLUSTER_NAME}"

echo "Loaded ${IMAGE} into kind cluster ${CLUSTER_NAME}"
