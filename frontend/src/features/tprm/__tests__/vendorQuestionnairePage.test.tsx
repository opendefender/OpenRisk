// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The public vendor questionnaire (#674): load, draft save, submit, and the
// dead-link states — with the token read from the fragment and never rendered.

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { AxiosError, type AxiosResponse } from 'axios';
import axe from 'axe-core';

vi.mock('../vendorQuestionnaireService', async () => {
  const actual = await vi.importActual<typeof import('../vendorQuestionnaireService')>(
    '../vendorQuestionnaireService',
  );
  return {
    ...actual,
    vendorQuestionnaireService: { get: vi.fn(), saveAnswers: vi.fn(), submit: vi.fn() },
  };
});

import {
  vendorQuestionnaireService,
  type VendorQuestionnaireView,
} from '../vendorQuestionnaireService';
import { VendorQuestionnairePage } from '../VendorQuestionnairePage';
import { tokenFromHash } from '../questionnaireToken';

const svc = vi.mocked(vendorQuestionnaireService);
const TOKEN = 'tok_SECRET_do_not_render_ABCDEFGHIJKLMNOPQRSTUV';

function view(overrides: Partial<VendorQuestionnaireView> = {}): VendorQuestionnaireView {
  return {
    organization_name: 'Banque Exemple',
    vendor_name: 'Acme Cloud',
    language: 'fr',
    due_at: '2026-03-15T00:00:00Z',
    status: 'sent',
    read_only: false,
    items: [
      {
        id: 'i-1',
        position: 1,
        text: 'Imposez-vous le MFA ?',
        help: '',
        answer_type: 'choice',
        options: [
          { value: 'yes', label: 'Oui' },
          { value: 'no', label: 'Non' },
        ],
        required: true,
        na_allowed: false,
        answer_value: null,
        answer_na: false,
        answer_comment: '',
      },
      {
        id: 'i-2',
        position: 2,
        text: 'Décrivez votre plan de continuité',
        help: 'Deux ou trois phrases suffisent.',
        answer_type: 'text',
        options: [],
        required: false,
        na_allowed: true,
        answer_value: null,
        answer_na: false,
        answer_comment: '',
      },
    ],
    ...overrides,
  };
}

function httpError(status: number): AxiosError {
  const response = { status, data: {}, statusText: '', headers: {} } as AxiosResponse;
  return new AxiosError('failed', 'ERR_BAD_RESPONSE', undefined, undefined, response);
}

function renderAt(hash = `#${TOKEN}`) {
  return render(
    <MemoryRouter initialEntries={[`/vendor-questionnaire${hash}`]}>
      <VendorQuestionnairePage />
    </MemoryRouter>,
  );
}

describe('VendorQuestionnairePage', () => {
  beforeEach(() => {
    svc.get.mockReset();
    svc.saveAnswers.mockReset();
    svc.submit.mockReset();
  });

  it('reads the token from the fragment, loads the questionnaire in the contact language, and never renders the token', async () => {
    svc.get.mockResolvedValue(view());

    const { container } = renderAt();

    expect(
      await screen.findByText(
        'Banque Exemple vous demande de compléter un questionnaire de sécurité pour Acme Cloud.',
      ),
    ).toBeInTheDocument();
    expect(svc.get).toHaveBeenCalledWith(TOKEN);
    expect(screen.getByLabelText('Oui')).toBeInTheDocument();
    expect(screen.getByLabelText(/Décrivez votre plan de continuité/)).toBeInTheDocument();
    expect(container.innerHTML).not.toContain(TOKEN);
    expect(document.head.querySelector('meta[name="referrer"]')?.getAttribute('content')).toBe(
      'no-referrer',
    );
  });

  it('shows a skeleton while loading, not a spinner', () => {
    svc.get.mockReturnValue(new Promise(() => {}));

    renderAt();

    expect(screen.getByRole('status', { name: 'Chargement du questionnaire…' })).toHaveAttribute(
      'aria-busy',
      'true',
    );
  });

  it('treats a link without a token as invalid, without calling the server', async () => {
    renderAt('');

    expect(await screen.findByText('Ce lien ne fonctionne pas')).toBeInTheDocument();
    expect(svc.get).not.toHaveBeenCalled();
  });

  it.each([
    [404, 'Ce lien ne fonctionne pas', false],
    [410, 'Ce lien ne fonctionne plus', false],
    [429, 'Trop de tentatives', true],
    [500, "Le questionnaire n'a pas pu être chargé", true],
  ])('answers a %i with its own words', async (status, title, retry) => {
    svc.get.mockRejectedValue(httpError(status));

    renderAt();

    expect(await screen.findByText(title)).toBeInTheDocument();
    const retryButton = screen.queryByRole('button', { name: 'Réessayer' });
    expect(Boolean(retryButton)).toBe(retry);
    if (retryButton) {
      svc.get.mockResolvedValue(view());
      fireEvent.click(retryButton);
      await screen.findByLabelText('Oui');
      expect(svc.get).toHaveBeenCalledTimes(2);
    }
  });

  it('saves only the answers that changed', async () => {
    svc.get.mockResolvedValue(view());
    svc.saveAnswers.mockImplementation(async () => view({ status: 'in_progress' }));
    renderAt();

    fireEvent.click(await screen.findByLabelText('Oui'));
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer le brouillon' }));

    await waitFor(() => expect(svc.saveAnswers).toHaveBeenCalledTimes(1));
    expect(svc.saveAnswers).toHaveBeenCalledWith(TOKEN, [
      { item_id: 'i-1', answer_value: 'yes', answer_na: false, answer_comment: '' },
    ]);
    expect(await screen.findByText(/Brouillon enregistré à/)).toBeInTheDocument();
  });

  it('sends N/A as N/A, with no value', async () => {
    svc.get.mockResolvedValue(view());
    svc.saveAnswers.mockResolvedValue(view());
    renderAt();

    fireEvent.click(await screen.findByLabelText('Non applicable'));
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer le brouillon' }));

    await waitFor(() =>
      expect(svc.saveAnswers).toHaveBeenCalledWith(TOKEN, [
        { item_id: 'i-2', answer_value: null, answer_na: true, answer_comment: '' },
      ]),
    );
  });

  it('refuses to submit while a required question is unanswered, and says which', async () => {
    svc.get.mockResolvedValue(view());
    renderAt();

    fireEvent.click(await screen.findByRole('button', { name: 'Soumettre mes réponses' }));

    expect(
      await screen.findByText('Répondez aux questions obligatoires avant de soumettre : 1.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Soumettre le questionnaire ?')).not.toBeInTheDocument();
    expect(svc.submit).not.toHaveBeenCalled();
  });

  it('asks for confirmation, saves then submits, and locks the page', async () => {
    svc.get.mockResolvedValue(view());
    svc.saveAnswers.mockResolvedValue(view({ status: 'in_progress' }));
    const locked = view({ status: 'submitted', read_only: true });
    locked.items[0].answer_value = 'yes';
    svc.submit.mockResolvedValue(locked);
    renderAt();

    fireEvent.click(await screen.findByLabelText('Oui'));
    fireEvent.click(screen.getByRole('button', { name: 'Soumettre mes réponses' }));
    expect(await screen.findByText('Soumettre le questionnaire ?')).toBeInTheDocument();
    expect(svc.submit).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Soumettre' }));

    expect(await screen.findByText('Merci — vos réponses ont été transmises')).toBeInTheDocument();
    expect(svc.saveAnswers).toHaveBeenCalledTimes(1);
    expect(svc.submit).toHaveBeenCalledWith(TOKEN);
    expect(svc.saveAnswers.mock.invocationCallOrder[0]).toBeLessThan(
      svc.submit.mock.invocationCallOrder[0],
    );
    expect(screen.getByLabelText('Oui')).toBeDisabled();
    expect(
      screen.queryByRole('button', { name: 'Soumettre mes réponses' }),
    ).not.toBeInTheDocument();
  });

  it('shows a questionnaire submitted earlier as read-only', async () => {
    svc.get.mockResolvedValue(view({ status: 'submitted', read_only: true }));

    renderAt();

    expect(
      await screen.findByText('Ce questionnaire a été soumis et ne peut plus être modifié.'),
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Oui')).toBeDisabled();
    expect(
      screen.queryByRole('button', { name: 'Enregistrer le brouillon' }),
    ).not.toBeInTheDocument();
  });

  it('reloads when a save finds the questionnaire already submitted (409)', async () => {
    svc.get
      .mockResolvedValueOnce(view())
      .mockResolvedValueOnce(view({ status: 'submitted', read_only: true }));
    svc.saveAnswers.mockRejectedValue(httpError(409));
    renderAt();

    fireEvent.click(await screen.findByLabelText('Oui'));
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer le brouillon' }));

    expect(
      await screen.findByText('Ce questionnaire a été soumis et ne peut plus être modifié.'),
    ).toBeInTheDocument();
    expect(svc.get).toHaveBeenCalledTimes(2);
  });

  it('turns a link that died mid-session (410) into the dead-link state', async () => {
    svc.get.mockResolvedValue(view());
    svc.saveAnswers.mockRejectedValue(httpError(410));
    renderAt();

    fireEvent.click(await screen.findByLabelText('Oui'));
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer le brouillon' }));

    expect(await screen.findByText('Ce lien ne fonctionne plus')).toBeInTheDocument();
  });

  it('keeps the answers on the page when a save is rate-limited (429)', async () => {
    svc.get.mockResolvedValue(view());
    svc.saveAnswers.mockRejectedValue(httpError(429));
    renderAt();

    fireEvent.click(await screen.findByLabelText('Oui'));
    fireEvent.click(screen.getByRole('button', { name: 'Enregistrer le brouillon' }));

    expect(await screen.findByText(/Trop d'enregistrements en peu de temps/)).toBeInTheDocument();
    expect(screen.getByLabelText('Oui')).toBeChecked();
  });

  it('switches language locally', async () => {
    svc.get.mockResolvedValue(view());
    renderAt();
    await screen.findByLabelText('Oui');

    fireEvent.click(screen.getByRole('button', { name: 'Passer en anglais' }));

    expect(
      await screen.findByText(
        'Banque Exemple asks you to complete a security questionnaire for Acme Cloud.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save draft' })).toBeInTheDocument();
  });

  it('has no serious or critical accessibility violations', async () => {
    svc.get.mockResolvedValue(view());
    const { container } = renderAt();
    await screen.findByLabelText('Oui');

    const results = await axe.run(container, {
      rules: { 'color-contrast': { enabled: false }, region: { enabled: false } },
    });
    const serious = results.violations.filter(
      (v) => v.impact === 'serious' || v.impact === 'critical',
    );
    expect(serious.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  });
});

describe('tokenFromHash', () => {
  it('reads the fragment and survives a malformed escape', () => {
    expect(tokenFromHash('#tok_value_42')).toBe('tok_value_42');
    expect(tokenFromHash('')).toBe('');
    expect(tokenFromHash('#%E0%A4%A')).toBe('');
  });
});
