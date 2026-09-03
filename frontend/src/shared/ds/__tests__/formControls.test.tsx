// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Form-control primitives: the guarantees a screen may rely on. #443, PR 1.
 *
 * Same doctrine as primitives.test.tsx — behaviour and accessibility, never
 * class names. What a screen depends on here is that a checkbox in a Field
 * announces that Field's error, that a group tells a screen reader what it is a
 * group OF, that a switch says "switch" and not "checkbox", and that the state
 * of every one of them survives being read aloud rather than being carried by
 * colour.
 */

import { describe, expect, it, vi } from 'vitest';
import { useState } from 'react';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { Checkbox, CheckboxGroup } from '../Checkbox';
import { RadioGroup } from '../RadioGroup';
import { Switch } from '../Switch';
import { Fieldset } from '../Fieldset';
import { InputGroup } from '../InputGroup';
import { Label } from '../Label';
import { Field, Input } from '../Field';

/* ---------------------------------------------------------------- Checkbox -- */

describe('Checkbox', () => {
  it('is named by its label and toggles with the keyboard', async () => {
    const user = userEvent.setup();
    render(<Checkbox label="Notify on breach" />);

    const box = screen.getByRole('checkbox', { name: 'Notify on breach' });
    expect(box).not.toBeChecked();

    await user.tab();
    expect(box).toHaveFocus();
    await user.keyboard(' ');
    expect(box).toBeChecked();
  });

  it('is toggled by clicking the label, not only the 16px box', async () => {
    const user = userEvent.setup();
    render(<Checkbox label="Notify on breach" />);
    await user.click(screen.getByText('Notify on breach'));
    expect(screen.getByRole('checkbox')).toBeChecked();
  });

  it('reaches the DOM indeterminate state, which markup cannot express', () => {
    render(<Checkbox label="Some selected" indeterminate />);
    const box = screen.getByRole('checkbox') as HTMLInputElement;
    expect(box.indeterminate).toBe(true);
  });

  it('describes itself with its description', () => {
    render(<Checkbox label="Notify" description="Sends one email per incident." />);
    expect(screen.getByRole('checkbox', { name: 'Notify' })).toHaveAccessibleDescription(
      'Sends one email per incident.',
    );
  });

  it('is named by the Field label when it supplies none of its own', () => {
    render(
      <Field label="I accept the terms">
        <Checkbox />
      </Field>,
    );
    expect(screen.getByRole('checkbox', { name: 'I accept the terms' })).toBeInTheDocument();
  });

  it('inherits its Field wiring, so a form error reaches it', () => {
    render(
      <Field label="Consent" status="invalid" message="You must accept to continue." required>
        <Checkbox label="I accept" />
      </Field>,
    );
    const box = screen.getByRole('checkbox', { name: 'I accept' });
    expect(box).toHaveAttribute('aria-invalid', 'true');
    expect(box).toHaveAttribute('aria-required', 'true');
    expect(box).toHaveAccessibleDescription(/You must accept to continue\./);
  });

  it('does not toggle when disabled', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Checkbox label="Locked" disabled onChange={onChange} />);
    await user.click(screen.getByText('Locked'));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole('checkbox')).not.toBeChecked();
  });
});

/* ----------------------------------------------------------- CheckboxGroup -- */

describe('CheckboxGroup', () => {
  function Harness() {
    const [value, setValue] = useState<string[]>(['email']);
    return (
      <CheckboxGroup
        legend="Notification channels"
        options={[
          { value: 'email', label: 'Email' },
          { value: 'slack', label: 'Slack' },
          { value: 'webhook', label: 'Webhook' },
        ]}
        value={value}
        onValueChange={setValue}
      />
    );
  }

  it('is a real group, named by its legend', () => {
    render(<Harness />);
    const group = screen.getByRole('group', { name: 'Notification channels' });
    expect(within(group).getAllByRole('checkbox')).toHaveLength(3);
  });

  it('adds and removes values without disturbing the others', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    expect(screen.getByRole('checkbox', { name: 'Email' })).toBeChecked();

    await user.click(screen.getByRole('checkbox', { name: 'Slack' }));
    expect(screen.getByRole('checkbox', { name: 'Email' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Slack' })).toBeChecked();

    await user.click(screen.getByRole('checkbox', { name: 'Email' }));
    expect(screen.getByRole('checkbox', { name: 'Email' })).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Slack' })).toBeChecked();
  });

  it('disables every option from the group', () => {
    render(
      <CheckboxGroup
        legend="Channels"
        options={[{ value: 'a', label: 'A' }]}
        value={[]}
        onValueChange={() => {}}
        disabled
      />,
    );
    expect(screen.getByRole('checkbox', { name: 'A' })).toBeDisabled();
  });
});

/* -------------------------------------------------------------- RadioGroup -- */

describe('RadioGroup', () => {
  function Harness() {
    const [value, setValue] = useState<string | null>('low');
    return (
      <RadioGroup
        legend="Default severity"
        options={[
          { value: 'low', label: 'Low' },
          { value: 'medium', label: 'Medium' },
          { value: 'high', label: 'High' },
        ]}
        value={value}
        onValueChange={setValue}
      />
    );
  }

  it('is a group named by its legend, with exactly one selection', () => {
    render(<Harness />);
    const group = screen.getByRole('group', { name: 'Default severity' });
    expect(within(group).getAllByRole('radio')).toHaveLength(3);
    expect(screen.getByRole('radio', { name: 'Low' })).toBeChecked();
  });

  /* The whole reason these are native radios. A div-based radiogroup that
     forgets the roving tab stop makes every option its own Tab stop, and this
     is the assertion that would catch that regression. */
  it('is ONE tab stop, and arrows move the selection inside it', async () => {
    const user = userEvent.setup();
    render(<Harness />);

    await user.tab();
    expect(screen.getByRole('radio', { name: 'Low' })).toHaveFocus();

    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('radio', { name: 'Medium' })).toHaveFocus();
    expect(screen.getByRole('radio', { name: 'Medium' })).toBeChecked();
    expect(screen.getByRole('radio', { name: 'Low' })).not.toBeChecked();
  });

  it('selects by clicking the label', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByText('High'));
    expect(screen.getByRole('radio', { name: 'High' })).toBeChecked();
  });
});

/* ------------------------------------------------------------------ Switch -- */

describe('Switch', () => {
  it('is announced as a switch, not as a checkbox', () => {
    render(<Switch label="Auto-escalate" />);
    expect(screen.getByRole('switch', { name: 'Auto-escalate' })).toBeInTheDocument();
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('reports its state to assistive tech, not only through colour', async () => {
    const user = userEvent.setup();
    render(<Switch label="Auto-escalate" />);
    const toggle = screen.getByRole('switch');
    expect(toggle).not.toBeChecked();
    await user.click(toggle);
    expect(toggle).toBeChecked();
  });

  it('toggles with Space from the keyboard', async () => {
    const user = userEvent.setup();
    render(<Switch label="Auto-escalate" />);
    await user.tab();
    expect(screen.getByRole('switch')).toHaveFocus();
    await user.keyboard(' ');
    expect(screen.getByRole('switch')).toBeChecked();
  });

  it('carries its description into the accessible description', () => {
    render(<Switch label="Auto-escalate" description="Raises severity after 24h with no owner." />);
    expect(screen.getByRole('switch')).toHaveAccessibleDescription(
      'Raises severity after 24h with no owner.',
    );
  });

  it('falls back to the md scale on an unexpected size', () => {
    // @ts-expect-error deliberately outside the union: a bad value must not
    // produce undefined classes and an invisible control.
    render(<Switch label="Odd" size="enormous" />);
    expect(screen.getByRole('switch').className).toContain('h-5');
  });
});

/* ---------------------------------------------------------------- Fieldset -- */

describe('Fieldset', () => {
  it('names the group and cascades disabled to its children', () => {
    render(
      <Fieldset legend="Scope" disabled>
        <Checkbox label="Include archived" />
      </Fieldset>,
    );
    expect(screen.getByRole('group', { name: 'Scope' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Include archived' })).toBeDisabled();
  });

  it('describes the group when a description is given', () => {
    render(
      <Fieldset legend="Scope" description="What the export covers.">
        <Checkbox label="A" />
      </Fieldset>,
    );
    expect(screen.getByRole('group', { name: 'Scope' })).toHaveAccessibleDescription(
      'What the export covers.',
    );
  });
});

/* -------------------------------------------------------------- InputGroup -- */

describe('InputGroup', () => {
  it('keeps the inner control reachable and labelled', async () => {
    const user = userEvent.setup();
    render(
      <Field label="Budget">
        <InputGroup prefix="€" suffix="per year">
          <Input />
        </InputGroup>
      </Field>,
    );
    const input = screen.getByRole('textbox', { name: 'Budget' });
    await user.type(input, '1200');
    expect(input).toHaveValue('1200');
  });

  it('renders its addons as text, so they are readable rather than decorative', () => {
    render(
      <InputGroup prefix="https://">
        <Input aria-label="Endpoint" />
      </InputGroup>,
    );
    expect(screen.getByText('https://')).toBeInTheDocument();
  });
});

/* ------------------------------------------------------------------- Label -- */

describe('Label', () => {
  it('puts the required marker in the accessible name as a word, not a glyph', () => {
    render(
      <>
        <Label htmlFor="owner" required>
          Owner
        </Label>
        <input id="owner" />
      </>,
    );
    // The accessible-name algorithm trims each node, so the marker arrives as
    // "Owner(required)" with no space. What matters is that the WORD is in the
    // name at all; the same permissive match is used for Field in
    // primitives.test.tsx.
    expect(screen.getByLabelText(/Owner.*required/i)).toBeInTheDocument();
  });
});

/* --------------------------------------------------------------------- axe -- */

/**
 * axe-core over every new primitive at once, in both a plain and a Field-wrapped
 * composition. #443 acceptance criterion 2.
 *
 * jsdom has no layout, so the colour-contrast rule cannot run here — that half
 * is covered arithmetically by scripts/check-contrast.mjs (240 token pairs,
 * both themes) and in a real browser by e2e/visual. What this catches is the
 * half jsdom CAN see and which a hand-written control gets wrong: an input with
 * no accessible name, a group with no legend, a duplicated id, an aria
 * attribute that is not allowed on the role it sits on.
 */
describe('accessibility (axe-core)', () => {
  it('finds no serious or critical violation across the form controls', async () => {
    const axe = (await import('axe-core')).default;

    const { container } = render(
      <form>
        <Fieldset legend="Notification settings" description="Applies to this tenant only.">
          <Field
            label="Owner email"
            description="Where the digest goes."
            status="invalid"
            message="Not a valid address."
            required
          >
            <Input defaultValue="not-an-email" />
          </Field>
          <Checkbox label="Notify on breach" description="One email per incident." />
          <Checkbox label="Partially selected" indeterminate />
          <Switch label="Auto-escalate" description="Raises severity after 24h with no owner." />
          <CheckboxGroup
            legend="Channels"
            options={[
              { value: 'email', label: 'Email' },
              { value: 'slack', label: 'Slack' },
            ]}
            value={['email']}
            onValueChange={() => {}}
          />
          <RadioGroup
            legend="Default severity"
            options={[
              { value: 'low', label: 'Low' },
              { value: 'high', label: 'High', description: 'Pages the on-call.' },
            ]}
            value="low"
            onValueChange={() => {}}
          />
          <Label htmlFor="budget" required>
            Budget
          </Label>
          <InputGroup prefix="€" suffix="per year">
            <Input id="budget" />
          </InputGroup>
        </Fieldset>
      </form>,
    );

    const results = await axe.run(container, {
      resultTypes: ['violations'],
      rules: { 'color-contrast': { enabled: false } },
    });

    const serious = results.violations.filter(
      (v) => v.impact === 'serious' || v.impact === 'critical',
    );

    expect(
      serious.map((v) => `${v.id}: ${v.help} — ${v.nodes.map((n) => n.html).join(' | ')}`),
    ).toEqual([]);
  }, 20_000);
});
