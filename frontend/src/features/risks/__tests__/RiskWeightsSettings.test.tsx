// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #864 — the weights are derived from the query, not copied into state by an
// effect. The page must still show the server's weights, keep the admin's
// edits, save exactly what is shown, and reset to the engine defaults.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

const SERVER = {
  business_criticality: 0.2,
  internet_exposure: 0.1,
  vulnerabilities: 0.2,
  control_maturity: 0.1,
  incident_history: 0.1,
  exploitability: 0.1,
  financial_value: 0.1,
  threat_intel: 0.1,
};

const mutateAsync = vi.fn();
let weightsData: typeof SERVER | undefined = SERVER;

vi.mock('../useSmartScore', () => ({
  useRiskWeights: () => ({ data: weightsData, isLoading: false, isError: false, refetch: vi.fn() }),
  useUpdateRiskWeights: () => ({ mutateAsync, isPending: false }),
}));
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (sel: (s: { hasRole: (r: string) => boolean }) => unknown) =>
    sel({ hasRole: () => true }),
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { RiskWeightsSettings } from '../RiskWeightsSettings';
import { useUIStore } from '../../../store/uiStore';

const renderPage = () =>
  render(
    <MemoryRouter>
      <RiskWeightsSettings />
    </MemoryRouter>,
  );

const sliders = () => screen.getAllByRole('slider') as HTMLInputElement[];

describe('RiskWeightsSettings', () => {
  beforeEach(() => {
    useUIStore.getState().setLang('en');
    mutateAsync.mockReset().mockResolvedValue(undefined);
    weightsData = SERVER;
  });

  it('shows the server’s weights on first render', () => {
    renderPage();
    expect(sliders()).toHaveLength(8);
    expect(sliders()[0]?.value).toBe('0.2');
  });

  it('keeps an edit and saves exactly the weights shown', async () => {
    renderPage();
    fireEvent.change(sliders()[0]!, { target: { value: '0.35' } });
    expect(sliders()[0]?.value).toBe('0.35');

    fireEvent.click(screen.getByText('Save'));
    await vi.waitFor(() => expect(mutateAsync).toHaveBeenCalledTimes(1));
    expect(mutateAsync).toHaveBeenCalledWith({ ...SERVER, business_criticality: 0.35 });
  });

  it('shows the server’s copy again after a save', async () => {
    const view = renderPage();
    fireEvent.change(sliders()[0]!, { target: { value: '0.35' } });
    fireEvent.click(screen.getByText('Save'));
    await vi.waitFor(() => expect(mutateAsync).toHaveBeenCalledTimes(1));

    // The refetch brings the saved value back from the server.
    weightsData = { ...SERVER, business_criticality: 0.4 };
    view.rerender(
      <MemoryRouter>
        <RiskWeightsSettings />
      </MemoryRouter>,
    );
    await vi.waitFor(() => expect(sliders()[0]?.value).toBe('0.4'));
  });

  it('resets to the engine defaults', () => {
    renderPage();
    fireEvent.click(screen.getByText('Defaults'));
    expect(sliders()[0]?.value).toBe('0.15');
  });
});
