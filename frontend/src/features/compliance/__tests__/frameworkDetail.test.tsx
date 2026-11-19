// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// FrameworkDetail header gauge (#307). The percentage is derived live from the
// loaded controls (not_applicable excluded from the denominator), never from
// the overview snapshot. The HTTP client is mocked at the boundary.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { AxiosResponse } from 'axios';

import { useUIStore } from '../../../store/uiStore';
import type { ComplianceControl, ControlStatus } from '../../../types/compliance';
import { FrameworkDetail } from '../FrameworkDetail';

vi.mock('../../../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../../../lib/api')>('../../../lib/api');
  return { ...actual, api: { ...actual.api, get: vi.fn() } };
});

import { api } from '../../../lib/api';

const get = vi.mocked(api.get);

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

// The overview snapshot deliberately reports a stale 100%: the header must
// ignore it once the controls have loaded.
function serve(controls: ComplianceControl[]) {
  get.mockImplementation((url: string) => {
    if (url === '/compliance/frameworks') {
      return ok([{ id: FRAMEWORK_ID, name: 'ISO 27001', version: '2022', description: 'Desc' }]);
    }
    if (url === `/compliance/frameworks/${FRAMEWORK_ID}/progress`) {
      return ok({ Total: 99, Applicable: 99, PercentComplete: 100, ByStatus: { implemented: 99 } });
    }
    if (url === `/compliance/frameworks/${FRAMEWORK_ID}/controls`) {
      return ok(controls);
    }
    return Promise.reject(new Error(`unexpected GET ${url}`));
  });
}

function renderDetail() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
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
  useUIStore.setState({ lang: 'en' });
});

describe('FrameworkDetail header progress', () => {
  it('shows 0% when every loaded control is not_implemented', async () => {
    serve([
      control(1, 'not_implemented'),
      control(2, 'not_implemented'),
      control(3, 'not_implemented'),
    ]);
    renderDetail();

    expect(await screen.findByText('Control 3')).toBeInTheDocument();
    expect(screen.getByText('0%')).toBeInTheDocument();
    expect(screen.getByText('0/3 controls')).toBeInTheDocument();
  });

  it('shows 50% for 1 implemented + 1 not_applicable + 1 not_implemented', async () => {
    serve([control(1, 'implemented'), control(2, 'not_applicable'), control(3, 'not_implemented')]);
    renderDetail();

    expect(await screen.findByText('Control 3')).toBeInTheDocument();
    expect(screen.getByText('50%')).toBeInTheDocument();
    expect(screen.getByText('1/3 controls')).toBeInTheDocument();
  });
});
