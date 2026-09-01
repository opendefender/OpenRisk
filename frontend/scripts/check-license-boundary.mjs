#!/usr/bin/env node
// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Asserts the three-licence boundary the repository actually has.
 *
 * #446's original "reuse lint" task assumed two licences. Since D-014 and D-016
 * (executed by #452) there are three, and a check configured on "everything
 * under frontend/src/** is AGPL" would be red where it should be green — or,
 * worse, green on the wrong answer:
 *
 *   AGPL-3.0-only                  the core
 *   Apache-2.0                     frontend/design-system/ AND frontend/src/shared/ds/
 *   LicenseRef-OpenRisk-Commercial the Enterprise Edition
 *
 * Written here rather than delegated to `reuse`: that tool is a new external
 * dependency, which CLAUDE.md makes an owner decision, and #443 is blocked on
 * exactly that question for two other packages. Adding one unasked while
 * enforcing a doctrine about restraint would be the wrong shape. The boundary
 * is four rules and a header grep; it does not need a toolchain.
 *
 * Why this matters beyond tidiness: #443 vendors ~50 third-party components into
 * frontend/src/shared/ds/. Apache-2.0 there is irreversible once published, so
 * the guarantee worth automating is that a file cannot land in that directory
 * carrying the core's AGPL header.
 *
 * Usage: npm run check:license-boundary
 */

import { readFileSync, readdirSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve, join, relative } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const FRONTEND = resolve(here, '..');
const REPO = resolve(FRONTEND, '..');

const APACHE = 'Apache-2.0';
const AGPL = 'AGPL-3.0-only';
const EE = 'LicenseRef-OpenRisk-Commercial';

/** Directories whose every file is Apache-2.0. Paths are repo-relative. */
const APACHE_DIRS = ['frontend/design-system', 'frontend/src/shared/ds'];

/**
 * Files with no header, legitimately.
 *
 * Generated output carries whatever its generator emits and is not authored
 * here; tests ship with no artefact and are covered by the repository licence.
 * Deliberately narrow — a pattern, not a list of names, so it cannot be used to
 * quietly excuse a real source file.
 */
const EXEMPT = [/\.generated\.ts$/, /[\\/]__tests__[\\/]/, /\.(test|spec)\.[tj]sx?$/];

const SPDX = /SPDX-License-Identifier:\s*([A-Za-z0-9.\-]+)/;

function walk(dir, out = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === 'node_modules' || entry.name === 'dist') continue;
    const full = join(dir, entry.name);
    if (entry.isDirectory()) walk(full, out);
    else if (/\.(tsx?|jsx?|css)$/.test(entry.name)) out.push(full);
  }
  return out;
}

/** The licence a path is REQUIRED to carry, or null when either is acceptable. */
function expectedFor(rel) {
  if (APACHE_DIRS.some((d) => rel.startsWith(`${d}/`))) return APACHE;
  return null; // core or EE — both legitimate under frontend/src
}

const files = [
  ...walk(join(FRONTEND, 'src')),
  ...(existsSync(join(FRONTEND, 'design-system')) ? walk(join(FRONTEND, 'design-system')) : []),
];

let missing = 0;
let wrong = 0;
let checked = 0;
const counts = {};

for (const file of files) {
  const rel = relative(REPO, file).split('\\').join('/');
  if (EXEMPT.some((re) => re.test(rel))) continue;

  const head = readFileSync(file, 'utf8').slice(0, 600);
  const m = head.match(SPDX);
  checked++;

  if (!m) {
    console.log(`MISSING  ${rel}`);
    console.log('  no SPDX-License-Identifier in the first 600 bytes\n');
    missing++;
    continue;
  }

  const found = m[1];
  counts[found] = (counts[found] ?? 0) + 1;
  const expected = expectedFor(rel);

  if (expected && found !== expected) {
    console.log(`WRONG    ${rel}`);
    console.log(`  declares ${found}, but this directory is ${expected}`);
    console.log('  (D-014 + D-016, executed by #452 — see frontend/design-system/NOTICE)\n');
    wrong++;
    continue;
  }
  if (!expected && found !== AGPL && found !== EE) {
    console.log(`WRONG    ${rel}`);
    console.log(`  declares ${found}; outside the design system a file is ${AGPL} or ${EE}\n`);
    wrong++;
  }
}

console.log(
  `${checked} files checked — ` +
    Object.entries(counts)
      .sort((a, b) => b[1] - a[1])
      .map(([k, v]) => `${v} ${k}`)
      .join(' · '),
);

if (missing || wrong) {
  console.error(
    `\nLicence boundary FAILED: ${missing} missing header(s), ${wrong} on the wrong side.\n` +
      'frontend/design-system/ and frontend/src/shared/ds/ are Apache-2.0; the rest of\n' +
      'frontend/src/ is AGPL-3.0-only, except Enterprise files which declare\n' +
      'LicenseRef-OpenRisk-Commercial. Apache-2.0 is irreversible once published, so a\n' +
      'file cannot be moved across this boundary by editing its header alone.',
  );
  process.exit(1);
}
console.log('Every file is on the correct side of the three-licence boundary.');
