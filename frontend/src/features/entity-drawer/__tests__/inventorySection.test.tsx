// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #910: the asset drawer's software inventory, in its three states, with each
// match error worded rather than hidden.

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { useUIStore } from '../../../store/uiStore';
import type { HostInventory } from '../../vulnerabilities/vulnerabilityService';

const get = vi.fn();
vi.mock('../../../lib/api', () => ({
  api: { get: (...a: unknown[]) => get(...a), defaults: { baseURL: '' } },
}));

import { InventorySection } from '../sections/InventorySection';

function inv(over: Partial<HostInventory>): HostInventory {
  return {
    id: 'i1',
    agent_id: 'a1',
    host_key: 'srv-core-01',
    hostname: 'srv-core-01',
    asset_id: 'x',
    platform: 'ubuntu',
    os: 'Ubuntu',
    os_version: '22.04.3 LTS',
    ecosystem: 'Ubuntu:22.04:LTS',
    package_count: 1243,
    uncheckable_count: 0,
    collected_at: '2026-10-14T06:00:00Z',
    matched_at: '2026-10-14T06:02:00Z',
    match_error: '',
    ...over,
  };
}

function renderSection() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <InventorySection assetId="x" enabled />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useUIStore.setState({ lang: 'fr' });
  get.mockReset();
});

describe('InventorySection', () => {
  it('shows each host: OS, packages, when collected and checked', async () => {
    get.mockResolvedValue({ data: { inventories: [inv({ uncheckable_count: 12 })] } });
    renderSection();
    const line = await screen.findByTestId('inventory-srv-core-01');
    expect(line).toHaveTextContent('Ubuntu 22.04.3 LTS');
    expect(line).toHaveTextContent(/1\s?243 paquets · 12 non vérifiables/);
    expect(line).toHaveTextContent('vérifié le');
    expect(get).toHaveBeenCalledWith('/assets/x/inventories');
  });

  it('words every match error', async () => {
    get.mockResolvedValue({
      data: {
        inventories: [
          inv({ id: 'a', host_key: 'h1', match_error: 'osquery_missing', package_count: 0 }),
          inv({ id: 'b', host_key: 'h2', match_error: 'source_unreachable' }),
          inv({ id: 'c', host_key: 'h3', match_error: 'lookup_disabled' }),
        ],
      },
    });
    renderSection();
    expect(await screen.findByText(/osquery requis/)).toBeInTheDocument();
    expect(screen.getByText(/injoignable/)).toBeInTheDocument();
    expect(screen.getByText(/Recherche en ligne désactivée/)).toBeInTheDocument();
  });

  it('says why there is no inventory', async () => {
    get.mockResolvedValue({ data: { inventories: [] } });
    renderSection();
    expect(await screen.findByText(/Aucun inventaire/)).toBeInTheDocument();
  });

  it('offers a retry when the read fails', async () => {
    get.mockRejectedValue(new Error('down'));
    renderSection();
    expect(await screen.findByRole('alert')).toBeInTheDocument();
  });
});
