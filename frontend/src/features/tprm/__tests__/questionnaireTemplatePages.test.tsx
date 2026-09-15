// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Questionnaire template list and editor (#681).

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AxiosError, type AxiosResponse } from 'axios';
import axe from 'axe-core';

import { useUIStore } from '../../../store/uiStore';

const auth = { manage: true };
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (selector: (s: { hasPermission: (p: string) => boolean }) => unknown) =>
    selector({ hasPermission: (p: string) => (p === 'vendors:manage' ? auth.manage : true) }),
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

import {
  questionnaireTemplateService,
  type QuestionnaireTemplate,
} from '../questionnaireTemplateService';
import { QuestionnaireTemplatesPage } from '../QuestionnaireTemplatesPage';
import { QuestionnaireTemplateEditorPage } from '../QuestionnaireTemplateEditorPage';

const svc = vi.mocked(questionnaireTemplateService);

function template(overrides: Partial<QuestionnaireTemplate> = {}): QuestionnaireTemplate {
  return {
    id: 't-1',
    name: 'Baseline sécurité',
    description: '',
    language: 'fr',
    version: 3,
    archived_at: null,
    created_at: '2026-03-01T09:00:00Z',
    updated_at: '2026-03-02T09:00:00Z',
    questions: [
      {
        id: 'q-1',
        template_id: 't-1',
        position: 1,
        text: 'Imposez-vous le MFA ?',
        answer_type: 'choice',
        weight: 5,
        required: true,
        options: [
          { value: 'yes', label: 'Oui', points: 1 },
          { value: 'no', label: 'Non', points: 0 },
        ],
      },
    ],
    ...overrides,
  };
}

function renderAt(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/vendors/questionnaires" element={<QuestionnaireTemplatesPage />} />
          <Route path="/vendors/questionnaires/new" element={<QuestionnaireTemplateEditorPage />} />
          <Route
            path="/vendors/questionnaires/:templateId"
            element={<QuestionnaireTemplateEditorPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  auth.manage = true;
  entitlement.enabled = true;
  useUIStore.getState().setLang('fr');
  for (const fn of Object.values(svc)) fn.mockReset();
});

describe('QuestionnaireTemplatesPage', () => {
  it('lists the templates with their version and status in words', async () => {
    svc.list.mockResolvedValue([
      template(),
      template({ id: 't-2', name: 'Ancien', version: 1, archived_at: '2026-03-03T00:00:00Z' }),
    ]);

    renderAt('/vendors/questionnaires');

    expect(await screen.findByText('Baseline sécurité')).toBeInTheDocument();
    expect(screen.getByText('v3', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('Archivé')).toBeInTheDocument();
    expect(svc.list).toHaveBeenCalledWith(false);
  });

  it('asks for archived templates only when the reader wants them', async () => {
    svc.list.mockResolvedValue([template()]);
    renderAt('/vendors/questionnaires');
    await screen.findByText('Baseline sécurité');

    fireEvent.click(screen.getByLabelText('Afficher les questionnaires archivés'));

    await waitFor(() => expect(svc.list).toHaveBeenCalledWith(true));
  });

  it('leads a manager from the empty state to the editor', async () => {
    svc.list.mockResolvedValue([]);
    renderAt('/vendors/questionnaires');

    expect(await screen.findByText("Aucun questionnaire pour l'instant")).toBeInTheDocument();
    const creates = screen.getAllByRole('button', { name: 'Nouveau questionnaire' });
    fireEvent.click(creates[creates.length - 1]);

    expect(await screen.findByLabelText(/^Nom/)).toBeInTheDocument();
  });

  it('tells a reader without the manage permission who can create one, with no dead button', async () => {
    auth.manage = false;
    svc.list.mockResolvedValue([]);
    renderAt('/vendors/questionnaires');

    expect(await screen.findByText(/Demandez à une personne habilitée/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Nouveau questionnaire' })).not.toBeInTheDocument();
  });

  it('shows the upsell lock to a plan without vendor risk', async () => {
    entitlement.enabled = false;
    svc.list.mockResolvedValue([]);
    renderAt('/vendors/questionnaires');

    expect(await screen.findByText('Gestion des risques fournisseurs (TPRM)')).toBeInTheDocument();
  });
});

describe('QuestionnaireTemplateEditorPage', () => {
  it('creates a questionnaire with its questions in the order the author set', async () => {
    svc.create.mockResolvedValue(template({ id: 't-new', version: 1 }));
    // After creating, the editor navigates to the new template and loads it.
    svc.get.mockResolvedValue(template({ id: 't-new', version: 1 }));
    renderAt('/vendors/questionnaires/new');

    fireEvent.change(await screen.findByLabelText(/^Nom/), { target: { value: 'Baseline' } });
    const firstText = screen.getAllByRole('textbox', { name: /^Question/ })[0];
    fireEvent.change(firstText, { target: { value: 'Première' } });

    fireEvent.click(screen.getByRole('button', { name: 'Ajouter une question' }));
    fireEvent.change(screen.getAllByRole('textbox', { name: /^Question/ })[1], {
      target: { value: 'Deuxième' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Monter la question 2' }));

    fireEvent.click(screen.getByRole('button', { name: 'Créer le questionnaire' }));

    await waitFor(() => expect(svc.create).toHaveBeenCalledTimes(1));
    const input = svc.create.mock.calls[0][0];
    expect(input.name).toBe('Baseline');
    expect(input.questions.map((q) => q.text)).toEqual(['Deuxième', 'Première']);
    expect(await screen.findByDisplayValue('Baseline sécurité')).toBeInTheDocument();
  });

  it('removes a question', async () => {
    renderAt('/vendors/questionnaires/new');
    await screen.findByLabelText(/^Nom/);
    fireEvent.click(screen.getByRole('button', { name: 'Ajouter une question' }));
    expect(screen.getAllByRole('textbox', { name: /^Question/ })).toHaveLength(2);

    fireEvent.click(screen.getByRole('button', { name: 'Retirer la question 1' }));

    expect(screen.getAllByRole('textbox', { name: /^Question/ })).toHaveLength(1);
  });

  it('refuses to save an invalid questionnaire and says what to fix, without calling the server', async () => {
    renderAt('/vendors/questionnaires/new');
    await screen.findByLabelText(/^Nom/);

    fireEvent.click(screen.getByRole('button', { name: 'Créer le questionnaire' }));

    expect(await screen.findByText('Donnez un nom au questionnaire.')).toBeInTheDocument();
    expect(screen.getByText('Rédigez la question.')).toBeInTheDocument();
    expect(
      screen.getByText("Certains champs doivent être corrigés avant d'enregistrer."),
    ).toBeInTheDocument();
    expect(svc.create).not.toHaveBeenCalled();
  });

  it('shows a server refusal in place', async () => {
    const response = {
      status: 400,
      data: { error: 'question 1: weight must be between 0 and 10' },
      statusText: '',
      headers: {},
    } as AxiosResponse;
    svc.create.mockRejectedValue(
      new AxiosError('bad', 'ERR_BAD_REQUEST', undefined, undefined, response),
    );
    renderAt('/vendors/questionnaires/new');

    fireEvent.change(await screen.findByLabelText(/^Nom/), { target: { value: 'Baseline' } });
    fireEvent.change(screen.getAllByRole('textbox', { name: /^Question/ })[0], {
      target: { value: 'Q' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Créer le questionnaire' }));

    expect(await screen.findByText(/weight must be between 0 and 10/)).toBeInTheDocument();
  });

  it('tells the author that saving creates a new version and leaves sent questionnaires alone', async () => {
    svc.get.mockResolvedValue(template());
    svc.update.mockResolvedValue(template({ version: 4 }));
    renderAt('/vendors/questionnaires/t-1');

    expect(
      await screen.findByText(
        'Enregistrer crée la version 4. Les questionnaires déjà envoyés gardent les questions avec lesquelles ils sont partis.',
      ),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/^Nom/), { target: { value: 'Baseline v4' } });
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer' }));

    await waitFor(() => expect(svc.update).toHaveBeenCalledTimes(1));
    expect(svc.update.mock.calls[0][0]).toBe('t-1');
    expect(svc.update.mock.calls[0][1].questions[0].options?.map((o) => o.value)).toEqual([
      'yes',
      'no',
    ]);
  });

  it('archives after confirmation', async () => {
    svc.get.mockResolvedValue(template());
    svc.archive.mockResolvedValue(template({ archived_at: '2026-03-05T00:00:00Z' }));
    renderAt('/vendors/questionnaires/t-1');

    fireEvent.click(await screen.findByRole('button', { name: 'Archiver' }));
    const dialog = await screen.findByRole('alertdialog');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Archiver' }));

    await waitFor(() => expect(svc.archive).toHaveBeenCalledWith('t-1'));
  });

  it('shows an archived questionnaire read-only', async () => {
    svc.get.mockResolvedValue(template({ archived_at: '2026-03-05T00:00:00Z' }));
    renderAt('/vendors/questionnaires/t-1');

    expect(
      await screen.findByText(
        'Ce questionnaire est archivé : il ne peut plus être modifié ni envoyé.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/^Nom/)).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Enregistrer' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Archiver' })).not.toBeInTheDocument();
  });

  it('shows the questionnaire read-only to a reader without the manage permission', async () => {
    auth.manage = false;
    svc.get.mockResolvedValue(template());
    renderAt('/vendors/questionnaires/t-1');

    expect(
      await screen.findByText(/Le modifier nécessite le droit de gérer les fournisseurs/),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Enregistrer' })).not.toBeInTheDocument();
  });

  it('says a foreign or unknown questionnaire does not exist', async () => {
    const response = { status: 404, data: {}, statusText: '', headers: {} } as AxiosResponse;
    svc.get.mockRejectedValue(
      new AxiosError('nf', 'ERR_BAD_REQUEST', undefined, undefined, response),
    );
    renderAt('/vendors/questionnaires/t-x');

    expect(
      await screen.findByText(/n'existe pas ou appartient à une autre organisation/),
    ).toBeInTheDocument();
  });

  it('has no serious or critical accessibility violations', async () => {
    svc.get.mockResolvedValue(template());
    const { container } = renderAt('/vendors/questionnaires/t-1');
    await screen.findByDisplayValue('Baseline sécurité');

    const results = await axe.run(container, {
      rules: { 'color-contrast': { enabled: false }, region: { enabled: false } },
    });
    const serious = results.violations.filter(
      (v) => v.impact === 'serious' || v.impact === 'critical',
    );
    expect(serious.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  });
});
