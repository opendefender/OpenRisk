// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useEffect, useState } from 'react';
import { Navigate, useNavigate } from 'react-router';
import { toast } from 'sonner';

import { useUIStore } from '../../store/uiStore';
import { clearSwitchStep, peekSwitchStep } from '../organization/pendingSwitchStep';
import { AuthLayout } from './AuthLayout';
import { MFAChallenge, MFAEnrollment } from './AuthScreen';
import { authCopy } from './authStrings';

/**
 * The code step of an organization switch (GHSA-45mr-j2w6-m4jv).
 *
 * The target organization keeps its own MFA posture: an authenticator enrolled
 * there is asked for, and its role policy applies, whatever the current session
 * proved. The switcher hands the step over in memory; the challenge or the
 * enrolment then issues the session for the target organization. Leaving, or a
 * step that expired, goes back to the organization the user was in, whose
 * session is untouched.
 */
export function SwitchOrgMFAScreen() {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const copy = authCopy(lang);
  // Read once. The step is cleared on unmount rather than on read, so the
  // StrictMode double render in development still finds it.
  const [step] = useState(peekSwitchStep);
  useEffect(() => clearSwitchStep, []);

  const back = useCallback(() => navigate('/', { replace: true }), [navigate]);
  const restart = useCallback(
    (message: string) => {
      toast.error(message);
      back();
    },
    [back],
  );

  if (!step) return <Navigate to="/" replace />;

  return (
    <AuthLayout>
      {step.enrolling ? (
        <MFAEnrollment token={step.token} onRestart={restart} />
      ) : (
        <MFAChallenge
          token={step.token}
          onCancel={back}
          cancelLabel={copy.mfaSwitchCancel}
          onRestart={restart}
        />
      )}
    </AuthLayout>
  );
}
