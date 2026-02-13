#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

SCRIPT="k8s/scripts/rollout-precheck.sh"

if [[ ! -f "${SCRIPT}" ]]; then
  echo "k8s rollout precheck: missing script ${SCRIPT}" >&2
  exit 1
fi

bash -n "${SCRIPT}"

if ! grep -q "rollout-precheck: PASSED" "${SCRIPT}"; then
  echo "k8s rollout precheck: expected success marker not found" >&2
  exit 1
fi

echo "k8s rollout precheck: script sanity passed"
