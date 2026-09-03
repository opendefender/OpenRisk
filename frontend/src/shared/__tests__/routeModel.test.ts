// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from 'vitest';
import {
  ROUTES,
  resolveRoute,
  routeTrail,
  parentHref,
  permissionFor,
  matchPath,
  fillPattern,
} from '../routeModel';

describe('matchPath', () => {
  it('matches static segments exactly', () => {
    expect(matchPath('/compliance/audits', '/compliance/audits')).toEqual({});
    expect(matchPath('/compliance/audits', '/compliance/remediation')).toBeNull();
  });

  it('captures params and rejects arity mismatches', () => {
    expect(matchPath('/compliance/audits/:auditId', '/compliance/audits/abc')).toEqual({
      auditId: 'abc',
    });
    expect(matchPath('/compliance/audits/:auditId', '/compliance/audits')).toBeNull();
    expect(matchPath('/compliance/audits/:auditId', '/compliance/audits/a/b')).toBeNull();
  });
});

describe('resolveRoute', () => {
  // '/reports/board' and '/reports/:reportId' both match /reports/board. The
  // static one must win, or the Board Reports screen becomes a report detail.
  it('prefers the more specific pattern over a param', () => {
    expect(resolveRoute('/reports/board')?.node.path).toBe('/reports/board');
    expect(resolveRoute('/reports/abc-123')?.node.path).toBe('/reports/:reportId');
  });

  it('returns null for unknown paths', () => {
    expect(resolveRoute('/nowhere')).toBeNull();
  });
});

describe('routeTrail', () => {
  it('builds a root-first chain', () => {
    const trail = routeTrail('/compliance/audits');
    expect(trail.map((t) => t.node.path)).toEqual(['/compliance', '/compliance/audits']);
  });

  // The bug this guards: a trail rendered from raw patterns would link back to
  // "/compliance/frameworks/:frameworkId" literally, a 404.
  it('fills ancestor params from the matched route', () => {
    const trail = routeTrail('/compliance/frameworks/fw-7/gaps');
    expect(trail.map((t) => t.href)).toEqual([
      '/compliance',
      '/compliance/frameworks/fw-7',
      '/compliance/frameworks/fw-7/gaps',
    ]);
  });

  it('is empty for an unknown path rather than throwing', () => {
    expect(routeTrail('/nowhere')).toEqual([]);
  });
});

describe('parentHref', () => {
  it('gives depth>=2 routes somewhere to go back to', () => {
    expect(parentHref('/compliance/audits/a1')).toBe('/compliance/audits');
    expect(parentHref('/risks/mitigations/m1')).toBe('/risks/mitigations');
    expect(parentHref('/reports/jobs/j1')).toBe('/reports');
  });

  it('is null at depth 1 — a top-level page is not a dead end', () => {
    expect(parentHref('/risks')).toBeNull();
    expect(parentHref('/')).toBeNull();
  });
});

describe('the tree itself', () => {
  // This is the structural guarantee behind "no screen without a way back".
  it('gives every non-root node a resolvable parent', () => {
    const paths = new Set(ROUTES.map((r) => r.path));
    for (const node of ROUTES) {
      const depth = node.path.split('/').filter(Boolean).length;
      if (depth < 2 || node.topLevel) continue;
      expect(node.parent, `${node.path} has no parent`).toBeTruthy();
      expect(paths.has(node.parent!), `${node.path} -> unknown parent ${node.parent}`).toBe(true);
    }
  });

  it('has no duplicate patterns', () => {
    const seen = new Set<string>();
    for (const r of ROUTES) {
      expect(seen.has(r.path), `duplicate route ${r.path}`).toBe(false);
      seen.add(r.path);
    }
  });

  it('cannot loop: walking parents always terminates', () => {
    for (const r of ROUTES) {
      const trail = routeTrail(
        fillPattern(
          r.path,
          Object.fromEntries(
            r.path
              .split('/')
              .filter((s) => s.startsWith(':'))
              .map((s) => [s.slice(1), 'x']),
          ),
        ),
      );
      expect(trail.length).toBeGreaterThan(0);
      expect(trail.length).toBeLessThan(10);
    }
  });

  it('labels every node exactly one way', () => {
    for (const r of ROUTES) {
      const hasKey = Boolean(r.labelKey);
      const hasLiteral = Boolean(r.label);
      expect(hasKey !== hasLiteral, `${r.path} must have exactly one of labelKey/label`).toBe(true);
    }
  });
});

describe('permissionFor', () => {
  it('names the permission a route needs', () => {
    expect(permissionFor('/compliance/audits/a1')).toBe('compliance:audits:read');
    expect(permissionFor('/risks')).toBe('risks:read');
  });

  it('is undefined for unguarded routes', () => {
    expect(permissionFor('/settings')).toBeUndefined();
    expect(permissionFor('/nowhere')).toBeUndefined();
  });
});
