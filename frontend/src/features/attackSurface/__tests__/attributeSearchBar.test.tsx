// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The `select-name` half of #589.
//
// `tests/e2e/a11y.spec.ts` reported "Select element must have an accessible
// name" on /assets, on BOTH desktop and mobile — so the offender was in shared
// markup, not a responsive branch. It is this bar: four selects and a text input
// with no label, no aria-label and no aria-labelledby between them. A screen
// reader announced four unnamed combo boxes in a row.
//
// These assert the resolved NAMES rather than the attributes, because the name
// is what the user hears; an aria-label that exists but resolves to an empty
// string would pass an attribute check and fail a person.

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';

vi.mock('../useAssetSchemas', () => ({
  useAssetSchemas: () => ({
    defsFor: (category: string) =>
      category
        ? [
            { key: 'os', label: 'Système', type: 'enum', enum: ['linux', 'windows'] },
            { key: 'exposed', label: 'Exposé', type: 'boolean' },
            { key: 'owner', label: 'Propriétaire', type: 'text' },
          ]
        : [],
  }),
}));

import { AttributeSearchBar } from '../AttributeSearchBar';

describe('AttributeSearchBar accessible names', () => {
  it('names the category select', () => {
    render(<AttributeSearchBar category="" attributes={{}} onChange={vi.fn()} />);
    expect(screen.getByRole('combobox', { name: /Catégorie d’actif/i })).toBeInTheDocument();
  });

  it('leaves no unnamed select once a category is chosen', () => {
    render(<AttributeSearchBar category="server" attributes={{}} onChange={vi.fn()} />);

    const unnamed = screen
      .getAllByRole('combobox')
      .filter((el) => !el.getAttribute('aria-label') && !el.getAttribute('aria-labelledby'));

    expect(unnamed).toEqual([]);
  });

  it('names each active filter’s remove button after the filter it removes', () => {
    render(<AttributeSearchBar category="server" attributes={{ os: 'linux' }} onChange={vi.fn()} />);
    // Not a bare "Retirer": the announcement has to say WHICH filter (WCAG 2.4.6).
    expect(
      screen.getByRole('button', { name: /Retirer le filtre Système = linux/i }),
    ).toBeInTheDocument();
  });
});
