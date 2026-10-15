// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Listbox, the console's select (#935).
 *
 * jsdom has no layout, so placement (flip, size, hide) is checked live, not
 * here. What is tested is the contract the owner asked for in D-069 C: the
 * keyboard, the search, the states, a value that survives Escape and Tab, the
 * Field wiring, Escape inside a modal, and axe.
 */

import { describe, expect, it, vi } from 'vitest';
import { useState } from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { Listbox, type ListboxOption } from '../Listbox';
import { Field } from '../Field';
import { Modal } from '../Modal';
import { ListboxLabelsContext, DEFAULT_LISTBOX_LABELS } from '../listboxLabels';

const ROLES: ListboxOption[] = [
  { value: 'viewer', label: 'Lecteur' },
  { value: 'analyst', label: 'Analyste' },
  { value: 'auditor', label: 'Auditeur', disabled: true },
  { value: 'admin', label: 'Administrateur' },
];

const MANY: ListboxOption[] = [
  'Règlement',
  'Réseau',
  'Contrôles',
  'Fournisseurs',
  'Incidents',
  'Actifs',
  'Risques',
  'Conformité',
  'Vulnérabilités',
  'Rapports',
].map((label, i) => ({ value: `v${i}`, label }));

function Harness({
  options = ROLES,
  initial = 'viewer',
  onChange,
  ...rest
}: Partial<React.ComponentProps<typeof Listbox>> & {
  initial?: string;
  onChange?: (v: string) => void;
}) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <Field label="Rôle">
        <Listbox
          value={value}
          options={options}
          onChange={(v) => {
            setValue(v);
            onChange?.(v);
          }}
          {...rest}
        />
      </Field>
      <button type="button">Après</button>
    </>
  );
}

const combo = () => screen.getByRole('combobox', { name: /Rôle/ });
const activeLabel = () => {
  const id = combo().getAttribute('aria-activedescendant');
  return id ? document.getElementById(id)?.textContent : null;
};

describe('Listbox — mouse', () => {
  it('opens on click, picks an option, closes and reports the value', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);

    expect(combo()).toHaveTextContent('Lecteur');
    expect(combo()).toHaveAttribute('aria-expanded', 'false');
    await user.click(combo());
    expect(screen.getByRole('listbox')).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Lecteur' })).toHaveAttribute(
      'aria-selected',
      'true',
    );

    await user.click(screen.getByRole('option', { name: 'Analyste' }));
    expect(onChange).toHaveBeenCalledWith('analyst');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(combo()).toHaveTextContent('Analyste');
    expect(combo()).toHaveFocus();
  });

  it('ignores a disabled option', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    await user.click(combo());
    const auditor = screen.getByRole('option', { name: 'Auditeur' });
    expect(auditor).toHaveAttribute('aria-disabled', 'true');
    await user.click(auditor);
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole('listbox')).toBeInTheDocument();
  });

  it('closes on an outside press without changing the value', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    await user.click(combo());
    await user.click(screen.getByRole('button', { name: 'Après' }));
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
  });
});

describe('Listbox — keyboard', () => {
  it.each(['{Enter}', ' ', '{Alt>}{ArrowDown}{/Alt}', '{ArrowDown}'])(
    'opens with %s on the current value',
    async (keys) => {
      const user = userEvent.setup();
      render(<Harness initial="analyst" />);
      combo().focus();
      await user.keyboard(keys);
      expect(combo()).toHaveAttribute('aria-expanded', 'true');
      expect(activeLabel()).toBe('Analyste');
    },
  );

  it('moves with the arrows, skips disabled options, does not wrap', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    combo().focus();
    await user.keyboard('{Enter}');
    expect(activeLabel()).toBe('Lecteur');
    await user.keyboard('{ArrowDown}');
    expect(activeLabel()).toBe('Analyste');
    await user.keyboard('{ArrowDown}');
    expect(activeLabel()).toBe('Administrateur'); // Auditeur is disabled
    await user.keyboard('{ArrowDown}');
    expect(activeLabel()).toBe('Administrateur');
    await user.keyboard('{ArrowUp}{ArrowUp}{ArrowUp}{ArrowUp}');
    expect(activeLabel()).toBe('Lecteur');
  });

  it('jumps with Home and End, and picks with Enter', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    combo().focus();
    await user.keyboard('{Enter}{End}');
    expect(activeLabel()).toBe('Administrateur');
    await user.keyboard('{Home}');
    expect(activeLabel()).toBe('Lecteur');
    await user.keyboard('{End}{Enter}');
    expect(onChange).toHaveBeenCalledWith('admin');
    expect(combo()).toHaveAttribute('aria-expanded', 'false');
    expect(combo()).toHaveFocus();
  });

  it('types ahead on the labels, open or closed', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    combo().focus();
    await user.keyboard('a');
    expect(combo()).toHaveAttribute('aria-expanded', 'true');
    expect(activeLabel()).toBe('Analyste');
    await user.keyboard('a'); // same letter again cycles, skipping disabled Auditeur
    expect(activeLabel()).toBe('Administrateur');
  });

  it('Escape closes and keeps the value', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    combo().focus();
    await user.keyboard('{Enter}{ArrowDown}{Escape}');
    expect(combo()).toHaveAttribute('aria-expanded', 'false');
    expect(combo()).toHaveTextContent('Lecteur');
    expect(onChange).not.toHaveBeenCalled();
    expect(combo()).toHaveFocus();
  });

  it('Tab closes without picking and moves on', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    combo().focus();
    await user.keyboard('{Enter}{ArrowDown}');
    await user.tab();
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Après' })).toHaveFocus();
  });

  it('Escape inside a modal closes the list, not the modal', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <Modal open onClose={onClose} title="Inviter">
        <Harness />
      </Modal>,
    );
    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument());
    combo().focus();
    await user.keyboard('{Enter}');
    expect(screen.getByRole('listbox')).toBeInTheDocument();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    await user.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

describe('Listbox — inside dialogs', () => {
  it('Tab from the last control of a modal stays in the modal', async () => {
    const user = userEvent.setup();
    render(
      <Modal open onClose={() => {}} title="Inviter">
        <Field label="Rôle">
          <Listbox value="viewer" options={ROLES} onChange={() => {}} />
        </Field>
      </Modal>,
    );
    const dialog = await screen.findByRole('dialog');
    combo().focus();
    await user.keyboard('{Enter}');
    await user.tab();
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    expect(combo()).not.toHaveFocus();
  });

  it('points aria-controls only at a real listbox', async () => {
    const user = userEvent.setup();
    render(<Harness loading />);
    await user.click(combo());
    expect(combo()).not.toHaveAttribute('aria-controls');
  });
});

describe('Listbox — search', () => {
  it('has no search field at or under the threshold', async () => {
    const user = userEvent.setup();
    render(<Harness options={MANY.slice(0, 8)} initial="v0" />);
    await user.click(combo());
    expect(screen.queryByRole('combobox', { name: 'Search the list' })).not.toBeInTheDocument();
  });

  it('filters past the threshold, ignoring accents, and picks with Enter', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness options={MANY} initial="v0" onChange={onChange} />);
    await user.click(combo());
    const search = screen.getByRole('combobox', { name: 'Search the list' });
    expect(search).toHaveFocus();

    await user.type(search, 'vulne');
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual(['Vulnérabilités']);
    await user.keyboard('{Enter}');
    expect(onChange).toHaveBeenCalledWith('v8');
    expect(combo()).toHaveFocus();
  });

  it('a letter on the closed control starts the search', async () => {
    const user = userEvent.setup();
    render(<Harness options={MANY} initial="v0" />);
    combo().focus();
    await user.keyboard('r');
    const search = screen.getByRole('combobox', { name: 'Search the list' });
    expect(search).toHaveValue('r');
    expect(search.getAttribute('aria-activedescendant')).toBeTruthy();
  });

  it('says so when nothing matches, and Escape keeps the value', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness options={MANY} initial="v3" onChange={onChange} />);
    await user.click(combo());
    await user.keyboard('zzz');
    expect(screen.getByRole('status')).toHaveTextContent('No match');
    await user.keyboard('{Escape}');
    expect(combo()).toHaveTextContent('Fournisseurs');
    expect(onChange).not.toHaveBeenCalled();
  });

  it('Tab from the search field leaves through the trigger', async () => {
    const user = userEvent.setup();
    render(<Harness options={MANY} initial="v0" />);
    await user.click(combo());
    await user.tab();
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Après' })).toHaveFocus();
  });
});

describe('Listbox — states', () => {
  it('disabled: does not open', async () => {
    const user = userEvent.setup();
    render(<Harness disabled />);
    expect(combo()).toBeDisabled();
    await user.click(combo());
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('disabled through the Field', () => {
    render(
      <Field label="Rôle" disabled>
        <Listbox value="viewer" options={ROLES} onChange={() => {}} />
      </Field>,
    );
    expect(combo()).toBeDisabled();
  });

  it('loading: busy trigger, a notice instead of options', async () => {
    const user = userEvent.setup();
    render(<Harness loading />);
    expect(combo()).toHaveAttribute('aria-busy', 'true');
    await user.click(combo());
    expect(screen.queryByRole('option')).not.toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('Loading…');
  });

  it('loading: keeps focus and blocks picking from the keyboard', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness loading onChange={onChange} />);
    combo().focus();
    await user.keyboard('{Enter}{ArrowDown}{Enter}');
    expect(onChange).not.toHaveBeenCalled();
    expect(combo()).toHaveFocus();
  });

  it('empty list: a notice, and the caller can word it', async () => {
    const user = userEvent.setup();
    render(<Harness options={[]} initial="" emptyLabel="Aucun membre" placeholder="Personne" />);
    expect(combo()).toHaveTextContent('Personne');
    await user.click(combo());
    expect(screen.getByRole('status')).toHaveTextContent('Aucun membre');
  });

  it('error: wired to the Field message', () => {
    render(
      <Field label="Rôle" status="invalid" message="Choisissez un rôle." required>
        <Listbox value="" options={ROLES} onChange={() => {}} />
      </Field>,
    );
    expect(combo()).toHaveAttribute('aria-invalid', 'true');
    expect(combo()).toHaveAttribute('aria-required', 'true');
    expect(combo()).toHaveAccessibleDescription('Choisissez un rôle.');
    expect(combo().className).toContain('border-danger');
  });

  it('submits through a hidden input when named', () => {
    const { container } = render(<Harness name="role" initial="admin" />);
    expect(container.querySelector('input[type="hidden"][name="role"]')).toHaveValue('admin');
  });
});

describe('Listbox — labels', () => {
  it('takes its words from the labels context', async () => {
    const user = userEvent.setup();
    render(
      <ListboxLabelsContext.Provider
        value={{
          ...DEFAULT_LISTBOX_LABELS,
          search: 'Rechercher dans la liste',
          placeholder: 'Choisir…',
        }}
      >
        <Harness options={MANY} initial="" />
      </ListboxLabelsContext.Provider>,
    );
    expect(combo()).toHaveTextContent('Choisir…');
    await user.click(combo());
    expect(screen.getByRole('combobox', { name: 'Rechercher dans la liste' })).toHaveFocus();
  });
});

describe('Listbox — accessibility (axe-core)', () => {
  it('finds no serious or critical violation, closed and open', async () => {
    const axe = (await import('axe-core')).default;
    const user = userEvent.setup();
    render(
      <>
        <Harness />
        <Field label="Domaine" status="invalid" message="Requis.">
          <Listbox value="" options={MANY} onChange={() => {}} />
        </Field>
        <Listbox value="v1" options={MANY} onChange={() => {}} aria-label="Filtre" size="sm" />
      </>,
    );
    const run = async () => {
      const results = await axe.run(document.body, {
        resultTypes: ['violations'],
        rules: { 'color-contrast': { enabled: false } },
      });
      return results.violations
        .filter((v) => v.impact === 'serious' || v.impact === 'critical')
        .map((v) => `${v.id}: ${v.help} — ${v.nodes.map((n) => n.html).join(' | ')}`);
    };

    expect(await run()).toEqual([]);
    await user.click(combo());
    expect(await run()).toEqual([]);
    await user.keyboard('{Escape}');
    await user.click(screen.getByRole('combobox', { name: 'Domaine' }));
    expect(await run()).toEqual([]);
  }, 20_000);
});
