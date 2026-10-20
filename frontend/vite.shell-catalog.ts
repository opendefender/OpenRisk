// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { readFileSync } from 'node:fs';
import { posix } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { Plugin, Rollup } from 'vite';

import { SHELL_NAMESPACES } from './src/i18n/shellNamespaces';

/*
 * The French catalogue, split in two (#858).
 *
 * `virtual:fr-shell-catalog` is the namespaces the app shell reads before any
 * screen loads (SHELL_NAMESPACES). It is all of `locales/fr.json` that the entry
 * bundle carries. The full file loads with the first lazy screen, in parallel
 * with its code. A virtual module rather than named JSON imports: Vite
 * stringifies a JSON file this large, and a stringified JSON has no named
 * exports to tree-shake.
 *
 * At build, every source module the entry loads statically is read for keys.
 * One that names a namespace outside the shell fails the build: that key would
 * render blank on first paint, before the rest of the catalogue arrives. So
 * does a key built from a template (t(`${…}`)), which cannot be checked.
 */
const FR_CATALOG = fileURLToPath(new URL('./src/locales/fr.json', import.meta.url));
const SHELL_CATALOG_ID = 'virtual:fr-shell-catalog';
const shellNamespaces: readonly string[] = SHELL_NAMESPACES;

export function shellCatalog(): Plugin {
  const readFr = (): Record<string, unknown> => JSON.parse(readFileSync(FR_CATALOG, 'utf8'));
  return {
    name: 'openrisk:shell-catalog',
    resolveId(id) {
      return id === SHELL_CATALOG_ID ? `\0${SHELL_CATALOG_ID}` : undefined;
    },
    load(id) {
      if (id !== `\0${SHELL_CATALOG_ID}`) return undefined;
      this.addWatchFile(FR_CATALOG);
      const fr = readFr();
      const shell = Object.fromEntries(
        shellNamespaces.filter((ns) => ns in fr).map((ns) => [ns, fr[ns]]),
      );
      return `export default JSON.parse(${JSON.stringify(JSON.stringify(shell))});`;
    },
    generateBundle(_options, bundle) {
      const outside = Object.keys(readFr()).filter((ns) => !shellNamespaces.includes(ns));
      const chunks = new Map<string, Rollup.OutputChunk>();
      for (const out of Object.values(bundle))
        if (out.type === 'chunk') chunks.set(out.fileName, out);
      const queue = [...chunks.values()].filter((c) => c.isEntry).map((c) => c.fileName);
      const seen = new Set<string>();
      const problems: string[] = [];
      while (queue.length) {
        const chunk = chunks.get(queue.pop()!);
        if (!chunk || seen.has(chunk.fileName)) continue;
        seen.add(chunk.fileName);
        queue.push(...chunk.imports);
        for (const id of Object.keys(chunk.modules)) {
          if (!/\/src\/.*\.tsx?$/.test(id) || id.includes('node_modules')) continue;
          const source = readFileSync(id, 'utf8');
          for (const ns of outside) {
            if (new RegExp(`['"\`]${ns}\\.[A-Za-z_]`).test(source))
              problems.push(`${posix.relative(process.cwd(), id)} names "${ns}.…"`);
          }
          if (/\bt\(\s*`\$\{/.test(source))
            problems.push(`${posix.relative(process.cwd(), id)} builds a key from a template`);
        }
      }
      if (problems.length)
        this.error(
          `Entry-bundle modules read French keys the shell catalogue does not carry (#858):\n  ` +
            problems.join('\n  ') +
            `\nAdd the namespace to src/i18n/shellNamespaces.ts, or move the code behind a lazy screen.`,
        );
    },
  };
}
