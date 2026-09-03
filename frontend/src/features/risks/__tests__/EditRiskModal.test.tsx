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
    // Values on the Score Engine scales: probability 0.0-1.0, impact 0.0-10.0.
    // description must satisfy the form's Zod min(10) so submit is not blocked.
    const risk = {
      id: '1',
      title: 'Old title',
      description: 'Old description text',
      impact: 7,
      probability: 0.4,
      tags: [],
    };
    render(<EditRiskModal isOpen={true} onClose={onClose} risk={risk} />);

    const title = screen.getByLabelText(/Titre/i);
    fireEvent.change(title, { target: { value: 'Updated Title' } });

    const submit = screen.getByRole('button', { name: /Enregistrer/i });
    fireEvent.click(submit);

    await waitFor(() => expect(updateRiskMock).toHaveBeenCalledWith('1', expect.any(Object)));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  // The bug this guards: the form declared probability/impact as 1-5 while the
  // Score Engine (and the server) use 0.0-1.0 and 0.0-10.0. Loading any real
  // risk therefore failed the resolver, handleSubmit never ran, and the Save
  // button did nothing at all — no error, no request, no clue.
  it('saves a risk whose probability is a real Score Engine value', async () => {
    const onClose = vi.fn();
    const risk = {
      id: '42',
      title: 'Fuite de données',
      description: 'Description suffisamment longue',
      impact: 9.5,
      probability: 0.7,
      tags: ['rgpd'],
    };
    render(<EditRiskModal isOpen={true} onClose={onClose} risk={risk} />);

    fireEvent.click(screen.getByRole('button', { name: /Enregistrer/i }));

    await waitFor(() =>
      expect(updateRiskMock).toHaveBeenCalledWith(
        '42',
        expect.objectContaining({ probability: 0.7, impact: 9.5 }),
      ),
    );
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
