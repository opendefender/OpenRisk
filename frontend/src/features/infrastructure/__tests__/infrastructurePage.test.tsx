// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// Infrastructure (#912): the "Lancer un scan" card starts the picked
// configuration with the picked scan type for an agent one (#1164), scans show
// their type, progress and counts, agents show status, OS,
// version and last seen, and every block has its loading, error and empty
// state. Deleting a configuration and revoking an agent go through a dialog
// and update the list before the server answers.

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { toast } from 'sonner';

import { useUIStore } from '../../../store/uiStore';
import { jobProgress } from '../scannerMeta';
import type { ScanConfig, ScanJob, ScannerAgent } from '../scannerService';

const perms = { write: true };
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (selector: (s: { hasPermission: (p: string) => boolean }) => unknown) =>
    selector({ hasPermission: () => perms.write }),
}));

vi.mock('sonner', () => {
  const fn = Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn(), info: vi.fn() });
  return { toast: fn };
});

// The drawer and the deploy modal have their own flows; here they only need
// to open.
vi.mock('../ScanConfigDrawer', () => ({
  ScanConfigDrawer: ({ provider }: { provider: string }) => (
    <div role="dialog" aria-label={`drawer:${provider}`} />
  ),
}));
vi.mock('../AgentDeployModal', () => ({
  AgentDeployModal: ({ config }: { config: ScanConfig }) => (
    <div role="dialog" aria-label={`deploy:${config.name}`} />
  ),
}));

const get = vi.fn();
const post = vi.fn();
const del = vi.fn();
vi.mock('../../../lib/api', () => ({
  api: {
    get: (...a: unknown[]) => get(...a),
    post: (...a: unknown[]) => post(...a),
    delete: (...a: unknown[]) => del(...a),
    put: vi.fn(),
    patch: vi.fn(),
    defaults: { baseURL: '' },
  },
}));

import { InfrastructurePage } from '../InfrastructurePage';

const now = new Date().toISOString();

function config(over: Partial<ScanConfig>): ScanConfig {
  return {
    id: 'c1',
    tenant_id: 't1',
    name: 'DMZ',
    provider: 'nmap',
    enabled: true,
    regions: null,
    targets: ['10.20.0.0/24'],
    agent_ids: null,
    schedule_minutes: 1440,
    next_run_at: now,
    created_by: 'u1',
    created_at: now,
    updated_at: now,
    ...over,
  };
}

function job(over: Partial<ScanJob>): ScanJob {
  return {
    id: 'aaaaaaaa-0000-0000-0000-000000000001',
    tenant_id: 't1',
    config_id: 'c1',
    provider: 'nmap',
    status: 'completed',
    scan_type: 'vulnerability',
    targets: ['10.20.0.0/24'],
    assets_found: 12,
    findings_found: 7,
    triggered_by: 'u1',
    created_at: now,
    updated_at: now,
    ...over,
  };
}

function agent(over: Partial<ScannerAgent>): ScannerAgent {
  return {
    id: 'g1',
    tenant_id: 't1',
    name: 'core',
    version: '1.8.2',
    status: 'online',
    last_heartbeat: now,
    ip: '10.0.0.1',
    hostname: 'srv-core-01',
    os: 'RHEL 9.4',
    registered_at: now,
    token_rotated_at: now,
    created_at: now,
    updated_at: now,
    ...over,
  };
}

let configs: ScanConfig[];
let jobs: ScanJob[];
let agents: ScannerAgent[];
let failing: Set<string>;

function renderPage() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/infrastructure']}>
        <InfrastructurePage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const section = (name: string) => screen.getByRole('region', { name });

beforeEach(() => {
  useUIStore.setState({ lang: 'fr' });
  perms.write = true;
  failing = new Set();
  configs = [
    config({}),
    config({
      id: 'c2',
      name: 'Prod AWS',
      provider: 'aws',
      targets: null,
      regions: ['eu-west-3'],
      schedule_minutes: 0,
      next_run_at: null,
    }),
  ];
  jobs = [
    job({}),
    job({
      id: 'bbbbbbbb-0000-0000-0000-000000000002',
      config_id: 'c2',
      provider: 'aws',
      status: 'running',
      assets_found: 0,
      findings_found: 0,
    }),
  ];
  agents = [agent({}), agent({ id: 'g2', hostname: 'agence-thies-gw', status: 'offline' })];
  get.mockReset();
  get.mockImplementation((url: string) => {
    if (failing.has(url)) return Promise.reject(new Error('boom'));
    if (url === '/scanner/configs') return Promise.resolve({ data: configs });
    if (url === '/scanner/jobs') return Promise.resolve({ data: jobs });
    if (url === '/scanner/agents') return Promise.resolve({ data: agents });
    return Promise.resolve({ data: {} });
  });
  post.mockReset();
  post.mockImplementation((url: string) =>
    Promise.resolve({
      data: job({
        id: 'cccccccc-0000-0000-0000-000000000003',
        status: 'queued',
        config_id: url.split('/')[3],
      }),
    }),
  );
  del.mockReset();
  del.mockResolvedValue({ data: {} });
  vi.mocked(toast.success).mockClear();
  vi.mocked(toast.error).mockClear();
});

describe('InfrastructurePage', () => {
  it('shows scans with progress and counts, and agents with status, OS, version and last seen', async () => {
    renderPage();
    const scans = section('Scans');
    await within(scans).findByText('7 constats');
    expect(scans).toHaveTextContent('12 actifs');
    const bars = within(scans).getAllByRole('progressbar');
    expect(bars[0]).toHaveAttribute('aria-valuenow', '100');
    // A running job has no percentage to report: the bar is indeterminate.
    expect(bars[1]).not.toHaveAttribute('aria-valuenow');
    // A finished scan reads as its findings, one still going as its status.
    expect(within(scans).getByText('En cours')).toBeInTheDocument();

    const ag = section('Agents');
    expect(within(ag).getByText('srv-core-01')).toBeInTheDocument();
    expect(within(ag).getByText(/RHEL 9\.4 · agent 1\.8\.2 · En ligne/)).toBeInTheDocument();
    expect(within(ag).getByText(/Hors ligne/)).toBeInTheDocument();
    expect(within(ag).getByText('1/2')).toBeInTheDocument();
  });

  it('starts the picked target from the "Lancer un scan" card', async () => {
    const user = userEvent.setup();
    renderPage();
    const card = section('Lancer un scan');
    const target = await within(card).findByRole('combobox', { name: 'Cible' });
    expect(target).toHaveTextContent('DMZ');

    await user.click(target);
    await user.click(screen.getByRole('option', { name: /Prod AWS/ }));
    await user.click(within(card).getByRole('button', { name: 'Démarrer' }));

    await waitFor(() => expect(post).toHaveBeenCalledWith('/scanner/configs/c2/scan'));
    expect(toast.success).toHaveBeenCalledWith('Scan lancé sur Prod AWS');
  });

  it('offers both scan types for an agent target and sends the picked one', async () => {
    const user = userEvent.setup();
    renderPage();
    const card = section('Lancer un scan');
    const type = await within(card).findByRole('combobox', { name: 'Type' });
    // Vulnerability is the default, as on the server.
    expect(type).toHaveTextContent('Vulnérabilités');

    await user.click(type);
    const options = screen.getAllByRole('option').map((o) => o.textContent);
    expect(options).toHaveLength(2);
    expect(options[0]).toMatch(/^Vulnérabilités/);
    expect(options[1]).toMatch(/^Découverte réseau/);
    // No configuration-compliance scan: nothing in the product runs one.
    expect(screen.queryByRole('option', { name: /Conformité/ })).not.toBeInTheDocument();
    await user.click(screen.getByRole('option', { name: /Découverte réseau/ }));
    await user.click(within(card).getByRole('button', { name: 'Démarrer' }));

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith('/scanner/configs/c1/scan', { scan_type: 'discovery' }),
    );
  });

  it('sends the vulnerability type when the agent target keeps the default', async () => {
    const user = userEvent.setup();
    renderPage();
    const card = section('Lancer un scan');
    await within(card).findByRole('combobox', { name: 'Type' });
    await user.click(within(card).getByRole('button', { name: 'Démarrer' }));
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith('/scanner/configs/c1/scan', {
        scan_type: 'vulnerability',
      }),
    );
  });

  it('offers no scan type for a cloud target and sends none', async () => {
    const user = userEvent.setup();
    renderPage();
    const card = section('Lancer un scan');
    await user.click(await within(card).findByRole('combobox', { name: 'Cible' }));
    await user.click(screen.getByRole('option', { name: /Prod AWS/ }));

    expect(within(card).queryByRole('combobox', { name: 'Type' })).not.toBeInTheDocument();
    expect(within(card).getByText('Défini par le fournisseur')).toBeInTheDocument();
    await user.click(within(card).getByRole('button', { name: 'Démarrer' }));
    await waitFor(() => expect(post).toHaveBeenCalledWith('/scanner/configs/c2/scan'));
    expect(post.mock.calls[0]).toHaveLength(1);
  });

  it('shows the type of each agent scan in the list', async () => {
    jobs = [
      job({}),
      job({ id: 'dddddddd-0000-0000-0000-000000000004', scan_type: 'discovery' }),
      job({
        id: 'bbbbbbbb-0000-0000-0000-000000000002',
        config_id: 'c2',
        provider: 'aws',
        status: 'running',
      }),
    ];
    renderPage();
    const scans = section('Scans');
    const rows = await within(scans).findAllByRole('listitem');
    expect(rows[0]).toHaveTextContent(/· Vulnérabilités ·/);
    expect(rows[1]).toHaveTextContent(/· Découverte réseau ·/);
    // A cloud job reads as its provider; it has no scan type of its own.
    expect(rows[2]).not.toHaveTextContent(/Vulnérabilités|Découverte réseau/);
  });

  it('labels the scan types in English', async () => {
    useUIStore.setState({ lang: 'en' });
    jobs = [job({ scan_type: 'discovery' })];
    const user = userEvent.setup();
    renderPage();
    expect(await within(section('Scans')).findByText(/Network discovery/)).toBeInTheDocument();
    await user.click(within(section('Run a scan')).getByRole('combobox', { name: 'Type' }));
    expect(screen.getByRole('option', { name: /Vulnerabilities/ })).toBeInTheDocument();
  });

  it('opens the preview of a completed scan', async () => {
    const user = userEvent.setup();
    renderPage();
    const open = await screen.findByRole('button', { name: /DMZ · Ouvrir l’aperçu du scan/ });
    await user.click(open);
    // The running job is not a link.
    expect(screen.queryByRole('button', { name: /Prod AWS · Ouvrir/ })).not.toBeInTheDocument();
  });

  it('keeps the provider cards, schedule and agent deployment', async () => {
    const user = userEvent.setup();
    renderPage();
    const list = section('Configurations de scan');
    await within(list).findByText('DMZ');
    expect(within(list).getByText('Quotidien')).toBeInTheDocument();

    await user.click(within(list).getByRole('button', { name: 'Déployer un agent' }));
    expect(screen.getByRole('dialog', { name: 'deploy:DMZ' })).toBeInTheDocument();

    const providers = section('Fournisseurs');
    await user.click(within(providers).getByRole('button', { name: /Google Cloud/ }));
    expect(screen.getByRole('dialog', { name: 'drawer:gcp' })).toBeInTheDocument();
  });

  it('deletes a configuration after the dialog, removing the row at once', async () => {
    let release: () => void = () => {};
    del.mockReturnValue(new Promise((r) => (release = () => r({ data: {} }))));
    const user = userEvent.setup();
    renderPage();
    const list = section('Configurations de scan');
    await user.click(await within(list).findByRole('button', { name: 'Supprimer DMZ' }));
    const dialog = screen.getByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: 'Supprimer' }));

    expect(del).toHaveBeenCalledWith('/scanner/configs/c1');
    await waitFor(() => expect(within(list).queryByText('DMZ')).not.toBeInTheDocument());
    configs = configs.filter((c) => c.id !== 'c1');
    release();
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Configuration supprimée'));
  });

  it('revokes an agent after the dialog', async () => {
    const user = userEvent.setup();
    renderPage();
    const ag = section('Agents');
    await user.click(await within(ag).findByRole('button', { name: 'Révoquer srv-core-01' }));
    await user.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Révoquer' }),
    );
    expect(del).toHaveBeenCalledWith('/scanner/agents/g1');
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Agent révoqué'));
  });

  it('says so when a list is empty', async () => {
    configs = [];
    jobs = [];
    agents = [];
    renderPage();
    expect(
      await screen.findByText(/Aucune configuration de scan pour l’instant/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Aucun scan pour l’instant/)).toBeInTheDocument();
    expect(screen.getByText(/Aucun agent connecté/)).toBeInTheDocument();
    expect(screen.getByText('Aucune configuration', { selector: 'strong' })).toBeInTheDocument();
  });

  it('shows an error with a retry per block', async () => {
    failing = new Set(['/scanner/jobs', '/scanner/agents', '/scanner/configs']);
    const user = userEvent.setup();
    renderPage();
    await waitFor(() => expect(screen.getAllByRole('alert')).toHaveLength(4));

    failing = new Set();
    await user.click(within(section('Scans')).getByRole('button', { name: /Réessayer/ }));
    expect(await within(section('Scans')).findByText('7 constats')).toBeInTheDocument();
  });

  it('hides the write actions from a read-only member', async () => {
    perms.write = false;
    renderPage();
    await screen.findByText('srv-core-01');
    expect(screen.queryByRole('region', { name: 'Lancer un scan' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Supprimer/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Révoquer/ })).not.toBeInTheDocument();
  });
});

describe('scannerMeta', () => {
  it('reports progress only where the status gives one', () => {
    expect(jobProgress('queued')).toBe(0);
    expect(jobProgress('running')).toBeNull();
    expect(jobProgress('claimed')).toBeNull();
    expect(jobProgress('completed')).toBe(100);
    expect(jobProgress('failed')).toBe(100);
  });
});
