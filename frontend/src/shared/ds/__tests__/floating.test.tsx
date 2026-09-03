// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Popover and Menu. #443 PR 3.
 *
 * Positioning is not tested here and deliberately so: jsdom has no layout, so
 * asserting coordinates would assert nothing. `flip` and `shift` are
 * @floating-ui's, already used by Tooltip, and their job is visible in the
 * gallery snapshot. What IS tested is the part that is ours and the part every
 * hand-rolled version in this codebase got wrong — the keyboard contract.
 */

import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Trash2 } from 'lucide-react';

import { Popover } from '../Popover';
import { Menu } from '../Menu';
import { Button } from '../Button';

/* ---------------------------------------------------------------- Popover -- */

describe('Popover', () => {
  const Harness = () => (
    <Popover trigger={<Button>Filters</Button>} label="Filters">
      <label htmlFor="q">Query</label>
      <input id="q" />
    </Popover>
  );

  it('opens on click and closes on a second click', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Filters' }));
    expect(await screen.findByRole('dialog', { name: 'Filters' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Filters' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('can be typed into — the content is reachable, not just visible', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole('button', { name: 'Filters' }));

    const input = await screen.findByLabelText('Query');
    await user.type(input, 'critical');
    expect(input).toHaveValue('critical');
  });

  it('closes on Escape and gives focus back to the trigger', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const trigger = screen.getByRole('button', { name: 'Filters' });

    await user.click(trigger);
    await screen.findByRole('dialog');
    await user.keyboard('{Escape}');

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });
});

/* ------------------------------------------------------------------- Menu -- */

describe('Menu', () => {
  const items = [
    { label: 'Duplicate', onSelect: vi.fn() },
    { label: 'Export', onSelect: vi.fn() },
    { label: 'Delete', onSelect: vi.fn(), destructive: true, icon: Trash2 },
  ];

  const Harness = (props: Partial<React.ComponentProps<typeof Menu>> = {}) => (
    <Menu trigger={<Button>Row actions</Button>} items={items} {...props} />
  );

  it('is a menu of menuitems, named by its trigger', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole('button', { name: 'Row actions' }));

    // The conventional pattern, and floating-ui's default: the menu takes its
    // name from the control that opened it.
    const menu = await screen.findByRole('menu', { name: 'Row actions' });
    expect(within(menu).getAllByRole('menuitem')).toHaveLength(3);
  });

  it('lets `label` override that, for an icon-only trigger', async () => {
    const user = userEvent.setup();
    render(<Harness label="Risk row actions" />);
    await user.click(screen.getByRole('button', { name: 'Row actions' }));

    // Regression guard: getFloatingProps() sets aria-labelledby to the trigger,
    // which wins over aria-label, so a label applied before the spread is
    // silently inert. It has to come after, clearing labelledby with it.
    expect(await screen.findByRole('menu', { name: 'Risk row actions' })).toBeInTheDocument();
  });

  /* The ARIA menu pattern is a roving tab stop, not a list of tab stops. This is
     the assertion that fails if someone rebuilds this with plain buttons. */
  it('moves through items with the arrow keys', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole('button', { name: 'Row actions' }));
    await screen.findByRole('menu');

    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('menuitem', { name: 'Duplicate' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('menuitem', { name: 'Export' })).toHaveFocus();
  });

  it('activates with Enter and closes', async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<Harness items={[{ label: 'Duplicate', onSelect }]} />);

    await user.click(screen.getByRole('button', { name: 'Row actions' }));
    await screen.findByRole('menu');
    await user.keyboard('{ArrowDown}{Enter}');

    expect(onSelect).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument());
  });

  it('sorts destructive actions to the end, whatever order they were passed', async () => {
    const user = userEvent.setup();
    render(
      <Harness
        items={[
          { label: 'Delete', onSelect: vi.fn(), destructive: true },
          { label: 'Duplicate', onSelect: vi.fn() },
        ]}
      />,
    );
    await user.click(screen.getByRole('button', { name: 'Row actions' }));
    const names = (await screen.findAllByRole('menuitem')).map((n) => n.textContent);
    expect(names).toEqual(['Duplicate', 'Delete']);
  });

  it('does not fire a disabled item', async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<Harness items={[{ label: 'Archive', onSelect, disabled: true }]} />);

    await user.click(screen.getByRole('button', { name: 'Row actions' }));
    const item = await screen.findByRole('menuitem', { name: 'Archive' });
    expect(item).toBeDisabled();
    await user.click(item);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('closes on Escape and gives focus back to the trigger', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const trigger = screen.getByRole('button', { name: 'Row actions' });

    await user.click(trigger);
    await screen.findByRole('menu');
    await user.keyboard('{Escape}');

    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
  });
});

/* -------------------------------------------------------------------- axe -- */

describe('accessibility (axe-core)', () => {
  it('finds no serious or critical violation on an open menu', async () => {
    const axe = (await import('axe-core')).default;
    const user = userEvent.setup();
    const { baseElement } = render(
      <Menu
        trigger={<Button>Row actions</Button>}
        items={[
          { label: 'Duplicate', onSelect: vi.fn() },
          { label: 'Delete', onSelect: vi.fn(), destructive: true },
        ]}
      />,
    );
    await user.click(screen.getByRole('button', { name: 'Row actions' }));
    await screen.findByRole('menu');

    /*
     * @floating-ui's FloatingFocusManager renders two `role="button"` focus
     * guards with no content, which axe flags as unnamed commands. They are
     * library internals, clipped to a 1px box, and exist precisely so focus
     * cannot escape the layer — never announced in practice. Excluded by
     * attribute rather than by turning the rule off, so an unnamed command in
     * OUR markup still fails.
     */
    const results = await axe.run(
      { include: [baseElement], exclude: [['[data-floating-ui-focus-guard]']] },
      { resultTypes: ['violations'], rules: { 'color-contrast': { enabled: false } } },
    );
    const serious = results.violations.filter(
      (v) => v.impact === 'serious' || v.impact === 'critical',
    );
    expect(serious.map((v) => `${v.id} — ${v.nodes.map((n) => n.html).join(' | ')}`)).toEqual([]);
  }, 20_000);
});
