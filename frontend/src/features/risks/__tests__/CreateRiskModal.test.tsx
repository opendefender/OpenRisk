// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/** @vitest-environment jsdom */
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

const createRiskMock = vi.fn(() => Promise.resolve({ id: 'r1', title: 'Fuite de données API' }));
const fetchAssetsMock = vi.fn(() => Promise.resolve());

vi.mock('../../../hooks/useAssetStore', () => ({
  useAssetStore: () => ({ assets: [], fetchAssets: fetchAssetsMock, isLoading: false }),
}));

vi.mock('../../../services/riskService', async () => {
  const actual = await vi.importActual<typeof import('../../../services/riskService')>(
    '../../../services/riskService',
  );
  return {
    ...actual,
    riskService: {
      ...actual.riskService,
      createRisk: (...args: unknown[]) => createRiskMock(...args),
    },
  };
});

vi.mock('../useTaxonomy', () => ({
  CATEGORIES_KEY: ['risk-categories'],
  IMPORTED_FRAMEWORKS_KEY: ['compliance', 'frameworks', 'imported'],
  useRiskCategories: () => ({ data: [] }),
  useImportedFrameworks: () => ({ data: [] }),
  useFrameworkControls: () => ({ data: [] }),
}));

vi.mock('../../onboarding/useActivation', () => ({
  useOnboardingSuggestions: () => ({ data: undefined }),
  useInvalidateActivation: () => vi.fn(),
}));

vi.mock('../../../hooks/useScore', () => ({
  useScorePreview: () => ({ data: undefined }),
}));

import { CreateRiskModal } from '../CreateRiskModal';

describe('CreateRiskModal', () => {
  beforeEach(() => {
    createRiskMock.mockClear();
  });

  // Review defect (#751 phase 2): flashSuccess() and the modal's own close
  // happened in the same batch, and Modal renders nothing once closed — so
  // the drawn check glyph was dead code, never actually painted. The fix
  // removes the wiring outright; the toast stays the only confirmation.
  it('never wires the drawn success check — the toast is the only confirmation', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <CreateRiskModal isOpen onClose={vi.fn()} />
      </QueryClientProvider>,
    );

    const title = document.querySelector('input[name="title"]') as HTMLInputElement;
    const description = document.querySelector(
      'textarea[name="description"]',
    ) as HTMLTextAreaElement;
    fireEvent.change(title, { target: { value: 'Fuite de données API' } });
    fireEvent.change(description, {
      target: { value: 'Description suffisamment longue pour la validation.' },
    });

    fireEvent.click(screen.getByRole('button', { name: /Enregistrer|Save/i }));

    await waitFor(() => expect(createRiskMock).toHaveBeenCalled());

    // Button's feedback="success" renders a drawn check as an svg <path>
    // with pathLength=1 / stroke-dasharray=1 (see shared/ds/Button.tsx). It
    // must never appear, because this component never passes `feedback`.
    expect(document.querySelector('path[stroke-dasharray="1"]')).not.toBeInTheDocument();
  });
});
