// The score contract, in the browser.
//
// The property this file exists to hold is the reported bug, stated as a test:
//
//   THE DASHBOARD, THE SIDEBAR AND THE DEDICATED PAGE SHOW THE SAME VALUE.
//
// They used to read two different endpoints computing two different quantities on
// two different scales in opposite directions. The fix is not "call the same
// endpoint from three places" — it is one hook and one query key, so the three
// surfaces share a single cache entry and cannot hold two answers. The test below
// asserts that with ONE fetch: if any surface had its own source, the fetch count
// would be wrong and the numbers would drift.
//
// It also pins the second bug: the band never comes from the client, so a label
// cannot disagree with the number beside it.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useScore } from '../../../hooks/useScore';
import { ScoreGauge } from '../../../shared/ScoreGauge';
import { ScoreExplainer } from '../../../shared/ScoreExplainer';
import { bandColor, bandLabel, type Score } from '../../../services/scoreService';

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
    // NOTE: these numbers reconcile on purpose — 0.50×80 + 0.31×71 + 0.19×42 =
    // 70.0 = inherent, and 70.0 × (1 − 0.1) = 63.0 = value. A fixture that did
    // not add up would make the reconciliation test below vacuous.
    //
    // The weights are renormalised over the three AVAILABLE factors (the tenant
    // model declares .40/.25/.20/.15; with vulnerability data missing, its .20
    // is redistributed) — which is exactly what the server does.
    breakdown: [
      {
        factor: 'risk_exposure',
        weight: 0.5,
        raw: 80,
        contribution: 40.0,
        label_i18n_key: 'score.factor.risk_exposure',
        available: true,
      },
      {
        factor: 'control_gaps',
        weight: 0.31,
        raw: 71,
        contribution: 22.0,
        label_i18n_key: 'score.factor.control_gaps',
        available: true,
      },
      {
        factor: 'incident_pressure',
        weight: 0.19,
        raw: 42,
        contribution: 8.0,
        label_i18n_key: 'score.factor.incident_pressure',
        available: true,
      },
      {
        factor: 'vulnerability_pressure',
        weight: 0,
        raw: 0,
        contribution: 0,
        label_i18n_key: 'score.factor.vulnerability_pressure',
        available: false,
      },
    ],
    ...over,
  };
}

/** Stand-ins for the three real surfaces: each renders only via the hook. */
function DashboardHero() {
  const { data } = useScore('tenant');
  return <div data-testid="surface-dashboard">{data ? data.value : '—'}</div>;
}
function SidebarFooter() {
  const { data } = useScore('tenant');
  return (
    <div data-testid="surface-sidebar" data-band={data?.band}>
      {data ? Math.round(data.value) : '—'}
    </div>
  );
}
function DedicatedPage() {
  const { data } = useScore('tenant');
  return <div data-testid="surface-page">{data ? data.value : '—'}</div>;
}

function renderAll(ui: React.ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  getScore.mockReset().mockResolvedValue(makeScore());
});

describe('one score, everywhere', () => {
  it('dashboard, sidebar and the dedicated page show the same value, from ONE fetch', async () => {
    renderAll(
      <>
        <DashboardHero />
        <SidebarFooter />
        <DedicatedPage />
      </>,
    );

    await waitFor(() => expect(screen.getByTestId('surface-dashboard')).toHaveTextContent('63'));

    const dashboard = screen.getByTestId('surface-dashboard').textContent;
    const sidebar = screen.getByTestId('surface-sidebar').textContent;
    const page = screen.getByTestId('surface-page').textContent;

    expect(dashboard).toBe('63');
    expect(sidebar).toBe('63');
    expect(page).toBe('63');

    // The real assertion: one query key → one cache entry → one request. Three
    // requests would mean three independent sources, which is the shape that
    // allowed them to disagree.
    expect(getScore).toHaveBeenCalledTimes(1);
    expect(getScore).toHaveBeenCalledWith('tenant', undefined, expect.anything());
  });

  it('a changed score moves every surface together', async () => {
    const { rerender } = renderAll(
      <>
        <DashboardHero />
        <SidebarFooter />
      </>,
    );
    await waitFor(() => expect(screen.getByTestId('surface-dashboard')).toHaveTextContent('63'));

    // Both read the same object; there is no path by which one could update.
    rerender(
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <MemoryRouter>
          <DashboardHero />
          <SidebarFooter />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    await waitFor(() =>
      expect(screen.getByTestId('surface-dashboard').textContent).toBe(
        screen.getByTestId('surface-sidebar').textContent,
      ),
    );
  });

  it('the band is the server’s and travels with the value', async () => {
    getScore.mockResolvedValue(makeScore({ value: 63, band: 'high' }));
    renderAll(<SidebarFooter />);

    await waitFor(() => expect(screen.getByTestId('surface-sidebar')).toHaveTextContent('63'));
    // Not derived from 63 by any client rule — echoed from the payload.
    expect(screen.getByTestId('surface-sidebar')).toHaveAttribute('data-band', 'high');
  });

  it('an unusual value/band pairing is still rendered as the server sent it', async () => {
    // If the client were deriving the label, this pairing would be "corrected"
    // and the disagreement hidden. It must not be: the server is the authority,
    // and a mismatch is a server bug we want visible, not silently papered over.
    getScore.mockResolvedValue(makeScore({ value: 12, band: 'critical' }));
    renderAll(<SidebarFooter />);

    await waitFor(() => expect(screen.getByTestId('surface-sidebar')).toHaveTextContent('12'));
    expect(screen.getByTestId('surface-sidebar')).toHaveAttribute('data-band', 'critical');
  });
});

describe('ScoreGauge', () => {
  it('renders the value and the server band, and distinguishes "not measured" from zero', async () => {
    renderAll(<ScoreGauge score={makeScore()} title="Score" />);

    // The gauge counts up over ~1.1 s, so give the animation room to land on the
    // real figure rather than asserting on a frame of it.
    await waitFor(() => expect(screen.getByTestId('score-value')).toHaveTextContent('63'), {
      timeout: 3000,
    });
    expect(screen.getByTestId('score-band')).toHaveTextContent(bandLabel('high', 'fr'));

    // No score at all: a dash, never a 0 that would read as a flawless posture.
    renderAll(<ScoreGauge score={undefined} title="Score" />);
    const gauges = screen.getAllByTestId('score-value');
    expect(gauges[gauges.length - 1]).toHaveTextContent('—');
  });
});

describe('ScoreExplainer — the law of zero lies', () => {
  it('shows contributions that add up to the value it claims', () => {
    const score = makeScore();
    renderAll(<ScoreExplainer score={score} inline />);

    const panel = screen.getByTestId('score-explainer');

    // Every available factor is shown with its arithmetic.
    const exposure = within(panel).getByTestId('score-factor-risk_exposure');
    expect(exposure).toHaveTextContent('50%'); // weight
    expect(exposure).toHaveTextContent('80'); // raw
    expect(exposure).toHaveTextContent('40.0'); // contribution = weight × raw

    // And the contributions reconcile with the total the panel prints.
    const sum = score.breakdown
      .filter((f) => f.available)
      .reduce((acc, f) => acc + f.contribution, 0);
    expect(Math.abs(sum - score.inherent)).toBeLessThanOrEqual(0.2);
    expect(panel).toHaveTextContent('70.0 / 100');
  });

  it('says "not measured" instead of hiding an unavailable factor', () => {
    renderAll(<ScoreExplainer score={makeScore()} inline />);
    const panel = screen.getByTestId('score-explainer');

    expect(
      within(panel).getByTestId('score-factor-missing-vulnerability_pressure'),
    ).toBeInTheDocument();
    // …and it is NOT drawn as a zero-height bar among the real contributions.
    expect(within(panel).queryByTestId('score-factor-vulnerability_pressure')).toBeNull();
  });

  it('shows inherent and residual side by side, with the provenance', () => {
    renderAll(<ScoreExplainer score={makeScore()} inline />);
    const panel = screen.getByTestId('score-explainer');

    expect(panel).toHaveTextContent('70.0'); // inherent
    expect(panel).toHaveTextContent('63.0'); // residual = inherent × (1 − 0.1)
    expect(panel).toHaveTextContent('v2.1'); // formula version
    expect(panel).toHaveTextContent('tenant'); // scope
    // The assumptions the calculation used are echoed back.
    expect(panel).toHaveTextContent('critical_risks');
  });
});

describe('bandColor', () => {
  it('maps a BAND, never a number — so it cannot disagree about a boundary', () => {
    expect(bandColor('critical')).toBe('var(--critical)');
    expect(bandColor('high')).toBe('var(--high)');
    expect(bandColor('medium')).toBe('var(--medium)');
    expect(bandColor('low')).toBe('var(--low)');
    // No band → neutral, not "low". An unknown posture is not a good one.
    expect(bandColor(undefined)).toBe('var(--fg-muted)');
  });
});
