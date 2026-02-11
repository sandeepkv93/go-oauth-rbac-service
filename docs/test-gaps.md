# Test Gaps

## Scope and Method

This gap analysis covers the full repository (all `internal/**`, `cmd/**`, and `test/integration/**`) using:

- Source inventory (`*.go` excluding generated tests)
- Test inventory from `docs/test_catalog.md`
- Route map from `internal/http/router/router.go`
- CI/task test commands from `scripts/ci/run_all.sh` and `taskfiles/go.yaml`

Current baseline from catalog:

- Test files: 64
- Unit test files: 45
- Integration test files: 19
- Declared test functions: 201

## High-Level Coverage Posture

Strong coverage already exists for:

- Local auth lifecycle, email verification, password reset flows
- Session management API flows
- RBAC forbidden path and permission-cache invalidation flow
- Admin list pagination/cache/singleflight/etag flows
- Idempotency and Redis race/replay scenarios
- Core middleware primitives (auth, RBAC, security headers/body limit, rate limiter behavior)
- Repository CRUD/filter/sort semantics for user/role/permission/local credential/verification token/oauth/session layers
- Redis-backed cache/guard/store semantics for admin-list, negative lookup, auth abuse, idempotency, and RBAC permission caches

Most meaningful gaps are concentrated in:

- Service business logic (`SessionService`, `UserService`)
- Observability utility surfaces with little/no tests
- Security and middleware adjuncts with sparse edge-path coverage
- CLI/tooling and startup wiring smoke paths

## P1 Gaps (Important)

### 9) Observability utility surfaces with little/no tests (unit)

Missing scenarios:

- `internal/observability/metrics.go`: each metric helper does not panic and sets expected label cardinality constraints.
- `internal/observability/logging.go`: request log field extraction and fallback values.
- `internal/observability/tracing.go`: tracer init no-op/fallback branches.
- `internal/observability/runtime.go`: runtime metrics start/stop behavior.

### 10) Security and middleware adjunct gaps (unit)

Missing scenarios:

- `security/cookie.go`: secure/samesite/domain cookie flags and clear-token semantics.
- `middleware/bypass_policy.go`: trusted CIDR parsing failures, actor bypass list behavior, method/path classification.
- `middleware/request_logging_middleware.go`: status/error logging fields and duration boundaries.
- `middleware/rate_limit_redis.go`: redis backend failure vs allow/deny policy semantics.

## P2 Gaps (Useful but Lower Immediate Risk)

### 11) Database/startup/tooling paths

Missing scenarios:

- `internal/database/postgres.go`: DSN handling, connect timeout, migration invocation failures.
- `internal/database/migrate.go`, `internal/database/seed.go`: command execution/reporting branches.
- `internal/app/app.go`: bootstrap/startup wiring smoke tests.
- `internal/tools/common/*`, `internal/tools/{migrate,seed,obscheck,ui}/command.go`: CLI arg validation, error propagation, output formatting.

### 12) Domain model tests

Current state:

- `internal/domain/*.go` has no tests.

Missing scenarios:

- Struct tag/backfill expectations (if relied upon by JSON/API contracts).
- Field defaults and status constants (if behaviorally significant).

Note:

- Domain models are mostly passive; prioritize above only if model logic/validation is added.

## Cross-Cutting Quality Gaps

- No fuzz tests (`Fuzz*`) currently present.
- No benchmark tests (`Benchmark*`) currently present.
- Redis race integration tests are skipped when docker unavailable; CI may miss these if environment lacks docker.
- No explicit flaky-test quarantine strategy in repo.

## Recommended Implementation Sequence

1. P1-9/10: Observability and security/middleware adjunct unit tests.
2. P2: Database/startup/tooling hardening coverage.

## Concrete New Test Files to Add

- `internal/security/cookie_test.go`
- `internal/http/middleware/bypass_policy_test.go`
- `internal/http/middleware/request_logging_middleware_test.go`
- `internal/http/middleware/rate_limit_redis_test.go`

## Assumptions and Unknowns

Assumptions:

- Existing integration harness (`newIntegrationHarness`) is the canonical API integration entrypoint.
- Redis race tests are intended to run in environments with docker available.

Unknowns:

- No explicit historical incident list is present; regression priorities are inferred from code complexity/security impact.
- Not all observability side effects are externally assertable without test hooks.
