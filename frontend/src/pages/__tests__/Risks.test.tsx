/ @vitest-environment jsdom /

import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import Risks from '../Risks';
import { vi, describe, it, expect, beforeEach } from 'vitest';

// Mock the useRiskStore hook so that both usages (no-arg and selector) work
const mockFetch = vi.fn();
const mockSetSelected = vi.fn();

const store = {
  risks: [
    {
      id: '',
      title: 'Risk ',
      description: 'Description ',
      score: ,
      impact: ,
      probability: ,
      status: 'OPEN',
      tags: ['tag'],
      source: 'test',
    },
  ],
  total: ,
  page: ,
  pageSize: ,
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

    // initial useEffect should call fetchRisks with page  and limit 
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith({ page: , limit: , sort_by: 'score', sort_dir: 'desc' });
    });
  });

  it('clicking next page calls fetchRisks with incremented page', async () => {
    render(<Risks />);

    // Wait for initial fetch
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());
    mockFetch.mockClear();

    // Find the page info element and its sibling next button
    const pageInfos = screen.getAllByText(/ \/ /);
    const pageInfo = pageInfos[];
    const nextButton = pageInfo.nextElementSibling as HTMLElement;
    expect(nextButton).toBeTruthy();

    fireEvent.click(nextButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith({ page: , limit: , sort_by: 'score', sort_dir: 'desc' });
    });
  });

  it('clicking previous page calls fetchRisks with decremented page (bounded)', async () => {
    render(<Risks />);

    // initial fetch
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());
    mockFetch.mockClear();

    const pageInfos = screen.getAllByText(/ \/ /);
    const pageInfo = pageInfos[];
    const prevButton = pageInfo.previousElementSibling as HTMLElement;
    expect(prevButton).toBeTruthy();

    // clicking previous on page  should stay on page  and not re-trigger fetch
    fireEvent.click(prevButton);

    // since the page value does not change, fetchRisks should not be called again
    await waitFor(() => {
      expect(mockFetch).not.toHaveBeenCalled();
    });
  });
});
