import type { FC } from 'react';

import type { Metric } from '../lib/api';

/**
 * PanelProps is what every panel component receives. DashboardView
 * filters this map to only the metric groups declared by the panel's
 * metricNames, so panels don't receive unrelated dashboard data.
 *
 * Note: there is no snapshot-level `timestamp` field. Each fetch
 * produces a new timestamp string, which would defeat the
 * structuralSharing in TanStack Query's select layer and cause every
 * panel to re-render on every poll. If a panel needs to display
 * "last sample time", derive it from the metric's own `time` field.
 */
export interface PanelProps {
  metrics: Record<string, Metric[] | undefined>;
}

/**
 * PanelDefinition is the unit of registration. A panel lives in its
 * own directory under panels/<id>/ and self-registers from an
 * `index.tsx` that calls registerPanel() at module top level.
 *
 * Dashboards then reference panels by `id` and provide a layout for
 * each. Same panel can appear in multiple dashboards.
 */
export interface PanelDefinition {
  /** Unique identifier across the whole webui, e.g. "cpu-usage". */
  id: string;

  /** Display title rendered in the panel header. */
  title: string;

  /** Optional one-line description shown under the title. */
  subtitle?: string;

  /** Future use (Phase 2+): top-nav dropdown grouping
   *  (e.g. "System" / "Apps" / "Hardware"). Undefined = top-level. */
  group?: string;

  /** Which top-level snapshot.metrics keys this panel reads. Lets the
   *  dashboard show "waiting for X" placeholders when those keys
   *  haven't arrived yet, and lets future tooling reason about data
   *  dependencies. */
  metricNames: string[];

  /** How often this panel wants its metric dependencies refreshed.
   *  DashboardView de-duplicates by metric name and uses the fastest
   *  requested interval when multiple panels read the same metric. */
  refreshIntervalMs?: number;

  /** The renderer. */
  Component: FC<PanelProps>;
}
