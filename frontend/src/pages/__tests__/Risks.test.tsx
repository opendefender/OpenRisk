// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/** @vitest-environment jsdom */

import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import Risks from '../Risks';
import { vi, describe, it, expect, beforeEach } from 'vitest';

// Mock the useRiskStore hook so that both usages (no-arg and selector) work
const mockFetch = vi.fn();
const mockSetSelected = vi.fn();

const store = {
  risks: [
    {
      id: '1',
      title: 'Risk 1',
      description: 'Description 1',
      score: 10,
      impact: 3,
      probability: 4,
      status: 'OPEN',
      tags: ['tag1'],
      source: 'test',
    },
  ],
  total: 12,
  page: 1,
  pageSize: 5,
  isLoading: false,
  fetchRisks: mockFetch,
  setPage: vi.fn(),
  setSelectedRisk: mockSetSelected,
};

vi.mock('../../hooks/useRiskStore', () => ({
  useRiskStore: (selector?: any) => (typeof selector === 'function' ? selector(store) : store),
}));

describe('Risks page - pagination', () => {
  beforeEach(() => {
    mockFetch.mockClear();
    mockSetSelected.mockClear();
  });

  it('calls fetchRisks on mount with initial page and limit', async () => {
    render(<Risks />);

    // initial useEffect should call fetchRisks with page 1 and limit 5
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith({ page: 1, limit: 5, sort_by: 'score', sort_dir: 'desc' });
    });
  });

  it('clicking next page calls fetchRisks with incremented page', async () => {
    render(<Risks />);

    // Wait for initial fetch
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());
    mockFetch.mockClear();

    // Find the page info element and its sibling next button
    const pageInfos = screen.getAllByText(/1 \/ 3/);
    const pageInfo = pageInfos[0];
    const nextButton = pageInfo.nextElementSibling as HTMLElement;
    expect(nextButton).toBeTruthy();

    fireEvent.click(nextButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith({ page: 2, limit: 5, sort_by: 'score', sort_dir: 'desc' });
    });
  });

  it('clicking previous page calls fetchRisks with decremented page (bounded)', async () => {
    render(<Risks />);

    // initial fetch
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());
    mockFetch.mockClear();

    const pageInfos = screen.getAllByText(/1 \/ 3/);
    const pageInfo = pageInfos[0];
    const prevButton = pageInfo.previousElementSibling as HTMLElement;
    expect(prevButton).toBeTruthy();

    // clicking previous on page 1 should stay on page 1 and not re-trigger fetch
    fireEvent.click(prevButton);

    // since the page value does not change, fetchRisks should not be called again
    await waitFor(() => {
      expect(mockFetch).not.toHaveBeenCalled();
    });
  });
});
