import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Backend origin for the dev proxy. Override with BACKEND_URL when the API runs
// somewhere other than the default docker compose mapping.
const backendURL = process.env.BACKEND_URL ?? 'http://localhost:8080';

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
          if (!id.includes('node_modules')) return undefined;
          // Heavy, rarely-on-first-paint libraries: their own chunks so they load
          // only with the routes that use them and never bloat the entry.
          // @visx/* must be listed explicitly: it matches none of the other
          // rules, so without this it falls through to `vendor`, which IS
          // preloaded — the exact way a chart library stops being lazy and
          // silently spends the initial-bundle budget (D-024).
          // `recharts` was removed from this list with the dependency itself,
          // once D-024 option A replaced it with the visx layer in shared/ds/charts.
          if (
            id.includes('@visx') ||
            id.includes('d3-') ||
            id.includes('victory') ||
            id.includes('internmap') ||
            // visx's own non-d3 runtime deps. None is imported anywhere in
            // src/ — they arrive only through @visx/* — and none matches a rule
            // above, so without naming them here they land in `vendor`, which
            // IS preloaded. Measured: they cost 5.2 KB of the initial bundle.
            id.includes('classnames') ||
            id.includes('reduce-css-calc') ||
            id.includes('postcss-value-parser') ||
            id.includes('react-use-measure')
          )
            return 'charts'
          // anime.js gets its OWN chunk (#445, D-028) rather than riding along in
          // `charts`. That is what makes the 12 KB gzip ceiling in
          // scripts/check-anime-budget.mjs measurable at all: mixed into the
          // charts chunk alongside visx there would be no number to assert.
          if (id.includes('animejs')) return 'anime'
          if (id.includes('@zxcvbn-ts')) return 'zxcvbn' // password-strength dictionaries (huge)
          if (id.includes('leaflet')) return 'maps'
          if (id.includes('@hello-pangea') || id.includes('react-grid-layout') || id.includes('react-resizable'))
            return 'dnd'
          if (id.includes('lodash')) return 'lodash'
          if (id.includes('date-fns')) return 'datefns'
          if (id.includes('react-hook-form') || id.includes('@hookform') || id.includes('/zod/'))
            return 'forms';
          if (id.includes('react-confetti') || id.includes('use-sound') || id.includes('howler'))
            return 'celebrate';
          if (id.includes('@floating-ui')) return 'floating';
          if (id.includes('react-use')) return 'reactuse';
          if (id.includes('zxcvbn')) return 'zxcvbn';
          if (id.includes('framer-motion') || id.includes('popmotion') || id.includes('@motionone'))
            return 'motion';
          if (id.includes('@tanstack')) return 'query';
          if (id.includes('react-router')) return 'router';
          if (id.includes('lucide-react')) return 'icons';
          if (id.includes('/react/') || id.includes('/react-dom/') || id.includes('/scheduler/'))
            return 'react';

          /*
           * Transitive engines of the LAZY libraries above.
           *
           * `vendor` is preloaded, because it also holds axios, zustand and
           * tailwind-merge, which the entry graph genuinely needs. Anything that
           * falls through to it is therefore fetched before first paint — and
           * these were falling through:
           *
           *   recharts v3          -> @reduxjs/toolkit, react-redux, redux,
           *                           reselect, immer, decimal.js-light
           *   @hello-pangea/dnd    -> react-redux, redux
           *   framer-motion v11    -> motion-dom, motion-utils
           *
           * Rules above catch `recharts` and `@hello-pangea` by name, but a file
           * inside `node_modules/react-redux` matches none of them, so the Redux
           * engine of a chart library the first screen never renders was being
           * downloaded on every cold load. ~240 KB of raw source.
           *
           * Verified before moving: NONE of these is imported by app code —
           * `grep -rl "from '<pkg>'" src` returns nothing for every one, and the
           * only zustand middleware in use is `zustand/middleware`, not the immer
           * one. So this chunk is reachable only through a lazy route, which is
           * what keeps it out of the preload.
           */
          if (
            id.includes('node_modules/@reduxjs/toolkit') ||
            id.includes('node_modules/react-redux') ||
            id.includes('node_modules/redux/') ||
            id.includes('node_modules/redux-thunk') ||
            id.includes('node_modules/reselect') ||
            id.includes('node_modules/immer') ||
            id.includes('node_modules/decimal.js-light') ||
            id.includes('node_modules/es-toolkit') ||
            id.includes('node_modules/tabbable') ||
            id.includes('node_modules/eventemitter3')
          )
            return 'lazy-engines';

          /* framer-motion v11 moved its engine into these two packages; the rule
             above matches the old names only, so they were landing in `vendor`
             rather than beside the animation code that uses them. */
          if (id.includes('node_modules/motion-dom') || id.includes('node_modules/motion-utils'))
            return 'motion';

          /* The toast surface. Nothing can be toasted until the user has acted,
             so it is never needed for first paint — but left to fall through to
             `vendor` it was preloaded anyway, because vendor also holds axios and
             zustand. Same trap as the engines above: no longer *imported*
             eagerly, yet still glued to a preloaded chunk. */
          if (id.includes('node_modules/sonner')) return 'toast';

          return 'vendor';
        },
      },
    },
  },
});
