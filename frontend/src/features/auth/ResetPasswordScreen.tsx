// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The second half of the reset flow: set a new password from the emailed link.

import { useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';
import axios from 'axios';
import { CheckCircle2, Eye, EyeOff } from 'lucide-react';

import { useUIStore } from '../../store/uiStore';
import { authCopy } from './authStrings';
import { resetPassword, type PasswordAssessment, type ResetPasswordErrorBody } from './authService';
import { AuthLayout } from './AuthLayout';
import { cascade, usePrefersReducedMotion } from './motion';
import { PasswordStrength } from './PasswordStrength';
import { ErrorBanner, Label, Shake } from './fields';
import { inputCls, inputStyle, primaryBtn, primaryStyle } from './formStyles';

export function ResetPasswordScreen() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const copy = authCopy(lang);
  const reduced = usePrefersReducedMotion();

  const token = params.get('token') ?? '';

  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState(token ? '' : copy.resetMissingToken);
  // The server's verdict when it refuses the password, so the meter shows the
  // authoritative reasons rather than the client's guess at them.
  const [serverAssessment, setServerAssessment] = useState<PasswordAssessment | null>(null);
  const [acceptable, setAcceptable] = useState(false);

  const canSubmit = Boolean(token) && acceptable && password.length > 0 && !busy;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token) return setError(copy.resetMissingToken);
    if (password !== confirm) return setError(copy.resetMismatch);

    setBusy(true);
    setError('');
    setServerAssessment(null);
    try {
      await resetPassword(token, password, lang);
      setDone(true);
    } catch (err) {
      if (axios.isAxiosError(err)) {
        const body = err.response?.data as ResetPasswordErrorBody | undefined;
        if (body?.code === 'weak_password' && body.assessment) {
          // Show exactly what the server objected to. The client meter may have
          // been happy — it cannot see the breach corpus.
          setServerAssessment(body.assessment);
          setError(body.error ?? '');
        } else if (body?.code === 'invalid_token') {
          setError(copy.resetTokenInvalid);
        } else {
          setError(body?.error ?? copy.registerFailed);
        }
      } else {
        setError(copy.registerFailed);
      }
    } finally {
      setBusy(false);
    }
  };

  if (done) {
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
            <CheckCircle2 size={22} />
          </div>
          <h1 className="disp text-[24px] font-bold text-ink mb-1.5">{copy.resetDoneTitle}</h1>
          <p className="text-[14px] text-ink-soft leading-relaxed mb-6">{copy.resetDoneBody}</p>
          <button
            type="button"
            data-testid="reset-to-login"
            onClick={() => navigate('/login')}
            className={primaryBtn}
            style={primaryStyle}
          >
            {copy.signIn}
          </button>
        </div>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout>
      <form onSubmit={submit} noValidate>
        <div style={cascade(0, reduced)}>
          <h1 className="disp text-[24px] font-bold text-ink mb-1.5">{copy.resetTitle}</h1>
          <p className="text-[14px] text-ink-soft mb-[26px] leading-relaxed">
            {copy.resetSubtitle}
          </p>
        </div>

        <ErrorBanner>{error}</ErrorBanner>

        <div className="mb-[15px]" style={cascade(1, reduced)}>
          <Label htmlFor="reset-password">{copy.newPassword}</Label>
          <Shake errorKey={error}>
            <div className="relative">
              <input
                id="reset-password"
                data-testid="reset-password"
                type={show ? 'text' : 'password'}
                autoComplete="new-password"
                autoFocus
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value);
                  // A fresh keystroke supersedes the server's last verdict.
                  setServerAssessment(null);
                }}
                className={inputCls}
                style={inputStyle(Boolean(error))}
              />
              <button
                type="button"
                onClick={() => setShow((v) => !v)}
                className="absolute right-2.5 top-[11px] w-[26px] h-[22px] flex items-center justify-center text-ink-muted"
                aria-label={copy.password}
              >
                {show ? <EyeOff size={17} /> : <Eye size={17} />}
              </button>
            </div>
          </Shake>
          <PasswordStrength
            password={password}
            onVerdict={setAcceptable}
            override={serverAssessment}
          />
        </div>

        <div className="mb-[18px]" style={cascade(2, reduced)}>
          <Label htmlFor="reset-confirm">{copy.confirmPassword}</Label>
          <input
            id="reset-confirm"
            data-testid="reset-confirm"
            type={show ? 'text' : 'password'}
            autoComplete="new-password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            className={inputCls}
            style={inputStyle(confirm.length > 0 && confirm !== password)}
          />
          {confirm.length > 0 && confirm !== password && (
            <div className="text-[11.5px] mt-1.5" style={{ color: 'var(--high)' }}>
              {copy.resetMismatch}
            </div>
          )}
        </div>

        <div style={cascade(3, reduced)}>
          <button
            type="submit"
            data-testid="reset-submit"
            disabled={!canSubmit}
            className={primaryBtn}
            style={{ ...primaryStyle, opacity: canSubmit ? 1 : 0.6 }}
          >
            {busy ? copy.resetting : copy.resetSubmit}
          </button>
        </div>

        <div
          className="text-center text-[13px] text-ink-soft mt-[18px]"
          style={cascade(4, reduced)}
        >
          <Link to="/login" className="font-semibold">
            {copy.backToSignIn}
          </Link>
        </div>
      </form>
    </AuthLayout>
  );
}

export default ResetPasswordScreen;
