import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Proxy /api → http://localhost:9100 so the dev server fetches stay
// same-origin and we don't need CORS in development. The agent's
// http_snapshot output already sets Access-Control-Allow-Origin: *,
// but routing through Vite avoids the preflight altogether and
// matches what a production reverse-proxy setup would look like.
//
// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api': 'http://localhost:9100',
      '/healthz': 'http://localhost:9100',
    },
  },
})
