// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// GHSA-45mr-j2w6-m4jv — the code step of an organization switch. The switcher
// hands the step over in memory; this screen shows the challenge or the
// enrolment with that token, and leaving goes back to the current organization.

import { StrictMode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';

const toastError = vi.fn();
vi.mock('sonner', () => ({ toast: { error: (m: string) => toastError(m) } }));
// The real challenge and enrolment screens are AuthScreen's and tested there;
// here only which one is shown, with which token, and where leaving goes.
vi.mock('../AuthScreen', () => ({
  MFAChallenge: ({
    token,
    cancelLabel,
    onCancel,
    onRestart,
  }: {
    token: string;
    cancelLabel?: string;
    onCancel: () => void;
    onRestart: (m: string) => void;
  }) => (
    <div data-testid="mfa-challenge">
      <span>{token}</span>
      <button onClick={onCancel}>{cancelLabel}</button>
      <button onClick={() => onRestart('expired')}>restart</button>
    </div>
  ),
  MFAEnrollment: ({
    token,
    cancelLabel,
    onCancel,
  }: {
    token: string;
    cancelLabel?: string;
    onCancel?: () => void;
  }) => (
    <div data-testid="mfa-enrolment">
      <span>{token}</span>
      <button onClick={onCancel}>{cancelLabel}</button>
    </div>
  ),
}));
vi.mock('../AuthLayout', () => ({
  AuthLayout: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

import { useUIStore } from '../../../store/uiStore';
import {
  clearSwitchStep,
  holdSwitchStep,
  peekSwitchStep,
} from '../../organization/pendingSwitchStep';
import { SwitchOrgMFAScreen } from '../SwitchOrgMFAScreen';

function renderScreen() {
  return render(
    <StrictMode>
      <MemoryRouter initialEntries={['/auth/switch-org/mfa']}>
        <Routes>
          <Route path="/auth/switch-org/mfa" element={<SwitchOrgMFAScreen />} />
          <Route path="/" element={<p>current organization</p>} />
        </Routes>
      </MemoryRouter>
    </StrictMode>,
  );
}

beforeEach(() => {
  clearSwitchStep();
  toastError.mockReset();
  useUIStore.setState({ lang: 'en' });
});

describe('SwitchOrgMFAScreen', () => {
  it('shows the challenge with the step token, and staying goes back', () => {
    holdSwitchStep({ token: 'mfa-for-b', enrolling: false });
    renderScreen();

    expect(screen.getByTestId('mfa-challenge')).toHaveTextContent('mfa-for-b');
    fireEvent.click(screen.getByRole('button', { name: 'Stay in the current organisation' }));
    expect(screen.getByText('current organization')).toBeInTheDocument();
    // Unmounted: the token is not kept around.
    expect(peekSwitchStep()).toBeNull();
  });

  it('shows the enrolment for an enrolment step', () => {
    holdSwitchStep({ token: 'enrol-in-b', enrolling: true });
    renderScreen();

    expect(screen.getByTestId('mfa-enrolment')).toHaveTextContent('enrol-in-b');
    // An enrolment that cannot finish must not trap the user outside the app.
    fireEvent.click(screen.getByRole('button', { name: 'Stay in the current organisation' }));
    expect(screen.getByText('current organization')).toBeInTheDocument();
  });

  it('goes back with the reason when the step is spent', () => {
    holdSwitchStep({ token: 'mfa-for-b', enrolling: false });
    renderScreen();

    fireEvent.click(screen.getByRole('button', { name: 'restart' }));
    expect(toastError).toHaveBeenCalledWith('expired');
    expect(screen.getByText('current organization')).toBeInTheDocument();
  });

  it('TestSwitchOrgMFA_NotFound — with no step (a reload), goes back to the current organization', () => {
    renderScreen();

    expect(screen.getByText('current organization')).toBeInTheDocument();
  });
});
