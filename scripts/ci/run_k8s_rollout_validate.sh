#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

for tool in kustomize conftest; do
  if ! command -v "${tool}" >/dev/null 2>&1; then
    echo "ci: missing required tool: ${tool}" >&2
    exit 1
  fi
done

target="k8s/overlays/rollouts/blue-green"

echo "ci: kustomize build ${target}"
manifest_file="$(mktemp /tmp/kustomize-rollout.XXXXXX.yaml)"
kustomize build "${target}" >"${manifest_file}"

echo "ci: conftest rollout policy"
conftest test \
  --policy policy/k8s \
  --namespace k8s.rollout \
  "${manifest_file}"

rm -f "${manifest_file}"

echo "ci: rollout overlay validation passed"
