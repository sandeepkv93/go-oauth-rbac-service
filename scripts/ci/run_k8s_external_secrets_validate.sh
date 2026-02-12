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

TARGETS=(
  "k8s/overlays/secrets/external-secrets/dev"
  "k8s/overlays/secrets/external-secrets/staging"
  "k8s/overlays/secrets/external-secrets/prod"
)

for target in "${TARGETS[@]}"; do
  echo "ci: kustomize build ${target}"
  manifest_file="$(mktemp /tmp/kustomize-external-secrets.XXXXXX.yaml)"
  kustomize build "${target}" >"${manifest_file}"

  echo "ci: conftest external secrets policy (${target})"
  conftest test \
    --policy policy/k8s \
    --namespace k8s.externalsecret \
    "${manifest_file}"

  rm -f "${manifest_file}"
done

echo "ci: external secrets overlay validation passed"
