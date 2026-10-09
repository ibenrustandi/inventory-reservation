import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// Where the dev server forwards API calls. The browser only ever talks to
// the dev server, so no CORS setup is needed on the backend.
const apiTarget = process.env.API_PROXY_TARGET ?? 'http://localhost:8080'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': { target: apiTarget, changeOrigin: true },
    },
  },
  test: {
    environment: 'node',
  },
})
