# Kubernetes Rollout Governance

This document defines ownership and promotion criteria for the optional Argo Rollouts blue/green path.

## Scope
- Overlay: `k8s/overlays/rollouts/blue-green`
- Commands:
  - `task k8s:deploy-rollout-bluegreen`
  - `task k8s:rollout-status`
  - `task k8s:rollout-precheck`
  - `task k8s:rollout-precheck-production`
  - `task k8s:rollout-promote`
  - `task k8s:rollout-abort`
  - `task k8s:rollout-promote-production ALLOW_PROD_ROLLOUTS=true`
  - `task k8s:rollout-abort-production ALLOW_PROD_ROLLOUTS=true`

## Controller Lifecycle Ownership (Decision Closure)
- Platform/SRE is the single owner for Argo Rollouts controller lifecycle:
  - CRD/controller install, version pinning, upgrades, rollback, and backup/restore validation.
  - Cluster-level RBAC and kubectl plugin support guidance.
  - Upgrade cadence and compatibility testing with cluster Kubernetes version.
- Application team owns rollout spec behavior only:
  - rollout strategy parameters, probe correctness, and promotion/abort execution.
- Promotion authority boundary:
  - staging promotions: app team on-call + service owner.
  - production promotions: app team + platform approver during approved change window.

## Environment Policy
- Staging is the mandatory proving ground for rollout operations.
- Production promote/abort requires explicit confirmation via `ALLOW_PROD_ROLLOUTS=true`.
- Production operations must occur only in approved change windows.

## SLO-Linked Promotion Criteria (Enforced)
Promotion now runs SLO-linked gates automatically via `k8s/scripts/rollout-precheck.sh` unless explicitly bypassed with `SKIP_PROMOTION_GATES=true`.

### Gate checks
1. Rollout non-degraded state:
- `status.phase` must not be `Degraded`.
2. Preview service health:
- `/health/live` and `/health/ready` on `secure-observable-api-preview` must pass.
3. Restart budget:
- staging: total API pod restarts `<= 2`
- production: total API pod restarts `<= 1`
4. Observability alert gate (`k8s/scripts/obs-alert-check.sh`):
- staging: optional by default (`ROLLOUT_REQUIRE_OBS_STAGING=false`)
- production: required by default (`ROLLOUT_REQUIRE_OBS_PRODUCTION=true`)

## Measurable Rollback Triggers (Abort Immediately)
Abort rollout (`task k8s:rollout-abort` / `task k8s:rollout-abort-production`) when any trigger holds:
1. Rollout enters `Degraded` phase.
2. Preview health endpoints fail (`/health/live` or `/health/ready`).
3. Restart budget breached:
- staging: restarts > 2
- production: restarts > 1
4. Observability gate fails:
- API 5xx ratio exceeds threshold (`ALERT_API_5XX_MAX`, default `0.05`)
- Redis error/saturation thresholds exceeded
- required metric checks missing when enforcement flags are true.

## Bake Windows
- Staging: minimum 10 minutes under representative traffic.
- Production: minimum 20 minutes under representative traffic with observability gate passing.

## Operational Overrides (Break-Glass)
- `SKIP_PROMOTION_GATES=true` bypasses SLO-linked prechecks.
- Must be used only under incident command or declared emergency maintenance.
- Every bypass requires an incident/audit note and post-incident review.

## Audit Trail Requirements
Record for each production promote/abort:
- operator identity,
- rollout revision hash,
- promotion timestamp,
- precheck evidence references (health + observability snapshots),
- outcome and any follow-up actions.
