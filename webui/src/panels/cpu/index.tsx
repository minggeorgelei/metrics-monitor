import type { Metric } from '../../lib/api';
import { registerPanel } from '../registry';
import { PanelShell } from '../shell/PanelShell';
import type { PanelProps } from '../types';
import { CPUChart } from './CPUChart';

/**
 * CPUPanel composes the panel chrome (PanelShell) with the per-core
 * chart (CPUChart) and a small stats summary rendered in the header's
 * "actions" slot.
 *
 * Computation lives here rather than inside CPUChart so the chart stays
 * single-purpose ("render this time series shape"). When Step D adds
 * other CPU-derived panels (e.g. "load average"), they share CPUChart
 * but each does its own summarisation.
 */
export function CPUPanel({ metrics }: PanelProps) {
  const cpuMetrics = metrics.cpu;
  const summary = summarize(cpuMetrics);

  const actions = (
    <dl className="stats">
      <div>
        <dt>Total</dt>
        <dd>{formatPercent(summary.totalBusy)}</dd>
      </div>
      <div>
        <dt>Cores</dt>
        <dd>{summary.coreCount || '-'}</dd>
      </div>
      <div>
        <dt>Sample</dt>
        <dd>
          {summary.sampleTime ? summary.sampleTime.toLocaleTimeString() : '-'}
        </dd>
      </div>
    </dl>
  );

  return (
    <PanelShell
      title="CPU usage"
      subtitle="% busy, last 60 seconds"
      actions={actions}
    >
      <CPUChart cpuMetrics={cpuMetrics} />
    </PanelShell>
  );
}

function summarize(cpuMetrics: Metric[] | undefined) {
  let totalBusy: number | undefined;
  let sampleTime: Date | undefined;
  let coreCount = 0;

  for (const metric of cpuMetrics ?? []) {
    const cpu = metric.tags.cpu;
    const idle = metric.fields.usage_idle;
    if (!cpu || typeof idle !== 'number') continue;

    if (cpu !== 'cpu-total') coreCount += 1;
    const metricTime = new Date(metric.time);
    if (!sampleTime || metricTime > sampleTime) sampleTime = metricTime;
    if (cpu === 'cpu-total') totalBusy = 100 - idle;
  }

  return { totalBusy, coreCount, sampleTime };
}

function formatPercent(value: number | undefined) {
  return typeof value === 'number' ? `${value.toFixed(1)}%` : '-';
}

// Self-register at module load. main.tsx blank-imports this file so
// registration happens before <App /> renders.
registerPanel({
  id: 'cpu-usage',
  title: 'CPU usage',
  metricNames: ['cpu'],
  refreshIntervalMs: 1000,
  Component: CPUPanel,
});
