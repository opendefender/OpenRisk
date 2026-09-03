// OR26-03 — the deferrable-MFA prompt.
//
// The e2e suite owns the claims that need a real server (a standard user
// reaching the dashboard, an admin blocked past the deadline). These tests own
// what the component is allowed to decide on its own — which is: nothing. Every
// case here proves the banner renders the SERVER's state faithfully, including
// the two cases a security UI most easily gets wrong:
//
//   • an unknown state must never render as "you're protected";
//   • a privileged countdown must not be dismissible.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { MFAEnrollmentBanner } from '../MFAEnrollmentBanner';
import { daysUntilDeadline, shouldPromptEnrollment, type MFAStatus } from '../mfaPolicyService';

// --- mocks -----------------------------------------------------------------

const fetchMFAStatus = vi.fn();
vi.mock('../mfaPolicyService', async () => {
  const actual = await vi.importActual<typeof import('../mfaPolicyService')>('../mfaPolicyService');
  return { ...actual, fetchMFAStatus: (...a: unknown[]) => fetchMFAStatus(...a) };
});

vi.mock('../MFAEnrollmentDialog', () => ({
  MFAEnrollmentDialog: ({ onClose }: { onClose: () => void }) => (
    <div data-testid="mfa-dialog">
      <button onClick={onClose}>close</button>
    </div>
  ),
}));

// --- fixtures --------------------------------------------------------------

function status(over: Partial<MFAStatus> = {}): MFAStatus {
  return {
    state: 'recommended',
    configured: false,
    required: false,
    privileged: false,
    grace_period_active: false,
    grace_days: 7,
    ...over,
  } as MFAStatus;
}

function renderBanner() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MFAEnrollmentBanner />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  sessionStorage.clear();
});

// --- tests -----------------------------------------------------------------

describe('MFAEnrollmentBanner — renders the server state, decides nothing', () => {
  it('invites a standard user without blocking anything', async () => {
    fetchMFAStatus.mockResolvedValue(status({ state: 'recommended' }));
    renderBanner();

    const banner = await screen.findByTestId('mfa-enrollment-banner');
    expect(banner).toHaveAttribute('data-mfa-state', 'recommended');
    // A recommendation is a status, not an alert: it must not interrupt a
    // screen reader mid-task.
    expect(banner).toHaveAttribute('role', 'status');
    expect(screen.getByRole('button', { name: /enable mfa|activer le mfa/i })).toBeInTheDocument();
  });

  it('says nothing at all once MFA is configured', async () => {
    fetchMFAStatus.mockResolvedValue(status({ state: 'configured', configured: true }));
    renderBanner();

    await waitFor(() => expect(fetchMFAStatus).toHaveBeenCalled());
    expect(screen.queryByTestId('mfa-enrollment-banner')).not.toBeInTheDocument();
  });

  it('shows the countdown for a privileged account inside its window', async () => {
    const deadline = new Date(Date.now() + 3 * 86_400_000).toISOString();
    fetchMFAStatus.mockResolvedValue(
      status({ state: 'grace_active', privileged: true, grace_period_active: true, deadline }),
    );
    renderBanner();

    const banner = await screen.findByTestId('mfa-enrollment-banner');
    expect(banner).toHaveAttribute('data-mfa-state', 'grace_active');
    expect(banner.textContent).toMatch(/3 (day|jour)/i);
  });

  it('never lets a privileged countdown be dismissed', async () => {
    fetchMFAStatus.mockResolvedValue(
      status({ state: 'grace_expiring', privileged: true, grace_period_active: true }),
    );
    renderBanner();

    await screen.findByTestId('mfa-enrollment-banner');
    expect(screen.queryByRole('button', { name: /dismiss|masquer/i })).not.toBeInTheDocument();
  });

  it('escalates to an assertive alert once enrolment is mandatory', async () => {
    fetchMFAStatus.mockResolvedValue(
      status({ state: 'required', required: true, privileged: true }),
    );
    renderBanner();

    const banner = await screen.findByTestId('mfa-enrollment-banner');
    expect(banner).toHaveAttribute('role', 'alert');
    expect(banner).toHaveAttribute('aria-live', 'assertive');
  });

  it('lets the soft recommendation be dismissed for the session only', async () => {
    fetchMFAStatus.mockResolvedValue(status({ state: 'recommended' }));
    renderBanner();

    await screen.findByTestId('mfa-enrollment-banner');
    await userEvent.click(screen.getByRole('button', { name: /dismiss|masquer/i }));

    expect(screen.queryByTestId('mfa-enrollment-banner')).not.toBeInTheDocument();
    expect(sessionStorage.getItem('openrisk_mfa_banner_dismissed')).toBe('1');
    // Nothing durable was written. A "skipped" flag in localStorage is exactly
    // the artefact somebody would later mistake for a security decision.
    expect(localStorage.getItem('openrisk_mfa_banner_dismissed')).toBeNull();
  });

  it('opens the enrolment dialog from the call to action', async () => {
    fetchMFAStatus.mockResolvedValue(status({ state: 'recommended' }));
    renderBanner();

    await screen.findByTestId('mfa-enrollment-banner');
    await userEvent.click(screen.getByRole('button', { name: /enable mfa|activer le mfa/i }));
    expect(screen.getByTestId('mfa-dialog')).toBeInTheDocument();
  });

  it('is honest when the state cannot be read, and never claims protection', async () => {
    fetchMFAStatus.mockRejectedValue(new Error('network down'));
    renderBanner();

    // Generous timeout: the hook retries once on purpose, so a transient
    // failure does not paint an error banner over an account that is fine.
    await waitFor(
      () => expect(screen.getByRole('button', { name: /retry|réessayer/i })).toBeInTheDocument(),
      {
        timeout: 5000,
      },
    );
    expect(screen.queryByTestId('mfa-enrollment-banner')).not.toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/protected|protégé/i);
  });

  it('treats an omitted mfa block as unknown, not as compliant', async () => {
    // The server omits the block when it cannot resolve the state. Rendering
    // that as "nothing to do" would turn a database blip into a false all-clear.
    fetchMFAStatus.mockResolvedValue(null);
    renderBanner();

    await waitFor(() =>
      expect(screen.getByRole('button', { name: /retry|réessayer/i })).toBeInTheDocument(),
    );
  });
});

describe('mfaPolicyService helpers', () => {
  const now = new Date('2026-08-24T12:00:00Z');

  it('rounds the countdown up so a live deadline never reads as zero', () => {
    const deadline = new Date('2026-08-25T06:00:00Z').toISOString(); // 18h away
    expect(daysUntilDeadline({ deadline } as MFAStatus, now)).toBe(1);
  });

  it('reports no countdown when there is no deadline', () => {
    expect(daysUntilDeadline(status(), now)).toBeNull();
    expect(daysUntilDeadline(null, now)).toBeNull();
    expect(daysUntilDeadline({ deadline: 'not-a-date' } as MFAStatus, now)).toBeNull();
  });

  it('clamps a passed deadline to zero rather than going negative', () => {
    const past = new Date('2026-08-20T12:00:00Z').toISOString();
    expect(daysUntilDeadline({ deadline: past } as MFAStatus, now)).toBe(0);
  });

  it('prompts for every state except configured', () => {
    expect(shouldPromptEnrollment(status({ state: 'configured' }))).toBe(false);
    expect(shouldPromptEnrollment(status({ state: 'recommended' }))).toBe(true);
    expect(shouldPromptEnrollment(status({ state: 'required' }))).toBe(true);
    expect(shouldPromptEnrollment(null)).toBe(false);
  });
});
