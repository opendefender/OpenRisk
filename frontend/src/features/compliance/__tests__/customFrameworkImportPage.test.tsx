// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Custom framework JSON import wizard (#307, criterion 8). The server is
// mocked at the service boundary.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import axe from 'axe-core';
import { AxiosError, type AxiosResponse } from 'axios';

import { useUIStore } from '../../../store/uiStore';
import { CustomFrameworkImportPage } from '../CustomFrameworkImportPage';

vi.mock('../../../services/complianceService', async () => {
  const actual = await vi.importActual<typeof import('../../../services/complianceService')>(
    '../../../services/complianceService',
  );
  return {
    ...actual,
    complianceService: { ...actual.complianceService, importFramework: vi.fn() },
  };
});

import { complianceService } from '../../../services/complianceService';

const importFramework = vi.mocked(complianceService.importFramework);

const VALID = {
  schema_version: 1,
  framework: { name: 'Circulaire 2026', version: '1.0', description: 'Interne' },
  controls: [
    { reference_code: 'A.1', name: 'Gouvernance', description: '', source_reference: 'Art. 1' },
    { reference_code: 'A.2', name: 'Accès', description: '', source_reference: '' },
  ],
};

function jsonFile(body: unknown, name = 'framework.json'): File {
  const text = typeof body === 'string' ? body : JSON.stringify(body);
  const file = new File([text], name, { type: 'application/json' });
  // jsdom's File has no text() in some versions.
  if (typeof file.text !== 'function') {
    Object.defineProperty(file, 'text', { value: () => Promise.resolve(text) });
  }
  return file;
}

function Harness() {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  return (
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/compliance/frameworks/import']}>
        <Routes>
          <Route path="/compliance/frameworks/import" element={<CustomFrameworkImportPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  const invalidate = vi.spyOn(client, 'invalidateQueries');
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/compliance/frameworks/import']}>
        <Routes>
          <Route path="/compliance/frameworks/import" element={<CustomFrameworkImportPage />} />
          <Route path="/compliance/frameworks/:frameworkId" element={<p>detail-page</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { invalidate };
}

async function choose(file: File) {
  const input = screen.getByTestId('framework-import-file-input') as HTMLInputElement;
  fireEvent.change(input, { target: { files: [file] } });
}

function axiosFailure(status: number, data: unknown): AxiosError {
  return new AxiosError('failed', 'ERR_BAD_REQUEST', undefined, undefined, {
    status,
    data,
  } as AxiosResponse);
}

beforeEach(() => {
  importFramework.mockReset();
  useUIStore.setState({ lang: 'en' });
});

describe('CustomFrameworkImportPage', () => {
  it('shows the empty state with a CTA before any file is chosen', () => {
    renderPage();
    const empty = screen.getByTestId('framework-import-empty');
    expect(within(empty).getByRole('button', { name: /choose a file/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /import the framework/i })).toBeNull();
  });

  it('renders in French too', () => {
    useUIStore.setState({ lang: 'fr' });
    renderPage();
    expect(
      screen.getByRole('heading', { name: /Importer un référentiel personnalisé \(JSON\)/ }),
    ).toBeInTheDocument();
  });

  it('refuses a non-JSON file name before reading it', async () => {
    renderPage();
    await choose(new File(['x'], 'framework.csv'));
    expect(await screen.findByRole('alert')).toHaveTextContent(/only \.json files/i);
  });

  it('previews name, version and control count', async () => {
    renderPage();
    await choose(jsonFile(VALID));
    const preview = await screen.findByTestId('framework-import-preview');
    expect(preview).toHaveTextContent('Circulaire 2026');
    expect(preview).toHaveTextContent('1.0');
    expect(preview).toHaveTextContent('2 controls');
    expect(screen.getByRole('button', { name: /import the framework/i })).toBeEnabled();
  });

  it('shows client-side shape problems before submit and disables submit', async () => {
    renderPage();
    const bad = {
      ...VALID,
      controls: [
        { reference_code: 'A.1', name: 'One' },
        { reference_code: 'A.1', name: 'Dup' },
        { reference_code: '', name: '' },
      ],
    };
    await choose(jsonFile(bad));
    const err = await screen.findByTestId('framework-import-error');
    const rows = within(err).getAllByRole('row').slice(1);
    // Row 2: duplicate code. Row 3: empty code and empty name.
    expect(rows[0]).toHaveTextContent('2');
    expect(rows[0]).toHaveTextContent('reference_code');
    expect(rows.length).toBe(3);
    expect(screen.getByRole('button', { name: /import the framework/i })).toBeDisabled();
    expect(importFramework).not.toHaveBeenCalled();
  });

  it('reports invalid JSON', async () => {
    renderPage();
    await choose(jsonFile('{not json'));
    expect(await screen.findByTestId('framework-import-error')).toHaveTextContent(
      /not valid json/i,
    );
  });

  it('renders the server 400 errors[] as a table (row = index + 1)', async () => {
    importFramework.mockRejectedValue(
      axiosFailure(400, {
        error: 'validation failed',
        errors: [
          { index: 1, field: 'name', message: 'name is required' },
          { index: null, field: 'framework.name', message: 'too long' },
        ],
      }),
    );
    renderPage();
    await choose(jsonFile(VALID));
    fireEvent.click(await screen.findByRole('button', { name: /import the framework/i }));
    const err = await screen.findByTestId('framework-import-error');
    const rows = within(err).getAllByRole('row').slice(1);
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent('2');
    expect(rows[0]).toHaveTextContent('name');
    expect(rows[0]).toHaveTextContent('name is required');
    expect(rows[1]).toHaveTextContent('framework.name');
  });

  it.each([
    [409, /already exists/i],
    [402, /limit is reached/i],
    [403, /do not have permission/i],
    [401, /session has expired/i],
  ])('shows a localized message for %i', async (status, message) => {
    importFramework.mockRejectedValue(axiosFailure(status, { error: 'x' }));
    renderPage();
    await choose(jsonFile(VALID));
    fireEvent.click(await screen.findByRole('button', { name: /import the framework/i }));
    expect(await screen.findByTestId('framework-import-error')).toHaveTextContent(message);
  });

  it('shows a skeleton while importing, then navigates to the new framework and invalidates', async () => {
    let resolve: (v: Awaited<ReturnType<typeof importFramework>>) => void = () => undefined;
    importFramework.mockReturnValue(new Promise((r) => (resolve = r)));
    const { invalidate } = renderPage();
    await choose(jsonFile(VALID));
    fireEvent.click(await screen.findByRole('button', { name: /import the framework/i }));
    expect(await screen.findByTestId('framework-import-skeleton')).toBeInTheDocument();

    resolve({ id: 'new-fw-id', name: 'Circulaire 2026', controls_created: 2 } as Awaited<
      ReturnType<typeof importFramework>
    >);
    expect(await screen.findByText('detail-page')).toBeInTheDocument();
    expect(importFramework).toHaveBeenCalledWith(
      expect.objectContaining({ schema_version: 1, controls: expect.any(Array) }),
    );
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ['compliance', 'frameworks'] }),
    );
  });
});

describe('CustomFrameworkImportPage accessibility', () => {
  async function expectNoSeriousViolations(container: HTMLElement) {
    const results = await axe.run(container, {
      rules: { 'color-contrast': { enabled: false }, region: { enabled: false } },
    });
    const serious = results.violations.filter(
      (v) => v.impact === 'serious' || v.impact === 'critical',
    );
    expect(serious.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  }

  it('has no serious or critical axe violations in the empty state', async () => {
    const { container } = render(<Harness />);
    await screen.findByTestId('framework-import-empty');
    await expectNoSeriousViolations(container);
  });

  it('has no serious or critical axe violations with a valid file previewed', async () => {
    const { container } = render(<Harness />);
    await choose(jsonFile(VALID));
    await screen.findByRole('button', { name: /import the framework/i });
    await expectNoSeriousViolations(container);
  });

  it('has no serious or critical axe violations with the client-side error table', async () => {
    const { container } = render(<Harness />);
    await choose(
      jsonFile({
        ...VALID,
        controls: [
          { reference_code: 'A.1', name: 'One' },
          { reference_code: 'A.1', name: 'Dup' },
        ],
      }),
    );
    await screen.findByTestId('framework-import-error');
    await expectNoSeriousViolations(container);
  });
});
