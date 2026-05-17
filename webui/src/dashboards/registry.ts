import type { DashboardDefinition } from './types';

/**
 * Dashboard registry. Same self-registration pattern as panels/registry:
 *
 *   dashboards/overview/index.ts → registerDashboard({...})
 *   dashboards/cpu/index.ts      → registerDashboard({...})
 *
 * main.tsx blank-imports each file to trigger registration before
 * <App /> renders.
 */
const registry = new Map<string, DashboardDefinition>();

export function registerDashboard(definition: DashboardDefinition): void {
  if (registry.has(definition.id)) {
    // Same HMR-tolerant pattern as panels/registry: overwrite in dev
    // (so file save doesn't crash), throw in prod (catch typos).
    if (import.meta.hot) {
      registry.set(definition.id, definition);
      return;
    }
    throw new Error(`dashboard "${definition.id}" is already registered`);
  }
  registry.set(definition.id, definition);
}

export function getDashboard(id: string): DashboardDefinition | undefined {
  return registry.get(id);
}

export function allDashboards(): DashboardDefinition[] {
  return Array.from(registry.values());
}

/** Returns the dashboard marked `isDefault`. Used by the "/" route
 *  resolver. Returns undefined if no dashboard is default. */
export function defaultDashboard(): DashboardDefinition | undefined {
  for (const d of registry.values()) {
    if (d.isDefault) return d;
  }
  return undefined;
}
