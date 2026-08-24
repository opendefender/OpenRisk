// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { WidgetState } from '../WidgetState';
import { isPermissionError } from '../widgetError';

/** The value a widget must never show when it has not read its data. */
const FABRICATED = '0';

function renderState(props: Partial<React.ComponentProps<typeof WidgetState>>) {
  return render(
    <WidgetState lang="en" isLoading={false} error={null} {...props}>
      <div data-testid="payload">{FABRICATED}</div>
    </WidgetState>
  );
}

describe('WidgetState', () => {
  it('renders the data when there is data', () => {
    renderState({});
    expect(screen.getByTestId('payload')).toBeInTheDocument();
  });

  // THE test of this file. Every failure mode this component exists for ended
  // with a widget rendering a plausible zero it had never read — a cyber score of
  // 0, an exposure of 0 FCFA, "no open vulnerabilities". None of those states may
  // reach the children at all.
  it.each([
    ['loading', { isLoading: true }],
    ['error', { error: new Error('boom') }],
    ['403', { error: { response: { status: 403 } } }],
    ['401', { error: { response: { status: 401 } } }],
    ['empty', { isEmpty: true }],
    ['empty because of the period', { isEmpty: true, emptyBecauseOfPeriod: true }],
  ])('never renders the payload in the %s state', (_name, props) => {
    renderState(props);
    expect(screen.queryByTestId('payload')).not.toBeInTheDocument();
  });

  it('says "you may not read this" for a 403, not "this broke"', () => {
    // The two have different remedies: one is a permission request, the other is
    // a retry. Collapsing them sends the user to the wrong person.
    renderState({ error: { response: { status: 403 } } });
    expect(screen.getByText(/not available to you/i)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /retry/i })).not.toBeInTheDocument();
  });

  it('offers a retry on a genuine failure', () => {
    renderState({ error: new Error('network'), retry: () => {} });
    expect(screen.getByText(/data unavailable/i)).toBeInTheDocument();
    // By role, not by text: the description also uses the word "retry", and the
    // thing being asserted is that there is a BUTTON to press.
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('distinguishes "nothing in this period" from "nothing exists"', () => {
    // Opposite facts, opposite remedies — only one of them is a reason to press
    // a Create button.
    renderState({
      isEmpty: true,
      emptyBecauseOfPeriod: true,
      onWidenPeriod: () => {},
      emptyTitle: 'No risks yet',
    });
    expect(screen.getByText(/nothing in this period/i)).toBeInTheDocument();
    expect(screen.queryByText('No risks yet')).not.toBeInTheDocument();

    cleanupAndRender({ isEmpty: true, emptyTitle: 'No risks yet' });
    expect(screen.getByText('No risks yet')).toBeInTheDocument();
  });

  it('renders the first-use copy and its action when genuinely empty', () => {
    renderState({
      isEmpty: true,
      emptyTitle: 'Empty matrix',
      emptyDescription: 'It fills in with your first risk.',
      emptyAction: <button type="button">Create a risk</button>,
    });
    expect(screen.getByText('Empty matrix')).toBeInTheDocument();
    expect(screen.getByText('It fills in with your first risk.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create a risk' })).toBeInTheDocument();
  });

  it('translates its own states', () => {
    render(
      <WidgetState lang="fr" isLoading={false} error={new Error('x')}>
        <div />
      </WidgetState>
    );
    expect(screen.getByText(/données indisponibles/i)).toBeInTheDocument();
  });
});

/** Re-render inside one `it`, since the suite only auto-cleans between tests. */
function cleanupAndRender(props: Partial<React.ComponentProps<typeof WidgetState>>) {
  document.body.innerHTML = '';
  renderState(props);
}

describe('isPermissionError', () => {
  it('recognises the two statuses that mean "not yours to read"', () => {
    expect(isPermissionError({ response: { status: 401 } })).toBe(true);
    expect(isPermissionError({ response: { status: 403 } })).toBe(true);
  });

  it('treats everything else as a failure to be retried', () => {
    for (const status of [400, 404, 409, 422, 429, 500, 502, 504]) {
      expect(isPermissionError({ response: { status } })).toBe(false);
    }
    // A timeout or a dropped connection has no response at all.
    expect(isPermissionError(new Error('timeout of 5000ms exceeded'))).toBe(false);
    expect(isPermissionError(null)).toBe(false);
    expect(isPermissionError(undefined)).toBe(false);
  });
});
