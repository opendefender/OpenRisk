#!/usr/bin/env node
// OpenAPI spec lint (#814) — BLOCKING in CI.
//
// `generate:api-types` is not a validator. It crashes on a broken $ref, but it
// accepts an invalid `type`, an undeclared path parameter or an empty
// `responses` and writes types from them. When this gate landed the spec had
// 65 structural errors nobody had seen: 48 OpenAPI 3.0 `nullable: true` that a
// 3.1 document does not allow (openapi-typescript tolerates them, stricter
// tools do not), and 17 flow-style descriptions whose commas YAML read as
// extra keys, which cut each description short at its first comma.
//
// Redocly's `recommended` ruleset. Errors fail; warnings are printed and left
// for later, so a PR is not red for findings it did not cause.
//
// Usage:
//   node scripts/lint-openapi.mjs                  lint docs/openapi.yaml
//   node scripts/lint-openapi.mjs path/to/spec.yaml
import { fileURLToPath } from 'node:url';
import { createConfig, lint } from '@redocly/openapi-core';

const spec = process.argv[2] ?? fileURLToPath(new URL('../../docs/openapi.yaml', import.meta.url));

const config = await createConfig({ extends: ['recommended'] });
const problems = await lint({ ref: spec, config });

const errors = problems.filter((p) => p.severity === 'error');
const warnings = problems.filter((p) => p.severity === 'warn');

for (const p of problems) {
  const where = p.location?.[0]?.pointer ?? '';
  const tag = p.severity === 'error' ? '::error::' : 'warn ';
  console.log(`${tag}${p.ruleId} ${where} — ${p.message}`);
}

console.log(`\nOpenAPI lint: ${errors.length} error(s), ${warnings.length} warning(s).`);
if (errors.length > 0) {
  console.log('✗ Fix the errors above in docs/openapi.yaml.');
  process.exit(1);
}
console.log('✓ No errors.');
