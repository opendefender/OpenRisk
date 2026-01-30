/ @vitest-environment jsdom /
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
    const risk = { id: '', title: 'Old', description: 'Old desc', impact: , probability: , tags: [] };
    render(<EditRiskModal isOpen={true} onClose={onClose} risk={risk} />);

    const title = screen.getByLabelText(/Titre/i);
    fireEvent.change(title, { target: { value: 'Updated Title' } });

    const submit = screen.getByRole('button', { name: /Enregistrer/i });
    fireEvent.click(submit);

    await waitFor(() => expect(updateRiskMock).toHaveBeenCalledWith('', expect.any(Object)));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
