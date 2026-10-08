// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #901 — the redesigned dashboard: which view a role gets, which four counters
// each view shows and from which field, which incident the red banner is
// about, and how a due date reads.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useUIStore } from '../../../store/uiStore';
import { variantFor } from '../dashboardVariant';
import { bannerIncident } from '../console/incidentPick';
import { daysUntil } from '../console/dates';
import type { Incident } from '../../incidents/incidentService';

const STATS = {
  total_risks: 14,
  live_risks: 13,
  critical_live: 9,
  without_mitigation: 6,
  in_progress_risks: 5,
  reviews_due_30d: 2,
  risk_matrix: [],
  risks_by_severity: {},
};

vi.mock('../../../lib/api', () => {
  const get = (url: string) => {
    if (url === '/stats') return Promise.resolve({ data: STATS });
    if (url === '/vulnerabilities/stats') return Promise.resolve({ data: { kev_open: 3, kev_count: 7 } });
    if (url === '/incidents/stats') return Promise.resolve({ data: { open_incidents: 5, critical_incidents: 1 } });
    if (url === '/compliance/gap-analysis')
      return Promise.resolve({
        data: {
          frameworks: [
            { percent_complete: 60, not_implemented: 10 },
            { percent_complete: 80, not_implemented: 4 },
          ],
        },
      });
    if (url.startsWith('/evidence/missing'))
      return Promise.resolve({
        data: { frameworks: [{ expiring_soon: 2, no_evidence: 7 }, { expiring_soon: 1, no_evidence: 3 }] },
      });
    if (url === '/compliance/audits') return Promise.resolve({ data: [] });
    if (url === '/analytics/financial')
      return Promise.resolve({
        data: {
          currency: 'XAF',
          fx_rate_xaf: 1,
          total_ale: { xaf: 117_200_000 },
          total_risk_reduction: { xaf: 68_700_000 },
          total_remediation: { xaf: 62_200_000 },
        },
      });
    return Promise.resolve({ data: {} });
  };
  return {
    api: {
      get,
      post: () => Promise.resolve({ data: {} }),
      interceptors: { request: { use: () => 0 }, response: { use: () => 0 } },
    },
    onMFARequired: () => () => undefined,
  };
});

vi.mock('../../../hooks/usePermissions', () => ({
  usePermissions: () => ({ can: () => true, isAdmin: () => true }),
}));

import { CounterList } from '../console/CounterList';

function renderCounters(variant: 'rssi' | 'rm' | 'audit' | 'exec') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <CounterList variant={variant} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const value = (key: string) =>
  within(screen.getByTestId(`dash-counter-${key}`)).getByText(/./, { selector: '.mono' });

beforeEach(() => useUIStore.setState({ lang: 'fr' }));

describe('variantFor', () => {
  it('maps business roles onto the four views, RSSI by default', () => {
    expect(variantFor('rssi')).toBe('rssi');
    expect(variantFor('security_analyst')).toBe('rssi');
    expect(variantFor('risk_manager')).toBe('rm');
    expect(variantFor('auditor')).toBe('audit');
    expect(variantFor('compliance_officer')).toBe('audit');
    expect(variantFor('executive')).toBe('exec');
    expect(variantFor(undefined)).toBe('rssi');
    expect(variantFor('something_new')).toBe('rssi');
  });
});

describe('CounterList', () => {
  it('RSSI: critical live risks, open KEV, open incidents, average coverage', async () => {
    renderCounters('rssi');
    await waitFor(() => expect(value('critical-risks')).toHaveTextContent('9'));
    await waitFor(() => expect(value('kev')).toHaveTextContent('3'));
    expect(value('incidents')).toHaveTextContent('5');
    expect(screen.getByTestId('dash-counter-incidents')).toHaveTextContent('1 critique');
    await waitFor(() => expect(value('coverage')).toHaveTextContent('70 %'));
  });

  it('risk manager: live, without mitigation, in treatment, reviews due', async () => {
    renderCounters('rm');
    await waitFor(() => expect(value('open-risks')).toHaveTextContent('13'));
    expect(value('without-mitigation')).toHaveTextContent('6');
    expect(value('in-treatment')).toHaveTextContent('5');
    expect(value('reviews')).toHaveTextContent('2');
  });

  it('auditor: gaps, expiring evidence, controls without evidence, no audit planned', async () => {
    renderCounters('audit');
    await waitFor(() => expect(value('gaps')).toHaveTextContent('14'));
    await waitFor(() => expect(value('expiring')).toHaveTextContent('3'));
    expect(value('no-evidence')).toHaveTextContent('10');
    await waitFor(() =>
      expect(screen.getByTestId('dash-counter-next-audit')).toHaveTextContent('aucun audit planifié'),
    );
  });

  it('executive: money in millions of the display currency, appetite not set yet', async () => {
    renderCounters('exec');
    await waitFor(() => expect(value('ale')).toHaveTextContent('117,2'));
    expect(value('reduction')).toHaveTextContent('68,7');
    expect(value('plan-cost')).toHaveTextContent('62,2');
    expect(screen.getByTestId('dash-counter-ale')).toHaveTextContent('M FCFA');
    expect(value('appetite')).toHaveTextContent('—');
  });
});

describe('bannerIncident', () => {
  const inc = (id: number, severity: string, status: string, created_at: string) =>
    ({ id, severity, status, created_at, title: `#${id}` }) as unknown as Incident;

  it('picks the most severe open one, the oldest first within a severity', () => {
    const list = [
      inc(1, 'high', 'open', '2026-10-01T00:00:00Z'),
      inc(2, 'critical', 'in_progress', '2026-10-05T00:00:00Z'),
      inc(3, 'critical', 'open', '2026-10-03T00:00:00Z'),
      inc(4, 'critical', 'resolved', '2026-09-01T00:00:00Z'),
    ];
    expect(bannerIncident(list)?.id).toBe(3);
  });

  it('shows nothing for medium or closed incidents', () => {
    expect(bannerIncident([inc(1, 'medium', 'open', '2026-10-01T00:00:00Z')])).toBeUndefined();
    expect(bannerIncident([inc(1, 'critical', 'closed', '2026-10-01T00:00:00Z')])).toBeUndefined();
  });
});

describe('daysUntil', () => {
  const now = new Date(2026, 9, 8, 15, 0);
  it('counts calendar days, negative when overdue', () => {
    expect(daysUntil(new Date(2026, 9, 8, 1).toISOString(), now)).toBe(0);
    expect(daysUntil(new Date(2026, 9, 9, 23).toISOString(), now)).toBe(1);
    expect(daysUntil(new Date(2026, 9, 6, 12).toISOString(), now)).toBe(-2);
  });
});
