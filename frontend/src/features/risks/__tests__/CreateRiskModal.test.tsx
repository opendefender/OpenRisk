// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/** @vitest-environment jsdom */
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach } from 'vitest';

// Mock stores
const createRiskMock = vi.fn(() => Promise.resolve());
const fetchAssetsMock = vi.fn(() => Promise.resolve());
const fetchRisksMock = vi.fn(() => Promise.resolve());

vi.mock('../../../hooks/useRiskStore', () => ({
  useRiskStore: () => ({ createRisk: createRiskMock, fetchRisks: fetchRisksMock }),
}));

vi.mock('../../../hooks/useAssetStore', () => ({
  useAssetStore: () => ({ assets: [], fetchAssets: fetchAssetsMock }),
}));

import { CreateRiskModal } from '../components/CreateRiskModal';

describe('CreateRiskModal', () => {
  beforeEach(() => {
    createRiskMock.mockClear();
    fetchAssetsMock.mockClear();
    fetchRisksMock.mockClear();
  });

  it('renders and submits form', async () => {
    const onClose = vi.fn();
    render(<CreateRiskModal isOpen={true} onClose={onClose} />);

    // Fill title and description
    const title = screen.getByLabelText(/Titre/i);
    const description = screen.getByRole('textbox', { name: /Description/i });
    fireEvent.change(title, { target: { value: 'Test Risk Title' } });
    fireEvent.change(description, { target: { value: 'This is a test description with enough length.' } });

    // Submit
    const submit = screen.getByRole('button', { name: /Créer le Risque/i });
    fireEvent.click(submit);

    await waitFor(() => expect(createRiskMock).toHaveBeenCalled());
    expect(fetchRisksMock).toHaveBeenCalled();
    // onClose should be called after success
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
