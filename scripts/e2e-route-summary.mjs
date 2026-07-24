#!/usr/bin/env node
// Reads the Playwright JSON report and emits a Markdown route×status table.
// Used by the E2E workflow ($GITHUB_STEP_SUMMARY) and to seed the audit table.
// Route/status/5xx/console signals come from the annotations attached by
// smoke.routes.spec.ts.

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const RESULTS = path.resolve(HERE, '../tests/e2e/.artifacts/results.json');
const OUT = path.resolve(HERE, '../tests/e2e/.artifacts/route-summary.md');

function collectSpecs(node, acc) {
  if (!node) return;
  if (Array.isArray(node.suites)) node.suites.forEach((s) => collectSpecs(s, acc));
  if (Array.isArray(node.specs)) node.specs.forEach((s) => acc.push(s));
}

function main() {
  if (!fs.existsSync(RESULTS)) {
    console.error(`no results at ${RESULTS}`);
    process.exit(0);
  }
  const report = JSON.parse(fs.readFileSync(RESULTS, 'utf8'));
  const specs = [];
  (report.suites || []).forEach((s) => collectSpecs(s, specs));

  const rows = [];
  for (const spec of specs) {
    for (const t of spec.tests || []) {
      const anns = (t.annotations || []).concat(
        ...(t.results || []).map((r) => r.annotations || []),
      );
      const status = anns.find((a) => a.type === 'route-status');
      if (!status) continue;
      const [routePath, name, state] = status.description.split('|').map((x) => x.trim());
      const fivexx = anns.find((a) => a.type === 'route-5xx');
      const outcome = (t.results || [])[t.results.length - 1]?.status || t.status || 'unknown';
      rows.push({ routePath, name, state, outcome, fivexx: fivexx?.description?.split('::')[1]?.trim() || '' });
    }
  }
  rows.sort((a, b) => a.routePath.localeCompare(b.routePath));

  const emoji = (s) => ({ OK: '✅', 'dégradé': '🟡', placeholder: '🔵', 'cassé': '🔴' }[s] || (s?.startsWith('redirect') ? '↪️' : '·'));
  let md = `## E2E route coverage (${rows.length} routes)\n\n`;
  md += `| Route | Écran | Statut | Test | 5xx |\n|---|---|---|---|---|\n`;
  for (const r of rows) {
    md += `| \`${r.routePath}\` | ${r.name} | ${emoji(r.state)} ${r.state} | ${r.outcome} | ${r.fivexx ? '⚠️ ' + r.fivexx : '—'} |\n`;
  }
  const broken = rows.filter((r) => r.state === 'cassé').length;
  const degraded = rows.filter((r) => r.state === 'dégradé').length;
  md += `\n**${rows.length} routes · ${broken} cassé · ${degraded} dégradé**\n`;

  fs.mkdirSync(path.dirname(OUT), { recursive: true });
  fs.writeFileSync(OUT, md);
  process.stdout.write(md);
}

main();
