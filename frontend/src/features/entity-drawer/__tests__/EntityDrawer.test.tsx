// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The drawer's rendering contract (W1-02 §27-§31, §40).
//
// What is asserted here is the honesty of the UI, not its looks:
//
//   - a missing score says so instead of showing 0;
//   - 403, 404 and a network failure are three different screens, and only the
//     last offers a retry;
//   - a permission-denied relation group shows neither items nor a count;
//   - one failing section does not blank the record;
//   - the drawer is a dialog with an accessible name, and Escape closes it.
//
// The server is mocked at the service boundary, which is the only place a
// fixture is allowed to exist (§60): these are unit tests, and nothing here
// ships in a production route.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useUIStore } from '../../../store/uiStore';
import { EntityDrawer } from '../EntityDrawer';
import type { EntityView, RelationGroup, TimelinePage, AuditPage } from '../types';

vi.mock('../entityService', async () => {
  const actual = await vi.importActual<typeof import('../entityService')>('../entityService');
  return {
    ...actual,
    fetchEntity: vi.fn(),
    fetchRelations: vi.fn(),
    fetchTimeline: vi.fn(),
    fetchAudit: vi.fn(),
  };
});

// The auth store supplies the tenant that scopes every cache key.
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (selector: (s: unknown) => unknown) =>
    selector({ user: { tenant_id: 'tenant-a' } }),
}));

import { fetchAudit, fetchEntity, fetchRelations, fetchTimeline } from '../entityService';

const mockEntity = vi.mocked(fetchEntity);
const mockRelations = vi.mocked(fetchRelations);
const mockTimeline = vi.mocked(fetchTimeline);
const mockAudit = vi.mocked(fetchAudit);

function view(overrides: Partial<EntityView> = {}): EntityView {
  return {
    summary: {
      type: 'risk',
      id: '42',
      title: 'Log4Shell exposure',
      subtitle: 'RCE on a public host',
      type_label: 'Risk',
      status: { value: 'open', label: 'Open', tone: 'warning' },
      severity: { value: 'critical', label: 'Critical', tone: 'critical' },
      score: {
        available: true,
        key: 'risk_score',
        label: 'Risk score',
        value: 25.65,
        max: 30,
        tone: 'critical',
        basis: 'Score Engine — probability × impact × asset criticality',
      },
      fields: [{ key: 'source', label: 'Source', value: 'Manual', kind: 'badge' }],
      created_at: '2026-08-01T12:00:00Z',
      updated_at: '2026-08-02T12:00:00Z',
      url: '/risks?drawer=risk&entity=42',
      sections: ['summary', 'relations', 'timeline'],
      ...overrides.summary,
    },
    actions: overrides.actions ?? [],
    sections: overrides.sections ?? ['summary', 'relations', 'timeline'],
  };
}

// The assertions below are written against the ENGLISH copy, so the locale is
// pinned rather than inherited. The app defaults to French (uiStore.legacyLocale),
// and before #411 criterion 17 these strings were hardcoded English regardless of
// locale — which is exactly what this test would otherwise keep enforcing.
beforeEach(() => {
  useUIStore.setState({ lang: 'en' });
});

function renderDrawer(props: Partial<React.ComponentProps<typeof EntityDrawer>> = {}) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const onClose = vi.fn();
  const onOpenEntity = vi.fn();
  const onTabChange = vi.fn();

  render(
    <MemoryRouter>
      <QueryClientProvider client={client}>
        <EntityDrawer
          type="risk"
          id="42"
          tab="summary"
          onTabChange={onTabChange}
          onClose={onClose}
          onOpenEntity={onOpenEntity}
          {...props}
        />
      </QueryClientProvider>
    </MemoryRouter>,
  );
  return { onClose, onOpenEntity, onTabChange };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockRelations.mockResolvedValue([]);
  mockTimeline.mockResolvedValue({ events: [], sources: ['audit'] } as TimelinePage);
  mockAudit.mockResolvedValue({ events: [], total: 0, limit: 25, offset: 0 } as AuditPage);
});

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

describe('summary', () => {
  it('renders the entity the server returned', async () => {
    mockEntity.mockResolvedValue(view());
    renderDrawer();

    expect(await screen.findByText('Log4Shell exposure')).toBeTruthy();
    expect(screen.getByText('Critical')).toBeTruthy();
    expect(screen.getByText('Open')).toBeTruthy();
  });

  it('shows the score the server computed, and names the engine', async () => {
    mockEntity.mockResolvedValue(view());
    renderDrawer();

    expect(await screen.findByText('25.65')).toBeTruthy();
    expect(screen.getByText(/Score Engine/)).toBeTruthy();
  });

  it('says a score is unavailable rather than showing zero', async () => {
    // The single most important assertion in this file. A control has an
    // implementation state, not a number; rendering 0 would read as "no risk".
    mockEntity.mockResolvedValue(
      view({
        summary: {
          ...view().summary,
          score: {
            available: false,
            key: 'coverage',
            label: 'Score',
            value: 0,
            max: 0,
            unavailable: 'a control has an implementation status, not a score',
          },
        },
      }),
    );
    renderDrawer();

    expect(await screen.findByTestId('score-unavailable')).toBeTruthy();
    expect(screen.getByText(/implementation status, not a score/)).toBeTruthy();
    expect(screen.queryByText('0')).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Error, permission and not-found states
// ---------------------------------------------------------------------------

describe('failure states', () => {
  it('shows a permission wall for 403, with no retry', async () => {
    mockEntity.mockRejectedValue({ status: 403, message: 'missing permission risks:read' });
    renderDrawer();

    expect(await screen.findByText(/do not have access/i)).toBeTruthy();
    // A Retry on a permission wall trains people to hammer a button that can
    // never work.
    expect(screen.queryByRole('button', { name: /retry/i })).toBeNull();
  });

  it('shows not-found for 404 without hinting the record exists elsewhere', async () => {
    mockEntity.mockRejectedValue({ status: 404, message: 'risk not found' });
    renderDrawer();

    expect(await screen.findByText('Record not found')).toBeTruthy();
    // The server answers a foreign id and a fabricated one identically on
    // purpose. Copy that distinguished them would undo that.
    expect(screen.queryByText(/another organi/i)).toBeNull();
    expect(screen.queryByText(/other tenant/i)).toBeNull();
  });

  it('offers a retry for a transport failure', async () => {
    mockEntity.mockRejectedValue({ status: 0, message: 'Network Error' });
    renderDrawer();

    expect(await screen.findByText(/could not be loaded/i)).toBeTruthy();
    expect(screen.getByRole('button', { name: /retry/i })).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// Section independence (§27)
// ---------------------------------------------------------------------------

describe('section independence', () => {
  it('keeps the record readable when relations fail', async () => {
    mockEntity.mockResolvedValue(view());
    mockRelations.mockRejectedValue({ status: 500, message: 'boom' });
    renderDrawer({ tab: 'relations' });

    // The header still carries the record.
    expect(await screen.findByText('Log4Shell exposure')).toBeTruthy();
    expect(await screen.findByText(/Relations could not be loaded/i)).toBeTruthy();
  });

  it('does not fetch a section whose tab is closed', async () => {
    mockEntity.mockResolvedValue(view());
    renderDrawer({ tab: 'summary' });

    await screen.findByText('Log4Shell exposure');
    // Opening on Overview must not pay for a relation or timeline query.
    expect(mockRelations).not.toHaveBeenCalled();
    expect(mockTimeline).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Relations
// ---------------------------------------------------------------------------

describe('relations', () => {
  const group = (over: Partial<RelationGroup> = {}): RelationGroup => ({
    key: 'assets',
    label: 'Assets',
    target_type: 'asset',
    items: [
      {
        type: 'asset',
        id: 'a-1',
        title: 'web-prod-01',
        subtitle: 'Server',
        severity: { value: 'critical', label: 'Critical', tone: 'critical' },
        url: '/assets?drawer=asset&entity=a-1',
      },
    ],
    total: 1,
    truncated: false,
    denied: false,
    ...over,
  });

  it('opens a related entity in the same drawer', async () => {
    mockEntity.mockResolvedValue(view());
    mockRelations.mockResolvedValue([group()]);
    const { onOpenEntity } = renderDrawer({ tab: 'relations' });

    const row = await screen.findByRole('button', { name: /web-prod-01/ });
    fireEvent.click(row);
    expect(onOpenEntity).toHaveBeenCalledWith('asset', 'a-1');
  });

  it('shows neither items nor a count for a denied group', async () => {
    // The count alone would tell the caller how many exist.
    mockEntity.mockResolvedValue(view());
    mockRelations.mockResolvedValue([group({ items: [], total: 0, denied: true })]);
    renderDrawer({ tab: 'relations' });

    expect(await screen.findByTestId('relation-denied-assets')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /web-prod-01/ })).toBeNull();
  });

  it('says a list was truncated rather than looking complete', async () => {
    mockEntity.mockResolvedValue(view());
    mockRelations.mockResolvedValue([group({ total: 312, truncated: true })]);
    renderDrawer({ tab: 'relations' });

    expect(await screen.findByText('1 of 312')).toBeTruthy();
  });

  it('shows an empty state rather than an empty list', async () => {
    mockEntity.mockResolvedValue(view());
    mockRelations.mockResolvedValue([]);
    renderDrawer({ tab: 'relations' });

    expect(await screen.findByTestId('empty-state')).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// Timeline
// ---------------------------------------------------------------------------

describe('timeline', () => {
  it('renders real events with their actor and journal', async () => {
    mockEntity.mockResolvedValue(view());
    mockTimeline.mockResolvedValue({
      events: [
        {
          id: 'e1',
          kind: 'update',
          occurred_at: '2026-08-02T12:00:00Z',
          actor: { id: 'u1', email: 'alice@example.com' },
          target: { type: 'risk', id: '42' },
          summary: 'Updated risk',
          changes: [{ field: 'criticality' }],
          source: 'audit',
        },
      ],
      sources: ['audit'],
    } as TimelinePage);
    renderDrawer({ tab: 'timeline' });

    expect(await screen.findByText('Updated risk')).toBeTruthy();
    expect(screen.getByText('alice@example.com')).toBeTruthy();
    expect(screen.getByText('Audit trail')).toBeTruthy();
    expect(screen.getByText('criticality')).toBeTruthy();
  });

  it('attributes a worker-made change to the system, not to a blank', async () => {
    mockEntity.mockResolvedValue(view());
    mockTimeline.mockResolvedValue({
      events: [
        {
          id: 'h1',
          kind: 'update',
          occurred_at: '2026-08-02T12:00:00Z',
          target: { type: 'risk', id: '42' },
          summary: 'Score recorded at 25.65',
          source: 'risk_history',
        },
      ],
      sources: ['risk_history'],
    } as TimelinePage);
    renderDrawer({ tab: 'timeline' });

    expect(await screen.findByText('Score recorded at 25.65')).toBeTruthy();
    expect(screen.getByText('System')).toBeTruthy();
    expect(screen.getByText('Score engine')).toBeTruthy();
  });

  it('shows an empty state instead of inventing "updated 2 hours ago"', async () => {
    mockEntity.mockResolvedValue(view());
    renderDrawer({ tab: 'timeline' });

    expect(await screen.findByText('No activity recorded')).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// Tabs and sections
// ---------------------------------------------------------------------------

describe('sections', () => {
  it('renders only the sections the server offered', async () => {
    mockEntity.mockResolvedValue(view({ sections: ['summary', 'timeline'] }));
    renderDrawer();

    await screen.findByText('Log4Shell exposure');
    expect(screen.getByRole('tab', { name: 'Overview' })).toBeTruthy();
    expect(screen.getByRole('tab', { name: 'Timeline' })).toBeTruthy();
    // No audit tab: this caller does not hold the audit permission, so a tab
    // would always answer 403.
    expect(screen.queryByRole('tab', { name: 'Audit' })).toBeNull();
  });

  it('falls back to the first offered section when the URL names a missing one', async () => {
    // An old link, or a caller who has since lost the audit permission.
    mockEntity.mockResolvedValue(view({ sections: ['summary', 'timeline'] }));
    renderDrawer({ tab: 'audit' });

    await screen.findByText('Log4Shell exposure');
    expect(screen.getByRole('tab', { name: 'Overview' }).getAttribute('aria-selected')).toBe(
      'true',
    );
  });
});

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

describe('actions', () => {
  it('renders nothing when the caller may do nothing', async () => {
    mockEntity.mockResolvedValue(view({ actions: [] }));
    renderDrawer();

    await screen.findByText('Log4Shell exposure');
    expect(screen.queryByRole('button', { name: 'Edit' })).toBeNull();
  });

  it('renders the permitted primary action', async () => {
    mockEntity.mockResolvedValue(
      view({
        actions: [
          {
            key: 'edit',
            label: 'Edit',
            kind: 'primary',
            method: 'PATCH',
            path: '/api/v1/risks/42',
            permission: 'risks:update',
          },
        ],
      }),
    );
    renderDrawer();

    expect(await screen.findByRole('button', { name: 'Edit' })).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// Accessibility (§40)
// ---------------------------------------------------------------------------

describe('accessibility', () => {
  it('is a dialog with an accessible name', async () => {
    mockEntity.mockResolvedValue(view());
    renderDrawer();

    const dialog = await screen.findByRole('dialog');
    expect(dialog.getAttribute('aria-modal')).toBe('true');
    await waitFor(() => expect(dialog.textContent).toContain('Log4Shell exposure'));
  });

  it('closes on Escape', async () => {
    mockEntity.mockResolvedValue(view());
    const { onClose } = renderDrawer();

    await screen.findByText('Log4Shell exposure');
    fireEvent.keyDown(document, { key: 'Escape' });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('announces the record so a relation click is not silent', async () => {
    mockEntity.mockResolvedValue(view());
    renderDrawer();

    await waitFor(() => {
      const live = document.querySelector('[aria-live="polite"]');
      expect(live?.textContent).toBe('Risk: Log4Shell exposure');
    });
  });
});

// ---------------------------------------------------------------------------
// Cache isolation (§32)
// ---------------------------------------------------------------------------

describe('cache keys', () => {
  it('scopes every query by tenant', async () => {
    const { entityKey } = await import('../useEntityDrawer');
    const a = entityKey('tenant-a', 'risk', '42');
    const b = entityKey('tenant-b', 'risk', '42');
    // Same entity, same id, different tenant: the keys must not collide, or a
    // cached answer could outlive the identity that fetched it.
    expect(a).not.toEqual(b);
    expect(a).toContain('tenant-a');
  });
});

// ---------------------------------------------------------------------------
// Localisation (#411 criterion 17)
// ---------------------------------------------------------------------------

describe('localisation', () => {
  // Proving the strings RESOLVE, not merely that keys exist. Before this, the
  // drawer rendered English to a French user whatever the locale said, and a
  // test asserting the English copy passed happily.
  it('renders its failure copy in French when the locale is French', async () => {
    useUIStore.setState({ lang: 'fr' });
    mockEntity.mockRejectedValue({ status: 404, message: 'risk not found' });

    renderDrawer();

    expect(await screen.findByText('Enregistrement introuvable')).toBeInTheDocument();
    expect(screen.queryByText('Record not found')).not.toBeInTheDocument();
  });

  it('renders its tabs in French when the locale is French', async () => {
    useUIStore.setState({ lang: 'fr' });
    mockEntity.mockResolvedValue(view({ sections: ['summary', 'relations'] }));

    renderDrawer();

    expect(await screen.findByRole('tab', { name: 'Aperçu' })).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Overview' })).not.toBeInTheDocument();
  });
});
