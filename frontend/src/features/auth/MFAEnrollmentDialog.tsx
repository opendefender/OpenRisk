// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// OR26-03 — in-app MFA enrolment.
//
// Before this, the only way to turn MFA on was the mandated path inside the
// login screen: you could not enrol voluntarily, which made "enrol before your
// deadline" advice with no button attached. This dialog is the button — the
// same two server calls (setup, then verify), reachable from the banner and
// from Settings → Security.

import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { ShieldCheck, X, Loader2, Copy, Check } from 'lucide-react';
import { toast } from 'sonner';

import { setupMFA, verifyMFA, type MFASetupResult } from './authService';
import { useInvalidateMFAStatus } from './useMfa';
import { useUIStore } from '../../store/uiStore';

type Phase = 'loading' | 'scan' | 'error';

export function MFAEnrollmentDialog({ onClose, onEnrolled }: { onClose: () => void; onEnrolled?: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const invalidateStatus = useInvalidateMFAStatus();

  const [phase, setPhase] = useState<Phase>('loading');
  const [setup, setSetup] = useState<MFASetupResult | null>(null);
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [copied, setCopied] = useState(false);
  const dialogRef = useRef<HTMLFormElement | null>(null);
  // POST /auth/mfa/setup is not idempotent — a second call for the same account
  // answers 400. React runs effects twice in development, so without this guard
  // a screen that had already loaded its QR code perfectly would paint an error
  // over itself. Tracked outside render so a re-render cannot repeat it.
  const requested = useRef(false);

  useEffect(() => {
    if (requested.current) return;
    requested.current = true;
    setupMFA()
      .then((r) => {
        setSetup(r);
        setPhase('scan');
      })
      .catch(() => {
        requested.current = false;
        setPhase('error');
      });
  }, []);

  // Esc closes, and focus starts inside the dialog rather than wherever the
  // page happened to leave it.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    dialogRef.current?.focus();
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError('');
    try {
      await verifyMFA(code.trim());
      await invalidateStatus();
      toast.success(tr('Authentification à deux facteurs activée', 'Two-factor authentication enabled'));
      onEnrolled?.();
      onClose();
    } catch {
      setError(tr('Code invalide. Vérifiez l’heure de votre appareil et réessayez.', 'Invalid code. Check your device clock and try again.'));
    } finally {
      setBusy(false);
    }
  };

  const qrSrc = setup?.qr_code
    ? setup.qr_code.startsWith('data:')
      ? setup.qr_code
      : `data:image/jpeg;base64,${setup.qr_code}`
    : '';

  const copySecret = async () => {
    if (!setup?.secret) return;
    try {
      await navigator.clipboard.writeText(setup.secret);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error(tr('Copie impossible', 'Could not copy'));
    }
  };

  return createPortal(
    <div
      className="fixed inset-0 z-80 flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(3px)' }}
      onClick={onClose}
    >
      <form
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="mfa-enrol-title"
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        onSubmit={submit}
        className="w-full max-w-[440px] max-h-[90vh] flex flex-col rounded-[16px] overflow-hidden outline-none"
        style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }}
      >
        <div className="px-[22px] pt-5 pb-4 flex items-center gap-3" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="w-9 h-9 rounded-[10px] flex items-center justify-center shrink-0" style={{ background: 'var(--accent-soft)', color: 'var(--accent-500)' }}>
            <ShieldCheck size={18} />
          </div>
          <div id="mfa-enrol-title" className="disp text-[17px] font-bold text-ink flex-1">
            {tr('Activer le MFA', 'Enable MFA')}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0 text-ink-soft hover:text-ink transition-colors"
            style={{ background: 'var(--bg-hover)' }}
            aria-label={tr('Fermer', 'Close')}
          >
            <X size={18} />
          </button>
        </div>

        <div className="px-[22px] py-5 overflow-y-auto flex flex-col gap-4">
          {phase === 'loading' && (
            <div className="flex items-center gap-2 text-[13px] text-ink-soft" role="status">
              <Loader2 size={16} className="animate-spin" />
              {tr('Préparation de votre authentificateur…', 'Preparing your authenticator…')}
            </div>
          )}

          {phase === 'error' && (
            <div className="text-[13px]" style={{ color: 'var(--critical)' }} role="alert">
              {tr(
                "Impossible de démarrer l'activation. Réessayez, ou contactez un administrateur si le problème persiste.",
                'Could not start enrollment. Try again, or contact an administrator if this persists.'
              )}
            </div>
          )}

          {phase === 'scan' && setup && (
            <>
              <p className="text-[13px] text-ink-soft leading-relaxed">
                {tr(
                  'Scannez ce QR code avec votre application d’authentification, puis saisissez le code à 6 chiffres qu’elle affiche.',
                  'Scan this QR code with your authenticator app, then enter the 6-digit code it shows.'
                )}
              </p>

              {qrSrc && (
                <div className="flex justify-center p-3 rounded-[13px]" style={{ background: '#fff' }}>
                  <img src={qrSrc} alt={tr('QR code d’activation MFA', 'MFA enrollment QR code')} width={168} height={168} />
                </div>
              )}

              <div className="flex items-center gap-2">
                <code className="flex-1 text-[12px] px-3 py-2 rounded-[9px] text-ink break-all" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
                  {setup.secret}
                </code>
                <button
                  type="button"
                  onClick={copySecret}
                  className="h-9 w-9 rounded-[9px] flex items-center justify-center text-ink-soft hover:text-ink transition-colors shrink-0"
                  style={{ background: 'var(--bg-hover)' }}
                  aria-label={tr('Copier la clé', 'Copy the key')}
                >
                  {copied ? <Check size={16} /> : <Copy size={16} />}
                </button>
              </div>

              {setup.backup_codes?.length > 0 && (
                <div>
                  <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5">
                    {tr('Codes de secours', 'Backup codes')}
                  </div>
                  <p className="text-[12px] text-ink-soft mb-2 leading-relaxed">
                    {tr(
                      'Conservez-les hors ligne. Chacun ne fonctionne qu’une fois, et ils sont votre seule issue si vous perdez votre appareil.',
                      'Keep these offline. Each works once, and they are your only way back in if you lose your device.'
                    )}
                  </p>
                  <div className="grid grid-cols-2 gap-1.5">
                    {setup.backup_codes.map((c) => (
                      <code key={c} className="text-[12px] px-2 py-1.5 rounded-[8px] text-ink text-center" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
                        {c}
                      </code>
                    ))}
                  </div>
                </div>
              )}

              <label className="flex flex-col gap-1.5">
                <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
                  {tr('Code à 6 chiffres', '6-digit code')}
                </span>
                <input
                  value={code}
                  onChange={(e) => {
                    setCode(e.target.value);
                    setError('');
                  }}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  maxLength={8}
                  aria-invalid={!!error}
                  aria-describedby={error ? 'mfa-enrol-error' : undefined}
                  className="w-full h-10 px-3 rounded-[10px] text-[15px] tracking-[.3em] text-ink outline-none focus:border-accent transition-colors"
                  style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
                />
              </label>

              {error && (
                <div id="mfa-enrol-error" role="alert" className="text-[12.5px]" style={{ color: 'var(--critical)' }}>
                  {error}
                </div>
              )}
            </>
          )}
        </div>

        <div className="px-[22px] py-4 flex justify-end gap-2" style={{ borderTop: '1px solid var(--border)' }}>
          <button
            type="button"
            onClick={onClose}
            className="h-9 px-3.5 rounded-[10px] text-[13px] font-semibold text-ink-soft hover:text-ink transition-colors"
            style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
          >
            {tr('Plus tard', 'Later')}
          </button>
          <button
            type="submit"
            disabled={busy || phase !== 'scan' || code.trim().length < 6}
            className="h-9 px-4 rounded-[10px] text-[13px] font-semibold text-text-primary inline-flex items-center gap-1.5 transition-all disabled:opacity-60"
            style={{ border: 'none', background: 'var(--accent-solid)', color: 'var(--text-on-solid)' }}
          >
            {busy && <Loader2 size={15} className="animate-spin" />}
            {tr('Activer', 'Enable')}
          </button>
        </div>
      </form>
    </div>,
    document.body
  );
}
