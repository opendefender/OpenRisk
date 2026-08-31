#!/usr/bin/env node
// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * One-shot codemod: move the product off src/components/ui/* onto the design
 * system in src/shared/ds.
 *
 * Kept in the repo rather than run and deleted, because the interesting part is
 * not the edit — it is the scoping. `isLoading` is a legitimate prop name on 28
 * other components in this codebase (WidgetState, every dashboard), so a
 * project-wide rename would have silently broken them. This only rewrites props
 * inside a <Button …> element, found by scanning for the tag and tracking
 * quote/brace depth to its closing angle bracket rather than by a regex that
 * would stop at the first `>` inside an expression.
 *
 * Usage: node scripts/migrate-primitives.mjs [--dry]
 */

import { readFileSync, writeFileSync } from 'node:fs';
import { readdirSync, statSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const SRC = resolve(here, '../src');
const DRY = process.argv.includes('--dry');

/** Every .tsx/.ts under src, excluding the design system itself. */
function walk(dir, out = []) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      if (entry === 'node_modules') continue;
      walk(full, out);
    } else if (/\.tsx?$/.test(full)) {
      out.push(full);
    }
  }
  return out;
}

/**
 * Finds each `<Tag …>` opening element and hands its attribute text to `fn`.
 * Tracks string and brace depth so an attribute like `onClick={() => f(a > b)}`
 * does not end the tag early.
 */
function rewriteOpeningTags(source, tag, fn) {
  let out = '';
  let index = 0;
  const opener = new RegExp(`<${tag}(?=[\\s/>])`, 'g');
  let match;

  while ((match = opener.exec(source)) !== null) {
    const start = match.index;
    let i = opener.lastIndex;
    let depth = 0;
    let quote = null;

    while (i < source.length) {
      const ch = source[i];
      if (quote) {
        if (ch === quote && source[i - 1] !== '\\') quote = null;
      } else if (ch === '"' || ch === "'" || ch === '`') {
        quote = ch;
      } else if (ch === '{') {
        depth += 1;
      } else if (ch === '}') {
        depth -= 1;
      } else if (ch === '>' && depth === 0) {
        break;
      }
      i += 1;
    }

    const attrs = source.slice(opener.lastIndex, i);
    out += source.slice(index, opener.lastIndex) + fn(attrs);
    index = i;
    opener.lastIndex = i;
  }

  return out + source.slice(index);
}

/**
 * Rewrites an import of components/ui/X to the design-system barrel.
 *
 * The path is computed with path.relative from the importing file, not by
 * string-editing the existing prefix: components/ui/Button and shared/ds sit at
 * different depths under src, and files reach them from four different
 * starting points (src/pages, src/features/x, src/features/x/y, src/components).
 */
function rewriteImports(source, file) {
  const target = resolve(SRC, 'shared/ds');
  let rel = relative(dirname(file), target).replace(/\\/g, '/');
  if (!rel.startsWith('.')) rel = `./${rel}`;

  return source.replace(
    /import\s*\{([^}]*)\}\s*from\s*'[^']*\/?ui\/(Button|Input|Badge|Drawer)'/g,
    (_full, names) => `import {${names}} from '${rel}'`,
  );
}


/**
 * `<Input label error />` -> `<Field label message status><Input /></Field>`.
 *
 * The old Input carried its own label and error markup. That is what made the
 * accessibility inconsistent: the label/description/error wiring lived inside
 * one control, so a Textarea or a Select next to it got none of it. Field owns
 * the wiring now, and Input is just the control — which means these call sites
 * have to be re-nested rather than renamed.
 *
 * Only self-closing <Input /> elements are handled; anything else is left for a
 * human and shows up as a type error rather than being half-transformed.
 */
function liftInputsIntoFields(source) {
  const opener = /^([ \t]*)<Input(?=[\s/>])/gm;
  let out = '';
  let index = 0;
  let match;

  while ((match = opener.exec(source)) !== null) {
    const indent = match[1];
    const tagStart = match.index + indent.length;
    let i = opener.lastIndex;
    let depth = 0;
    let quote = null;

    while (i < source.length) {
      const ch = source[i];
      if (quote) {
        if (ch === quote && source[i - 1] !== '\\') quote = null;
      } else if (ch === '"' || ch === "'" || ch === '`') quote = ch;
      else if (ch === '{') depth += 1;
      else if (ch === '}') depth -= 1;
      else if (ch === '>' && depth === 0) break;
      i += 1;
    }
    if (source[i - 1] !== '/') continue; // not self-closing: leave it alone

    const tag = source.slice(tagStart, i + 1);
    const attrs = source.slice(tagStart + '<Input'.length, i - 1);

    const label = readAttr(attrs, 'label');
    const error = readAttr(attrs, 'error');
    if (!label && !error) continue;

    let rest = attrs;
    if (label) rest = rest.replace(label.raw, '');
    if (error) rest = rest.replace(error.raw, '');
    rest = rest.replace(/\n\s*\n/g, '\n').replace(/[ \t]+$/gm, '');

    // Re-indent the control one level deeper, and put the self-closing bracket
    // back on its own line at the control's new indent.
    const body = rest.replace(/\s+$/, '');
    const inner = [
      `<Input${body}`
        .split('\n')
        .map((line, n) => (n === 0 ? line : `  ${line}`))
        .join('\n'),
      `${indent}  />`,
    ].join('\n');

    const fieldAttrs = [
      label ? ` label=${label.value}` : '',
      error ? ` message=${error.value}` : '',
      error ? ` status={${unwrap(error.value)} ? 'invalid' : 'default'}` : '',
    ].join('');

    const replacement = `<Field${fieldAttrs}>\n${indent}  ${inner}\n${indent}</Field>`;

    out += source.slice(index, tagStart) + replacement;
    index = i + 1;
    opener.lastIndex = index;
  }

  return out + source.slice(index);
}

/** Reads one JSX attribute, returning its raw text and its value expression. */
function readAttr(attrs, name) {
  const start = attrs.search(new RegExp(`\\b${name}=`));
  if (start === -1) return null;
  let i = start + name.length + 1;
  const opening = attrs[i];
  if (opening === '"' || opening === "'") {
    const end = attrs.indexOf(opening, i + 1);
    return { raw: attrs.slice(start, end + 1), value: attrs.slice(i, end + 1) };
  }
  if (opening !== '{') return null;
  let depth = 0;
  for (; i < attrs.length; i += 1) {
    if (attrs[i] === '{') depth += 1;
    else if (attrs[i] === '}') {
      depth -= 1;
      if (depth === 0) break;
    }
  }
  return { raw: attrs.slice(start, i + 1), value: attrs.slice(start + name.length + 1, i + 1) };
}

/** `{expr}` -> `expr`, so it can be embedded in a larger expression. */
function unwrap(value) {
  return value.startsWith('{') ? value.slice(1, -1) : value;
}

/** Collapses `import {A} from 'x'; import {B} from 'x';` into one statement. */
function mergeDesignSystemImports(source) {
  const seen = new Map();
  const pattern = /import\s*\{([^}]*)\}\s*from\s*'([^']*shared\/ds)';\n/g;
  let match;
  while ((match = pattern.exec(source)) !== null) {
    const names = match[1].split(',').map((n) => n.trim()).filter(Boolean);
    const existing = seen.get(match[2]) ?? [];
    seen.set(match[2], [...existing, ...names]);
  }
  if (seen.size === 0) return source;

  let first = true;
  return source.replace(pattern, (_full, _names, path) => {
    if (!first) return '';
    first = false;
    const merged = [...new Set(seen.get(path))].sort().join(', ');
    return `import { ${merged} } from '${path}';\n`;
  });
}

let changed = 0;
const touched = [];

for (const file of walk(SRC)) {
  if (file.includes('/shared/ds/')) continue;
  const original = readFileSync(file, 'utf8');
  if (!/\bui\/(Button|Input|Badge|Drawer)'/.test(original)) continue;

  let next = original;

  // 1. Props, scoped to <Button> elements only.
  next = rewriteOpeningTags(next, 'Button', (attrs) =>
    attrs
      .replace(/\bisLoading=/g, 'loading=')
      .replace(/variant="danger"/g, 'variant="destructive"'),
  );

  // 2. Drawer: isOpen -> open, and the free-form width class -> a named size.
  //    The old drawer had no role, no focus trap and no focus restoration, and
  //    it reset document.body.overflow unconditionally on close, which unlocks
  //    the page while an outer layer still wants it locked.
  next = rewriteOpeningTags(next, 'Drawer', (attrs) =>
    attrs
      .replace(/\bisOpen=/g, 'open=')
      .replace(/\s*widthClassName="max-w-2xl"/g, ' size="lg"')
      .replace(/\s*widthClassName="max-w-xl"/g, ' size="md"')
      .replace(/\s*widthClassName="max-w-3xl"/g, ' size="xl"'),
  );

  // 3. Input's built-in label/error markup becomes a Field wrapper.
  const beforeFields = next;
  next = liftInputsIntoFields(next);

  // 4. Import paths.
  next = rewriteImports(next, file);
  if (next !== beforeFields && next.includes('<Field')) {
    // Field is now referenced; add it to the design-system import.
    next = next.replace(
      /import\s*\{([^}]*)\}\s*from\s*'([^']*shared\/ds)'/,
      (full, names, path) =>
        names.includes('Field') ? full : `import {${names.trimEnd()}, Field } from '${path}'`,
    );
  }

  next = mergeDesignSystemImports(next);

  if (next !== original) {
    changed += 1;
    touched.push(relative(SRC, file));
    if (!DRY) writeFileSync(file, next);
  }
}

console.log(`${DRY ? '[dry] ' : ''}${changed} files rewritten`);
for (const file of touched) console.log(`  ${file}`);
