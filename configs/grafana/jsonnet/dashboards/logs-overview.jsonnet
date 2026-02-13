local dashboard = import '../lib/dashboard.libsonnet';
local panels = import '../lib/panels.libsonnet';
local q = import '../lib/queries/logs.libsonnet';

local panelList = [
  panels.logs(1, 'Application Logs Stream', 0, 0, 24, 10, q.appLogs),

  panels.stat(2, 'Error Count (1h)', 0, 10, 6, 6, 'loki', 'loki', q.errorCountLastHour, 'short', null, 'range'),
  panels.stat(3, 'Debug Log Count (1h)', 6, 10, 6, 6, 'loki', 'loki', q.warningCountLastHour, 'short', null, 'range'),
  panels.stat(4, 'Path-Tagged Logs (1h)', 12, 10, 6, 6, 'loki', 'loki', q.requestCountLastHour, 'short', null, 'range'),
  panels.stat(5, 'Path-Tagged Logs Volume (1h)', 18, 10, 6, 6, 'loki', 'loki', q.uniqueTraceIDsLastHour, 'short', null, 'range'),

  panels.timeseriesTargets(6, 'Log Volume by Level', 0, 16, 12, 8, 'loki', 'loki', [
    panels.target('A', q.logVolumeError, 'error', 'range'),
    panels.target('B', q.logVolumeWarn, 'warn', 'range'),
    panels.target('C', q.logVolumeInfo, 'info', 'range'),
    panels.target('D', q.logVolumeDebug, 'debug', 'range'),
  ], 'short'),

  panels.timeseriesTargets(7, 'HTTP Status Classes from Logs', 12, 16, 12, 8, 'loki', 'loki', [
    panels.target('A', q.status2xx, '2xx', 'range'),
    panels.target('B', q.status4xx, '4xx', 'range'),
    panels.target('C', q.status5xx, '5xx', 'range'),
  ], 'short'),

  panels.logs(8, 'Debug Logs', 0, 24, 12, 10, q.warningLogs),
  panels.logs(9, 'Path-Tagged Logs', 12, 24, 12, 10, q.requestLogs),

  panels.timeseries(10, 'Error/Warn Trends', 0, 34, 24, 8, 'loki', 'loki', q.errorByLevel, '{{level}}', 'short', null, null, 'range'),
  panels.logs(11, 'Trace-Correlated Logs', 0, 42, 24, 10, q.traceCorrelatedLogs),
];

dashboard.new('Logs Overview', 'logs-overview', 2, ['otel', 'logs', 'loki'], panelList)
