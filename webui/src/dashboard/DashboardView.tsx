import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
// In react-grid-layout v2 the main entry switched to a hook-based
// API; the classic Responsive + WidthProvider components live under
// the /legacy subpath. Same component API as v1.
import { Responsive, WidthProvider } from 'react-grid-layout/legacy';
import type { Layout, LayoutItem } from 'react-grid-layout';

import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';

import { getDashboard } from '../dashboards/registry';
import type { DashboardDefinition, GridItem } from '../dashboards/types';
import { fetchSnapshot, type Metric, type Snapshot } from '../lib/api';
import { getPanel } from '../panels/registry';
import type { PanelDefinition } from '../panels/types';

import {
  loadLayouts,
  mergeLayouts,
  saveLayouts,
  type DashboardLayouts,
} from './layout-storage';

const ResponsiveGridLayout = WidthProvider(Responsive);

const BREAKPOINTS = { lg: 1200, md: 996, sm: 768, xs: 480 } as const;
const COLS = { lg: 24, md: 18, sm: 12, xs: 4 } as const;
const BREAKPOINT_ORDER = ['lg', 'md', 'sm', 'xs'] as const;
const ROW_HEIGHT = 30; // px
const MARGIN: [number, number] = [12, 12];
const DEFAULT_REFRESH_INTERVAL_MS = 1000;
type BreakpointName = (typeof BREAKPOINT_ORDER)[number];

interface Props {
  dashboardId: string;
}

/**
 * DashboardView is the unified renderer for any dashboard. It:
 *
 *   1. Looks up the dashboard definition.
 *   2. Groups its panels by refresh interval, computes the union of
 *      metric names per group.
 *   3. Renders each panel inside a PanelHost that uses
 *      `useQuery + select` against the group's shared queryKey —
 *      one HTTP request per group, per-panel re-render isolation
 *      via TanStack Query's structuralSharing on the select output.
 */
export function DashboardView({ dashboardId }: Props) {
  return <DashboardViewInner key={dashboardId} dashboardId={dashboardId} />;
}

interface PanelGroup {
  intervalMs: number;
  /** Union of metric names needed by any panel sharing this group's
   *  interval. Sorted so the queryKey hashes the same across all
   *  sibling panels. */
  names: string[];
}

function DashboardViewInner({ dashboardId }: Props) {
  const dashboard = useMemo(() => getDashboard(dashboardId), [dashboardId]);

  const groupByPanelId = useMemo<Map<string, PanelGroup>>(
    () => (dashboard ? buildPanelGroups(dashboard) : new Map()),
    [dashboard],
  );

  const defaultLayouts = useMemo<DashboardLayouts>(
    () => (dashboard ? buildDefaultLayouts(dashboard) : {}),
    [dashboard],
  );

  const [layouts, setLayouts] = useState<DashboardLayouts>(() => {
    const saved = loadLayouts(dashboardId);
    return saved ? mergeLayouts(saved, defaultLayouts) : defaultLayouts;
  });

  if (!dashboard) {
    return (
      <div className="py-20 text-center text-[15px] text-red-700">
        unknown dashboard "{dashboardId}"
      </div>
    );
  }

  const handleLayoutChange = (_current: Layout, all: DashboardLayouts) => {
    setLayouts(all);
    saveLayouts(dashboardId, all);
  };

  return (
    <div className="relative">
      {dashboard.description && (
        <p className="mx-1 mb-3.5 text-sm text-muted">
          {dashboard.description}
        </p>
      )}

      <ResponsiveGridLayout
        className="dashboard-grid"
        layouts={layouts}
        breakpoints={BREAKPOINTS}
        cols={COLS}
        rowHeight={ROW_HEIGHT}
        margin={MARGIN}
        draggableHandle=".panel-heading"
        onLayoutChange={handleLayoutChange}
        compactType="vertical"
      >
        {dashboard.panels.map((dp) => {
          const group = groupByPanelId.get(dp.panelId);
          return (
            <div key={dp.panelId}>
              <PanelHost panelId={dp.panelId} group={group} />
            </div>
          );
        })}
      </ResponsiveGridLayout>
    </div>
  );
}

interface PanelHostProps {
  panelId: string;
  group: PanelGroup | undefined;
}

/**
 * PanelHost subscribes the panel to its group's snapshot query and
 * slices out the metrics that this specific panel needs.
 *
 * Sharing: every panel in the same group passes the same `names` and
 * `intervalMs`, so they hash to the same queryKey. TanStack Query
 * dedupes — one HTTP call serves the whole group.
 *
 * Render isolation: `select` returns only this panel's slice. TanStack
 * Query compares slice outputs with replaceEqualDeep (structuralSharing
 * default) and only re-renders this subscriber when its slice
 * genuinely changed — even though siblings receive new data on the
 * same tick.
 */
function PanelHost({ panelId, group }: PanelHostProps) {
  const panel: PanelDefinition | undefined = getPanel(panelId);
  const panelMetricNames = panel?.metricNames ?? [];
  const intervalMs = group?.intervalMs ?? DEFAULT_REFRESH_INTERVAL_MS;
  const groupNames = group?.names ?? [];

  const { data: metrics, error } = useQuery({
    queryKey: ['snapshot', intervalMs, groupNames],
    queryFn: ({ signal }) => fetchSnapshot(groupNames, signal),
    refetchInterval: intervalMs,
    retry: false,
    enabled: panel !== undefined && groupNames.length > 0,
    select: (snapshot: Snapshot) => {
      const m: Record<string, Metric[] | undefined> = {};
      for (const name of panelMetricNames) {
        m[name] = snapshot.metrics[name];
      }
      return m;
    },
  });

  if (!panel) {
    return (
      <div className="py-20 text-center text-[15px] text-red-700">
        panel "{panelId}" not registered
      </div>
    );
  }

  if (error) {
    return (
      <div className="py-20 text-center text-[15px] text-red-700">
        {panel.title}: {error.message}
      </div>
    );
  }

  return <panel.Component metrics={metrics ?? {}} />;
}

/** Group dashboard panels by refresh interval. Every panel in a
 *  group ends up with the same `names` array (union of metric names
 *  needed by any panel at that interval), sorted for stable hashing
 *  so the queryKey is identity-stable across renders. */
function buildPanelGroups(
  dashboard: DashboardDefinition,
): Map<string, PanelGroup> {
  // intervalMs → set of metric names
  const namesByInterval = new Map<number, Set<string>>();
  for (const dp of dashboard.panels) {
    const panel = getPanel(dp.panelId);
    if (!panel) continue;
    const intervalMs = panel.refreshIntervalMs ?? DEFAULT_REFRESH_INTERVAL_MS;
    let set = namesByInterval.get(intervalMs);
    if (!set) {
      set = new Set();
      namesByInterval.set(intervalMs, set);
    }
    for (const name of panel.metricNames) set.add(name);
  }

  // Pre-flatten each interval's set into a sorted array.
  const flatByInterval = new Map<number, string[]>();
  for (const [intervalMs, set] of namesByInterval) {
    flatByInterval.set(intervalMs, Array.from(set).sort());
  }

  // Resolve each panel id to its group.
  const lookup = new Map<string, PanelGroup>();
  for (const dp of dashboard.panels) {
    const panel = getPanel(dp.panelId);
    if (!panel) continue;
    const intervalMs = panel.refreshIntervalMs ?? DEFAULT_REFRESH_INTERVAL_MS;
    const names = flatByInterval.get(intervalMs) ?? [];
    lookup.set(dp.panelId, { intervalMs, names });
  }
  return lookup;
}

/** Translate a DashboardDefinition's per-panel layouts into the shape
 *  react-grid-layout expects. Larger breakpoints cascade as defaults
 *  for smaller ones if not explicitly specified. */
function buildDefaultLayouts(dashboard: DashboardDefinition): DashboardLayouts {
  const out: Record<BreakpointName, LayoutItem[]> = {
    lg: [],
    md: [],
    sm: [],
    xs: [],
  };
  for (const dp of dashboard.panels) {
    let inherited: GridItem | undefined;
    for (const bp of BREAKPOINT_ORDER) {
      const item = dp.layouts[bp] ?? inherited;
      if (item) {
        out[bp].push(layoutFor(dp.panelId, item));
        inherited = item;
      }
    }
  }
  return out;
}

function layoutFor(panelId: string, item: GridItem): LayoutItem {
  return {
    i: panelId,
    x: item.x,
    y: item.y,
    w: item.w,
    h: item.h,
    minW: item.minW,
    minH: item.minH,
  };
}
