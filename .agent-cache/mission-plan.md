# Mission Plan

## Objective
Enforce production/staging guardrails so sensitive Redis outage policy scopes cannot be configured to `fail_open`.

## Scope
- `internal/config/config.go`
- `internal/config/config_profile_test.go`
- `README.md`

## Success Criteria
- Validation rejects `fail_open` in production/staging for: auth, forgot, route_login, route_admin_write, route_admin_sync.
- Existing configurable behavior remains for non-sensitive scopes and non-prod envs.
- Tests cover negative and positive paths.
- Full test suite passes.

## DAG
- T1: Add validation rules for prod/staging sensitive scopes.
- T2: Add targeted config tests for reject/accept behavior.
- T3: Update production hardening docs.
- T4: Run targeted + full validation, commit, push.
