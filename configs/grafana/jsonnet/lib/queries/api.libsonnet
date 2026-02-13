local c = import 'common.libsonnet';

{
  requestRateByStatus: 'sum(rate(http_server_request_duration_seconds_count{job="%s"}[5m])) by (http_response_status_code)' % c.job,
  requestLatencyP95: 'histogram_quantile(0.95, sum(rate(http_server_request_duration_seconds_bucket{job="%s"}[5m])) by (le))' % c.job,
  requestLatencyP99: 'histogram_quantile(0.99, sum(rate(http_server_request_duration_seconds_bucket{job="%s"}[5m])) by (le))' % c.job,
  activeRequests: 'sum(http_server_active_requests{job="%s"}) or vector(0)' % c.job,

  requestClass2xx: 'sum(rate(http_server_request_duration_seconds_count{job="%s",http_response_status_code=~"2.."}[5m])) or vector(0)' % c.job,
  requestClass4xx: 'sum(rate(http_server_request_duration_seconds_count{job="%s",http_response_status_code=~"4.."}[5m])) or vector(0)' % c.job,
  requestClass5xx: 'sum(rate(http_server_request_duration_seconds_count{job="%s",http_response_status_code=~"5.."}[5m])) or vector(0)' % c.job,

  authLoginAttempts: 'sum(rate(auth_login_attempts_total{job="%s"}[5m])) by (status)' % c.job,
  authRefreshAttempts: 'sum(rate(auth_refresh_attempts_total{job="%s"}[5m])) by (status) or vector(0)' % c.job,
  rateLimitDecisions: 'sum(rate(http_rate_limit_decisions_total{job="%s"}[5m])) by (outcome)' % c.job,
  repositoryErrors: 'sum(rate(repository_operations_total{job="%s",outcome="error"}[5m])) by (repo) or vector(0)' % c.job,
  unhealthyChecks: 'sum(rate(health_check_results_total{job="%s",outcome="unhealthy"}[5m])) by (check) or vector(0)' % c.job,

  redisCommandLatencyP95: 'histogram_quantile(0.95, sum(rate(redis_command_duration_seconds_bucket{job="%s"}[5m])) by (le, command))' % c.job,
  redisErrorRate: 'redis_command_error_rate_ratio{job="%s"} or vector(0)' % c.job,
  redisPoolSaturation: 'redis_pool_saturation_ratio{job="%s"} or vector(0)' % c.job,
  redisHitRatio: 'redis_keyspace_hit_ratio_ratio{job="%s"} or vector(0)' % c.job,
  redisOpsByStatus: 'sum(rate(redis_command_total{job="%s"}[5m])) by (status)' % c.job,
  redisTopCommands: 'topk(5, sum(rate(redis_command_total{job="%s"}[5m])) by (command))' % c.job,
}
