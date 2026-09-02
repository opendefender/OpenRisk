// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Feedback primitives: Spinner, AlertDialog and Empty. #443 PR 2 (Empty landed
 * later, once D-021 unblocked the licence question that kept it out).
 *
 * Same doctrine as the other two suites — behaviour and accessibility, never
 * class names. For AlertDialog the behaviour under test is almost entirely about
 * what happens when the user does the reflexive thing: presses Enter, presses
 * Escape, clicks twice. A confirmation dialog is the one component where the
 * default action being wrong costs data.
 */

import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { Spinner } from '../Spinner';
import { AlertDialog } from '../AlertDialog';
import { LoadingState } from '../States';
import { Empty } from '../Empty';
import { EmptyState } from '../../EmptyState';

/* ----------------------------------------------------------------- Spinner -- */

describe('Spinner', () => {
  it('is silent by default, because adjacent text already says what is happening', () => {
    const { container } = render(<Spinner />);
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
    expect(container.querySelector('[aria-hidden="true"]')).toBeInTheDocument();
  });

  it('becomes a live region when it is the only indication', () => {
    render(<Spinner label="Loading results" />);
    // role="status" is not named from its contents, so the assertion is on what
    // the live region will announce rather than on an accessible name.
    expect(screen.getByRole('status')).toHaveTextContent('Loading results');
  });

  it('falls back to the md size on an unexpected value', () => {
    // @ts-expect-error outside the union on purpose: a bad value must not
    // produce an icon with no size.
    const { container } = render(<Spinner size="enormous" />);
    expect(container.querySelector('svg')).toHaveAttribute('width', '18');
  });

  it('is the glyph LoadingState renders, so there is one spinner in the system', () => {
    render(<LoadingState label="Loading the register" />);
    const status = screen.getByRole('status');
    // The block owns the live region and the caption; the atom inside it is
    // hidden, so the label is announced once rather than twice.
    expect(status).toHaveTextContent('Loading the register');
    expect(status.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });
});

/* ------------------------------------------------------------- AlertDialog -- */

function Harness(props: Partial<React.ComponentProps<typeof AlertDialog>> = {}) {
  return (
    <AlertDialog
      open
      onCancel={() => {}}
      onConfirm={() => {}}
      title="Delete ISO 27001:2022?"
      description="This removes the framework and all 93 of its controls."
      confirmLabel="Delete framework"
      {...props}
    />
  );
}

describe('AlertDialog', () => {
  it('is an alertdialog, not a dialog, and is named and described by its own text', () => {
    render(<Harness />);
    const dialog = screen.getByRole('alertdialog', { name: 'Delete ISO 27001:2022?' });
    expect(dialog).toHaveAccessibleDescription(/93 of its controls/);
  });

  /* The assertion this component exists for. A dialog that focuses the
     destructive button turns a reflexive Enter into data loss. */
  it('puts initial focus on the SAFE answer, even when the confirm is destructive', async () => {
    render(<Harness tone="destructive" />);
    const cancel = screen.getByRole('button', { name: 'Cancel' });
    await vi.waitFor(() => expect(cancel).toHaveFocus());
  });

  it('so pressing Enter immediately cancels rather than confirming', async () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(<Harness tone="destructive" onConfirm={onConfirm} onCancel={onCancel} />);

    await vi.waitFor(() => expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus());
    await user.keyboard('{Enter}');

    expect(onConfirm).not.toHaveBeenCalled();
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('treats Escape as "no"', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(<Harness onCancel={onCancel} />);
    await user.keyboard('{Escape}');
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('confirms only on a deliberate click of the confirm button', async () => {
    const onConfirm = vi.fn();
    const user = userEvent.setup();
    render(<Harness onConfirm={onConfirm} />);
    await user.click(screen.getByRole('button', { name: 'Delete framework' }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('cannot be dismissed or double-submitted while the action is in flight', async () => {
    const onCancel = vi.fn();
    const onConfirm = vi.fn();
    const user = userEvent.setup();
    render(<Harness busy onCancel={onCancel} onConfirm={onConfirm} />);

    await user.keyboard('{Escape}');
    expect(onCancel).not.toHaveBeenCalled();

    expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled();

    // The confirm button is in its loading state and must not fire again.
    await user.click(screen.getByRole('button', { name: /Delete framework/ }));
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('renders nothing when closed', () => {
    render(<Harness open={false} />);
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
  });
});

/* --------------------------------------------------------------------- axe -- */

describe('accessibility (axe-core)', () => {
  it('finds no serious or critical violation on the open dialog', async () => {
    const axe = (await import('axe-core')).default;
    const { baseElement } = render(<Harness tone="destructive" />);

    const results = await axe.run(baseElement, {
      resultTypes: ['violations'],
      rules: { 'color-contrast': { enabled: false } },
    });
    const serious = results.violations.filter(
      (v) => v.impact === 'serious' || v.impact === 'critical',
    );
    expect(serious.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  }, 20_000);
});

/* ------------------------------------------------------------------- Empty -- */

describe('Empty', () => {
  it('announces itself, because it replaces content the user was expecting', () => {
    render(<Empty title="No risks yet" />);
    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.getByText('No risks yet')).toBeInTheDocument();
  });

  it('exposes the variant, which is what the E2E fresh-tenant sweep asserts on', () => {
    const { rerender } = render(<Empty title="Nothing" />);
    // first-use is the default on purpose: a fresh tenant has not failed at
    // anything, and "no data" is the wrong thing to say to it.
    expect(screen.getByTestId('empty-state')).toHaveAttribute('data-variant', 'first-use');

    rerender(<Empty title="Nothing" variant="no-results" />);
    expect(screen.getByTestId('empty-state')).toHaveAttribute('data-variant', 'no-results');
  });

  it('renders both actions when given them, and neither slot when not', () => {
    const { rerender } = render(<Empty title="Nothing" />);
    expect(screen.queryByRole('button')).not.toBeInTheDocument();

    rerender(
      <Empty
        title="Nothing"
        primaryAction={<button type="button">Create</button>}
        secondaryAction={<button type="button">Import</button>}
      />,
    );
    expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Import' })).toBeInTheDocument();
  });

  it('opens the learn-more link safely in a new tab', () => {
    render(<Empty title="Nothing" learnMoreHref="https://docs.example/risks" />);
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('target', '_blank');
    // rel=noreferrer, or the new tab can reach back through window.opener.
    expect(link).toHaveAttribute('rel', 'noreferrer');
  });

  it('is still reachable under its old name and path (D-021 shim)', () => {
    // The 24 existing importers were deliberately NOT rewritten — Résolution 1
    // point 4. If this fails, the shim is gone and every one of them is broken.
    expect(EmptyState).toBe(Empty);
  });
});
