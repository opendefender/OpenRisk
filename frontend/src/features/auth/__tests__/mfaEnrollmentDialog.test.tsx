// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 5 — same auto-submit contract as the AuthScreen enrolment field
// (see mfaOtpAutoSubmit.test.tsx), plus this dialog's own copy-to-clipboard
// icon/label swap.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

const setupMFA = vi.fn();
const verifyMFA = vi.fn();
const invalidateStatus = vi.fn();
const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock('../authService', () => ({
  setupMFA: (...a: unknown[]) => setupMFA(...a),
  verifyMFA: (...a: unknown[]) => verifyMFA(...a),
}));
vi.mock('../useMfa', () => ({
  useInvalidateMFAStatus: () => invalidateStatus,
}));
vi.mock('sonner', () => ({
  toast: { success: (...a: unknown[]) => toastSuccess(...a), error: (...a: unknown[]) => toastError(...a) },
}));

import { MFAEnrollmentDialog } from '../MFAEnrollmentDialog';
import { useUIStore } from '../../../store/uiStore';

function renderDialog() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const onClose = vi.fn();
  const onEnrolled = vi.fn();
  render(
    <QueryClientProvider client={qc}>
      <MFAEnrollmentDialog onClose={onClose} onEnrolled={onEnrolled} />
    </QueryClientProvider>,
  );
  return { onClose, onEnrolled };
}

beforeEach(() => {
  vi.clearAllMocks();
  // The store defaults to French (the primary market) — pin English directly
  // so the aria-label/copy assertions below are deterministic.
  useUIStore.setState({ lang: 'en' });
  setupMFA.mockResolvedValue({
    secret: 'ABCDEF',
    qr_code: '/9j/rawbase64',
    backup_codes: ['ABCDEFGH2345'],
  });
});

describe('MFAEnrollmentDialog', () => {
  it('has an OtpField labelled by the control itself, not just its wrapping text', async () => {
    renderDialog();
    // OtpField's `label` prop sets aria-label directly, so this resolves even
    // if the surrounding markup is later un-nested from a wrapping <label>.
    expect(await screen.findByRole('textbox', { name: '6-digit code' })).toBeInTheDocument();
  });

  it('submits once the sixth digit lands, with no button press', async () => {
    verifyMFA.mockResolvedValue(undefined);
    const { onEnrolled } = renderDialog();
    const user = userEvent.setup();

    const field = await screen.findByRole('textbox', { name: '6-digit code' });
    await user.click(field);
    await user.paste('654321');

    await waitFor(() => expect(verifyMFA).toHaveBeenCalledTimes(1));
    expect(verifyMFA).toHaveBeenCalledWith('654321');
    await waitFor(() => expect(onEnrolled).toHaveBeenCalled());
  });

  it('does not fire a second submit while the first is still in flight', async () => {
    let resolveVerify!: () => void;
    verifyMFA.mockReturnValue(new Promise<void>((resolve) => (resolveVerify = resolve)));
    renderDialog();
    const user = userEvent.setup();

    const field = await screen.findByRole('textbox', { name: '6-digit code' });
    await user.click(field);
    await user.paste('654321');
    await waitFor(() => expect(verifyMFA).toHaveBeenCalledTimes(1));

    await user.click(field);
    await user.paste('654321');

    resolveVerify();
    await waitFor(() => expect(invalidateStatus).toHaveBeenCalled());
    expect(verifyMFA).toHaveBeenCalledTimes(1);
  });

  it('clears a rejected code’s error as soon as the user edits it', async () => {
    verifyMFA.mockRejectedValueOnce(new Error('invalid'));
    renderDialog();
    const user = userEvent.setup();

    const field = await screen.findByRole('textbox', { name: '6-digit code' });
    await user.click(field);
    await user.paste('111111');

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());

    // The field is already at its 6-character maxLength, so typing another
    // digit is a no-op — clear it first so the value actually changes. One
    // digit is enough to prove the error clears, without completing the code
    // again and firing a second submit.
    await user.clear(field);
    await user.type(field, '2');

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('names the copy button by the result of pressing it, and announces the copy', async () => {
    renderDialog();
    const user = userEvent.setup();

    const copyButton = await screen.findByRole('button', { name: 'Copy the key' });
    // jsdom ships a real Clipboard implementation (backed by an in-memory item
    // list, not the OS clipboard) that is only reachable once the document has
    // rendered — spy on its own `writeText` here rather than at module scope.
    const clipboardWriteText = vi
      .spyOn(navigator.clipboard, 'writeText')
      .mockResolvedValue(undefined);
    await user.click(copyButton);

    expect(clipboardWriteText).toHaveBeenCalledWith('ABCDEF');
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument();
    expect(toastSuccess).toHaveBeenCalledWith('Key copied');
  });
});
