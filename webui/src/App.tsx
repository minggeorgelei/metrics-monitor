import { BrowserRouter, Route, Routes } from 'react-router-dom';

import { TopNav } from './nav/TopNav';
import { OverviewPage } from './pages/OverviewPage';
import { PlaceholderPage } from './pages/PlaceholderPage';
import './App.css';

/**
 * App is now a thin router shell. Every dashboard is a route below;
 * Overview is the default. Step C will swap the hardcoded route table
 * for one generated from a dashboards registry.
 */
export default function App() {
  return (
    <BrowserRouter>
      <div className="app">
        <TopNav />
        <main className="page">
          <Routes>
            <Route path="/" element={<OverviewPage />} />
            <Route
              path="/cpu"
              element={<PlaceholderPage title="CPU detail" />}
            />
            <Route
              path="/memory"
              element={<PlaceholderPage title="Memory" />}
            />
            <Route path="/disk" element={<PlaceholderPage title="Disk" />} />
            <Route
              path="/network"
              element={<PlaceholderPage title="Network" />}
            />
            <Route
              path="/processes"
              element={<PlaceholderPage title="Processes" />}
            />
            <Route
              path="*"
              element={
                <PlaceholderPage
                  title="Not found"
                  description="No such dashboard."
                />
              }
            />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  );
}
