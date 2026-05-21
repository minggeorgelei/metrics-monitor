import { useEffect, useRef, useState } from 'react';
import type { EChartsOption } from 'echarts';

import { useECharts } from '../../hooks/useECharts';
import type { Metric } from '../../lib/api';

/**
 * CPUChart shows per-core CPU usage (= 100 - usage_idle) over a sliding
 * 60-second window.
 *
 * Two state mechanisms cooperate here:
 *
 *   1. `historyRef` — a Map<seriesName, Point[]> living in a ref so
 *      successive renders accumulate points instead of resetting the
 *      buffer.
 *   2. `chartRef` from useECharts — the ECharts instance itself. We
 *      issue imperative `setOption({ xAxis, series })` updates on
 *      every cpuMetrics change. This is the perf-critical path:
 *      ECharts only re-evaluates the bits we ship, not the whole
 *      scaffolding.
 *
 * Each Metric arrives with tag.cpu = "cpu0", "cpu1", ..., "cpu-total".
 * "cpu-total" is rendered thicker and pinned first in the legend.
 */

const WINDOW_SECONDS = 60;

interface Point {
  t: number; // unix ms
  value: number;
}

interface Props {
  /** All metrics from snapshot.metrics["cpu"] for this render. */
  cpuMetrics: Metric[] | undefined;
}

// STATIC_OPTION holds everything that does NOT change frame-to-frame:
// axes scaffolding, grid, legend, tooltip, colors, animation. ECharts
// keeps these in place between renders so the per-tick update only
// has to ship `xAxis.min/max` and `series[].data`.
const STATIC_OPTION: EChartsOption = {
  animation: false, // streaming data — disable the per-update tween
  color: [
    '#2563eb',
    '#16a34a',
    '#ea580c',
    '#9333ea',
    '#0891b2',
    '#dc2626',
    '#65a30d',
    '#7c3aed',
    '#0f766e',
    '#c2410c',
  ],
  grid: { left: 46, right: 18, top: 42, bottom: 32 },
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'line' },
    valueFormatter: (value) => `${Number(value).toFixed(1)}%`,
  },
  legend: {
    top: 4,
    type: 'scroll',
    itemWidth: 18,
    itemHeight: 10,
    textStyle: { color: '#5b6472' },
  },
  xAxis: {
    type: 'time',
    splitLine: { show: false },
    axisLine: { lineStyle: { color: '#d6dbe3' } },
    axisLabel: { color: '#697386' },
  },
  yAxis: {
    type: 'value',
    min: 0,
    max: 100,
    axisLabel: { formatter: '{value}%', color: '#697386' },
    axisLine: { show: false },
    splitLine: { lineStyle: { color: '#e7eaf0' } },
  },
};

export function CPUChart({ cpuMetrics }: Props) {
  const historyRef = useRef<Map<string, Point[]>>(new Map());
  const [hasData, setHasData] = useState(false);
  const { containerRef, chartRef } = useECharts(STATIC_OPTION);

  useEffect(() => {
    if (!cpuMetrics) return;

    // 1. Append new samples into the per-series history buffer.
    //    Done in an effect (not during render) so React StrictMode's
    //    double-invocation in dev doesn't double-mutate the ref.
    const cutoff = Date.now() - WINDOW_SECONDS * 1000;
    for (const m of cpuMetrics) {
      const cpu = m.tags.cpu;
      if (!cpu) continue;
      // % busy = 100 - usage_idle. The first Gather only emits
      // time_* counters (no usage_idle yet), so we skip.
      const idle = m.fields.usage_idle;
      if (typeof idle !== 'number') continue;
      const busy = 100 - idle;

      const buf = historyRef.current.get(cpu) ?? [];
      const t = new Date(m.time).getTime();
      const lastPoint = buf.at(-1);
      if (lastPoint?.t === t) {
        // Same sample arrived again (snapshot polled before next
        // input gather). Replace in place rather than duplicating.
        lastPoint.value = busy;
      } else {
        buf.push({ t, value: busy });
      }
      while (buf.length > 0 && buf[0].t < cutoff) buf.shift();
      historyRef.current.set(cpu, buf);
    }

    setHasData(historyHasData(historyRef.current));

    // 2. Ship JUST the changed bits to ECharts. No full option
    //    rebuild, no notMerge — the static scaffolding stays put.
    const chart = chartRef.current;
    if (!chart) return;

    const names = Array.from(historyRef.current.keys()).sort((a, b) => {
      if (a === 'cpu-total') return -1;
      if (b === 'cpu-total') return 1;
      return a.localeCompare(b, undefined, { numeric: true });
    });
    const latestTs =
      Math.max(
        0,
        ...Array.from(historyRef.current.values()).map(
          (pts) => pts.at(-1)?.t ?? 0,
        ),
      ) || Date.now();
    const windowEnd = latestTs + 1000;
    const windowStart = windowEnd - WINDOW_SECONDS * 1000;

    chart.setOption({
      xAxis: { min: windowStart, max: windowEnd },
      series: names.map((cpu) => ({
        // `id` makes ECharts merge by identity rather than index;
        // safe even if a series joins/leaves the legend mid-stream.
        id: cpu,
        name: cpu,
        type: 'line',
        showSymbol: true,
        symbolSize: cpu === 'cpu-total' ? 5 : 3,
        smooth: true,
        lineStyle: { width: cpu === 'cpu-total' ? 3 : 1.4 },
        data: (historyRef.current.get(cpu) ?? []).map((p) => [p.t, p.value]),
      })),
    });
  }, [cpuMetrics, chartRef]);

  return (
    <div className="relative min-h-0 flex-1 px-3 pb-4 pt-3">
      <div ref={containerRef} className="h-full w-full" />
      {!hasData && (
        <div className="pointer-events-none absolute inset-3 grid place-items-center text-[15px] text-muted">
          waiting for CPU usage samples...
        </div>
      )}
    </div>
  );
}

function historyHasData(history: Map<string, Point[]>) {
  for (const points of history.values()) {
    if (points.length > 0) return true;
  }
  return false;
}
