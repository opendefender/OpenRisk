import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Backend origin for the dev proxy. Override with BACKEND_URL when the API runs
// somewhere other than the default docker compose mapping.
const backendURL = process.env.BACKEND_URL ?? 'http://localhost:8080'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Proxy the API so the browser sees a single origin.
    //
    // This is what makes the HttpOnly session cookies work in development.
    // Served directly, the SPA (5173) and the API (8080) are different origins,
    // so the session cookie would be cross-site: SameSite=Lax withholds it, and
    // relaxing that to SameSite=None requires Secure, hence HTTPS. Proxying
    // keeps the cookies first-party and makes the dev setup faithful to
    // production, where both sit behind one host.
    proxy: {
      '/api': {
        target: backendURL,
        changeOrigin: true,
      },
    },
  },
})
