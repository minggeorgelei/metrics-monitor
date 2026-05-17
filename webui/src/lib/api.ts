// API client + type definitions. Shapes mirror what the backend
// http_snapshot output emits — keep these in sync if you change the
// Go struct in source/plugins/outputs/http_snapshot/http_snapshot.go.

/** ValueType integer matches source/core/metric.go ValueType. */
export type MetricType = 0 | 1 | 2 | 3 | 4;

/** One sample of one series. */
export interface Metric {
  name: string;
  tags: Record<string, string>;
  fields: Record<string, number>;
  time: string; // RFC3339 UTC
  type: MetricType;
}

/** Full snapshot returned by GET /api/v1/metrics. Metrics grouped by
 * name — each entry has one series per (name, tags) combination
 * currently in the agent's cache. */
export interface Snapshot {
  timestamp: string;
  metrics: Record<string, Metric[]>;
}

export interface Health {
  status: string;
  timestamp: string;
}

/**
 * Fetch a snapshot. When `names` is provided and non-empty, the
 * backend filters its response to those metric groups — this is how
 * panels at the same refresh interval share a single HTTP call.
 *
 *   fetchSnapshot()                  → every metric in the cache
 *   fetchSnapshot(['cpu', 'mem'])    → only those two groups
 */
export async function fetchSnapshot(
  names?: string[],
  signal?: AbortSignal,
): Promise<Snapshot> {
  let url = '/api/v1/metrics';
  if (names && names.length > 0) {
    url += `?names=${encodeURIComponent(names.join(','))}`;
  }
  const res = await fetch(url, { signal });
  if (!res.ok) {
    throw new Error(`GET ${url} failed: ${res.status} ${res.statusText}`);
  }
  return res.json() as Promise<Snapshot>;
}

/** Lightweight liveness check for chrome/status UI. */
export async function fetchHealth(signal?: AbortSignal): Promise<Health> {
  const res = await fetch('/healthz', { signal });
  if (!res.ok) {
    throw new Error(`GET /healthz failed: ${res.status} ${res.statusText}`);
  }
  const body = (await res.json()) as { status?: string };
  return {
    status: body.status ?? 'unknown',
    timestamp: new Date().toISOString(),
  };
}
