import type { ReactNode } from 'react';

interface Props {
  title: string;
  subtitle?: string;
  /** Right-side header content. Typically a stats row (CPUPanel) or
   *  action buttons (future: refresh / fullscreen / settings). */
  actions?: ReactNode;
  children: ReactNode;
}

/**
 * PanelShell is the reusable visual frame every panel renders inside.
 * It owns the chrome (border, header, layout); the panel only fills
 * the content slot.
 *
 * Why not let each panel handle its own chrome:
 *  - Visual consistency across dashboards (Grafana-style uniformity)
 *  - One place to add features later: drag-handle (Step C), fullscreen
 *    button, panel actions menu, error boundary
 *  - Panels stay focused on data → rendering, no boilerplate
 */
export function PanelShell({ title, subtitle, actions, children }: Props) {
  return (
    <section className="panel">
      <header className="panel__heading">
        <div>
          <h2>{title}</h2>
          {subtitle && <p>{subtitle}</p>}
        </div>
        {actions && <div className="panel__actions">{actions}</div>}
      </header>
      {children}
    </section>
  );
}
