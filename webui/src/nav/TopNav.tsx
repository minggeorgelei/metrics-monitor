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
    <header className="topnav">
      <div className="topnav__brand">metrics-monitor</div>
      <nav className="topnav__tabs">
        {TABS.map((t) => (
          <NavLink
            key={t.path}
            to={t.path}
            end={t.end}
            className={({ isActive }) =>
              'topnav__tab' + (isActive ? ' topnav__tab--active' : '')
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
    return <span className="badge badge--pending">connecting…</span>;
  if (error) return <span className="badge badge--error">error</span>;
  if (timestamp) {
    return (
      <span className="badge badge--ok" title={timestamp}>
        live · {new Date(timestamp).toLocaleTimeString()}
      </span>
    );
  }
  return null;
}
