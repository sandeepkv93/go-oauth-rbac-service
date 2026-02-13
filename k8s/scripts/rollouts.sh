#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${K8S_NAMESPACE:-secure-observable}"
ROLLOUT_NAME="${ROLLOUT_NAME:-secure-observable-api}"
ROLLOUT_ENV="${ROLLOUT_ENV:-staging}"
ALLOW_PROD_ROLLOUTS="${ALLOW_PROD_ROLLOUTS:-false}"
ACTION="${1:-status}"

has_rollouts_plugin() {
  kubectl argo rollouts version >/dev/null 2>&1
}

require_prod_confirmation() {
  if [[ "${ROLLOUT_ENV}" == "production" && "${ALLOW_PROD_ROLLOUTS}" != "true" ]]; then
    echo "Refusing ${ACTION} for production rollout without ALLOW_PROD_ROLLOUTS=true" >&2
    echo "Set ALLOW_PROD_ROLLOUTS=true only for approved production rollout windows." >&2
    exit 1
  fi
}

case "${ACTION}" in
  status)
    if has_rollouts_plugin; then
      kubectl argo rollouts get rollout "${ROLLOUT_NAME}" -n "${NAMESPACE}"
    else
      echo "rollouts plugin not found; falling back to kubectl get rollout"
      kubectl -n "${NAMESPACE}" get rollout "${ROLLOUT_NAME}" -o wide
    fi
    ;;
  promote)
    require_prod_confirmation
    if ! has_rollouts_plugin; then
      echo "kubectl argo rollouts plugin is required for promote action" >&2
      exit 1
    fi
    kubectl argo rollouts promote "${ROLLOUT_NAME}" -n "${NAMESPACE}"
    ;;
  abort)
    require_prod_confirmation
    if ! has_rollouts_plugin; then
      echo "kubectl argo rollouts plugin is required for abort action" >&2
      exit 1
    fi
    kubectl argo rollouts abort "${ROLLOUT_NAME}" -n "${NAMESPACE}"
    ;;
  *)
    echo "Usage: $0 [status|promote|abort]" >&2
    exit 1
    ;;
esac
