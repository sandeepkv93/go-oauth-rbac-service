#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${K8S_NAMESPACE:-secure-observable}"

if ! kubectl get crd rollouts.argoproj.io >/dev/null 2>&1; then
  echo "ERROR: Argo Rollouts CRD (rollouts.argoproj.io) not found in cluster." >&2
  echo "Install Argo Rollouts before applying blue/green overlay." >&2
  exit 1
fi

if ! kubectl -n argo-rollouts get deploy argo-rollouts >/dev/null 2>&1; then
  echo "WARN: argo-rollouts controller deployment not found in namespace argo-rollouts." >&2
  echo "Continuing because controller namespace/deployment may be customized." >&2
fi

if ! kubectl argo rollouts version >/dev/null 2>&1; then
  echo "WARN: kubectl argo rollouts plugin not available; promote/abort commands will fail until installed." >&2
fi

echo "rollouts-controller-check: ok for namespace ${NAMESPACE}"
