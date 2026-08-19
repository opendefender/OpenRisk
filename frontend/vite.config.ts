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
  build: {
    // Perf budget (task §2): keep the INITIAL bundle small. Feature pages are
    // already route-split with React.lazy; here we additionally split the heavy
    // vendors into their own long-cached chunks so the entry chunk stays lean and
    // a library upgrade never busts the whole app's cache. Charting (recharts) is
    // only pulled by the lazy chart routes, so it never lands in the entry.
    chunkSizeWarningLimit: 300,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined
          // Heavy, rarely-on-first-paint libraries: their own chunks so they load
          // only with the routes that use them and never bloat the entry.
          if (id.includes('recharts') || id.includes('d3-') || id.includes('victory') || id.includes('internmap'))
            return 'charts'
          if (id.includes('@zxcvbn-ts')) return 'zxcvbn' // password-strength dictionaries (huge)
          if (id.includes('leaflet')) return 'maps'
          if (id.includes('@hello-pangea') || id.includes('react-grid-layout') || id.includes('react-resizable'))
            return 'dnd'
          if (id.includes('lodash')) return 'lodash'
          if (id.includes('date-fns')) return 'datefns'
          if (id.includes('react-hook-form') || id.includes('@hookform') || id.includes('/zod/'))
            return 'forms'
          if (id.includes('react-confetti') || id.includes('use-sound') || id.includes('howler'))
            return 'celebrate'
          if (id.includes('@floating-ui')) return 'floating'
          if (id.includes('react-use')) return 'reactuse'
          if (id.includes('zxcvbn')) return 'zxcvbn'
          if (id.includes('framer-motion') || id.includes('popmotion') || id.includes('@motionone'))
            return 'motion'
          if (id.includes('@tanstack')) return 'query'
          if (id.includes('react-router')) return 'router'
          if (id.includes('lucide-react')) return 'icons'
          if (id.includes('/react/') || id.includes('/react-dom/') || id.includes('/scheduler/'))
            return 'react'
          return 'vendor'
        },
      },
    },
  },
})
