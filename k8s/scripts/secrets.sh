#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${K8S_NAMESPACE:-secure-observable}"
TEMPLATE_FILE="k8s/base/secrets/app-secrets.env.template"
LOCAL_ENV_FILE="${K8S_APP_SECRETS_FILE:-.secrets.k8s.app.env}"
SOPS_ENCRYPTED_FILE="k8s/secrets/app-secrets.enc.env"

usage() {
  cat <<'EOF'
Usage:
  bash k8s/scripts/secrets.sh apply
  bash k8s/scripts/secrets.sh encrypt

Behavior:
  - apply: prefers SOPS encrypted file (k8s/secrets/app-secrets.enc.env) if present,
           otherwise uses local env file (.secrets.k8s.app.env).
  - encrypt: creates encrypted file from local env file using sops.
EOF
}

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "${cmd} not found" >&2
    exit 1
  fi
}

ensure_namespace() {
  require_cmd kubectl
  kubectl get ns "${NAMESPACE}" >/dev/null 2>&1 || kubectl create ns "${NAMESPACE}" >/dev/null
}

apply_from_env_file() {
  local env_file="$1"
  kubectl -n "${NAMESPACE}" create secret generic app-secrets \
    --from-env-file="${env_file}" \
    --dry-run=client -o yaml | kubectl apply -f -
}

encrypt_env_file() {
  require_cmd sops
  if [[ -z "${SOPS_AGE_RECIPIENTS:-}" ]]; then
    echo "SOPS_AGE_RECIPIENTS is required for encrypt mode" >&2
    exit 1
  fi
  mkdir -p "$(dirname "${SOPS_ENCRYPTED_FILE}")"
  sops --encrypt \
    --input-type dotenv \
    --output-type dotenv \
    --age "${SOPS_AGE_RECIPIENTS}" \
    "${LOCAL_ENV_FILE}" > "${SOPS_ENCRYPTED_FILE}"
  echo "Encrypted secret written to ${SOPS_ENCRYPTED_FILE}"
}

decrypt_to_temp_and_apply() {
  require_cmd sops
  local tmp_file
  tmp_file="$(mktemp)"
  trap 'rm -f "${tmp_file}"' EXIT
  sops --decrypt "${SOPS_ENCRYPTED_FILE}" > "${tmp_file}"
  apply_from_env_file "${tmp_file}"
}

cmd="${1:-}"
case "${cmd}" in
  apply)
    ensure_namespace
    if [[ -f "${SOPS_ENCRYPTED_FILE}" ]]; then
      decrypt_to_temp_and_apply
      echo "Applied app-secrets from SOPS encrypted file"
      exit 0
    fi
    if [[ ! -f "${LOCAL_ENV_FILE}" ]]; then
      cp "${TEMPLATE_FILE}" "${LOCAL_ENV_FILE}"
      echo "Created ${LOCAL_ENV_FILE} from template; update values before re-running" >&2
      exit 1
    fi
    apply_from_env_file "${LOCAL_ENV_FILE}"
    echo "Applied app-secrets from ${LOCAL_ENV_FILE}"
    ;;
  encrypt)
    if [[ ! -f "${LOCAL_ENV_FILE}" ]]; then
      cp "${TEMPLATE_FILE}" "${LOCAL_ENV_FILE}"
      echo "Created ${LOCAL_ENV_FILE} from template; update values before re-running" >&2
      exit 1
    fi
    encrypt_env_file
    ;;
  *)
    usage
    exit 1
    ;;
esac
