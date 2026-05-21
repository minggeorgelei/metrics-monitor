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
    <dl className="m-0 grid grid-cols-[minmax(96px,1fr)_minmax(96px,1fr)_minmax(126px,1.15fr)] gap-2.5 max-[760px]:grid-cols-1">
      <Stat label="Total" value={formatPercent(summary.totalBusy)} />
      <Stat label="Cores" value={summary.coreCount || '-'} />
      <Stat
        label="Sample"
        value={
          summary.sampleTime ? summary.sampleTime.toLocaleTimeString() : '-'
        }
      />
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

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="min-w-0 rounded-md border border-line bg-bg px-2.5 py-2">
      <dt className="text-xs font-bold uppercase leading-tight text-muted">
        {label}
      </dt>
      <dd className="mt-1 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-[15px] leading-tight text-fg-strong">
        {value}
      </dd>
    </div>
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
