import type { LayoutItem, ResponsiveLayouts } from 'react-grid-layout';

export type DashboardLayouts = ResponsiveLayouts<string>;

/**
 * localStorage-backed persistence of the user's drag/resize state,
 * keyed per dashboard so each page remembers its own arrangement.
 *
 * On reload:
 *   1. Load saved layouts from localStorage
 *   2. Merge with the dashboard's default layouts — adds new panels
 *      that didn't exist when the user last saved, removes saved
 *      entries for panels that no longer exist
 *
 * No network sync, no cross-device sync. localStorage is per-origin
 * per-browser, which matches our "single machine, local user" scope.
 */

const STORAGE_KEY_PREFIX = 'mm-layouts:';

function key(dashboardId: string): string {
  return STORAGE_KEY_PREFIX + dashboardId;
}

export function loadLayouts(dashboardId: string): DashboardLayouts | null {
  try {
    const raw = localStorage.getItem(key(dashboardId));
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (typeof parsed !== 'object' || parsed === null) return null;
    return parsed as DashboardLayouts;
  } catch {
    // Parse error or storage disabled — fall back to defaults.
    return null;
  }
}

export function saveLayouts(
  dashboardId: string,
  layouts: DashboardLayouts,
): void {
  try {
    localStorage.setItem(key(dashboardId), JSON.stringify(layouts));
  } catch {
    // localStorage may be full, in private mode, or disabled by
    // policy. Silent failure is fine — user just loses persistence,
    // not the running session.
  }
}

export function clearLayouts(dashboardId: string): void {
  try {
    localStorage.removeItem(key(dashboardId));
  } catch {
    // ignore
  }
}

/**
 * Reconcile saved layouts with the current default. Three rules:
 *
 *   1. Panel exists in defaults AND saved → use saved (user's choice)
 *   2. Panel exists in defaults but not saved → add it with default
 *      position (newly registered panel)
 *   3. Panel exists in saved but not defaults → drop it (panel was
 *      deleted or renamed since the user's last save)
 */
export function mergeLayouts(
  saved: DashboardLayouts,
  defaults: DashboardLayouts,
): DashboardLayouts {
  const result: DashboardLayouts = {};
  for (const breakpoint of Object.keys(defaults)) {
    const defaultLayout = defaults[breakpoint] ?? [];
    const savedLayout = saved[breakpoint] ?? [];
    const validIds = new Set(defaultLayout.map((l) => l.i));
    const savedById = new Map<string, LayoutItem>(
      savedLayout.filter((l) => validIds.has(l.i)).map((l) => [l.i, l]),
    );
    result[breakpoint] = defaultLayout.map((d) => savedById.get(d.i) ?? d);
  }
  return result;
}
