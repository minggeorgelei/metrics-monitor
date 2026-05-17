import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import App from './App';
import './index.css';

// --- Panel registrations ---------------------------------------------
// Each panel package self-registers via a top-level call to
// registerPanel(). Adding a new panel = `import './panels/<id>'` here
// + new directory under panels/. main.tsx is the one centralized place
// where panel modules are wired into the bundle.
import './panels/cpu';

// --- Dashboard registrations -----------------------------------------
// Same pattern, one level up: dashboards reference panels by id. Order
// matters here only to the extent that panels must be registered
// before dashboards reference them, which we satisfy by importing
// panel files first above.
import './dashboards/overview';
// ----------------------------------------------------------------------

// Single shared QueryClient. TanStack Query holds polling state,
// caching, and the AbortController for in-flight requests here.
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // We refetch on a tight interval and care about freshness over
      // dedup — disable the "is data still fresh?" optimization.
      staleTime: 0,
      // Don't auto-refetch when the tab regains focus; the polling
      // interval already covers that case.
      refetchOnWindowFocus: false,
    },
  },
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
);
