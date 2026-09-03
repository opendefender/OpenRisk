// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Design-system primitives: the guarantees a screen is allowed to rely on.
 *
 * These tests are deliberately about BEHAVIOUR and ACCESSIBILITY, not about
 * class names. Asserting `className` contains 'bg-accent-solid' would pass
 * whether or not the button is usable and would fail on every refactor — it
 * tests the implementation back to itself. What a screen actually depends on is
 * that a loading button cannot be double-submitted, that a field's error is
 * announced, that a dialog gives focus back, and that a badge's meaning survives
 * being read aloud.
 *
 * Visual correctness is covered separately, by the pixel snapshots in
 * e2e/visual, which is the right tool for it.
 */

import { describe, expect, it, vi } from 'vitest';
import { useState } from 'react';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Bug, Trash2 } from 'lucide-react';

import { Badge } from '../Badge';
import { riskStatusIntent, severityIntent } from '../badgeIntents';
import { Button } from '../Button';
import { Field, Input, Select, Textarea } from '../Field';
import { Modal } from '../Modal';
import { Drawer } from '../Drawer';
import { TabPanel, Tabs } from '../Tabs';
import { PermissionDenied } from '../States';
import { categorical, seriesColor, severity, chartAccessibleProps } from '../chart';

/* ------------------------------------------------------------------ Button -- */

describe('Button', () => {
  it('is named by its text', () => {
    render(<Button>Create risk</Button>);
    expect(screen.getByRole('button', { name: 'Create risk' })).toBeInTheDocument();
  });

  it('takes its accessible name from aria-label when it is icon-only', () => {
    // The type system requires aria-label here; this proves the name actually
    // reaches the accessibility tree rather than just satisfying the compiler.
    render(<Button icon={Trash2} aria-label="Delete asset" />);
    expect(screen.getByRole('button', { name: 'Delete asset' })).toBeInTheDocument();
  });

  it('cannot be activated twice while loading', async () => {
    const onClick = vi.fn();
    const user = userEvent.setup();
    render(
      <Button loading onClick={onClick}>
        Save
      </Button>,
    );

    const button = screen.getByRole('button', { name: 'Save' });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute('aria-busy', 'true');

    await user.click(button);
    expect(onClick).not.toHaveBeenCalled();
  });

  it('keeps its label visible while loading', () => {
    // A spinner that replaces the label leaves a screen-reader user with a
    // button that has no name for the duration of the request.
    render(<Button loading>Approve</Button>);
    expect(screen.getByRole('button', { name: 'Approve' })).toBeInTheDocument();
  });

  it('defaults to type=button so it cannot submit a form by accident', () => {
    render(<Button>Filter</Button>);
    expect(screen.getByRole('button', { name: 'Filter' })).toHaveAttribute('type', 'button');
  });

  it('submits when asked to', () => {
    render(
      <form>
        <Button type="submit">Sign in</Button>
      </form>,
    );
    expect(screen.getByRole('button', { name: 'Sign in' })).toHaveAttribute('type', 'submit');
  });
});

/* ------------------------------------------------------------------- Field -- */

describe('Field', () => {
  it('associates its label with the control', () => {
    render(
      <Field label="Asset name">
        <Input />
      </Field>,
    );
    expect(screen.getByLabelText('Asset name')).toBeInTheDocument();
  });

  it('exposes the description and the error to the control, in that order', () => {
    render(
      <Field
        label="CVSS"
        description="Base score from the scanner"
        message="Must be 0–10"
        status="invalid"
      >
        <Input />
      </Field>,
    );

    const input = screen.getByLabelText(/CVSS/);
    const described = (input.getAttribute('aria-describedby') ?? '').split(' ');
    expect(described).toHaveLength(2);

    // The hint first, then the reason it was rejected — a screen reader reads
    // them in this order, and "what this is" before "why it failed" is the
    // order that makes the failure understandable.
    expect(document.getElementById(described[0])).toHaveTextContent('Base score from the scanner');
    expect(document.getElementById(described[1])).toHaveTextContent('Must be 0–10');
  });

  it('marks the control invalid and announces the error', () => {
    render(
      <Field label="Email" message="That address is already registered" status="invalid">
        <Input />
      </Field>,
    );
    expect(screen.getByLabelText('Email')).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByRole('alert')).toHaveTextContent('That address is already registered');
  });

  it('does not announce a message that is not an error', () => {
    // The same slot carries hints and warnings. Only a rejection interrupts.
    render(
      <Field label="Tags" message="Comma separated">
        <Input />
      </Field>,
    );
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('puts "required" in the accessible name, not only in a red asterisk', () => {
    render(
      <Field label="Title" required>
        <Input />
      </Field>,
    );
    // WCAG 1.4.1: a coloured glyph is not information a screen reader receives.
    expect(screen.getByLabelText(/Title.*required/i)).toBeRequired();
  });

  it('wires a Textarea and a Select the same way as an Input', () => {
    const { unmount } = render(
      <Field label="Description">
        <Textarea />
      </Field>,
    );
    expect(screen.getByLabelText('Description')).toBeInTheDocument();
    unmount();

    render(
      <Field label="Severity">
        <Select>
          <option>Critical</option>
        </Select>
      </Field>,
    );
    expect(screen.getByLabelText('Severity')).toBeInTheDocument();
  });

  it('lets a control stand alone without a Field', () => {
    // A bare search box has a placeholder and no label stack; it must still
    // render rather than depending on a context that is not there.
    render(<Input aria-label="Search" />);
    expect(screen.getByLabelText('Search')).toBeInTheDocument();
  });
});

/* ------------------------------------------------------------------- Badge -- */

describe('Badge', () => {
  it('always carries its meaning as text', () => {
    render(<Badge intent="danger">Critical</Badge>);
    expect(screen.getByText('Critical')).toBeInTheDocument();
  });

  it('hides the decorative dot from assistive technology', () => {
    const { container } = render(
      <Badge intent="warning" dot>
        Degraded
      </Badge>,
    );
    expect(container.querySelector('[aria-hidden="true"]')).toBeInTheDocument();
  });

  it('announces changes only when it is the live state of the page', () => {
    const { rerender } = render(<Badge intent="neutral">Draft</Badge>);
    expect(screen.queryByRole('status')).not.toBeInTheDocument();

    rerender(
      <Badge intent="success" live>
        Mitigated
      </Badge>,
    );
    expect(screen.getByRole('status')).toHaveTextContent('Mitigated');
  });

  it('maps every domain value to an intent', () => {
    // Exhaustiveness is enforced by the type, so this guards the OTHER half:
    // that no value was mapped to something meaningless.
    expect(Object.values(riskStatusIntent).every(Boolean)).toBe(true);

    // Medium must not collapse into high, or the badge stops distinguishing
    // the two values it exists to distinguish.
    expect(severityIntent.medium).not.toBe(severityIntent.high);
    expect(severityIntent.critical).toBe('danger');
  });
});

/* ------------------------------------------------------------------- Modal -- */

function ModalHarness({ dismissable = true }: { dismissable?: boolean }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Open dialog
      </button>
      <Modal
        open={open}
        onClose={() => setOpen(false)}
        title="Revoke token"
        subtitle="ci-pipeline-token"
        dismissable={dismissable}
        footer={<Button variant="destructive">Revoke</Button>}
      >
        <Input aria-label="Reason" />
      </Modal>
    </>
  );
}

describe('Modal', () => {
  it('is a dialog labelled by its title and described by its subtitle', async () => {
    const user = userEvent.setup();
    render(<ModalHarness />);
    await user.click(screen.getByRole('button', { name: 'Open dialog' }));

    const dialog = screen.getByRole('dialog', { name: 'Revoke token' });
    expect(dialog).toHaveAttribute('aria-modal', 'true');
    expect(dialog).toHaveAccessibleDescription('ci-pipeline-token');
  });

  it('moves focus into the dialog and gives it back to the trigger on close', async () => {
    const user = userEvent.setup();
    render(<ModalHarness />);

    const trigger = screen.getByRole('button', { name: 'Open dialog' });
    await user.click(trigger);

    const dialog = await screen.findByRole('dialog');
    // Focus lands somewhere inside — not on the page behind. Awaited because
    // it is deliberately deferred a frame: focusing an element that is still
    // mid-transform makes some browsers scroll the container.
    await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true));

    await user.keyboard('{Escape}');

    // The part that is usually missing: without the restore, dismissing drops
    // a keyboard user at the top of the document and they lose their place.
    expect(document.activeElement).toBe(trigger);
  });

  it('keeps Tab inside the dialog', async () => {
    const user = userEvent.setup();
    render(<ModalHarness />);
    await user.click(screen.getByRole('button', { name: 'Open dialog' }));

    const dialog = await screen.findByRole('dialog');
    // Enough tabs to walk past every control in the dialog and out the other
    // side, if the trap were not there.
    for (let i = 0; i < 8; i += 1) {
      await user.tab();
      expect(dialog.contains(document.activeElement)).toBe(true);
    }
  });

  it('cannot be dismissed while its action is in flight', async () => {
    const user = userEvent.setup();
    render(<ModalHarness dismissable={false} />);
    await user.click(screen.getByRole('button', { name: 'Open dialog' }));

    await user.keyboard('{Escape}');

    // Still open: closing here would leave the user with no feedback about an
    // operation that is already half-applied.
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    // And there is no close affordance offering to do it either.
    expect(within(screen.getByRole('dialog')).queryByRole('button', { name: /close/i })).toBeNull();
  });

  it('restores the page scroll when it closes', async () => {
    const user = userEvent.setup();
    render(<ModalHarness />);

    await user.click(screen.getByRole('button', { name: 'Open dialog' }));
    expect(document.body.style.overflow).toBe('hidden');

    await user.keyboard('{Escape}');
    expect(document.body.style.overflow).not.toBe('hidden');
  });
});

/* ------------------------------------------------------------------ Drawer -- */

describe('Drawer', () => {
  it('is a dialog too, with the same focus contract', async () => {
    const user = userEvent.setup();

    function Harness() {
      const [open, setOpen] = useState(false);
      return (
        <>
          <button type="button" onClick={() => setOpen(true)}>
            Open asset
          </button>
          <Drawer open={open} onClose={() => setOpen(false)} title="web-prod-01" subtitle="Server">
            <p>Detail</p>
          </Drawer>
        </>
      );
    }

    render(<Harness />);
    const trigger = screen.getByRole('button', { name: 'Open asset' });
    await user.click(trigger);

    expect(screen.getByRole('dialog', { name: 'web-prod-01' })).toBeInTheDocument();

    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(document.activeElement).toBe(trigger);
  });
});

/* -------------------------------------------------------------------- Tabs -- */

function TabsHarness() {
  const [value, setValue] = useState('overview');
  const items = [
    { id: 'overview', label: 'Overview' },
    { id: 'evidence', label: 'Evidence', count: 3 },
    { id: 'audit', label: 'Audit' },
  ] as const;
  return (
    <>
      <Tabs id="t" items={items} value={value} onChange={setValue} label="Risk sections" />
      {items.map((item) => (
        <TabPanel key={item.id} tabsId="t" id={item.id} active={item.id === value}>
          {item.label} panel
        </TabPanel>
      ))}
    </>
  );
}

describe('Tabs', () => {
  it('exposes the WAI-ARIA tabs structure', () => {
    render(<TabsHarness />);
    expect(screen.getByRole('tablist', { name: 'Risk sections' })).toBeInTheDocument();
    expect(screen.getAllByRole('tab')).toHaveLength(3);
    expect(screen.getByRole('tab', { name: /Overview/ })).toHaveAttribute('aria-selected', 'true');
  });

  it('keeps exactly one tab in the tab order', () => {
    // Roving tabindex: Tab enters the tablist once and then moves ON to the
    // panel, rather than walking through every tab.
    render(<TabsHarness />);
    const inOrder = screen
      .getAllByRole('tab')
      .filter((tab) => tab.getAttribute('tabindex') === '0');
    expect(inOrder).toHaveLength(1);
  });

  it('moves between tabs with the arrow keys and wraps at the end', async () => {
    const user = userEvent.setup();
    render(<TabsHarness />);

    screen.getByRole('tab', { name: /Overview/ }).focus();
    await user.keyboard('{ArrowRight}');
    expect(screen.getByRole('tab', { name: /Evidence/ })).toHaveAttribute('aria-selected', 'true');

    await user.keyboard('{ArrowRight}{ArrowRight}');
    // Wrapped back to the first: a dead end at the edge of a tablist reads as
    // the app being broken.
    expect(screen.getByRole('tab', { name: /Overview/ })).toHaveAttribute('aria-selected', 'true');

    await user.keyboard('{End}');
    expect(screen.getByRole('tab', { name: 'Audit' })).toHaveAttribute('aria-selected', 'true');
  });

  it('renders only the active panel', () => {
    render(<TabsHarness />);
    expect(screen.getByText('Overview panel')).toBeInTheDocument();
    // Hiding with CSS would leave the other panels' focusable content reachable.
    expect(screen.queryByText('Audit panel')).not.toBeInTheDocument();
  });
});

/* -------------------------------------------------------- permission denied -- */

describe('PermissionDenied', () => {
  it('names the permission that would grant access', () => {
    render(<PermissionDenied resource="the audit trail" requiredPermission="governance:read" />);

    expect(screen.getByText(/audit trail/)).toBeInTheDocument();
    // The difference between a dead end and something a user can act on.
    expect(screen.getByText('governance:read')).toBeInTheDocument();
  });
});

/* ------------------------------------------------------------------- chart -- */

describe('chart palette', () => {
  it('gives every series a colour, wrapping rather than running out', () => {
    expect(seriesColor(0)).toBe(categorical[0]);
    expect(seriesColor(categorical.length)).toBe(categorical[0]);
    expect(seriesColor(11)).toBeTruthy();
  });

  it('resolves through tokens so charts follow the theme', () => {
    // The whole reason these are var() strings and not hex: an SVG attribute
    // reads the variable, so a theme switch needs no re-render.
    expect(categorical.every((c) => c.startsWith('var(--chart-'))).toBe(true);
  });

  it('keeps categories out of the semantic palette', () => {
    // A category is not a verdict. Reusing the risk scale for "series 4" is
    // how a neutral bar chart ends up looking like an alert.
    const risk = Object.values(severity);
    expect(categorical.some((c) => risk.includes(c as (typeof risk)[number]))).toBe(false);
  });

  it('makes a chart declare whether it carries meaning', () => {
    expect(chartAccessibleProps('Open vulnerabilities by month')).toEqual({
      role: 'img',
      'aria-label': 'Open vulnerabilities by month',
    });
    expect(chartAccessibleProps({ decorativeBecauseTableFollows: true })).toEqual({
      role: 'presentation',
      'aria-hidden': true,
    });
  });
});

/* ------------------------------------------------------------------- icons -- */

describe('icon usage', () => {
  it('hides a button icon from the accessible name', () => {
    // Otherwise the name becomes "bug Vulnerabilities".
    render(<Button icon={Bug}>Vulnerabilities</Button>);
    expect(screen.getByRole('button', { name: 'Vulnerabilities' })).toBeInTheDocument();
  });
});
