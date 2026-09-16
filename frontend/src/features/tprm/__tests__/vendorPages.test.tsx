// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Vendor register, vendor page and questionnaire review (#673).

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AxiosError, type AxiosResponse } from 'axios';
import axe from 'axe-core';
import { toast } from 'sonner';

import { useUIStore } from '../../../store/uiStore';

const auth = { manage: true, createAsset: true };
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (selector: (s: { hasPermission: (p: string) => boolean }) => unknown) =>
    selector({
      hasPermission: (p: string) => {
        if (p === 'vendors:manage') return auth.manage;
        if (p === 'assets:create') return auth.createAsset;
        return true;
      },
    }),
}));

const entitlement = { enabled: true };
vi.mock('../../billing/useEntitlements', async () => {
  const actual = await vi.importActual<typeof import('../../billing/useEntitlements')>(
    '../../billing/useEntitlements',
  );
  return {
    ...actual,
    useFeature: () => ({ enabled: entitlement.enabled, requiredPlan: 'business', loading: false }),
  };
});

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

// The asset form has its own tests; here it only has to open.
vi.mock('../../assets/CreateAssetModal', () => ({
  CreateAssetModal: ({ isOpen, initialType }: { isOpen: boolean; initialType?: string }) =>
    isOpen ? <div role="dialog" aria-label={`create-asset-${initialType ?? ''}`} /> : null,
}));

vi.mock('../vendorService', async () => {
  const actual = await vi.importActual<typeof import('../vendorService')>('../vendorService');
  return {
    ...actual,
    vendorService: {
      list: vi.fn(),
      chain: vi.fn(),
      link: vi.fn(),
      unlink: vi.fn(),
      listAssessments: vi.fn(),
      getAssessment: vi.fn(),
      send: vi.fn(),
      revoke: vi.fn(),
      resend: vi.fn(),
    },
  };
});

vi.mock('../questionnaireTemplateService', async () => {
  const actual = await vi.importActual<typeof import('../questionnaireTemplateService')>(
    '../questionnaireTemplateService',
  );
  return {
    ...actual,
    questionnaireTemplateService: {
      list: vi.fn(),
      get: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      archive: vi.fn(),
    },
  };
});

vi.mock('../../../services/assetService', async () => {
  const actual = await vi.importActual<typeof import('../../../services/assetService')>(
    '../../../services/assetService',
  );
  return {
    ...actual,
    assetService: { ...actual.assetService, listAssets: vi.fn(), getAsset: vi.fn() },
  };
});

import {
  vendorService,
  type VendorAssessment,
  type VendorChain,
  type VendorRegisterEntry,
} from '../vendorService';
import {
  questionnaireTemplateService,
  type QuestionnaireTemplate,
} from '../questionnaireTemplateService';
import { assetService } from '../../../services/assetService';
import type { Asset } from '../../../types/asset';
import { VendorsPage } from '../VendorsPage';
import { VendorDetailPage } from '../VendorDetailPage';
import { VendorAssessmentPage } from '../VendorAssessmentPage';

const svc = vi.mocked(vendorService);
const tplSvc = vi.mocked(questionnaireTemplateService);
const assetSvc = vi.mocked(assetService);

function httpError(status: number, message = 'refused'): AxiosError {
  const response = {
    status,
    statusText: '',
    headers: {},
    config: {},
    data: { error: message },
  } as AxiosResponse;
  return new AxiosError(message, String(status), undefined, undefined, response);
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function entry(overrides: Partial<VendorRegisterEntry> = {}): VendorRegisterEntry {
  return {
    id: 'v-1',
    name: 'Acme Cloud',
    legal_name: 'Acme Cloud SAS',
    country: 'FR',
    service_provided: 'Hébergement',
    service_criticality: 'haute',
    contract_end: '2027-01-31',
    criticality: 'HIGH',
    owner: '',
    linked_assets: 2,
    latest_assessment: {
      id: 'as-1',
      status: 'submitted',
      score: 42.5,
      tier: 'high',
      due_at: '2026-10-01T21:59:59Z',
    },
    ...overrides,
  };
}

function chain(): VendorChain {
  return {
    vendor_id: 'v-1',
    vendor_name: 'Acme Cloud',
    links: [
      {
        link_id: 'l-1',
        verb: 'hosted_by',
        asset: {
          id: 'a-1',
          name: 'ERP',
          type: 'Application',
          category: 'application',
          criticality: 'HIGH',
        },
        risks: [
          {
            id: 'r-1',
            title: 'Fuite de données ERP',
            score: 6.3,
            criticality: 'high',
            status: 'open',
          },
        ],
      },
    ],
  };
}

function assessment(overrides: Partial<VendorAssessment> = {}): VendorAssessment {
  return {
    id: 'as-1',
    vendor_asset_id: 'v-1',
    template_id: 't-1',
    template_version: 2,
    status: 'sent',
    contact_email: 'secu@acme.test',
    contact_language: 'fr',
    due_at: '2026-10-01T21:59:59Z',
    sent_at: '2026-09-10T08:00:00Z',
    score: null,
    tier: null,
    confidence: 0.5,
    source: 'vendor_questionnaire',
    ...overrides,
  };
}

function template(): QuestionnaireTemplate {
  return {
    id: 't-1',
    name: 'Baseline sécurité',
    description: '',
    language: 'fr',
    version: 3,
    archived_at: null,
    created_at: '2026-03-01T09:00:00Z',
    updated_at: '2026-03-02T09:00:00Z',
  };
}

const ASSETS = [
  { id: 'a-1', name: 'ERP', type: 'Application', category: 'application', criticality: 'HIGH' },
  { id: 'a-2', name: 'CRM', type: 'SaaS', category: 'application', criticality: 'MEDIUM' },
  {
    id: 'v-2',
    name: 'Autre fournisseur',
    type: 'Supplier',
    category: 'vendor',
    criticality: 'LOW',
  },
] as Asset[];

function renderAt(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/vendors" element={<VendorsPage />} />
          <Route path="/vendors/:vendorId" element={<VendorDetailPage />} />
          <Route
            path="/vendors/:vendorId/assessments/:assessmentId"
            element={<VendorAssessmentPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

async function expectNoSeriousViolations(container: HTMLElement) {
  const results = await axe.run(container, {
    rules: { 'color-contrast': { enabled: false }, region: { enabled: false } },
  });
  const serious = results.violations.filter(
    (v) => v.impact === 'serious' || v.impact === 'critical',
  );
  expect(serious.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
}

beforeEach(() => {
  auth.manage = true;
  auth.createAsset = true;
  entitlement.enabled = true;
  useUIStore.getState().setLang('fr');
  for (const fn of Object.values(svc)) fn.mockReset();
  for (const fn of Object.values(tplSvc)) fn.mockReset();
  assetSvc.listAssets.mockReset();
  assetSvc.getAsset.mockReset();
  vi.mocked(toast.success).mockReset();
  vi.mocked(toast.error).mockReset();

  tplSvc.list.mockResolvedValue([template()]);
  svc.chain.mockResolvedValue(chain());
  svc.listAssessments.mockResolvedValue([assessment()]);
  assetSvc.listAssets.mockResolvedValue(ASSETS);
  assetSvc.getAsset.mockResolvedValue({
    id: 'v-1',
    name: 'Acme Cloud',
    attributes: { contact_email: 'secu@acme.test', legal_name: 'Acme Cloud SAS' },
  } as Asset);
});

/* ---------------------------------------------------------------- register */

describe('VendorsPage', () => {
  it('lists vendors with their latest questionnaire in words', async () => {
    svc.list.mockResolvedValue({ items: [entry()], total: 1, limit: 50, offset: 0 });
    renderAt('/vendors');

    expect(await screen.findByText('Acme Cloud')).toBeInTheDocument();
    expect(screen.getByText('Soumis')).toBeInTheDocument();
    expect(screen.getByText('42,5 · Élevé')).toBeInTheDocument();
    expect(screen.getByText('Haute')).toBeInTheDocument();
    expect(svc.list).toHaveBeenCalledWith({ limit: 50, offset: 0 });
  });

  it('shows a skeleton while the register loads', () => {
    svc.list.mockReturnValue(new Promise(() => {}));
    renderAt('/vendors');
    expect(screen.getByTestId('table-skeleton')).toBeInTheDocument();
  });

  it('offers a retry when the register fails', async () => {
    svc.list.mockRejectedValueOnce(httpError(500));
    svc.list.mockResolvedValue({ items: [entry()], total: 1, limit: 50, offset: 0 });
    renderAt('/vendors');

    fireEvent.click(await screen.findByRole('button', { name: /réessayer/i }));
    expect(await screen.findByText('Acme Cloud')).toBeInTheDocument();
  });

  it('leads someone who can create assets from the empty register to the vendor form', async () => {
    svc.list.mockResolvedValue({ items: [], total: 0, limit: 50, offset: 0 });
    renderAt('/vendors');

    expect(await screen.findByText("Aucun fournisseur pour l'instant")).toBeInTheDocument();
    const buttons = screen.getAllByRole('button', { name: 'Ajouter un fournisseur' });
    fireEvent.click(buttons[buttons.length - 1]);
    expect(screen.getByRole('dialog', { name: 'create-asset-Supplier' })).toBeInTheDocument();
  });

  it('tells a reader who can add a vendor instead of offering a dead button', async () => {
    auth.createAsset = false;
    svc.list.mockResolvedValue({ items: [], total: 0, limit: 50, offset: 0 });
    renderAt('/vendors');

    expect(await screen.findByText(/Demandez à une personne autorisée/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Ajouter un fournisseur' })).toBeNull();
  });

  it('shows the upsell lock on a plan without vendor risk', async () => {
    entitlement.enabled = false;
    svc.list.mockResolvedValue({ items: [entry()], total: 1, limit: 50, offset: 0 });
    renderAt('/vendors');

    expect(await screen.findByText(/Gestion des risques fournisseurs/)).toBeInTheDocument();
    expect(screen.queryByText('Acme Cloud')).toBeNull();
  });
});

/* ------------------------------------------------------------- vendor page */

describe('VendorDetailPage', () => {
  it('shows the chain, each item opening its entity drawer', async () => {
    renderAt('/vendors/v-1');

    const asset = await screen.findByRole('link', { name: 'ERP' });
    expect(asset.getAttribute('href')).toContain('drawer=asset&entity=a-1');
    const risk = screen.getByRole('link', { name: 'Fuite de données ERP' });
    expect(risk.getAttribute('href')).toContain('drawer=risk&entity=r-1');
    expect(screen.getByText('Hébergé par le fournisseur')).toBeInTheDocument();
  });

  it('adds a link at once and takes it back when the server refuses', async () => {
    const pending = deferred<never>();
    svc.link.mockReturnValue(pending.promise);
    renderAt('/vendors/v-1');

    // Not /^Actif/ alone: the chain section is named "Actifs et risques associés".
    const select = await screen.findByRole('combobox', { name: /^Actif(?!s)/ });
    await waitFor(() =>
      expect(within(select).getByRole('option', { name: 'CRM' })).toBeInTheDocument(),
    );
    // A vendor cannot be linked to a vendor, so none is offered.
    expect(within(select).queryByRole('option', { name: 'Autre fournisseur' })).toBeNull();

    fireEvent.change(select, { target: { value: 'a-2' } });
    fireEvent.click(screen.getByRole('button', { name: 'Relier' }));

    expect(await screen.findByRole('link', { name: 'CRM' })).toBeInTheDocument();
    expect(screen.getByText('Enregistrement…')).toBeInTheDocument();
    expect(svc.link).toHaveBeenCalledWith('v-1', { asset_id: 'a-2', verb: 'managed_by' });

    pending.reject(httpError(409));
    await waitFor(() => expect(screen.queryByRole('link', { name: 'CRM' })).toBeNull());
    expect(
      screen.getByText('Cet actif est déjà relié à ce fournisseur avec cette relation.'),
    ).toBeInTheDocument();
  });

  it('removes a link at once and restores it when the server refuses', async () => {
    const pending = deferred<void>();
    svc.unlink.mockReturnValue(pending.promise);
    renderAt('/vendors/v-1');

    fireEvent.click(await screen.findByRole('button', { name: 'Retirer le lien vers ERP' }));
    await waitFor(() => expect(screen.queryByRole('link', { name: 'ERP' })).toBeNull());

    pending.reject(httpError(500));
    expect(await screen.findByRole('link', { name: 'ERP' })).toBeInTheDocument();
    expect(toast.error).toHaveBeenCalled();
  });

  it('validates the send form before calling the server', async () => {
    renderAt('/vendors/v-1');

    fireEvent.click(await screen.findByRole('button', { name: 'Envoyer un questionnaire' }));
    const dialog = await screen.findByRole('dialog');
    await within(dialog).findByLabelText(/^Questionnaire/);

    fireEvent.change(within(dialog).getByLabelText(/^Échéance/), {
      target: { value: '2020-01-01' },
    });
    fireEvent.change(within(dialog).getByLabelText(/^Email de contact/), {
      target: { value: 'pas-un-email' },
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Envoyer' }));

    expect(within(dialog).getByText('Choisissez un questionnaire.')).toBeInTheDocument();
    expect(within(dialog).getByText("L'échéance doit être dans le futur.")).toBeInTheDocument();
    expect(within(dialog).getByText('Saisissez une adresse email valide.')).toBeInTheDocument();
    expect(svc.send).not.toHaveBeenCalled();
  });

  it('requires a contact email when the vendor has none on record', async () => {
    assetSvc.getAsset.mockResolvedValue({ id: 'v-1', name: 'Acme Cloud', attributes: {} } as Asset);
    renderAt('/vendors/v-1');

    expect(await screen.findByText(/Aucun enregistré/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Envoyer un questionnaire' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(await within(dialog).findByLabelText(/^Questionnaire/), {
      target: { value: 't-1' },
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Envoyer' }));

    expect(
      within(dialog).getByText("Saisissez l'email de contact du fournisseur."),
    ).toBeInTheDocument();
    expect(svc.send).not.toHaveBeenCalled();
  });

  it('shows the questionnaire link once when the email could not go out', async () => {
    svc.send.mockResolvedValue({
      assessment: assessment({ id: 'as-2' }),
      delivery: 'unavailable',
      questionnaire_url: 'https://app.test/vendor-questionnaire#token-abc',
    });
    renderAt('/vendors/v-1');

    fireEvent.click(await screen.findByRole('button', { name: 'Envoyer un questionnaire' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(await within(dialog).findByLabelText(/^Questionnaire/), {
      target: { value: 't-1' },
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Envoyer' }));

    const link = await screen.findByLabelText('Lien du questionnaire');
    expect((link as HTMLInputElement).value).toBe(
      'https://app.test/vendor-questionnaire#token-abc',
    );
    expect(screen.getByText(/rien n'a été envoyé/)).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).toBeNull();

    const [, input] = svc.send.mock.calls[0];
    expect(input.template_id).toBe('t-1');
    expect(typeof input.due_at).toBe('string');
    // Left empty, so the server uses the vendor's own contact email.
    expect(input.contact_email).toBeUndefined();

    fireEvent.click(screen.getByRole('button', { name: 'Terminé' }));
    expect(screen.queryByLabelText('Lien du questionnaire')).toBeNull();
  });

  it('revokes an open questionnaire after confirmation', async () => {
    svc.revoke.mockResolvedValue(assessment({ status: 'revoked' }));
    renderAt('/vendors/v-1');

    fireEvent.click(await screen.findByRole('button', { name: /^Révoquer le questionnaire/ }));
    const dialog = await screen.findByRole('alertdialog');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Révoquer' }));

    await waitFor(() => expect(svc.revoke).toHaveBeenCalledWith('as-1'));
  });

  it('says so when the vendor does not exist', async () => {
    svc.chain.mockRejectedValue(httpError(404));
    renderAt('/vendors/v-404');

    expect(
      await screen.findByText(
        "Ce fournisseur n'existe pas ou appartient à une autre organisation.",
      ),
    ).toBeInTheDocument();
  });

  it('has no serious accessibility violation', async () => {
    const { container } = renderAt('/vendors/v-1');
    await screen.findByRole('link', { name: 'ERP' });
    await screen.findByText('Baseline sécurité · v2');
    await expectNoSeriousViolations(container);
  });
});

/* ------------------------------------------------------------------ review */

function submitted(): VendorAssessment {
  return assessment({
    status: 'submitted',
    submitted_at: '2026-09-20T10:00:00Z',
    // Deliberately a tier a client-side 70/40/20 cut would NOT give 37.5: the
    // screen must show the backend's tier, not derive its own.
    score: 37.5,
    tier: 'high',
    scoring_version: 'vendorscore/1',
    score_breakdown: [
      { item_id: 'i-1', weight: 4, points: 0.5, na: false, contribution: 25 },
      { item_id: 'i-2', weight: 2, points: 0, na: true, contribution: 0 },
    ],
    items: [
      {
        id: 'i-2',
        position: 2,
        text: 'Avez-vous un PCA ?',
        answer_type: 'choice',
        options: [
          { value: 'yes', label: 'Oui', points: 1 },
          { value: 'no', label: 'Non', points: 0 },
        ],
        weight: 2,
        required: true,
        na_allowed: true,
        answer_value: null,
        answer_na: true,
      },
      {
        id: 'i-1',
        position: 1,
        text: 'Imposez-vous le MFA ?',
        answer_type: 'choice',
        options: [
          { value: 'yes', label: 'Oui', points: 1 },
          { value: 'partial', label: 'Partiellement', points: 0.5 },
          { value: 'no', label: 'Non', points: 0 },
        ],
        weight: 4,
        required: true,
        na_allowed: false,
        control_ref: 'ISO 27001 A.8.5',
        answer_value: 'partial',
        answer_na: false,
        answer_comment: 'Sauf comptes de service',
      },
      {
        id: 'i-3',
        position: 3,
        text: 'Décrivez votre gestion des sauvegardes.',
        answer_type: 'text',
        weight: 0,
        required: false,
        na_allowed: false,
        answer_value: 'Quotidiennes, chiffrées',
        answer_na: false,
      },
    ],
  });
}

describe('VendorAssessmentPage', () => {
  it('shows the score, tier and contributions exactly as the backend returned them', async () => {
    svc.getAssessment.mockResolvedValue(submitted());
    renderAt('/vendors/v-1/assessments/as-1');

    expect(await screen.findByText('37,5')).toBeInTheDocument();
    expect(screen.getByText('Élevé')).toBeInTheDocument();
    expect(screen.getByText('Formule vendorscore/1')).toBeInTheDocument();

    const questions = screen.getAllByRole('listitem');
    expect(within(questions[0]).getByText('Imposez-vous le MFA ?')).toBeInTheDocument();
    expect(within(questions[0]).getByText('Réponse : Partiellement')).toBeInTheDocument();
    expect(
      within(questions[0]).getByText('Commentaire : Sauf comptes de service'),
    ).toBeInTheDocument();
    expect(within(questions[0]).getByText('25')).toBeInTheDocument();
    expect(within(questions[1]).getByText('Non applicable : exclue du score.')).toBeInTheDocument();
    expect(within(questions[2]).getByText('Réponse libre : non notée.')).toBeInTheDocument();
  });

  it('explains that the score comes with submission', async () => {
    svc.getAssessment.mockResolvedValue(assessment({ status: 'in_progress' }));
    renderAt('/vendors/v-1/assessments/as-1');

    expect(await screen.findByText(/Pas encore soumis/)).toBeInTheDocument();
  });

  it('refuses an assessment reached under another vendor', async () => {
    svc.getAssessment.mockResolvedValue(assessment({ vendor_asset_id: 'v-9' }));
    renderAt('/vendors/v-1/assessments/as-1');

    expect(
      await screen.findByText(
        "Ce questionnaire n'existe pas ou appartient à une autre organisation.",
      ),
    ).toBeInTheDocument();
  });

  it('has no serious accessibility violation', async () => {
    svc.getAssessment.mockResolvedValue(submitted());
    const { container } = renderAt('/vendors/v-1/assessments/as-1');
    await screen.findByText('37,5');
    await expectNoSeriousViolations(container);
  });
});
