// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/** @vitest-environment jsdom */
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useUIStore } from '../../../store/uiStore';

const createRiskMock = vi.fn<(payload: unknown) => Promise<{ id: string; title: string }>>(() =>
  Promise.resolve({ id: 'r1', title: 'Fuite de données API' }),
);
const fetchAssetsMock = vi.fn(() => Promise.resolve());
const assetList = vi.hoisted(() => ({
  current: [] as { id: string; name: string }[],
}));
const preview = vi.hoisted(() => ({ current: undefined as unknown }));
const suggestions = vi.hoisted(() => ({ current: undefined as unknown }));

vi.mock('../../../hooks/useAssetStore', () => ({
  useAssetStore: () => ({
    assets: assetList.current,
    fetchAssets: fetchAssetsMock,
    isLoading: false,
  }),
}));

vi.mock('../../../services/riskService', async () => {
  const actual = await vi.importActual<typeof import('../../../services/riskService')>(
    '../../../services/riskService',
  );
  return {
    ...actual,
    riskService: {
      ...actual.riskService,
      createRisk: (payload: unknown) => createRiskMock(payload),
    },
  };
});

vi.mock('../useTaxonomy', () => ({
  CATEGORIES_KEY: ['risk-categories'],
  IMPORTED_FRAMEWORKS_KEY: ['compliance', 'frameworks', 'imported'],
  useRiskCategories: () => ({ data: [{ id: 'cat-1', name: 'Cyber' }] }),
  useImportedFrameworks: () => ({ data: [] }),
  useFrameworkControls: () => ({ data: [] }),
}));

vi.mock('../useRiskRegisterData', () => ({
  useRegisterMembers: () => ({
    members: [{ user_id: 'u1', email: 'aicha@acme.test', full_name: 'Aïcha Diallo' }],
    isLoading: false,
    isError: false,
  }),
}));

vi.mock('../../onboarding/useActivation', () => ({
  useOnboardingSuggestions: () => ({ data: suggestions.current }),
  useInvalidateActivation: () => vi.fn(),
}));

vi.mock('../../../hooks/useScore', () => ({
  useScorePreview: () => ({ data: preview.current }),
}));

vi.mock('sonner', () => ({
  toast: Object.assign(vi.fn(), { success: vi.fn(), error: vi.fn(), warning: vi.fn() }),
}));

import { CreateRiskModal } from '../CreateRiskModal';

function renderModal(props: Partial<React.ComponentProps<typeof CreateRiskModal>> = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const onClose = vi.fn();
  const onCreated = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <CreateRiskModal isOpen onClose={onClose} onCreated={onCreated} {...props} />
    </QueryClientProvider>,
  );
  return { onClose, onCreated };
}

const submit = () => screen.getByRole('button', { name: 'Créer le risque' });
const titleInput = () => document.querySelector('input[name="title"]') as HTMLInputElement;

describe('CreateRiskModal', () => {
  beforeEach(() => {
    createRiskMock.mockClear();
    fetchAssetsMock.mockClear();
    assetList.current = [];
    preview.current = undefined;
    suggestions.current = undefined;
    useUIStore.setState({ lang: 'fr' });
  });

  // Review defect (#751 phase 2): flashSuccess() and the modal's own close
  // happened in the same batch, and Modal renders nothing once closed — so
  // the drawn check glyph was dead code, never actually painted. The fix
  // removes the wiring outright; the toast stays the only confirmation.
  it('never wires the drawn success check — the toast is the only confirmation', async () => {
    renderModal();
    fireEvent.change(titleInput(), { target: { value: 'Fuite de données API' } });
    fireEvent.click(submit());

    await waitFor(() => expect(createRiskMock).toHaveBeenCalled());

    // Button's feedback="success" renders a drawn check as an svg <path>
    // with pathLength=1 / stroke-dasharray=1 (see shared/ds/Button.tsx). It
    // must never appear, because this component never passes `feedback`.
    expect(document.querySelector('path[stroke-dasharray="1"]')).not.toBeInTheDocument();
  });

  const submitForm = () =>
    fireEvent.submit(document.getElementById('create-risk-form') as HTMLElement);

  it('keeps "Créer le risque" disabled until the title has 4 characters', () => {
    renderModal();
    expect(submit()).toBeDisabled();
    fireEvent.change(titleInput(), { target: { value: 'abc' } });
    expect(submit()).toBeDisabled();
    fireEvent.change(titleInput(), { target: { value: 'abcd' } });
    expect(submit()).toBeEnabled();
  });

  it('still refuses a short title in words when the form is submitted (Zod), and sends nothing', async () => {
    renderModal();
    fireEvent.change(titleInput(), { target: { value: 'abc' } });
    submitForm();
    expect(
      await screen.findByText('L’intitulé doit comporter au moins 4 caractères.'),
    ).toBeInTheDocument();
    expect(createRiskMock).not.toHaveBeenCalled();
  });

  it('refuses an empty form, and the description is optional', async () => {
    renderModal();
    submitForm();
    expect(await screen.findByText(/au moins 4 caractères/)).toBeInTheDocument();
    expect(createRiskMock).not.toHaveBeenCalled();
    expect(screen.getByText(/facultatif/)).toBeInTheDocument();

    fireEvent.change(titleInput(), { target: { value: 'Compromission de la messagerie' } });
    fireEvent.click(submit());
    await waitFor(() => expect(createRiskMock).toHaveBeenCalled());
    expect(createRiskMock.mock.calls[0][0]).toMatchObject({ description: undefined });
  });

  // Review C1: the explainer opens the 0-100 model score. No second scale here.
  it('shows no second scale anywhere: no "/100", no score explainer', () => {
    preview.current = {
      risk_score: 3.5,
      risk_criticality: 'medium',
      inherent: 42.4,
      inherent_band: 'high',
      value: 42.4,
      band: 'high',
    };
    renderModal();
    const dialog = screen.getByRole('dialog');
    expect(dialog.textContent).not.toMatch(/\/\s*100/);
    expect(within(dialog).queryByRole('button', { name: /explain|expliqu/i })).toBeNull();
  });

  it('puts the sector suggestions below the form, collapsed, and says "Actifs concernés"', async () => {
    suggestions.current = {
      risks: [
        {
          key: 'k1',
          title: 'Ransomware sur le SI',
          description: 'Chiffrement des serveurs.',
          probability: 0.5,
          impact: 8,
        },
      ],
    };
    renderModal();
    const box = screen.getByTestId('risk-suggestions');
    // After the title field, not before it: the form starts with the title.
    expect(
      titleInput().compareDocumentPosition(box) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.queryByTestId('risk-suggestion-k1')).not.toBeInTheDocument();
    await userEvent.click(within(box).getByRole('button', { name: /Partir d’un risque courant/ }));
    await userEvent.click(screen.getByTestId('risk-suggestion-k1'));
    expect(titleInput().value).toBe('Ransomware sur le SI');
    expect(screen.getByText('Actifs concernés')).toBeInTheDocument();
    expect(screen.queryByText('Assets impactés')).not.toBeInTheDocument();
  });

  it('starts on level 3 of each and writes the midpoint of the level that is picked', async () => {
    renderModal();
    expect(screen.getByTestId('create-risk-p-3')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('create-risk-i-3')).toHaveAttribute('aria-pressed', 'true');

    await userEvent.click(screen.getByTestId('create-risk-p-4'));
    await userEvent.click(screen.getByTestId('create-risk-i-5'));
    // The exact value is always printed beside the level.
    expect(screen.getByTestId('create-risk-p')).toHaveTextContent('0,70');
    expect(screen.getByTestId('create-risk-i')).toHaveTextContent('9,0');

    fireEvent.change(titleInput(), { target: { value: 'Fuite de données API' } });
    fireEvent.click(submit());
    await waitFor(() => expect(createRiskMock).toHaveBeenCalled());
    expect(createRiskMock.mock.calls[0][0]).toMatchObject({
      title: 'Fuite de données API',
      probability: 0.7,
      impact: 9,
      source: 'manual',
    });
  });

  it('sends the category, the owner and the linked assets, and hands the new risk back', async () => {
    assetList.current = [
      { id: 'a1', name: 'Exchange' },
      { id: 'a2', name: 'Active Directory' },
    ];
    const { onCreated, onClose } = renderModal();
    fireEvent.change(titleInput(), { target: { value: 'Compromission de la messagerie' } });

    await userEvent.click(screen.getByTestId('create-risk-category'));
    await userEvent.click(await screen.findByRole('option', { name: 'Cyber' }));
    await userEvent.click(screen.getByTestId('create-risk-owner'));
    await userEvent.click(await screen.findByRole('option', { name: 'Aïcha Diallo' }));
    await userEvent.click(screen.getByTestId('create-risk-assets'));
    await userEvent.click(await screen.findByRole('option', { name: 'Exchange' }));
    // The picked asset is a removable chip, and no longer offered.
    expect(screen.getByRole('button', { name: 'Retirer Exchange' })).toBeInTheDocument();

    fireEvent.click(submit());
    await waitFor(() => expect(createRiskMock).toHaveBeenCalled());
    expect(createRiskMock.mock.calls[0][0]).toMatchObject({
      category_id: 'cat-1',
      owner_id: 'u1',
      asset_ids: ['a1'],
    });
    await waitFor(() =>
      expect(onCreated).toHaveBeenCalledWith({ id: 'r1', title: 'Fuite de données API' }),
    );
    expect(onClose).toHaveBeenCalled();
  });

  it('shows the live score the server computed, on the register scale, with the server band', () => {
    preview.current = {
      risk_score: 3.5,
      risk_criticality: 'medium',
      // The 0-100 model score rides along; it is not what this card shows.
      inherent: 42.4,
      inherent_band: 'high',
    };
    renderModal();
    const card = screen.getByTestId('create-risk-score');
    // One decimal, no scale suffix: the same format the register prints.
    expect(card).toHaveTextContent('3,5');
    expect(card).not.toHaveTextContent('/ 100');
    expect(card).not.toHaveTextContent('42');
    // The band is the server's `risk_criticality`, not a threshold applied here.
    expect(card).toHaveTextContent('Moyen');
    expect(card).toHaveTextContent('Score inhérent (probabilité × impact × criticité)');
  });

  it('shows no figure rather than a zero until the server has answered', () => {
    renderModal();
    expect(screen.getByTestId('create-risk-score')).toHaveTextContent('—');
  });

  it('keeps its submit button reachable: header, scrolling body and pinned footer', () => {
    renderModal();
    const dialog = screen.getByRole('dialog');
    expect(dialog.className).toMatch(/max-h-/);
    expect(dialog.className).toMatch(/flex-col/);
    const footer = within(dialog).getByRole('contentinfo');
    expect(within(footer).getByRole('button', { name: 'Créer le risque' })).toBeInTheDocument();
    // The submit lives outside the form, wired by `form=`, so the footer can stay pinned.
    expect(submit()).toHaveAttribute('form', 'create-risk-form');
  });

  it('closes on Escape', async () => {
    const { onClose } = renderModal();
    fireEvent.keyDown(document, { key: 'Escape' });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('starts from the finding it is opened for (title and assets)', () => {
    renderModal({ initialTitle: 'Faille SSL-VPN', initialAssetIds: ['a1'] });
    expect(titleInput().value).toBe('Faille SSL-VPN');
  });

  it('speaks English when the language is English', () => {
    useUIStore.setState({ lang: 'en' });
    renderModal();
    expect(screen.getByRole('button', { name: 'Create risk' })).toBeInTheDocument();
    expect(
      screen.getByText('It will be added to the register with the status “Open”.'),
    ).toBeInTheDocument();
  });
});
