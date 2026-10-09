// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The Activity journal of the redesign (#905), against GET /timeline.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useUIStore } from '../../../store/uiStore';
import type { TimelineEvent, TimelinePage } from '../../entity-drawer/types';
import { groupByDay, initialsOf, phraseKeys, relativeDay } from '../journal';

const calls = vi.fn<(params: Record<string, unknown>) => void>();
let pages: Record<string, TimelinePage> = {};
let fail = false;

vi.mock('../../../lib/api', () => ({
  api: {
    get: (url: string, cfg?: { params?: Record<string, unknown> }) => {
      if (url === '/timeline') {
        const params = cfg?.params ?? {};
        calls(params);
        if (fail) return Promise.reject(new Error('boom'));
        const key = `${(params.domain as string) ?? ''}|${(params.cursor as string) ?? ''}`;
        return Promise.resolve({ data: pages[key] ?? { events: [], sources: ['audit'] } });
      }
      return Promise.resolve({ data: {} });
    },
    post: () => Promise.resolve({ data: {} }),
    interceptors: { request: { use: () => 0 }, response: { use: () => 0 } },
  },
  onMFARequired: () => () => undefined,
}));

import { ActivityPage } from '../ActivityPage';

const NOW = new Date();
function iso(daysAgo: number, h: number, m: number): string {
  const d = new Date(NOW);
  d.setDate(d.getDate() - daysAgo);
  d.setHours(h, m, 0, 0);
  return d.toISOString();
}

function ev(over: Partial<TimelineEvent>): TimelineEvent {
  return {
    id: Math.random().toString(36).slice(2),
    kind: 'create',
    occurred_at: iso(0, 9, 15),
    actor: { id: 'u1', email: 'fatou@example.test', label: 'Fatou Ndiaye' },
    target: { id: 't1' },
    summary: '',
    source: 'audit',
    ...over,
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <ActivityPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  calls.mockReset();
  fail = false;
  useUIStore.setState({ lang: 'fr' });
  pages = {
    '|': {
      events: [
        ev({
          domain: 'mitigation',
          object: "Bastion d'administration (PAM)",
          target_url: '/mitigations',
        }),
        ev({
          kind: 'update',
          occurred_at: iso(0, 8, 5),
          domain: 'risk',
          object: 'Rançongiciel',
          target: { type: 'risk', id: 'r1' },
          changes: [{ field: 'status' }],
        }),
        ev({
          actor: undefined,
          occurred_at: iso(1, 17, 32),
          domain: 'incident',
          object: 'Indisponibilité du canal USSD',
          target: { type: 'incident', id: 'i1' },
        }),
      ],
      next_cursor: 'c2',
      sources: ['audit'],
    },
    '|c2': {
      events: [
        ev({ occurred_at: iso(9, 14, 5), domain: 'report', object: 'Comité des risques — T3' }),
      ],
      sources: ['audit'],
    },
    'mitigation|': {
      events: [ev({ domain: 'mitigation', object: 'Segmentation réseau' })],
      sources: ['audit'],
    },
  };
});

describe('#905 — wording helpers', () => {
  it('builds the phrase from the verb, the domain and status changes', () => {
    expect(phraseKeys(ev({ domain: 'risk' }))).toEqual({
      verb: 'activity.verb.create',
      noun: 'activity.noun.risk',
    });
    expect(
      phraseKeys(ev({ kind: 'update', domain: 'risk', changes: [{ field: 'status' }] })),
    ).toEqual({
      verb: 'activity.verb.status',
      noun: 'activity.nounAfterDe.risk',
    });
    expect(phraseKeys(ev({ kind: 'escalate', domain: 'nope' }))).toEqual({
      verb: 'activity.verb.fallback',
      noun: null,
    });
  });

  it('groups by local day and names today and yesterday', () => {
    const g = groupByDay([
      ev({ occurred_at: iso(0, 9, 0) }),
      ev({ occurred_at: iso(0, 8, 0) }),
      ev({ occurred_at: iso(1, 9, 0) }),
    ]);
    expect(g.map((d) => d.events.length)).toEqual([2, 1]);
    expect(relativeDay(iso(0, 1, 0), NOW)).toBe('today');
    expect(relativeDay(iso(1, 1, 0), NOW)).toBe('yesterday');
    expect(relativeDay(iso(4, 1, 0), NOW)).toBeNull();
  });

  it('takes initials from a name or an address', () => {
    expect(initialsOf('Fatou Ndiaye')).toBe('FN');
    expect(initialsOf('admin@opendefender.io')).toBe('AD');
    expect(initialsOf('jean-marc.ekotto@x.io')).toBe('JM');
  });
});

describe('#905 — Activity page', () => {
  it('reads each entry as a sentence, grouped by day', async () => {
    renderPage();
    const entries = await screen.findAllByTestId('act-entry');
    expect(entries[0]).toHaveTextContent(
      "Fatou Ndiaye a créé la mitigation Bastion d'administration (PAM)",
    );
    expect(entries[0]).toHaveTextContent('FN');
    expect(entries[1]).toHaveTextContent('a changé le statut du risque Rançongiciel');
    const days = screen.getAllByTestId('act-day');
    expect(within(days[0]).getByRole('heading')).toHaveTextContent("Aujourd'hui");
    expect(within(days[1]).getByRole('heading')).toHaveTextContent('Hier');
  });

  it('shows the product mark for system entries', async () => {
    renderPage();
    await screen.findAllByTestId('act-entry');
    expect(screen.getAllByTestId('act-avatar-system')).toHaveLength(1);
    expect(screen.getAllByTestId('act-entry')[2]).toHaveTextContent("OpenRisk a créé l'incident");
  });

  it('links objects without a drawer to their page', async () => {
    renderPage();
    const link = await screen.findByRole('link', { name: "Bastion d'administration (PAM)" });
    expect(link).toHaveAttribute('href', '/mitigations');
  });

  it('loads older entries with the button', async () => {
    renderPage();
    fireEvent.click(await screen.findByTestId('act-more'));
    expect(await screen.findByText('Comité des risques — T3')).toBeInTheDocument();
    expect(screen.getByText('Début du journal')).toBeInTheDocument();
    expect(calls).toHaveBeenCalledWith(expect.objectContaining({ cursor: 'c2' }));
  });

  it('filters by domain on the server', async () => {
    renderPage();
    await screen.findAllByTestId('act-entry');
    fireEvent.click(screen.getByTestId('act-filter-mitigation'));
    expect(await screen.findByText('Segmentation réseau')).toBeInTheDocument();
    expect(calls).toHaveBeenCalledWith(expect.objectContaining({ domain: 'mitigation' }));
    expect(screen.getByTestId('act-filter-mitigation')).toHaveAttribute('aria-pressed', 'true');
  });

  it('empty filter offers to show everything', async () => {
    renderPage();
    await screen.findAllByTestId('act-entry');
    fireEvent.click(screen.getByTestId('act-filter-evidence'));
    expect(
      await screen.findByText("Rien dans cette catégorie pour l'instant."),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Tout afficher' }));
    await waitFor(() => expect(screen.getAllByTestId('act-entry').length).toBeGreaterThan(0));
  });

  it('a failed load offers a retry', async () => {
    fail = true;
    renderPage();
    expect(await screen.findByRole('alert')).toBeInTheDocument();
  });
});

describe('#905 — governance entries', () => {
  it('name their object without a noun', () => {
    expect(phraseKeys(ev({ kind: 'defer', domain: 'governance' }))).toEqual({
      verb: 'activity.verb.defer',
      noun: null,
    });
  });
});
