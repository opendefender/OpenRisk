// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * openrisk/no-mock-data
 *
 * Forbids fabricated data in application source.
 *
 * A new tenant must show zero everywhere. The product's worst defect was that it
 * did not: the dashboard rendered a fully populated probability x impact matrix,
 * the bell listed incidents that had never happened, and the leaderboard ranked
 * seven colleagues who did not exist — all from literals compiled into the
 * components that displayed them. A CISO evaluating the product met invented
 * numbers within ten seconds and had no way to tell which parts were real.
 *
 * Reviewers do not catch this. A `const people = [...]` beside a dozen other
 * constants reads as configuration, and the diff that introduces it usually
 * looks like UI work. So it is enforced mechanically, at error level.
 *
 * What is forbidden:
 *   1. Identifiers named for fabrication — MOCK_RISKS, fakeUsers, sampleData,
 *      dummyRows, DEMO_ASSETS, seedIncidents, stubResponse, FIXTURE_*.
 *      Matched on the name, whatever the shape of the value.
 *   2. TanStack Query's `placeholderData`, when given a literal. It renders
 *      invented content as though it were fetched, which is the exact deception
 *      this rule exists to stop.
 *
 *      Two forms are explicitly allowed, because neither invents anything:
 *        - `placeholderData: keepPreviousData` shows the PREVIOUS page while the
 *          next one loads. That data was really fetched; keeping it on screen is
 *          what stops a table flashing empty on every pagination click.
 *        - `initialData` seeds the cache with data the caller genuinely has,
 *          from an SSR payload or a sibling query.
 *
 * What is deliberately NOT forbidden: array literals in general. Colour maps,
 * option lists, column definitions, taxonomies and enum orderings are all
 * legitimate literals, and a rule broad enough to catch fixture arrays by shape
 * would flag hundreds of them. The naming convention is the reliable signal, so
 * the guard keys on it and the review burden stays where a human can carry it.
 *
 * Where demonstration data is allowed to live: dev/fixtures/*.json, loaded by
 * the Go seeder under DEMO_MODE only. It is outside src/ and outside the bundle,
 * so this rule never has to make an exception for it.
 *
 * Escape hatch: none built in. A genuine one-off needs an
 * eslint-disable-next-line with a comment justifying why the data cannot come
 * from the API — which makes the exception visible in review rather than
 * invisible in a diff.
 */

// Words that mark a value as fabricated. Matched case-insensitively against
// identifier names, on word boundaries so `demoted`, `sampler`, `formatted` and
// `seedling` do not trip it.
//
// Note the absence of a bare `placeholder`: it is the name of a legitimate HTML
// input attribute and appears in every form and i18n string table in the app.
// Only TanStack's `placeholderData` is fabrication, and that is matched exactly
// by its own check below.
const FABRICATION_WORDS = [
  'mock',
  'mocked',
  'fake',
  'dummy',
  'stub',
  'stubbed',
  'fixture',
  'fixtures',
  'sample',
  'samples',
  'demo',
  'seed',
  'seeded',
  'lorem',
  'faker',
];

/**
 * Splits an identifier into lowercase words, handling SCREAMING_SNAKE_CASE,
 * camelCase and PascalCase alike: MOCK_RISKS -> [mock, risks],
 * fakeUserList -> [fake, user, list].
 */
function words(name) {
  return name
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .split(/[^A-Za-z0-9]+/)
    .flatMap((part) => part.split(/(?=[A-Z][a-z])/))
    .map((w) => w.toLowerCase())
    .filter(Boolean);
}

/**
 * True for `keepPreviousData` and `(prev) => prev`-style identity functions —
 * placeholder values that re-show the previous real response rather than
 * inventing one.
 */
function isPreviousDataKeeper(value) {
  if (!value) return false;
  if (value.type === 'Identifier' && value.name === 'keepPreviousData') return true;
  // `placeholderData: (prev) => prev`
  if (
    (value.type === 'ArrowFunctionExpression' || value.type === 'FunctionExpression') &&
    value.params.length >= 1 &&
    value.params[0].type === 'Identifier' &&
    value.body.type === 'Identifier' &&
    value.body.name === value.params[0].name
  ) {
    return true;
  }
  return false;
}

function isFabricationName(name) {
  if (typeof name !== 'string' || !name) return false;
  const parts = words(name);
  return parts.some((w) => FABRICATION_WORDS.includes(w));
}

export default {
  meta: {
    type: 'problem',
    docs: {
      description:
        'Forbid fabricated data (mock/fake/sample/fixture identifiers, placeholderData) in application source',
    },
    schema: [],
    messages: {
      fabricatedIdentifier:
        '"{{name}}" names fabricated data. A fresh tenant must show zero everywhere — render real API data, or an <EmptyState variant="first-use" /> explaining what will fill the screen. Demonstration data belongs in dev/fixtures/, loaded by the Go seeder under DEMO_MODE.',
      placeholderData:
        '`placeholderData` with a literal displays invented content as if it had been fetched. Use a loading skeleton while the request is in flight, and <EmptyState /> when the response is genuinely empty. (`placeholderData: keepPreviousData` and `initialData` are fine — both show data that was really fetched.)',
    },
  },

  create(context) {
    /** Reports any identifier in a binding pattern whose name reads as fabricated. */
    function checkPattern(node) {
      if (!node) return;
      switch (node.type) {
        case 'Identifier':
          if (isFabricationName(node.name)) {
            context.report({ node, messageId: 'fabricatedIdentifier', data: { name: node.name } });
          }
          break;
        case 'ObjectPattern':
          node.properties.forEach((p) => checkPattern(p.value ?? p.argument));
          break;
        case 'ArrayPattern':
          node.elements.forEach((el) => el && checkPattern(el));
          break;
        default:
          break;
      }
    }

    return {
      VariableDeclarator(node) {
        checkPattern(node.id);
      },

      FunctionDeclaration(node) {
        if (node.id && isFabricationName(node.id.name)) {
          context.report({ node: node.id, messageId: 'fabricatedIdentifier', data: { name: node.id.name } });
        }
      },

      // Class fields and object properties: `const cfg = { mockRows: [...] }`.
      PropertyDefinition(node) {
        if (node.key?.type === 'Identifier' && isFabricationName(node.key.name)) {
          context.report({ node: node.key, messageId: 'fabricatedIdentifier', data: { name: node.key.name } });
        }
      },

      Property(node) {
        // `placeholderData: ...` in a useQuery options object.
        if (
          !node.computed &&
          ((node.key.type === 'Identifier' && node.key.name === 'placeholderData') ||
            (node.key.type === 'Literal' && node.key.value === 'placeholderData'))
        ) {
          if (!isPreviousDataKeeper(node.value)) {
            context.report({ node, messageId: 'placeholderData' });
          }
          return;
        }
        // Shorthand `{ mockRows }` is already reported at its declaration; only
        // flag explicit keys, to avoid double-reporting the same binding.
        if (!node.computed && !node.shorthand && node.key.type === 'Identifier' && isFabricationName(node.key.name)) {
          context.report({ node: node.key, messageId: 'fabricatedIdentifier', data: { name: node.key.name } });
        }
      },
    };
  },
};
