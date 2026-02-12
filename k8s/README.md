# Kubernetes Deployment (Phase 1 MVP)

This directory contains the initial Kubernetes baseline for:
- API (`secure-observable-api`)
- Postgres
- Redis

The manifests are organized with Kustomize and target local/dev workflows first.

## Prerequisites

- `kubectl` (with Kustomize support)
- Kubernetes cluster (local or remote)
- Docker (if building image locally)
- Optional: `kind` for local cluster workflows
- Optional: `sops` + age keys for encrypted secret workflow

## Layout

```text
k8s/
  base/
    kustomization.yaml
    namespace.yaml
    configmaps/
    secrets/
    deployments/
    services/
    persistentvolumes/
    ingress/
```

## 1) Build API image

For local clusters (for example Kind), build the app image first:

```bash
docker build -t secure-observable-api:dev .
```

Or use automation:

```bash
task k8s:image-build-load-kind
```

## 2) Create app secret from template

A template is provided at:

`k8s/base/secrets/app-secrets.env.template`

Create a local copy and set strong values:

```bash
cp k8s/base/secrets/app-secrets.env.template .secrets.k8s.app.env
```

Create/replace Kubernetes secret:

```bash
kubectl -n secure-observable create secret generic app-secrets \
  --from-env-file=.secrets.k8s.app.env \
  --dry-run=client -o yaml | kubectl apply -f -
```

Task wrapper:

```bash
task k8s:secrets-apply
```

Encrypted secret path (recommended):

```bash
export SOPS_AGE_RECIPIENTS='age1yourrecipientpublickey'
task k8s:secrets-encrypt
```

When `k8s/secrets/app-secrets.enc.env` exists, `task k8s:secrets-apply`
decrypts and applies that secret instead of plaintext env file.

## 3) Apply manifests

```bash
kubectl apply -k k8s/base
kubectl -n secure-observable rollout status statefulset/postgres
kubectl -n secure-observable rollout status statefulset/redis
kubectl -n secure-observable rollout status deployment/secure-observable-api
```

Task wrappers:

```bash
task k8s:deploy-base
task k8s:rollout
```

## 4) Verify health

```bash
kubectl -n secure-observable port-forward svc/secure-observable-api 8080:8080
curl -sSf http://localhost:8080/health/live
curl -sSf http://localhost:8080/health/ready
```

## Notes

- `AUTH_GOOGLE_ENABLED` is disabled in this Phase 1 baseline.
- Observability components are intentionally out of this baseline and will be added in a later phase.
- Ingress is enabled in base (`secure-observable.local`, `ingressClassName: nginx`).
- One-command local setup is available:

```bash
task k8s:setup-full
```
