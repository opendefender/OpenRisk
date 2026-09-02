#!/usr/bin/env node
// ESLint ratchet (#341, D-022) — BLOCKING in CI.
//
// `npm run lint` reports 321 errors on a clean master. That is not a reason to
// stop linting, and it is not a reason to drop the rules to 'warn': ABSOLUTE
// RULE 5 is "zero `any`", and a rule nothing enforces is not a rule. So the
// gate is a RATCHET — the debt is frozen at a per-rule ceiling and a PR fails
// only if it makes a rule WORSE.
//
// The practical effect is the point: before this, a genuine regression was
// indistinguishable from the standing noise, so no frontend PR was actually
// checked. Now every rule not already in debt is enforced at zero, and every
// rule in debt can only improve.
//
// Per RULE, not one global total, deliberately. A single number lets a PR
// delete two unused variables and add two `any`s and still pass; that is
// exactly the trade the ceiling exists to refuse.
//
// THESE NUMBERS MAY ONLY EVER GO DOWN. Raising one to land a PR is the edit
// that makes the gate meaningless — fix the finding, or add the file to the
// narrow ignore list in eslint.config.js with a reason.
//
// Usage:
//   node scripts/check-lint-ceiling.mjs            check (CI)
//   node scripts/check-lint-ceiling.mjs --update   re-freeze after improving
import { readFileSync, writeFileSync } from 'node:fs'
import { spawnSync } from 'node:child_process'

const CEILING_FILE = new URL('../.lint-ceiling.json', import.meta.url)
const update = process.argv.includes('--update')

// eslint exits 1 when it reports errors, which is the normal case here, so the
// exit code tells us nothing. Only unparseable stdout means the tool failed.
const run = spawnSync('npx', ['eslint', '.', '-f', 'json'], {
  encoding: 'utf8',
  maxBuffer: 64 * 1024 * 1024,
})

let report
try {
  report = JSON.parse(run.stdout)
} catch {
  console.error('✗ Could not run ESLint or parse its JSON report.')
  console.error(run.stderr || run.stdout || '(no output)')
  process.exit(2)
}

const counts = {}
let total = 0
for (const file of report) {
  for (const msg of file.messages) {
    if (msg.severity !== 2) continue // warnings are not gated
    const rule = msg.ruleId ?? '(parse error)'
    counts[rule] = (counts[rule] ?? 0) + 1
    total++
  }
}

const sorted = Object.fromEntries(Object.entries(counts).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])))

if (update) {
  writeFileSync(CEILING_FILE, JSON.stringify({ total, rules: sorted }, null, 2) + '\n')
  console.log(`✓ Ceiling re-frozen at ${total} errors across ${Object.keys(sorted).length} rules.`)
  process.exit(0)
}

let ceiling
try {
  ceiling = JSON.parse(readFileSync(CEILING_FILE, 'utf8'))
} catch {
  console.error(`✗ ${CEILING_FILE.pathname} missing or invalid. Run with --update to create it.`)
  process.exit(2)
}

const regressions = []
for (const [rule, count] of Object.entries(counts)) {
  const max = ceiling.rules[rule] ?? 0
  if (count > max) regressions.push({ rule, count, max })
}

const improvements = []
for (const [rule, max] of Object.entries(ceiling.rules)) {
  const count = counts[rule] ?? 0
  if (count < max) improvements.push({ rule, count, max })
}

console.log(`ESLint errors: ${total} (ceiling ${ceiling.total})`)

if (regressions.length > 0) {
  console.error('\n✗ ESLint regressed. These rules got worse:\n')
  for (const { rule, count, max } of regressions.sort((a, b) => b.count - b.max - (a.count - a.max))) {
    console.error(`    ${rule}: ${count} (ceiling ${max}, +${count - max})`)
  }
  console.error('\nRun `npm run lint` to see the offending lines. Fix them — do not raise the ceiling.')
  process.exit(1)
}

if (improvements.length > 0) {
  const paid = improvements.reduce((n, i) => n + (i.max - i.count), 0)
  console.log(`\n✓ No regression, and ${paid} error(s) paid down:\n`)
  for (const { rule, count, max } of improvements) console.log(`    ${rule}: ${max} -> ${count}`)
  console.log('\nTighten the ratchet in this PR: `npm run lint:ceiling -- --update` and commit .lint-ceiling.json.')
} else {
  console.log('✓ No regression.')
}
