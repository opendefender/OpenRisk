/ @vitest-environment jsdom /
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
    const submit = screen.getByRole('button', { name: /Crer le Risque/i });
    fireEvent.click(submit);

    await waitFor(() => expect(createRiskMock).toHaveBeenCalled());
    expect(fetchRisksMock).toHaveBeenCalled();
    // onClose should be called after success
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
