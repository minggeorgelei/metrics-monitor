import { registerDashboard } from '../registry';

/**
 * The default landing dashboard. Step C ships it with only the CPU
 * panel (we don't have other inputs yet); Step D inserts mem / disk /
 * network / process panels alongside by adding new entries here.
 *
 * Layouts use a 24-column grid at the lg breakpoint (mirrors Grafana's
 * node_exporter dashboards). Heights are in row units; row height is
 * 30px, so h=10 ≈ 300px tall.
 */
registerDashboard({
  id: 'overview',
  title: 'Overview',
  isDefault: true,
  description: 'Live host snapshot across all collected metrics.',
  panels: [
    {
      panelId: 'cpu-usage',
      layouts: {
        // 24-col desktop: full width, 300px tall.
        lg: { x: 0, y: 0, w: 24, h: 10, minW: 8, minH: 6 },
        // 18-col tablet landscape.
        md: { x: 0, y: 0, w: 18, h: 10, minW: 6, minH: 6 },
        // 12-col tablet portrait.
        sm: { x: 0, y: 0, w: 12, h: 9, minW: 6, minH: 6 },
        // 4-col phone.
        xs: { x: 0, y: 0, w: 4, h: 8, minW: 4, minH: 6 },
      },
    },
  ],
});
