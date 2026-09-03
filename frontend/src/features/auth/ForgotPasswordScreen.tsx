// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "I forgot my password" — the request half of the reset flow.

import { useState } from 'react';
import { Link } from 'react-router';
import axios from 'axios';
import { ArrowLeft, MailCheck } from 'lucide-react';

import { useUIStore } from '../../store/uiStore';
import { authCopy } from './authStrings';
import { requestPasswordReset } from './authService';
import { AuthLayout } from './AuthLayout';
import { cascade, usePrefersReducedMotion } from './motion';
import { ErrorBanner, Label, Shake } from './fields';
import { inputCls, inputStyle, primaryBtn, primaryStyle } from './formStyles';

export function ForgotPasswordScreen() {
  const lang = useUIStore((s) => s.lang);
  const copy = authCopy(lang);
  const reduced = usePrefersReducedMotion();

  const [email, setEmail] = useState('');
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email.trim()) return;

    setBusy(true);
    setError('');
    try {
      await requestPasswordReset(email.trim(), lang);
      setSent(true);
    } catch (err) {
      // The only failure the user can act on is the rate limit. Everything else
      // — including "no such account", which the server never tells us — lands
      // on the same acknowledgement screen, because showing an error for an
      // unknown address is exactly the enumeration leak the backend avoids.
      if (axios.isAxiosError(err) && err.response?.status === 429) {
        setError(copy.forgotRateLimited);
      } else {
        setSent(true);
      }
    } finally {
      setBusy(false);
    }
  };

  if (sent) {
    return (
      <AuthLayout>
        <div style={cascade(0, reduced)}>
          <div
            className="w-11 h-11 rounded-[13px] flex items-center justify-center mb-4"
            style={{
              background: 'color-mix(in srgb, var(--low) 14%, transparent)',
              color: 'var(--low)',
            }}
          >
            <MailCheck size={22} />
          </div>
          <h1 className="disp text-[24px] font-bold text-ink mb-1.5">{copy.forgotSentTitle}</h1>
          {/* Phrased conditionally on purpose — see the backend's uniform
              acknowledgement. "We sent you an email" would be a lie half the
              time, and someone who mistyped their address would wait forever. */}
          <p className="text-[14px] text-ink-soft leading-relaxed mb-6">{copy.forgotSentBody}</p>
          <Link
            to="/login"
            className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-ink-soft hover:text-ink transition-colors"
          >
            <ArrowLeft size={15} />
            {copy.backToSignIn}
          </Link>
        </div>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout>
      <form onSubmit={submit} noValidate>
        <div style={cascade(0, reduced)}>
          <h1 className="disp text-[24px] font-bold text-ink mb-1.5">{copy.forgotTitle}</h1>
          <p className="text-[14px] text-ink-soft mb-[26px] leading-relaxed">
            {copy.forgotSubtitle}
          </p>
        </div>

        <ErrorBanner>{error}</ErrorBanner>

        <div className="mb-[18px]" style={cascade(1, reduced)}>
          <Label htmlFor="forgot-email">{copy.email}</Label>
          <Shake errorKey={error}>
            <input
              id="forgot-email"
              data-testid="forgot-email"
              type="email"
              autoComplete="email"
              autoFocus
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className={inputCls}
              style={inputStyle(Boolean(error))}
            />
          </Shake>
        </div>

        <div style={cascade(2, reduced)}>
          <button
            type="submit"
            data-testid="forgot-submit"
            disabled={busy || !email.trim()}
            className={primaryBtn}
            style={{ ...primaryStyle, opacity: busy || !email.trim() ? 0.6 : 1 }}
          >
            {busy ? copy.forgotSending : copy.forgotSubmit}
          </button>
        </div>

        <div className="text-center mt-[18px]" style={cascade(3, reduced)}>
          <Link
            to="/login"
            className="inline-flex items-center gap-1.5 text-[13px] text-ink-soft hover:text-ink transition-colors"
          >
            <ArrowLeft size={15} />
            {copy.backToSignIn}
          </Link>
        </div>
      </form>
    </AuthLayout>
  );
}

export default ForgotPasswordScreen;
