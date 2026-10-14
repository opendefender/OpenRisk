import { posix } from 'node:path';
import { defineConfig, type Rollup } from 'vite';
import react from '@vitejs/plugin-react';

// Backend origin for the dev proxy. Override with BACKEND_URL when the API runs
// somewhere other than the default docker compose mapping.
const backendURL = process.env.BACKEND_URL ?? 'http://localhost:8080';

/*
 * The lucide icons the entry chunk draws (#966), as module ids.
 *
 * Every icon used to go into one `icons` chunk, which the entry imported, so
 * all of them were preloaded for a first screen that draws a few dozen. Left
 * entirely to Rollup instead, each icon used only behind lazy routes became a
 * chunk of its own, some eighty files of 200 bytes. So: the icons the entry
 * graph go to `icons` with lucide's runtime, and every other icon goes to one
 * `icons-lazy` chunk that only lazy chunks import.
 *
 * Both are named explicitly. Rollup pulls a manual chunk's unassigned
 * dependencies into it, so leaving the entry's icons and lucide's runtime
 * unassigned put them in `icons-lazy`, and the entry then imported that.
 *
 * "Names" is read, not guessed: the names come from the `import { … } from
 * 'lucide-react'` statements of the modules statically reachable from the
 * entry, and each name is mapped to its file through lucide's own barrel,
 * which is a list of `export { default as Name } from './icons/name.js'`.
 *
 * A name this misses (an `import * as` or a re-export from 'lucide-react',
 * none of which exist today) fails the build, not the app: its icon lands in
 * `icons-lazy`, the entry then imports that chunk statically, index.html
 * preloads it, and `npm run budget` goes over. Module ids use forward slashes on
 * every platform, hence `posix`. The cache is cleared at each build start.
 */
let entryIconIds: Set<string> | undefined;
function iconsOfEntry(meta: Rollup.ManualChunkMeta): Set<string> {
  if (entryIconIds) return entryIconIds;
  const names = new Set<string>();
  let barrel: string | undefined;
  const seen = new Set<string>();
  const queue = [...meta.getModuleIds()].filter((id) => meta.getModuleInfo(id)?.isEntry);
  while (queue.length) {
    const id = queue.pop()!;
    if (seen.has(id)) continue;
    seen.add(id);
    const info = meta.getModuleInfo(id);
    if (!info) continue;
    for (const dep of info.importedIds) {
      if (dep.includes('lucide-react/dist/esm/lucide-react')) barrel = dep;
      if (!dep.includes('node_modules')) queue.push(dep);
    }
    for (const m of (info.code ?? '').matchAll(
      /import\s*\{([^}]*)\}\s*from\s*["']lucide-react["']/g,
    )) {
      for (const spec of m[1].split(',')) {
        const name = spec.trim().split(/\s+as\s+/)[0];
        if (name) names.add(name);
      }
    }
  }
  entryIconIds = new Set();
  const barrelCode = barrel ? (meta.getModuleInfo(barrel)?.code ?? '') : '';
  for (const m of barrelCode.matchAll(/export\s*\{([^}]*)\}\s*from\s*'(\.\/icons\/[^']+)'/g)) {
    const exported = m[1].split(',').map((spec) => spec.trim().split(/\s+as\s+/)[1]);
    if (exported.some((name) => names.has(name)))
      entryIconIds.add(posix.join(posix.dirname(barrel!), m[2]));
  }
  return entryIconIds;
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    {
      name: 'openrisk:reset-entry-icons',
      buildStart() {
        entryIconIds = undefined;
      },
    },
  ],
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
    // NOT Vite's default `assets`: that puts a real `dist/assets/` directory
    // where the app has its `/assets` inventory route. nginx's
    // `try_files $uri $uri/` then matches the directory, redirects `/assets` to
    // `/assets/` and answers 403 (no index, autoindex off), so a reload or a
    // shared link to the inventory broke (#758). No route starts with `static`.
    assetsDir: 'static',
    // Perf budget (task §2): keep the INITIAL bundle small. Feature pages are
    // already route-split with React.lazy; here we additionally split the heavy
    // vendors into their own long-cached chunks so the entry chunk stays lean and
    // a library upgrade never busts the whole app's cache. Charting (recharts) is
    // only pulled by the lazy chart routes, so it never lands in the entry.
    chunkSizeWarningLimit: 300,
    rollupOptions: {
      output: {
        manualChunks(id, meta) {
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
            return 'charts';
          // anime.js gets its OWN chunk (#445, D-028) rather than riding along in
          // `charts`. That is what makes the gzip ceiling in
          // scripts/check-anime-budget.mjs measurable at all: mixed into the
          // charts chunk alongside visx there would be no number to assert.
          if (id.includes('animejs')) return 'anime';
          // password-strength dictionaries (huge), and the edit-distance helper
          // @zxcvbn-ts/core depends on, which otherwise fell through to `vendor`.
          if (id.includes('@zxcvbn-ts') || id.includes('fastest-levenshtein')) return 'zxcvbn';
          if (id.includes('leaflet')) return 'maps';
          if (
            id.includes('@hello-pangea') ||
            id.includes('react-grid-layout') ||
            id.includes('react-resizable')
          )
            return 'dnd';
          if (id.includes('lodash')) return 'lodash';
          if (id.includes('date-fns')) return 'datefns';
          if (id.includes('react-hook-form') || id.includes('@hookform') || id.includes('/zod/'))
            return 'forms';
          if (id.includes('react-confetti') || id.includes('use-sound') || id.includes('howler'))
            return 'celebrate';
          if (id.includes('@floating-ui')) return 'floating';
          if (id.includes('react-use')) return 'reactuse';
          if (id.includes('zxcvbn')) return 'zxcvbn';
          if (id.includes('framer-motion') || id.includes('popmotion') || id.includes('@motionone'))
            return 'motion';
          /* The virtualizer serves the long tables (shared/datatable), all of
             them behind lazy routes. Under the `@tanstack` rule below it rode in
             `query`, which is preloaded for the QueryClient in main.tsx. */
          if (id.includes('@tanstack/react-virtual') || id.includes('@tanstack/virtual-core'))
            return 'virtual';
          if (id.includes('@tanstack')) return 'query';
          if (id.includes('react-router')) return 'router';
          /* The entry's icons and lucide's runtime: `icons`, preloaded. Every
             other icon: `icons-lazy`, fetched with the first route that draws
             one. See iconsOfEntry. */
          if (id.includes('lucide-react/dist/esm/icons/'))
            return iconsOfEntry(meta).has(id) ? 'icons' : 'icons-lazy';
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
