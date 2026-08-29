// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Sign in / register, plus the two MFA legs (challenge and mandated enrolment).
//
// The split-screen shell, motion policy and language switcher live in
// AuthLayout; this file is the forms.

import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';
import { toast } from 'sonner';
import axios from 'axios';
import { Eye, EyeOff, ShieldCheck } from 'lucide-react';

import { api } from '../../lib/api';
import { useAuthStore } from '../../hooks/useAuthStore';
import { landingForBusinessRole } from '../../shared/navModel';
import { useUIStore } from '../../store/uiStore';
import { authCopy, providerLabel, type OAuthErrorCode } from './authStrings';
import { challengeMFA, setupMFA, verifyMFA } from './authService';
import { AuthLayout } from './AuthLayout';
import { cascade, usePrefersReducedMotion } from './motion';
import { PasswordStrength } from './PasswordStrength';
import { ErrorBanner, Label, Shake } from './fields';
import { inputCls, inputStyle, primaryBtn, primaryStyle } from './formStyles';

type View = 'login' | 'register';

const OAUTH_PROVIDERS: { id: 'google' | 'github' | 'azure'; label: string }[] = [
  { id: 'google', label: 'Google' },
  { id: 'github', label: 'GitHub' },
  { id: 'azure', label: 'Microsoft' },
];

export function AuthScreen({ initialView = 'login' }: { initialView?: View }) {
  const [view, setView] = useState<View>(initialView);
  return (
    <AuthLayout>
      {view === 'login' ? (
        <LoginForm onRegister={() => setView('register')} />
      ) : (
        <RegisterForm onLogin={() => setView('login')} />
      )}
    </AuthLayout>
  );
}

// ---------------------------------------------------------------------------
// Sign in
// ---------------------------------------------------------------------------

function LoginForm({ onRegister }: { onRegister: () => void }) {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const lang = useUIStore((s) => s.lang);
  const copy = authCopy(lang);
  const reduced = usePrefersReducedMotion();
  const login = useAuthStore((s) => s.login);

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  // Bumped on every failure so the shake fires again even for an identical
  // message — otherwise retyping the same wrong password gives no feedback.
  const [errorNonce, setErrorNonce] = useState(0);

  // Second-factor state, when login stops short of a session.
  const [mfa, setMfa] = useState<{ token: string; enrolling: boolean } | null>(null);

  // --- OAuth failures come back as ?error= on this URL --------------------
  // The backend redirects here rather than rendering JSON at a URL the browser
  // navigated to, which would be an unstyled dead end with no way back.
  const oauthError = useMemo(() => {
    const code = params.get('error') as OAuthErrorCode | null;
    if (!code) return '';
    if (code === 'provider_conflict') {
      const existing = params.get('existing_provider');
      // Naming the provider that DOES own the address is what makes this
      // recoverable — a bare refusal strands someone on their own account.
      return existing ? copy.oauthConflictWith(providerLabel(existing)) : copy.oauth.provider_conflict;
    }
    return copy.oauth[code] ?? copy.oauth.internal;
  }, [params, copy]);

  useEffect(() => {
    if (!oauthError) return;
    setError(oauthError);
    setErrorNonce((n) => n + 1);
    // Clear the query so a reload doesn't resurrect a stale failure.
    const next = new URLSearchParams(params);
    ['error', 'provider', 'existing_provider', 'lang'].forEach((k) => next.delete(k));
    setParams(next, { replace: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [oauthError]);

  const fail = (message: string) => {
    setError(message);
    setErrorNonce((n) => n + 1);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const result = await login(email, password);

      // Login can stop short of a session in two ways: an enrolled second factor
      // to challenge, or a role that mandates MFA with nothing enrolled yet.
      if (result.status === 'mfa_required') {
        setMfa({ token: result.mfa_token, enrolling: false });
        return;
      }
      if (result.status === 'mfa_enrollment_required') {
        setMfa({ token: result.mfa_token, enrolling: true });
        return;
      }

      toast.success(copy.signInTitle);
      navigate(landingForBusinessRole(useAuthStore.getState().user?.business_role));
    } catch {
      fail(copy.signInFailed);
    } finally {
      setBusy(false);
    }
  };

  // Full-page navigation: the OAuth flow is a browser redirect, and the backend
  // now answers /login/:provider with a 302 straight to the provider.
  const startOAuth = (provider: string) => {
    const base = api.defaults.baseURL ?? '';
    window.location.href = `${base}/auth/oauth2/login/${provider}?lang=${lang}`;
  };

  if (mfa) {
    return mfa.enrolling ? (
      <MFAEnrollment token={mfa.token} />
    ) : (
      <MFAChallenge token={mfa.token} onCancel={() => setMfa(null)} />
    );
  }

  return (
    <form onSubmit={submit} noValidate>
      <div style={cascade(0, reduced)}>
        <h1 className="disp text-[24px] font-bold text-ink mb-1.5">{copy.signInTitle}</h1>
        <div className="text-[14px] text-ink-soft mb-[26px]">{copy.signInSubtitle}</div>
      </div>

      <ErrorBanner>{error}</ErrorBanner>

      <div className="mb-[15px]" style={cascade(1, reduced)}>
        <Label htmlFor="login-email">{copy.email}</Label>
        <Shake errorKey={errorNonce}>
          <input
            id="login-email"
            data-testid="login-email"
            type="email"
            autoComplete="username"
            autoFocus
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className={inputCls}
            style={inputStyle(Boolean(error))}
          />
        </Shake>
      </div>

      <div className="mb-[15px]" style={cascade(2, reduced)}>
        <Label htmlFor="login-password">{copy.password}</Label>
        <Shake errorKey={errorNonce}>
          <div className="relative">
            <input
              id="login-password"
              data-testid="login-password"
              type={show ? 'text' : 'password'}
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
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
      </div>

      <div className="flex items-center justify-between my-1 mb-5" style={cascade(3, reduced)}>
        <label className="flex items-center gap-[7px] text-[12.5px] text-ink-soft cursor-pointer">
          <input type="checkbox" style={{ accentColor: 'var(--accent)' }} />
          {copy.rememberMe}
        </label>
        {/* Was an href="#" with preventDefault — a control that looked live and
            did nothing. It now goes to the real flow. */}
        <Link to="/forgot-password" data-testid="forgot-password-link" className="text-[12.5px] font-medium">
          {copy.forgotPassword}
        </Link>
      </div>

      <div style={cascade(4, reduced)}>
        <button
          type="submit"
          data-testid="login-submit"
          disabled={busy}
          className={primaryBtn}
          style={{ ...primaryStyle, opacity: busy ? 0.7 : 1 }}
        >
          {busy ? copy.signingIn : copy.signIn}
        </button>
      </div>

      <div className="flex items-center gap-3 my-[18px]" style={cascade(5, reduced)}>
        <div className="flex-1 h-px" style={{ background: 'var(--border)' }} />
        <span className="text-[12px] text-ink-muted">{copy.orContinueWith}</span>
        <div className="flex-1 h-px" style={{ background: 'var(--border)' }} />
      </div>

      <div className="grid grid-cols-3 gap-2.5 mb-2" style={cascade(6, reduced)}>
        {OAUTH_PROVIDERS.map((p) => (
          <button
            key={p.id}
            type="button"
            data-testid={`oauth-${p.id}`}
            onClick={() => startOAuth(p.id)}
            className="h-11 rounded-[11px] text-[12.5px] font-semibold text-ink flex items-center justify-center gap-2 hover:bg-hover transition-colors"
            style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
          >
            {p.label}
          </button>
        ))}
      </div>

      <div className="text-center text-[13px] text-ink-soft mt-[18px]" style={cascade(7, reduced)}>
        {copy.noAccount}{' '}
        <a
          href="#"
          onClick={(e) => {
            e.preventDefault();
            onRegister();
          }}
          className="font-semibold"
        >
          {copy.createAccount}
        </a>
      </div>
    </form>
  );
}

// ---------------------------------------------------------------------------
// MFA — challenge
// ---------------------------------------------------------------------------

function MFAChallenge({ token, onCancel }: { token: string; onCancel: () => void }) {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const copy = authCopy(lang);
  const reduced = usePrefersReducedMotion();
  const adoptSession = useAuthStore((s) => s.adoptSession);

  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [nonce, setNonce] = useState(0);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const result = await challengeMFA(code.trim(), token);
      // The challenge sets the session cookies server-side and returns the pair;
      // adopt it so the store has the user and permissions.
      if (result.token_pair?.access_token) {
        await adoptSession(result.token_pair.access_token);
      }
      navigate(landingForBusinessRole(useAuthStore.getState().user?.business_role));
    } catch {
      setError(copy.mfaInvalid);
      setNonce((n) => n + 1);
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} noValidate>
      <div style={cascade(0, reduced)}>
        <div
          className="w-11 h-11 rounded-[13px] flex items-center justify-center mb-4"
          style={{ background: 'var(--accent-soft)', color: 'var(--accent-500)' }}
        >
          <ShieldCheck size={22} />
        </div>
        <h1 className="disp text-[24px] font-bold text-ink mb-1.5">{copy.mfaTitle}</h1>
        <p className="text-[14px] text-ink-soft mb-[26px] leading-relaxed">{copy.mfaSubtitle}</p>
      </div>

      <ErrorBanner>{error}</ErrorBanner>

      <div className="mb-[18px]" style={cascade(1, reduced)}>
        <Label htmlFor="mfa-code">{copy.mfaCode}</Label>
        <Shake errorKey={nonce}>
          <input
            id="mfa-code"
            data-testid="mfa-code"
            inputMode="numeric"
            autoComplete="one-time-code"
            autoFocus
            value={code}
            onChange={(e) => setCode(e.target.value)}
            className={`${inputCls} mono tracking-[0.3em] text-center`}
            style={inputStyle(Boolean(error))}
          />
        </Shake>
        <p className="text-[11.5px] text-ink-muted mt-1.5">{copy.mfaUseBackup}</p>
      </div>

      <div style={cascade(2, reduced)}>
        <button
          type="submit"
          data-testid="mfa-submit"
          disabled={busy || !code.trim()}
          className={primaryBtn}
          style={{ ...primaryStyle, opacity: busy || !code.trim() ? 0.6 : 1 }}
        >
          {busy ? copy.signingIn : copy.mfaSubmit}
        </button>
      </div>

      <div className="text-center text-[13px] text-ink-soft mt-[18px]" style={cascade(3, reduced)}>
        <button type="button" onClick={onCancel} className="font-semibold">
          {copy.backToSignIn}
        </button>
      </div>
    </form>
  );
}

// ---------------------------------------------------------------------------
// MFA — mandated enrolment
// ---------------------------------------------------------------------------

/**
 * Enrolment for a role that requires MFA.
 *
 * Reached with an MFA_ENROLLMENT token: the password is proved but no session
 * exists, and none will until an authenticator is verified. Confirming the code
 * completes the login in the same step, so the user is not asked for the
 * password they typed a minute ago.
 */
function MFAEnrollment({ token }: { token: string }) {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const copy = authCopy(lang);
  const reduced = usePrefersReducedMotion();
  const adoptSession = useAuthStore((s) => s.adoptSession);

  const [secret, setSecret] = useState('');
  const [qr, setQr] = useState('');
  const [backupCodes, setBackupCodes] = useState<string[]>([]);
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [nonce, setNonce] = useState(0);
  // POST /auth/mfa/setup is NOT idempotent: a second call for the same account
  // answers 400 "duplicated key not allowed". React runs effects twice in
  // development, so the second call failed and painted "la création du compte a
  // échoué" over a screen that had already loaded its QR code perfectly — the
  // account existed, nothing had failed, and the user was told to start over.
  // One call per token, tracked outside render so a re-render cannot repeat it.
  const requested = useRef<string | null>(null);

  useEffect(() => {
    if (requested.current === token) return;
    requested.current = token;
    setupMFA(token)
      .then((r) => {
        setSecret(r.secret);
        setQr(r.qr_code);
        setBackupCodes(r.backup_codes ?? []);
      })
      .catch(() => {
        // A setup failure is not a registration failure: the account is already
        // created. Say what actually went wrong, and allow another attempt.
        requested.current = null;
        setError(copy.mfaSetupFailed);
      });
  }, [token, copy.mfaSetupFailed]);

  const qrSrc = qr.startsWith('data:') ? qr : `data:image/jpeg;base64,${qr}`;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const result = await verifyMFA(code.trim(), token);
      // Mandated enrolment completes the login: the server issues the session in
      // the same response, so the user is not asked for their password again.
      if (result.token_pair?.access_token) {
        await adoptSession(result.token_pair.access_token);
      }
      navigate(landingForBusinessRole(useAuthStore.getState().user?.business_role));
    } catch {
      setError(copy.mfaInvalid);
      setNonce((n) => n + 1);
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} noValidate>
      <div style={cascade(0, reduced)}>
        <h1 className="disp text-[24px] font-bold text-ink mb-1.5">{copy.mfaEnrolTitle}</h1>
        <p className="text-[14px] text-ink-soft mb-5 leading-relaxed">{copy.mfaEnrolSubtitle}</p>
      </div>

      <ErrorBanner>{error}</ErrorBanner>

      {qr && (
        <div className="mb-4" style={cascade(1, reduced)}>
          <p className="text-[12.5px] text-ink-soft mb-2">{copy.mfaEnrolScan}</p>
          <div className="flex justify-center p-3 rounded-[13px]" style={{ background: '#fff' }}>
            {/* The backend returns raw base64 (a JPEG), not a data URI, so
                assigning it straight to src made the browser treat it as a
                relative URL and render a broken image — on the one screen a new
                account cannot get past. Tolerate both shapes so a later change
                on either side cannot break it again. */}
            <img src={qrSrc} alt="" width={168} height={168} />
          </div>
          <p className="text-[11.5px] text-ink-muted mt-2">{copy.mfaEnrolManual}</p>
          <code className="mono text-[12px] text-ink break-all">{secret}</code>
        </div>
      )}

      {backupCodes.length > 0 && (
        <div
          className="mb-4 p-3 rounded-[11px]"
          style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
          data-testid="backup-codes"
        >
          <div className="text-[12.5px] font-semibold text-ink mb-1">{copy.mfaEnrolBackupTitle}</div>
          <p className="text-[11.5px] text-ink-muted mb-2 leading-snug">{copy.mfaEnrolBackupBody}</p>
          <div className="grid grid-cols-2 gap-1.5">
            {backupCodes.map((c) => (
              <code key={c} className="mono text-[12px] text-ink">
                {c}
              </code>
            ))}
          </div>
        </div>
      )}

      <div className="mb-[18px]" style={cascade(2, reduced)}>
        <Label htmlFor="enrol-code">{copy.mfaCode}</Label>
        <Shake errorKey={nonce}>
          <input
            id="enrol-code"
            data-testid="mfa-enrol-code"
            inputMode="numeric"
            autoComplete="one-time-code"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            className={`${inputCls} mono tracking-[0.3em] text-center`}
            style={inputStyle(Boolean(error))}
          />
        </Shake>
      </div>

      <button
        type="submit"
        data-testid="mfa-enrol-submit"
        disabled={busy || !code.trim()}
        className={primaryBtn}
        style={{ ...primaryStyle, opacity: busy || !code.trim() ? 0.6 : 1 }}
      >
        {busy ? copy.signingIn : copy.mfaEnrolConfirm}
      </button>
    </form>
  );
}

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------

function RegisterForm({ onLogin }: { onLogin: () => void }) {
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const copy = authCopy(lang);
  const reduced = usePrefersReducedMotion();
  const login = useAuthStore((s) => s.login);
  // Signing up creates an organisation with you as its owner, and an owner's
  // role mandates MFA — so registration ALWAYS lands on enrolment. Handling it
  // here is not an edge case, it is the only path.
  const [mfa, setMfa] = useState<{ token: string; enrolling: boolean } | null>(null);

  const [fullName, setFullName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [show, setShow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [nonce, setNonce] = useState(0);
  // Comes from the shared policy meter, which defers to the server.
  const [acceptable, setAcceptable] = useState(false);

  const canSubmit = Boolean(fullName.trim()) && acceptable && !busy;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!fullName.trim()) {
      setError(copy.registerNameRequired);
      setNonce((n) => n + 1);
      return;
    }

    setBusy(true);
    setError('');
    try {
      const local = (email.split('@')[0] || 'user').replace(/[^a-zA-Z0-9_.-]/g, '');
      const username = local.length >= 3 ? local : `${local || 'user'}${Date.now().toString().slice(-4)}`;
      const company = `${fullName.trim()}${lang === 'fr' ? ' — espace' : ' — workspace'}`;

      await api.post('/auth/register', {
        email: email.trim(),
        username,
        password,
        full_name: fullName.trim(),
        company_name: company,
      });

      // The account exists now; the session may not. login() can stop short in
      // two ways and RETURNS which one — ignoring that return was the bug: the
      // form declared success and navigated with no session, so the route guard
      // bounced the brand-new user straight back to the login screen.
      const result = await login(email.trim(), password);

      if (result.status === 'mfa_enrollment_required') {
        setMfa({ token: result.mfa_token, enrolling: true });
        return;
      }
      if (result.status === 'mfa_required') {
        setMfa({ token: result.mfa_token, enrolling: false });
        return;
      }

      toast.success(copy.registerTitle);
      navigate(landingForBusinessRole(useAuthStore.getState().user?.business_role));
    } catch (err) {
      const status = axios.isAxiosError(err) ? err.response?.status : undefined;
      if (status === 409) {
        setError(copy.registerEmailExists);
      } else if (status === 400) {
        const msg = axios.isAxiosError(err) ? (err.response?.data as { error?: string })?.error : undefined;
        setError(msg || copy.registerFailed);
      } else {
        setError(copy.registerFailed);
      }
      setNonce((n) => n + 1);
    } finally {
      setBusy(false);
    }
  };

  // Hand off to the same enrolment/challenge screens the login form uses, rather
  // than a second implementation that would drift from it.
  if (mfa) {
    return mfa.enrolling ? (
      <MFAEnrollment token={mfa.token} />
    ) : (
      <MFAChallenge token={mfa.token} onCancel={() => setMfa(null)} />
    );
  }

  return (
    <form onSubmit={submit} noValidate>
      <div style={cascade(0, reduced)}>
        <h1 className="disp text-[24px] font-bold text-ink mb-1.5">{copy.registerTitle}</h1>
        <div className="text-[14px] text-ink-soft mb-[26px]">{copy.registerSubtitle}</div>
      </div>

      <ErrorBanner>{error}</ErrorBanner>

      <div className="mb-[15px]" style={cascade(1, reduced)}>
        <Label htmlFor="reg-name">{copy.fullName}</Label>
        <Shake errorKey={nonce}>
          <input
            id="reg-name"
            data-testid="register-name"
            value={fullName}
            autoFocus
            onChange={(e) => setFullName(e.target.value)}
            className={inputCls}
            style={inputStyle(false)}
          />
        </Shake>
      </div>

      <div className="mb-[15px]" style={cascade(2, reduced)}>
        <Label htmlFor="reg-email">{copy.email}</Label>
        <input
          id="reg-email"
          data-testid="register-email"
          type="email"
          autoComplete="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className={inputCls}
          style={inputStyle(false)}
        />
      </div>

      <div className="mb-[18px]" style={cascade(3, reduced)}>
        <Label htmlFor="reg-password">{copy.password}</Label>
        <div className="relative">
          <input
            id="reg-password"
            data-testid="register-password"
            type={show ? 'text' : 'password'}
            autoComplete="new-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className={inputCls}
            style={inputStyle(false)}
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
        {/* Same meter, same policy, same server as the reset screen. The old form
            enforced 8 characters here while the server required 12 — a mismatch
            users met as a rejection after they thought they were done. */}
        <PasswordStrength
          password={password}
          email={email}
          name={fullName}
          onVerdict={setAcceptable}
        />
      </div>

      <div style={cascade(4, reduced)}>
        <button
          type="submit"
          data-testid="register-submit"
          disabled={!canSubmit}
          className={primaryBtn}
          style={{ ...primaryStyle, opacity: canSubmit ? 1 : 0.6 }}
        >
          {busy ? copy.signingIn : copy.createAccount}
        </button>
      </div>

      <div className="text-center text-[13px] text-ink-soft mt-[18px]" style={cascade(5, reduced)}>
        {copy.haveAccount}{' '}
        <a
          href="#"
          onClick={(e) => {
            e.preventDefault();
            onLogin();
          }}
          className="font-semibold"
        >
          {copy.signInLink}
        </a>
      </div>
    </form>
  );
}

export default AuthScreen;
