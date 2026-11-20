// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// What the register's URL says and what the API is asked for (#913): the view
// tab (`f.scope`), the band pills, the status and owner listboxes, the layout
// (`view=matrix`, kept for the dashboard link), and the review date state.

import { describe, expect, it } from 'vitest';
import { EMPTY_TABLE_STATE, type TableState } from '../../../shared/datatable';
import {
  MATRIX_LIMIT,
  RISK_VIEWS,
  buildRiskParams,
  buildSummaryParams,
  parseBands,
  parseLayout,
  parseStatus,
  parseView,
  readFilters,
  reviewState,
  toStatusValue,
} from '../riskViews';

const state = (over: Partial<TableState>): TableState => ({ ...EMPTY_TABLE_STATE, ...over });

describe('parseView', () => {
  it('knows the five views', () => {
    expect([...RISK_VIEWS]).toEqual(['all', 'mine', 'hot', 'review_due', 'accepted']);
    expect(parseView('review_due')).toBe('review_due');
  });

  it('normalizes casing and refuses anything unknown (it falls back to "all")', () => {
    expect(parseView('HOT')).toBe('hot');
    expect(parseView('  Mine ')).toBe('mine');
    expect(parseView('everything')).toBe('all');
    expect(parseView(null)).toBe('all');
    expect(parseView(undefined)).toBe('all');
  });
});

describe('parseLayout — ?view=matrix keeps working', () => {
  it('opens the matrix for the dashboard link, in any casing', () => {
    expect(parseLayout('matrix')).toBe('matrix');
    expect(parseLayout('Matrix')).toBe('matrix');
  });

  it('is the table for anything else', () => {
    expect(parseLayout(null)).toBe('table');
    expect(parseLayout('hot')).toBe('table');
  });
});

describe('parseBands / parseStatus', () => {
  it('keeps only real bands, in the pills order', () => {
    expect(parseBands(['LOW', 'critical', 'purple'])).toEqual(['critical', 'low']);
    expect(parseBands(undefined)).toEqual([]);
  });

  it('keeps one real status or none', () => {
    expect(parseStatus(['In_Progress'])).toBe('in_progress');
    expect(parseStatus(['nope'])).toBe('');
    expect(parseStatus([])).toBe('');
  });
});

describe('toStatusValue — both live vocabularies', () => {
  it('maps the legacy words onto the five statuses, closed included', () => {
    expect(toStatusValue('ACTIVE')).toBe('in_progress');
    expect(toStatusValue('in_progress')).toBe('in_progress');
    expect(toStatusValue('mitigated')).toBe('mitigated');
    expect(toStatusValue('Accepted')).toBe('accepted');
    expect(toStatusValue('closed')).toBe('closed');
    expect(toStatusValue('DONE')).toBe('closed');
  });

  it('never throws on an unknown or missing status: it reads as open', () => {
    expect(toStatusValue('DRAFT')).toBe('open');
    expect(toStatusValue('???')).toBe('open');
    expect(toStatusValue(undefined)).toBe('open');
  });
});

describe('readFilters', () => {
  it('reads tab, bands, status and owner from the table state', () => {
    expect(
      readFilters({
        scope: ['hot'],
        criticality: ['high', 'critical'],
        status: ['open'],
        owner_id: ['u-1'],
      }),
    ).toEqual({ view: 'hot', bands: ['critical', 'high'], status: 'open', ownerId: 'u-1' });
  });

  it('is empty by default', () => {
    expect(readFilters({})).toEqual({ view: 'all', bands: [], status: '', ownerId: '' });
  });
});

describe('buildRiskParams', () => {
  it('pages server-side and maps the tab onto the API `view` param', () => {
    const p = buildRiskParams(
      state({
        page: 2,
        pageSize: 50,
        q: ' vpn ',
        sort: { key: 'score', dir: 'desc' },
        filters: {
          scope: ['review_due'],
          criticality: ['critical', 'high'],
          status: ['open'],
          owner_id: ['u-1'],
          category_id: ['c-1'],
        },
      }),
      'table',
    );
    expect(p).toEqual({
      page: 2,
      limit: 50,
      q: 'vpn',
      sort_by: 'score',
      sort_dir: 'desc',
      criticality: 'critical,high',
      status: 'open',
      owner_id: 'u-1',
      category_id: 'c-1',
      view: 'review_due',
    });
  });

  it('never sends view=all, nor the tab as a filter of its own', () => {
    const p = buildRiskParams(state({ filters: { scope: ['all'] } }), 'table');
    expect(p).not.toHaveProperty('view');
    expect(p).not.toHaveProperty('scope');
    expect(buildRiskParams(state({}), 'table')).not.toHaveProperty('view');
  });

  it('asks for the largest page in matrix mode, ranked by score', () => {
    const p = buildRiskParams(
      state({ page: 4, pageSize: 25, sort: { key: 'title', dir: 'asc' } }),
      'matrix',
    );
    expect(p).toMatchObject({ page: 1, limit: MATRIX_LIMIT, sort_by: 'score', sort_dir: 'desc' });
  });
});

describe('buildSummaryParams', () => {
  it('asks the summary for the tab and the search, nothing else', () => {
    expect(
      buildSummaryParams(
        state({ q: ' x ', filters: { scope: ['mine'], criticality: ['low'], status: ['open'] } }),
      ),
    ).toEqual({ view: 'mine', q: 'x' });
  });
});

describe('reviewState — the review column and card', () => {
  const now = new Date('2026-11-19T12:00:00Z');
  const at = (days: number) => new Date(now.getTime() + days * 86_400_000).toISOString();

  it('is "closed" for a closed risk whatever its date', () => {
    expect(reviewState(at(-30), 'closed', now)).toEqual({ kind: 'closed' });
  });

  it('is "none" without a date or with an unreadable one', () => {
    expect(reviewState(null, 'open', now)).toEqual({ kind: 'none' });
    expect(reviewState('not a date', 'open', now)).toEqual({ kind: 'none' });
  });

  it('is overdue as soon as the instant has passed', () => {
    expect(reviewState(at(-3), 'open', now).kind).toBe('overdue');
    expect(reviewState(new Date(now.getTime() - 3_600_000).toISOString(), 'open', now).kind).toBe(
      'overdue',
    );
  });

  it('is "soon" within 14 days, "later" beyond', () => {
    expect(reviewState(at(0.5), 'open', now).kind).toBe('soon');
    expect(reviewState(at(14), 'open', now).kind).toBe('soon');
    expect(reviewState(at(15), 'open', now)).toMatchObject({ kind: 'later', days: 15 });
  });
});
