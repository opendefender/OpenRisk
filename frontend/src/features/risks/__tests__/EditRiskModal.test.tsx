// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/** @vitest-environment jsdom */
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach } from 'vitest';

const updateRiskMock = vi.fn(() => Promise.resolve());
const fetchAssetsMock = vi.fn(() => Promise.resolve());

vi.mock('../../../hooks/useRiskStore', () => ({
  useRiskStore: () => ({ updateRisk: updateRiskMock }),
}));

vi.mock('../../../hooks/useAssetStore', () => ({
  useAssetStore: () => ({ assets: [], fetchAssets: fetchAssetsMock }),
}));

import { EditRiskModal } from '../components/EditRiskModal';

describe('EditRiskModal', () => {
  beforeEach(() => {
    updateRiskMock.mockClear();
  });

  it('renders existing risk and updates', async () => {
    const onClose = vi.fn();
    const risk = { id: '1', title: 'Old', description: 'Old desc', impact: 2, probability: 2, tags: [] };
    render(<EditRiskModal isOpen={true} onClose={onClose} risk={risk} />);

    const title = screen.getByLabelText(/Titre/i);
    fireEvent.change(title, { target: { value: 'Updated Title' } });

    const submit = screen.getByRole('button', { name: /Enregistrer/i });
    fireEvent.click(submit);

    await waitFor(() => expect(updateRiskMock).toHaveBeenCalledWith('1', expect.any(Object)));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
