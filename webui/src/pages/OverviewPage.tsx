import { DashboardView } from '../dashboard/DashboardView';

/**
 * Step C: OverviewPage is now a thin route handler that hands the
 * heavy lifting to DashboardView. The dashboard's content
 * (which panels, where) lives in dashboards/overview/index.ts; this
 * file only knows which dashboard id to render.
 *
 * Other dashboard routes (cpu/memory/disk/...) will follow the same
 * one-line pattern once their dashboard definitions land in Step D.
 */
export function OverviewPage() {
  return <DashboardView dashboardId="overview" />;
}
