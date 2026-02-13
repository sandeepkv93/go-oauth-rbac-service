local c = import 'common.libsonnet';

local selector = '{service_name="%s"}' % c.serviceName;

{
  appLogs: selector,
  warningLogs: '%s | json | level="debug"' % selector,
  requestLogs: '%s | json | path!=""' % selector,
  traceCorrelatedLogs: '%s | json | trace_id!=""' % selector,

  errorCountLastHour: '(sum(count_over_time(%s | json | level="error" [1h])) or vector(0))' % selector,
  warningCountLastHour: 'sum(count_over_time(%s | json | level="debug" [1h]))' % selector,
  requestCountLastHour: 'sum(count_over_time(%s | json | path!="" [1h]))' % selector,
  uniqueTraceIDsLastHour: 'sum(count_over_time(%s | json | path!="" [1h]))' % selector,

  logVolumeError: '(sum(count_over_time(%s | json | level="error" [5m])) or vector(0))' % selector,
  logVolumeWarn: 'sum(count_over_time(%s | json | level="debug" [5m]))' % selector,
  logVolumeInfo: 'sum(count_over_time(%s | json | level="info" [5m]))' % selector,
  logVolumeDebug: 'sum(count_over_time(%s | json | level="debug" [5m]))' % selector,

  status2xx: 'sum(count_over_time(%s | json | status >= 200 and status < 300 [5m]))' % selector,
  status4xx: 'sum(count_over_time(%s | json | status >= 400 and status < 500 [5m]))' % selector,
  status5xx: '(sum(count_over_time(%s | json | status >= 500 and status < 600 [5m])) or vector(0))' % selector,

  errorByLevel: 'sum by (level) (count_over_time(%s | json | level=~"error|warn|debug" [5m]))' % selector,
}
