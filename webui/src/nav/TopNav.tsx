import { NavLink } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';

import { fetchHealth } from '../lib/api';

/**
 * TopNav is the persistent header shown above every dashboard. It
 * does two things:
 *
 *   1. Surfaces dashboard tabs (NavLink handles active styling).
 *   2. Shows a connection status badge by polling the lightweight
 *      health endpoint, so dashboard panels can fetch their metric
 *      groups independently.
 *
 * In Step A the tab list is hardcoded; Step C replaces it with a
 * generated list from the dashboards registry.
 */

interface Tab {
  path: string;
  label: string;
  end?: boolean; // require exact match (only "/" needs it)
}

const TABS: Tab[] = [
  { path: '/', label: 'Overview', end: true },
  { path: '/cpu', label: 'CPU' },
  { path: '/memory', label: 'Memory' },
  { path: '/disk', label: 'Disk' },
  { path: '/network', label: 'Network' },
  { path: '/processes', label: 'Processes' },
];

export function TopNav() {
  const { data, error, isPending } = useQuery({
    queryKey: ['health'],
    queryFn: ({ signal }) => fetchHealth(signal),
    refetchInterval: 1000,
    retry: false,
  });

  return (
    <header className="sticky top-0 z-10 flex items-center gap-6 border-b border-line bg-surface px-6 py-3 max-[760px]:gap-4 max-[760px]:px-4 max-[760px]:py-2.5">
      <div className="whitespace-nowrap text-base font-bold tracking-tight text-fg-strong">
        metrics-monitor
      </div>
      <nav className="flex min-w-0 flex-1 gap-1 overflow-x-auto [scrollbar-width:thin]">
        {TABS.map((t) => (
          <NavLink
            key={t.path}
            to={t.path}
            end={t.end}
            className={({ isActive }) =>
              'whitespace-nowrap rounded-md px-3 py-1.5 text-sm font-medium no-underline transition-colors duration-100 hover:bg-bg hover:text-fg-strong' +
              (isActive ? ' bg-bg font-semibold text-fg-strong' : ' text-muted')
            }
          >
            {t.label}
          </NavLink>
        ))}
      </nav>
      <ConnectionBadge
        timestamp={data?.timestamp}
        error={error}
        isPending={isPending}
      />
    </header>
  );
}

const BADGE_BASE =
  'inline-flex items-center whitespace-nowrap rounded-full px-2.5 py-1 font-mono text-[13px]';

function ConnectionBadge({
  timestamp,
  error,
  isPending,
}: {
  timestamp: string | undefined;
  error: Error | null;
  isPending: boolean;
}) {
  if (isPending)
    return (
      <span className={`${BADGE_BASE} bg-blue-600/10 text-blue-700`}>
        connecting…
      </span>
    );
  if (error)
    return (
      <span className={`${BADGE_BASE} bg-red-600/10 text-red-700`}>error</span>
    );
  if (timestamp) {
    return (
      <span
        className={`${BADGE_BASE} bg-green-600/10 text-green-700`}
        title={timestamp}
      >
        live · {new Date(timestamp).toLocaleTimeString()}
      </span>
    );
  }
  return null;
}
