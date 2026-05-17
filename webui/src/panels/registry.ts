import type { PanelDefinition } from './types';

/**
 * Panel registry. Mirrors the backend's plugins/inputs|outputs/registry.go
 * pattern: each panel package self-registers via a top-level
 * `registerPanel(...)` call, triggered by a blank import in main.tsx.
 *
 *   panels/cpu/index.tsx     → registerPanel({ id: 'cpu-usage', ...})
 *   panels/mem/index.tsx     → registerPanel({ id: 'mem-usage', ...})
 *
 * Dashboards (Step C) then look panels up by id and place them in a
 * grid. Same panel can appear in multiple dashboards with different
 * layouts — that's why we registerPanel(definition), not <Panel /> JSX.
 */
const registry = new Map<string, PanelDefinition>();

export function registerPanel(definition: PanelDefinition): void {
  if (registry.has(definition.id)) {
    // During Vite Fast Refresh the panel module re-executes on each
    // edit, calling registerPanel() again. Treat that as an update
    // rather than an error so HMR keeps working. In production
    // (no import.meta.hot), duplicate IDs are almost certainly a
    // typo or double-import — fail loudly there.
    if (import.meta.hot) {
      registry.set(definition.id, definition);
      return;
    }
    throw new Error(`panel "${definition.id}" is already registered`);
  }
  registry.set(definition.id, definition);
}

export function getPanel(id: string): PanelDefinition | undefined {
  return registry.get(id);
}

export function allPanels(): PanelDefinition[] {
  return Array.from(registry.values());
}
