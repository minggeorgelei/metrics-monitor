/**
 * Dashboard registry types.
 *
 * A dashboard is "which panels go where". It does NOT own the panels
 * themselves — those live in panels/registry. The dashboard merely
 * references panel IDs and specifies a layout per breakpoint.
 *
 * This split lets the same panel appear in multiple dashboards with
 * different sizes (e.g. CPU chart is full-width on Overview but
 * shares half-width on a CPU detail dashboard).
 */

/** Grid coordinate for a panel within one breakpoint. Matches
 *  react-grid-layout's LayoutItem type (minus `i`, which is the panel id
 *  and gets filled in automatically by DashboardView). */
export interface GridItem {
  x: number;
  y: number;
  w: number;
  h: number;
  /** Minimum width to prevent users dragging panels into uselessness. */
  minW?: number;
  /** Minimum height. */
  minH?: number;
}

/** Layout for one panel across responsive breakpoints. Omit a
 *  breakpoint to inherit from the next-larger one. */
export interface PanelLayouts {
  lg?: GridItem;
  md?: GridItem;
  sm?: GridItem;
  xs?: GridItem;
}

export interface DashboardPanel {
  /** Reference into the panels registry. */
  panelId: string;
  layouts: PanelLayouts;
}

export interface DashboardDefinition {
  /** URL slug, e.g. "overview" / "cpu" / "memory". */
  id: string;

  /** Display name (used in nav and titles). */
  title: string;

  /** Optional one-line description (shown above the grid). */
  description?: string;

  /** Marks the dashboard rendered at the root URL "/". Exactly one
   *  registered dashboard should set this; the registry has a helper
   *  to enforce that at lookup time. */
  isDefault?: boolean;

  /** Future use (Phase 2+): nav dropdown grouping. */
  group?: string;

  panels: DashboardPanel[];
}
