import { useEffect, useRef } from 'react';
import type { RefObject } from 'react';
import * as echarts from 'echarts';
import type { ECharts, EChartsOption } from 'echarts';

/**
 * useECharts mounts an ECharts instance into a div ref. The
 * `staticOption` is applied exactly once at mount — anything that
 * changes per-frame (xAxis range, series data) belongs in subsequent
 * imperative `chartRef.current.setOption(...)` calls.
 *
 * Why the split:
 *   - With `notMerge: true` on every update, ECharts re-evaluates the
 *     full option, recreates series, re-allocates colors, and re-runs
 *     layout on every tick. Fine for one chart, expensive for a
 *     dashboard with many panels.
 *   - With this hook the "scaffolding" (axes/grid/legend/tooltip/colors)
 *     is set once. Per-tick updates only ship the bits that changed.
 *
 * The hook returns both the container ref (for the <div>) and the
 * chart ref (so the component can call setOption / dispatchAction /
 * appendData imperatively without going through React state).
 */
export function useECharts(staticOption: EChartsOption): {
  containerRef: RefObject<HTMLDivElement | null>;
  chartRef: RefObject<ECharts | null>;
} {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<ECharts | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;
    const element = containerRef.current;
    const chart = echarts.init(element);
    chart.setOption(staticOption);
    chartRef.current = chart;

    const onResize = () => chart.resize();
    const resizeObserver = new ResizeObserver(onResize);
    resizeObserver.observe(element);
    window.addEventListener('resize', onResize);

    return () => {
      window.removeEventListener('resize', onResize);
      resizeObserver.disconnect();
      chart.dispose();
      chartRef.current = null;
    };
    // staticOption is intentionally NOT a dependency. The contract
    // is "set once at mount" — callers that want to re-theme should
    // unmount and remount the component, or grab chartRef and apply
    // the change themselves.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return { containerRef, chartRef };
}
