#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

FUZZ_SMOKE_TIME="${FUZZ_SMOKE_TIME:-3s}"

echo "ci: fuzz smoke (${FUZZ_SMOKE_TIME})"

go test ./internal/http/response -run=^$ -fuzz=FuzzErrorContentNegotiationAndEnvelope -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/security -run=^$ -fuzz=FuzzParseAccessTokenRobustness -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/security -run=^$ -fuzz=FuzzParseRefreshTokenRobustness -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/security -run=^$ -fuzz=FuzzVerifySignedStateRobustness -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/http/middleware -run=^$ -fuzz=FuzzIdempotencyMiddlewareKeyAndBodyRobustness -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/http/middleware -run=^$ -fuzz=FuzzRequestBypassEvaluatorRobustness -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/http/middleware -run=^$ -fuzz=FuzzParseRedisInt64Robustness -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/http/middleware -run=^$ -fuzz=FuzzRedisFixedWindowLimiterAllowKeyFallback -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/service -run=^$ -fuzz=FuzzAuthServiceParseUserID -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/service -run=^$ -fuzz=FuzzAuthServiceTokenHandlingRejectsInvalid -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/service -run=^$ -fuzz=FuzzClassifyOAuthErrorRobustness -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/repository -run=^$ -fuzz=FuzzNormalizePageRequestInvariants -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/repository -run=^$ -fuzz=FuzzCalcTotalPagesInvariants -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/http/handler -run=^$ -fuzz=FuzzParseAdminListPageRequestRobustness -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/http/handler -run=^$ -fuzz=FuzzParseAdminListSortParamsRobustness -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/config -run=^$ -fuzz=FuzzNormalizeConfigProfileRobustness -fuzztime="${FUZZ_SMOKE_TIME}"
go test ./internal/tools/common -run=^$ -fuzz=FuzzLoadEnvFileRobustness -fuzztime="${FUZZ_SMOKE_TIME}"

echo "ci: fuzz smoke passed"
