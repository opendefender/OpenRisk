// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// <ScoreWorking> (#486): the working is the server's, a term's origin is the
// entry the server cites, and a missing, withheld or altered origin is said as
// such — never filled in.

/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import type { ScoreWorking as Working } from '../scoreWorkingService';

const getMock = vi.fn<(id: string) => Promise<Working>>();
vi.mock('../scoreWorkingService', () => ({
  scoreWorkingService: { get: (id: string) => getMock(id) },
}));

let canReadAudit = true;
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: <T,>(sel: (s: { hasPermission: (p: string) => boolean }) => T) =>
    sel({ hasPermission: () => canReadAudit }),
}));

import { ScoreWorking } from '../components/ScoreWorking';

const at = '2026-09-20T10:00:00Z';
function working(over: Partial<Working> = {}): Working {
  return {
    risk_id: 'r1',
    formula: 'probability × impact × asset_criticality',
    terms: [
      {
        key: 'probability',
        value: 0.5,
        min: 0,
        max: 1,
        source: {
          event_id: 'e1',
          sequence: 1,
          hash: 'a'.repeat(64),
          prev_hash: '',
          hash_valid: true,
          action: 'create',
          actor_type: 'user',
          actor_email: 'analyst@bank.example',
          at,
          value: 0.5,
        },
      },
      {
        key: 'impact',
        value: 4,
        min: 0,
        max: 10,
        source: {
          event_id: 'e2',
          sequence: 2,
          hash: 'b'.repeat(64),
          prev_hash: 'a'.repeat(64),
          hash_valid: false,
          action: 'update',
          actor_type: 'service_token',
          actor_label: 'c0ffee00-1111',
          actor_email: 'ops@bank.example',
          at,
          value: 4,
        },
      },
      { key: 'asset_criticality', value: 3, min: 0.1, max: 3, source: null },
    ],
    assets: [{ id: 'a1', name: 'Core banking', criticality: 'CRITICAL', factor: 3, source: null }],
    asset_criticality_defaulted: false,
    computed: 6,
    criticality: 'high',
    explanation: '0.500 × 4.000 × 3.000 = 6.000 → High',
    stored: 6,
    stored_criticality: 'high',
    consistent: true,
    score_source: {
      event_id: 'e3',
      sequence: 3,
      hash: 'c'.repeat(64),
      prev_hash: 'b'.repeat(64),
      hash_valid: true,
      action: 'update',
      actor_type: 'job',
      actor_label: 'score-engine',
      at,
      value: 6,
    },
    sources_visible: true,
    ...over,
  };
}

function renderIt() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <ScoreWorking riskId="r1" storedScore={6} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  getMock.mockReset();
  canReadAudit = true;
});

describe('ScoreWorking', () => {
  it('shows a skeleton while loading', () => {
    getMock.mockReturnValue(new Promise(() => {}));
    renderIt();
    expect(screen.getByTestId('score-working-loading')).toBeTruthy();
  });

  it('shows an error state with a retry', async () => {
    getMock.mockRejectedValue(new Error('boom'));
    renderIt();
    expect(
      await screen.findByText(/could not load the score working|impossible d’afficher/i),
    ).toBeTruthy();
  });

  it('renders the server working and cites each term', async () => {
    getMock.mockResolvedValue(working());
    renderIt();
    const eq = await screen.findByTestId('score-working-equation');
    expect(eq.textContent).toContain('0.500');
    expect(eq.textContent).toContain('4.0');
    expect(eq.textContent).toContain('6.000');

    const prob = screen.getByTestId('score-working-source-probability');
    expect(prob.textContent).toContain('analyst@bank.example');
    expect(prob.textContent).toContain('#1');

    // A token is named as a token, and the engine as a job — never "system".
    expect(screen.getByTestId('score-working-source-impact').textContent).toMatch(
      /(token|jeton) c0ffee00/,
    );
    expect(screen.getByTestId('score-working-source-score').textContent).toMatch(
      /(job|tâche) score-engine/,
    );
    expect(screen.queryByText(/system/i)).toBeNull();

    // An asset with no recorded change says so instead of inventing an origin.
    expect(screen.getByTestId('score-working-asset-a1-none')).toBeTruthy();
    expect(screen.queryByTestId('score-working-inconsistent')).toBeNull();
  });

  it('flags a source entry that no longer verifies', async () => {
    getMock.mockResolvedValue(working());
    renderIt();
    const impact = await screen.findByTestId('score-working-source-impact');
    expect(impact.textContent).toMatch(/altered since it was sealed|altérée depuis son scellement/);
    expect(screen.getByTestId('score-working-source-probability').textContent).not.toMatch(
      /altered|altérée/,
    );
  });

  it('says so when the stored score does not match the working', async () => {
    getMock.mockResolvedValue(working({ stored: 2, consistent: false }));
    renderIt();
    const warn = await screen.findByTestId('score-working-inconsistent');
    expect(warn.textContent).toContain('2.000');
    expect(warn.textContent).toContain('6.000');
  });

  it('withholds origins from a reader without audit access, and says it did', async () => {
    canReadAudit = false;
    getMock.mockResolvedValue(
      working({
        sources_visible: false,
        score_source: null,
        terms: working().terms.map((t) => ({ ...t, source: null })),
      }),
    );
    renderIt();
    expect(await screen.findByTestId('score-working-withheld')).toBeTruthy();
    expect(screen.queryByTestId('score-working-source-probability-none')).toBeNull();
    expect(screen.queryByText(/verify the integrity|vérifier l’intégrité/i)).toBeNull();
  });
});
