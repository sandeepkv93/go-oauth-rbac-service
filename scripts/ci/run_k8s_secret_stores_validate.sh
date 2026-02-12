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
  "k8s/overlays/secrets/stores/aws-irsa"
  "k8s/overlays/secrets/stores/aws-static"
  "k8s/overlays/secrets/stores/vault-kubernetes"
  "k8s/overlays/secrets/stores/vault-token"
)

for target in "${TARGETS[@]}"; do
  echo "ci: kustomize build ${target}"
  manifest_file="$(mktemp /tmp/kustomize-secret-store.XXXXXX.yaml)"
  kustomize build "${target}" >"${manifest_file}"

  echo "ci: conftest cluster secret store policy (${target})"
  conftest test \
    --policy policy/k8s \
    --namespace k8s.clustersecretstore \
    "${manifest_file}"

  rm -f "${manifest_file}"
done

echo "ci: secret store overlays validation passed"
