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
 *
 * The `panel-heading` class on <header> is the react-grid-layout
 * draggable-handle selector — see DashboardView's draggableHandle prop.
 */
export function PanelShell({ title, subtitle, actions, children }: Props) {
  return (
    <section className="flex h-full flex-col overflow-hidden rounded-lg border border-line bg-surface shadow-[0_14px_40px_rgba(15,23,42,0.08)]">
      <header className="panel-heading flex cursor-move select-none items-start justify-between gap-[22px] border-b border-line px-5 py-[18px] max-[760px]:flex-col max-[760px]:items-stretch">
        <div>
          <h2 className="text-xl font-bold leading-tight text-fg-strong">
            {title}
          </h2>
          {subtitle && <p className="mt-1 text-sm text-muted">{subtitle}</p>}
        </div>
        {actions && <div>{actions}</div>}
      </header>
      {children}
    </section>
  );
}
