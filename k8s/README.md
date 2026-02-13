# Kubernetes Guide

This directory contains the complete Kubernetes deployment stack for the service, including local Kind workflows, environment overlays, rollout governance, observability, and CI validation gates.

## What Is Here

- Base app stack: API, Postgres, Redis, namespace, services, ingress, and core config.
- Environment overlays: `development`, `prod-like`, `staging`, `production`.
- Optional observability stack: OTel Collector + Tempo + Loki + Mimir + Grafana.
- Optional blue/green rollout overlay using Argo Rollouts.
- Secret bootstrapping and optional external secret store overlays.
- Local automation scripts + Task aliases.
- CI validation scripts (manifest schema, OPA policies, rollout gates).

## Directory Layout

```text
k8s/
  base/                         # Core manifests (API + Postgres + Redis)
    configmaps/
    deployments/
    services/
    ingress/
    secrets/
    persistentvolumes/
  overlays/
    development/                # Local dev profile (NodePort API)
    prod-like/                  # Hardened baseline profile
    staging/                    # Maintenance-friendly availability profile
    production/                 # Strict no-downtime API profile
    observability-base/         # Shared observability components
    observability/
      dev/                      # dev observability profile
      ci/                       # low-resource CI profile
      prod-like/                # persistent + retention tuning
      prod-like-ha/             # optional HA knobs
    rollouts/blue-green/        # Argo Rollouts blue/green resources
    secrets/
      external-secrets/         # ExternalSecret overlays (dev/staging/prod)
      stores/                   # ClusterSecretStore variants (AWS/Vault)
  scripts/                      # Deploy/status/health/rollout/secrets helpers
  kind-config.yaml              # Kind config for ingress-ready local cluster
  kind-config-simple.yaml       # Minimal Kind config
  README.md
```

## Environment Profiles

### App profiles

- `base`: minimal deployable app stack.
- `development`: local-focused profile.
- `prod-like`: baseline hardening (`replicas: 2`, API PDB `minAvailable: 1`).
- `staging`: faster rollouts and controlled disruption (`maxUnavailable: 1`).
- `production`: strict API availability (`replicas: 3`, `maxUnavailable: 0`, API PDB `minAvailable: 2`).

### Observability profiles

- `observability`: default overlay.
- `observability-dev`: local lightweight behavior.
- `observability-ci`: constrained resources for CI.
- `observability-prod-like`: PVC-backed persistence + retention defaults.
- `observability-prod-like-ha`: optional additional HA scaling knobs.

## Prerequisites

- `kubectl` (with kustomize support)
- `docker`
- `kind` (for local cluster flows)
- `task`
- `go` (for tooling like `loadgen` in runtime checks)
- Optional: `sops` for encrypted secret workflows

## Quick Start (Local Kind)

1. Create secrets file:

```bash
cp k8s/base/secrets/app-secrets.env.template .secrets.k8s.app.env
```

2. Bring up local stack:

```bash
task k8s:setup-full
```

3. Verify health:

```bash
task k8s:health-check
```

4. Inspect status:

```bash
task k8s:status
```

5. Cleanup:

```bash
task k8s:cleanup
task k8s:cluster-delete
```

## Deploy Commands

### App deploys

```bash
task k8s:deploy-base
task k8s:deploy-dev
task k8s:deploy-prod-like
task k8s:deploy-staging
task k8s:deploy-production
```

### Observability deploys

```bash
task k8s:deploy-observability
task k8s:deploy-observability-dev
task k8s:deploy-observability-ci
task k8s:deploy-observability-prod-like
task k8s:deploy-observability-prod-like-ha
```

## Rollout (Blue/Green)

Overlay: `k8s/overlays/rollouts/blue-green`

```bash
task k8s:deploy-rollout-bluegreen
task k8s:rollout-status
task k8s:rollout-precheck
task k8s:rollout-promote
task k8s:rollout-abort
```

Production rollout controls:

```bash
task k8s:rollout-precheck-production
task k8s:rollout-promote-production ALLOW_PROD_ROLLOUTS=true
task k8s:rollout-abort-production ALLOW_PROD_ROLLOUTS=true
```

Notes:

- Promotion runs SLO-linked prechecks by default.
- Break-glass bypass exists: `SKIP_PROMOTION_GATES=true`.
- Governance details: `docs/k8s-rollout-governance.md`.

## Secrets

### Local env-file secret path

```bash
task k8s:secrets-generate
task k8s:secrets-apply
```

### Optional encrypted secret path

```bash
task k8s:secrets-encrypt
```

### Optional external secrets/store overlays

```bash
task k8s:validate-external-secrets
task k8s:validate-secret-stores

task k8s:apply-external-secrets-dev
task k8s:apply-external-secrets-staging
task k8s:apply-external-secrets-prod

task k8s:apply-secret-store-aws-irsa
task k8s:apply-secret-store-aws-static
task k8s:apply-secret-store-vault-kubernetes
task k8s:apply-secret-store-vault-token
```

## Validation and Policy Gates

### Core CI-aligned checks

```bash
task k8s:validate-manifests
task k8s:policy-check
task k8s:validate-availability-profiles
task k8s:validate-rollout-overlay
task k8s:validate-rollout-precheck-script
task k8s:validate-rollout-runtime-script
task k8s:validate-obs-alert-script
```

### Observability/SLO helper checks

```bash
task k8s:obs-status
task k8s:obs-capacity-check
task k8s:obs-alert-check
```

## Runtime Kind Rollout Check (CI)

Workflow: `.github/workflows/k8s-kind-smoke.yml`

Runtime stage executes:

- Kind cluster bring-up
- Argo Rollouts plugin + controller setup
- Observability CI overlay deploy
- Blue/green rollout deploy
- Traffic generation (`loadgen`)
- Rollout precheck execution
- Evidence artifact upload (`k8s-rollout-evidence`)

Evidence artifact typically contains:

- `rollout-precheck.txt`
- `loadgen.txt`
- `rollout.yaml`
- `pods.txt`, `services.txt`, `events.txt`
- Prometheus query snapshots (5xx ratio, redis error/saturation)

## Troubleshooting

- Cluster resources:

```bash
task k8s:status
task k8s:obs-status
```

- API logs:

```bash
task k8s:logs-api
```

- Port-forward API/Grafana:

```bash
task k8s:port-forward-api
task k8s:port-forward-grafana
```

- Rollout/controller checks:

```bash
bash k8s/scripts/check-rollouts-controller.sh
task k8s:rollout-status
```

## Source of Truth

- Task entrypoints: `taskfiles/k8s.yaml`
- Operational rollout policy: `docs/k8s-rollout-governance.md`
- CI gates: `.github/workflows/ci.yml`, `.github/workflows/k8s-kind-smoke.yml`
