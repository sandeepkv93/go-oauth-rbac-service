#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${K8S_NAMESPACE:-secure-observable}"
ROLLOUT_NAME="${ROLLOUT_NAME:-secure-observable-api}"
ROLLOUT_ENV="${ROLLOUT_ENV:-staging}"
PREVIEW_SERVICE="${ROLLOUT_PREVIEW_SERVICE:-secure-observable-api-preview}"
HEALTH_PORT="${K8S_ROLLOUT_HEALTH_PORT:-18081}"

MAX_RESTARTS_STAGING="${ROLLOUT_MAX_RESTARTS_STAGING:-2}"
MAX_RESTARTS_PRODUCTION="${ROLLOUT_MAX_RESTARTS_PRODUCTION:-1}"
REQUIRE_OBS_STAGING="${ROLLOUT_REQUIRE_OBS_STAGING:-false}"
REQUIRE_OBS_PRODUCTION="${ROLLOUT_REQUIRE_OBS_PRODUCTION:-true}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "rollout-precheck: missing required tool: $1" >&2
    exit 1
  fi
}

for tool in kubectl jq curl; do
  require_cmd "$tool"
done

select_env_value() {
  local staging_val="$1"
  local prod_val="$2"
  if [[ "${ROLLOUT_ENV}" == "production" ]]; then
    echo "${prod_val}"
  else
    echo "${staging_val}"
  fi
}

check_rollout_not_degraded() {
  local phase
  phase="$(kubectl -n "${NAMESPACE}" get rollout "${ROLLOUT_NAME}" -o json | jq -r '.status.phase // "Unknown"')"

  if [[ "${phase}" == "Degraded" ]]; then
    echo "rollout-precheck: rollout is Degraded" >&2
    return 1
  fi

  echo "rollout-precheck: rollout phase is ${phase}"
}

check_preview_service_health() {
  kubectl -n "${NAMESPACE}" port-forward "svc/${PREVIEW_SERVICE}" "${HEALTH_PORT}:8080" >/tmp/k8s-rollout-preview-pf.log 2>&1 &
  local pf_pid=$!
  trap 'kill ${pf_pid} >/dev/null 2>&1 || true' RETURN

  sleep 3
  curl -fsS "http://127.0.0.1:${HEALTH_PORT}/health/live" >/dev/null
  curl -fsS "http://127.0.0.1:${HEALTH_PORT}/health/ready" >/dev/null

  kill "${pf_pid}" >/dev/null 2>&1 || true
  trap - RETURN

  echo "rollout-precheck: preview service health checks passed"
}

check_restart_budget() {
  local max_restarts total_restarts
  max_restarts="$(select_env_value "${MAX_RESTARTS_STAGING}" "${MAX_RESTARTS_PRODUCTION}")"

  total_restarts="$(kubectl -n "${NAMESPACE}" get pods -l app.kubernetes.io/name=secure-observable-api -o json | jq '[.items[].status.containerStatuses[]? | .restartCount] | add // 0')"

  if [[ "${total_restarts}" -gt "${max_restarts}" ]]; then
    echo "rollout-precheck: restart budget exceeded (${total_restarts} > ${max_restarts})" >&2
    return 1
  fi

  echo "rollout-precheck: restart budget ok (${total_restarts} <= ${max_restarts})"
}

check_observability_slo_gate() {
  local require_obs
  require_obs="$(select_env_value "${REQUIRE_OBS_STAGING}" "${REQUIRE_OBS_PRODUCTION}")"

  if [[ "${require_obs}" != "true" ]]; then
    echo "rollout-precheck: observability gate skipped for ${ROLLOUT_ENV}"
    return 0
  fi

  echo "rollout-precheck: running observability alert gate"
  K8S_NAMESPACE="${NAMESPACE}" bash k8s/scripts/obs-alert-check.sh
}

main() {
  check_rollout_not_degraded
  check_preview_service_health
  check_restart_budget
  check_observability_slo_gate
  echo "rollout-precheck: PASSED"
}

main "$@"
