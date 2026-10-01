// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * D-060 follow-up fix (#751 phase 3): the KPI/score reel must roll on a
 * genuinely FRESH load and stay plain on cached navigation — the real
 * distinction is fresh vs cached data, not mount vs update. This is the
 * integration-level proof, through a real caller (`ScorePage`) and a real
 * `QueryClient`, that `useScore('tenant')`'s `isFetchedAfterMount` correctly
 * drives `ScoreGauge`'s `fresh` prop and therefore `SlotReel`'s
 * `rollOnMount` — not a unit test of `SlotReel` in isolation.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { ScorePage } from '../ScorePage';
import { scoreQueryKey } from '../../../hooks/useScore';
import type { Score } from '../../../services/scoreService';

const getScore = vi.fn();
vi.mock('../../../services/scoreService', async () => {
  const actual = await vi.importActual<typeof import('../../../services/scoreService')>(
    '../../../services/scoreService',
  );
  return {
    ...actual,
    scoreService: {
      get: (...args: unknown[]) => getScore(...args),
      preview: vi.fn(),
      model: vi.fn(),
    },
  };
});

function makeScore(over: Partial<Score> = {}): Score {
  return {
    scope: 'tenant',
    measured: true,
    value: 63,
    band: 'high',
    band_label_i18n_key: 'score.band.high',
    inherent: 70,
    inherent_band: 'high',
    residual: 63,
    residual_band: 'high',
    mitigation_effectiveness: 0.1,
    computed_at: '2026-08-10T12:00:00.000Z',
    formula_version: '2.1',
    inputs: { critical_risks: 4, applicable_controls: 100 },
    breakdown: [],
    ...over,
  };
}

function renderPage(client: QueryClient) {
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <ScorePage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  getScore.mockReset();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('ScoreGauge/SlotReel — fresh vs cached, through a real caller', () => {
  it('a genuinely fresh fetch rolls the reel from 0', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    getScore.mockResolvedValue(makeScore());
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });

    renderPage(client);

    // The fetch resolves after mount — genuinely fresh, per
    // `isFetchedAfterMount`.
    await waitFor(() => expect(screen.getByTestId('score-value')).toHaveTextContent('63'));
    expect(getScore).toHaveBeenCalledTimes(1);

    const reel = screen.getByTestId('slot-reel');
    // Rolling was observed at some point during the mount-roll window...
    await waitFor(() => expect(reel).toHaveAttribute('data-rolling', 'true'));
    // ...and settles back afterwards, per the same motion budget as any
    // other SlotReel roll.
    act(() => vi.advanceTimersByTime(600));
    await waitFor(() => expect(reel).not.toHaveAttribute('data-rolling'));
    expect(reel).toHaveAttribute('data-settled', 'true');
  });

  it('cached navigation (data already in the QueryClient) renders plain, never rolls', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    getScore.mockResolvedValue(makeScore());
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    // Simulates a revisit: the cache already holds the answer from an
    // earlier mount, well inside useScore's 30s staleTime, so THIS mount
    // never has to fetch at all.
    client.setQueryData(scoreQueryKey('tenant'), makeScore());

    renderPage(client);

    expect(screen.getByTestId('score-value')).toHaveTextContent('63');
    // No fetch happened for this mount — the whole point of the cache hit.
    expect(getScore).not.toHaveBeenCalled();

    const reel = screen.getByTestId('slot-reel');
    expect(reel).not.toHaveAttribute('data-rolling');
    expect(reel).toHaveAttribute('data-settled', 'true');

    // Advancing time (past where a roll would have settled, had one
    // started) confirms this was never a delayed roll either.
    act(() => vi.advanceTimersByTime(600));
    expect(reel).not.toHaveAttribute('data-rolling');
  });
});
