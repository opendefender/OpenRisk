// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * The app side of the design-system Listbox (#935): its words come from the
 * catalogue, and Escape over a dialog closed through useEscapeToClose closes
 * the list first. Lives outside shared/ds, which may not import the AGPL core.
 */

import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { Listbox, type ListboxOption } from '../ds';
import { ListboxI18nProvider } from '../ListboxI18n';
import { useEscapeToClose } from '../useBackTo';

const MANY: ListboxOption[] = Array.from({ length: 10 }, (_, i) => ({
  value: `v${i}`,
  label: `Option ${i}`,
}));

describe('ListboxI18nProvider', () => {
  it('gives the Listbox its words in the interface language', async () => {
    const user = userEvent.setup();
    render(
      <ListboxI18nProvider>
        <Listbox value="" options={MANY} onChange={() => {}} aria-label="Domaine" />
      </ListboxI18nProvider>,
    );
    const combo = screen.getByRole('combobox', { name: 'Domaine' });
    expect(combo).toHaveTextContent('Choisir…');
    await user.click(combo);
    expect(screen.getByRole('combobox', { name: 'Rechercher dans la liste' })).toHaveFocus();
  });
});

describe('Listbox over useEscapeToClose', () => {
  it('Escape closes the list, then the dialog', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    function EscDialog() {
      useEscapeToClose(true, onClose);
      return (
        <Listbox value="v0" options={MANY.slice(0, 4)} onChange={() => {}} aria-label="Rôle" />
      );
    }
    render(<EscDialog />);
    const combo = screen.getByRole('combobox', { name: 'Rôle' });
    combo.focus();
    await user.keyboard('{Enter}');
    expect(screen.getByRole('listbox')).toBeInTheDocument();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    await user.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
