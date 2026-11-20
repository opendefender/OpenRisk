// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// FrameworkDetail header (#307). The gauge comes from GET .../progress
// (criterion 9), never from the loaded controls or the overview snapshot; a
// status change patches that cache optimistically. Also the JSON export button
// and an axe pass. The HTTP client is mocked at the boundary.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import axe from 'axe-core';
import { MemoryRouter, Route, Routes } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { AxiosResponse } from 'axios';

import { toast } from 'sonner';
import { useAuthStore } from '../../../hooks/useAuthStore';
import { useUIStore } from '../../../store/uiStore';
import type {
  ComplianceControl,
  ControlStatus,
  FrameworkProgress,
} from '../../../types/compliance';
import { FrameworkDetail } from '../FrameworkDetail';
import { moveProgress } from '../useCompliance';

vi.mock('../../../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../../../lib/api')>('../../../lib/api');
  return { ...actual, api: { ...actual.api, get: vi.fn(), patch: vi.fn() } };
});

import { api } from '../../../lib/api';

const get = vi.mocked(api.get);
const patch = vi.mocked(api.patch);

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    // Settle like sonner does so the localized messages can be asserted.
    promise: vi.fn(
      (p: Promise<unknown>, m: { success: string; error: string }) =>
        void p.then(
          () => toast.success(m.success),
          () => toast.error(m.error),
        ),
    ),
  },
}));

const FRAMEWORK_ID = 'fw-1';

function control(n: number, status: ControlStatus): ComplianceControl {
  return {
    id: `c-${n}`,
    framework_id: FRAMEWORK_ID,
    reference_code: `A.${n}`,
    name: `Control ${n}`,
    status,
    evidence_count: 0,
  };
}

function ok<T>(data: T): Promise<AxiosResponse<T>> {
  return Promise.resolve({ data } as AxiosResponse<T>);
}

// The overview snapshot and the controls deliberately disagree with /progress:
// the header must show what /progress says. `progress` is the wire body (Go
// field names, as the live server sends them) or an Error to simulate a failure.
const ZERO = { Total: 3, ByStatus: { not_implemented: 3 }, Applicable: 3, PercentComplete: 0 };

function serve(
  controls: ComplianceControl[],
  progress: unknown = ZERO,
  extra: Record<string, unknown> = {},
) {
  get.mockImplementation((url: string) => {
    if (url === '/compliance/frameworks') {
      return ok([
        { id: FRAMEWORK_ID, name: 'ISO 27001', version: '2022', description: 'Desc', ...extra },
      ]);
    }
    if (url === `/compliance/frameworks/${FRAMEWORK_ID}/progress`) {
      return progress instanceof Error ? Promise.reject(progress) : ok(progress);
    }
    if (url === `/compliance/frameworks/${FRAMEWORK_ID}/controls`) {
      return ok(controls);
    }
    return Promise.reject(new Error(`unexpected GET ${url}`));
  });
}

function renderDetail() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/compliance/frameworks/${FRAMEWORK_ID}`]}>
        <Routes>
          <Route path="/compliance/frameworks/:frameworkId" element={<FrameworkDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  get.mockReset();
  patch.mockReset();
  vi.mocked(toast.success).mockReset();
  vi.mocked(toast.error).mockReset();
  useAuthStore.setState({ hasPermission: () => true });
  useUIStore.setState({ lang: 'en' });
});

describe('FrameworkDetail header progress', () => {
  it('shows what /progress says (0%) even when the loaded controls would compute 50%', async () => {
    serve([control(1, 'implemented'), control(2, 'not_applicable'), control(3, 'not_implemented')]);
    renderDetail();

    expect(await screen.findByText('Control 3')).toBeInTheDocument();
    expect(await screen.findByText('0%')).toBeInTheDocument();
    expect(screen.queryByText('50%')).toBeNull();
    expect(screen.getByText('0/3 controls')).toBeInTheDocument();
  });

  it('shows the /progress percentage when it is not zero', async () => {
    serve([control(1, 'not_implemented')], {
      Total: 4,
      ByStatus: { implemented: 1, not_implemented: 3 },
      Applicable: 4,
      PercentComplete: 25,
    });
    renderDetail();

    expect(await screen.findByText('25%')).toBeInTheDocument();
    expect(screen.getByText('1/4 controls')).toBeInTheDocument();
  });

  it('also reads the documented snake_case body', async () => {
    serve([control(1, 'not_implemented')], {
      framework_id: FRAMEWORK_ID,
      total: 2,
      by_status: { implemented: 1, not_implemented: 1 },
      applicable: 2,
      percent_complete: 50,
    });
    renderDetail();

    expect(await screen.findByText('50%')).toBeInTheDocument();
    expect(screen.getByText('1/2 controls')).toBeInTheDocument();
  });

  it('shows a skeleton while /progress is pending, not a number', async () => {
    serve([control(1, 'not_implemented')]);
    const base = get.getMockImplementation() as (u: string) => Promise<AxiosResponse>;
    // The header's own /progress request is the first one; the overview's
    // (which lets the card render) is the second and resolves.
    let progressCalls = 0;
    get.mockImplementation((url: string) =>
      url.endsWith('/progress') && progressCalls++ === 0 ? new Promise(() => undefined) : base(url),
    );
    renderDetail();

    expect(await screen.findByTestId('framework-progress-skeleton')).toBeInTheDocument();
    expect(screen.queryByText('0%')).toBeNull();
  });

  it('falls back to an honest dash with a hint when /progress fails', async () => {
    serve([control(1, 'implemented')], new Error('boom'));
    renderDetail();

    const fallback = await screen.findByTestId('framework-progress-error');
    expect(fallback).toHaveTextContent('Progress unavailable');
    expect(screen.queryByText(/%$/)).toBeNull();
    expect(screen.getByText('— (progress unavailable)')).toBeInTheDocument();
  });

  it('patches the progress cache optimistically on a status change, then settles on the server', async () => {
    serve([control(1, 'not_implemented'), control(2, 'not_implemented')], {
      Total: 2,
      ByStatus: { not_implemented: 2 },
      Applicable: 2,
      PercentComplete: 0,
    });
    // Evidence is required to implement: use a control that has some.
    const withEvidence = { ...control(1, 'not_implemented'), evidence_count: 1 };
    serve([withEvidence, control(2, 'not_implemented')], {
      Total: 2,
      ByStatus: { not_implemented: 2 },
      Applicable: 2,
      PercentComplete: 0,
    });
    let release: (v: AxiosResponse) => void = () => undefined;
    patch.mockReturnValue(new Promise<AxiosResponse>((r) => (release = r)));

    renderDetail();
    expect(await screen.findByText('Control 1')).toBeInTheDocument();
    expect(await screen.findByText('0%')).toBeInTheDocument();

    const selects = screen.getAllByRole('combobox');
    fireEvent.change(selects[0], { target: { value: 'implemented' } });

    // Optimistic: the request is still pending and the gauge already moved.
    expect(await screen.findByText('50%')).toBeInTheDocument();
    expect(screen.getByText('1/2 controls')).toBeInTheDocument();
    expect(patch).toHaveBeenCalledWith('/compliance/controls/c-1', { status: 'implemented' });

    release({ data: {} } as AxiosResponse);
    // Settled: the progress key is refetched (now served the real figure).
    await waitFor(() => {
      const progressCalls = get.mock.calls.filter((c) => String(c[0]).endsWith('/progress'));
      expect(progressCalls.length).toBeGreaterThan(1);
    });
  });

  it('rolls the progress cache back when the status change fails', async () => {
    const withEvidence = { ...control(1, 'not_implemented'), evidence_count: 1 };
    serve([withEvidence, control(2, 'not_implemented')], {
      Total: 2,
      ByStatus: { not_implemented: 2 },
      Applicable: 2,
      PercentComplete: 0,
    });
    // After the two initial /progress reads (header + overview) any refetch
    // never answers: the gauge can only return to 0% from the restored cache.
    const base = get.getMockImplementation() as (u: string) => Promise<AxiosResponse>;
    let progressCalls = 0;
    get.mockImplementation((url: string) =>
      url.endsWith('/progress') && progressCalls++ >= 2 ? new Promise(() => undefined) : base(url),
    );
    patch.mockRejectedValue(new Error('500'));

    renderDetail();
    expect(await screen.findByText('Control 1')).toBeInTheDocument();
    expect(await screen.findByText('0%')).toBeInTheDocument();

    fireEvent.change(screen.getAllByRole('combobox')[0], { target: { value: 'implemented' } });

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Update failed'));
    await waitFor(() => expect(screen.getByText('0%')).toBeInTheDocument());
    expect(screen.queryByText('50%')).toBeNull();
    expect(screen.getByText('0/2 controls')).toBeInTheDocument();
    // The refetch was requested (invalidate on settle) and is still pending.
    await waitFor(() => expect(progressCalls).toBeGreaterThan(2));
  });
});

describe('moveProgress', () => {
  const base: FrameworkProgress = {
    total: 4,
    applicable: 4,
    byStatus: { not_implemented: 3, implemented: 1 },
    percentComplete: 25,
  };

  it('moves one control between statuses and recomputes the percentage', () => {
    const next = moveProgress(base, 'not_implemented', 'implemented');
    expect(next.byStatus).toEqual({ not_implemented: 2, implemented: 2 });
    expect(next.percentComplete).toBe(50);
    expect(base.byStatus.not_implemented).toBe(3);
  });

  it('shrinks applicable when moving to not_applicable', () => {
    const next = moveProgress(base, 'not_implemented', 'not_applicable');
    expect(next.applicable).toBe(3);
    expect(next.percentComplete).toBeCloseTo(100 / 3);
  });

  it('grows applicable when moving away from not_applicable', () => {
    const start: FrameworkProgress = {
      total: 2,
      applicable: 1,
      byStatus: { not_applicable: 1, implemented: 1 },
      percentComplete: 100,
    };
    const next = moveProgress(start, 'not_applicable', 'not_implemented');
    expect(next.applicable).toBe(2);
    expect(next.percentComplete).toBe(50);
  });

  it('returns 0% without dividing by zero when nothing is applicable', () => {
    const start: FrameworkProgress = {
      total: 1,
      applicable: 1,
      byStatus: { implemented: 1 },
      percentComplete: 100,
    };
    const next = moveProgress(start, 'implemented', 'not_applicable');
    expect(next.applicable).toBe(0);
    expect(next.percentComplete).toBe(0);
    expect(Number.isFinite(next.percentComplete)).toBe(true);
  });

  it('clamps the source count at 0 when the cache is out of date', () => {
    const next = moveProgress(base, 'in_progress', 'implemented');
    expect(next.byStatus.in_progress).toBe(0);
    expect(next.byStatus.implemented).toBe(2);
  });

  it('is a no-op when the status does not change', () => {
    const next = moveProgress(base, 'implemented', 'implemented');
    expect(next).toEqual(base);
  });
});

describe('FrameworkDetail JSON export button', () => {
  function exportCalls() {
    return get.mock.calls.filter((c) => String(c[0]).endsWith('/export'));
  }

  it('requests the export of this framework and triggers the download', async () => {
    serve([control(1, 'not_implemented')]);
    const base = get.getMockImplementation();
    get.mockImplementation((url: string, config?: unknown) =>
      url === `/compliance/frameworks/${FRAMEWORK_ID}/export`
        ? Promise.resolve({
            data: new Blob(['{}']),
            headers: { 'content-disposition': 'attachment; filename="iso-27001.json"' },
          } as unknown as AxiosResponse)
        : (base as (u: string, c?: unknown) => Promise<AxiosResponse>)(url, config),
    );
    const createObjectURL = vi.fn(() => 'blob:x');
    const revokeObjectURL = vi.fn();
    Object.assign(URL, { createObjectURL, revokeObjectURL });
    let downloaded = '';
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
      this: HTMLAnchorElement,
    ) {
      downloaded = this.download;
    });

    renderDetail();
    fireEvent.click(await screen.findByRole('button', { name: 'Export (JSON)' }));

    await waitFor(() => expect(click).toHaveBeenCalledTimes(1));
    expect(exportCalls()).toHaveLength(1);
    expect(get).toHaveBeenCalledWith(`/compliance/frameworks/${FRAMEWORK_ID}/export`, {
      responseType: 'blob',
    });
    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(downloaded).toBe('iso-27001.json');
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Framework exported'));
    click.mockRestore();
  });

  it('shows the localized error toast when the export fails', async () => {
    serve([control(1, 'not_implemented')]);
    const base = get.getMockImplementation();
    get.mockImplementation((url: string, config?: unknown) =>
      url.endsWith('/export')
        ? Promise.reject(new Error('500'))
        : (base as (u: string, c?: unknown) => Promise<AxiosResponse>)(url, config),
    );

    renderDetail();
    fireEvent.click(await screen.findByRole('button', { name: 'Export (JSON)' }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Failed to export the framework'));
    expect(toast.success).not.toHaveBeenCalled();
  });

  it('uses the French label and error message in French', async () => {
    useUIStore.setState({ lang: 'fr' });
    serve([control(1, 'not_implemented')]);
    const base = get.getMockImplementation();
    get.mockImplementation((url: string, config?: unknown) =>
      url.endsWith('/export')
        ? Promise.reject(new Error('500'))
        : (base as (u: string, c?: unknown) => Promise<AxiosResponse>)(url, config),
    );

    renderDetail();
    fireEvent.click(await screen.findByRole('button', { name: 'Exporter (JSON)' }));

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith("Échec de l'export du référentiel"),
    );
  });
});

describe('FrameworkDetail accessibility', () => {
  it('has no serious or critical axe violations (header, Export (JSON), controls)', async () => {
    serve([control(1, 'not_implemented'), control(2, 'implemented')]);
    const { container } = renderDetail();
    expect(await screen.findByText('Control 2')).toBeInTheDocument();
    expect(await screen.findByText('0%')).toBeInTheDocument();

    const results = await axe.run(container, {
      rules: { 'color-contrast': { enabled: false }, region: { enabled: false } },
    });
    const serious = results.violations.filter(
      (v) => v.impact === 'serious' || v.impact === 'critical',
    );
    expect(serious.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  });
});

describe('FrameworkDetail catalogue version and revision (#1478)', () => {
  const chip = () => screen.findByTestId('framework-catalog-version');

  it('shows version and revision, hash in monospace', async () => {
    serve([control(1, 'not_implemented')], ZERO, {
      catalog_key: 'iso27001-2022',
      catalog_version: '2022',
      catalog_revision: 'a1b2c3d4e5f6',
    });
    renderDetail();
    const el = await chip();
    expect(el).toHaveTextContent('Catalogue version: 2022 · revision a1b2c3d4e5f6');
    expect(el.querySelector('.mono')).toHaveTextContent('a1b2c3d4e5f6');
    expect(el).not.toHaveAttribute('title');
    // The chip itself is the focus target: no inert help button beside it.
    expect(screen.queryByRole('button', { name: /catalogue revision/i })).toBeNull();
    el.focus();
    expect(el).toHaveFocus();
    fireEvent.focus(el);
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Fingerprint of the catalogue');
    expect(el).toHaveAccessibleDescription(expect.stringContaining('Fingerprint'));
  });

  it('shows revision unknown when only the revision is empty', async () => {
    serve([control(1, 'not_implemented')], ZERO, {
      catalog_key: 'iso27001-2022',
      catalog_version: '2022',
      catalog_revision: '',
    });
    renderDetail();
    expect(await chip()).toHaveTextContent('Catalogue version: 2022 · revision unknown');
  });

  it('shows both unknown on a pre-#1478 row', async () => {
    serve([control(1, 'not_implemented')], ZERO, { catalog_key: 'iso27001-2022' });
    renderDetail();
    expect(await chip()).toHaveTextContent('Catalogue version: Unknown · revision unknown');
  });

  it('shows the French variant', async () => {
    useUIStore.setState({ lang: 'fr' });
    serve([control(1, 'not_implemented')], ZERO, {
      catalog_key: 'soc2',
      catalog_version: '',
      catalog_revision: 'a1b2c3d4e5f6',
    });
    renderDetail();
    expect(await chip()).toHaveTextContent(
      'Version du catalogue : Inconnue · révision a1b2c3d4e5f6',
    );
  });

  it('shows nothing about a catalogue on a hand-built framework', async () => {
    serve([control(1, 'not_implemented')], ZERO, {
      catalog_key: '',
      catalog_version: '',
      catalog_revision: '',
    });
    renderDetail();
    expect(await screen.findByText('Control 1')).toBeInTheDocument();
    expect(screen.queryByTestId('framework-catalog-version')).toBeNull();
    expect(screen.queryByText(/catalogue|revision/i)).toBeNull();
  });
});
