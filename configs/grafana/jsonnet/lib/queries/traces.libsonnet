local c = import 'common.libsonnet';

local selector = '{service_name="%s"}' % c.serviceName;

{
  traceCorrelatedLogRate: 'sum(rate(%s | json | path!="" [5m]))' % selector,
  authRequestP95: 'histogram_quantile(0.95, sum(rate(auth_request_duration_seconds_bucket{job="%s"}[5m])) by (le, endpoint))' % c.job,
  traceEventsByType: 'sum by (path) (rate(%s | json | path!="" [5m]))' % selector,
  recentTraceCorrelatedLogs: '%s | json | path!=""' % selector,

  uniqueTraceIDsLastHour: 'sum(count_over_time(%s | json | path!="" [1h]))' % selector,
  traceStatus2xxRate: 'sum(rate(%s | json | status >= 200 and status < 300 [5m]))' % selector,
  traceStatus4xxRate: 'sum(rate(%s | json | status >= 400 and status < 500 [5m]))' % selector,
  traceStatus5xxRate: '(sum(rate(%s | json | status >= 500 and status < 600 [5m])) or vector(0))' % selector,
  traceLinkedRequests: 'sum(rate(%s | json | path!="" [5m]))' % selector,
}
