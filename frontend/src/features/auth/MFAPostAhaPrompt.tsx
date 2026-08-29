// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// OR26-03 step 3 — re-propose MFA once the user has seen the value.
//
// The whole point of deferring enrolment is that "secure your account" lands
// badly before someone knows what the account is for. Once they have reached the
// Aha moment — a cyber score computed on their own data with a real compliance
// gap identified — the same ask is a reasonable one, and this is where it goes.
//
// THE TRIGGER IS THE SERVER'S, NOT OURS. It reads activation.aha_reached_at,
// which the executive dashboard use case records once per tenant from real data.
// Nothing here sets or simulates that flag: a prompt that fires because the
// client decided the moment had arrived would be a prompt that fires at the
// wrong time for every user who took a different path through the product.

import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { ShieldCheck, X } from 'lucide-react';

import { useMFAStatus } from './useMfa';
import { MFAEnrollmentDialog } from './MFAEnrollmentDialog';
import { useActivationState } from '../onboarding/useActivation';
import { useUIStore } from '../../store/uiStore';

/**
 * Cooldown so the same ask does not reappear on every navigation.
 *
 * Twenty-four hours: long enough that declining is respected for the rest of the
 * working day, short enough that a member who genuinely means to enrol is
 * reminded tomorrow. Cosmetic only — it gates a prompt, never an obligation, and
 * a privileged account's deadline runs regardless of what is stored here.
 */
const COOLDOWN_KEY = 'openrisk_mfa_post_aha_prompt_at';
const COOLDOWN_MS = 24 * 60 * 60 * 1000;

function withinCooldown(now = Date.now()): boolean {
  const raw = localStorage.getItem(COOLDOWN_KEY);
  if (!raw) return false;
  const at = Number(raw);
  return Number.isFinite(at) && now - at < COOLDOWN_MS;
}

export function MFAPostAhaPrompt() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const { data: status } = useMFAStatus();
  const { data: activation } = useActivationState();

  const [open, setOpen] = useState(false);
  const [enrolling, setEnrolling] = useState(false);

  const ahaReached = !!activation?.aha_reached_at;
  const notConfigured = !!status && status.state !== 'configured';
  // Deliberately silent once enrolment is mandatory: the banner is already an
  // alert and the server is already refusing requests. A modal on top of that
  // is noise stacked on a blocker, not urgency.
  const alreadyBlocked = status?.state === 'required';

  useEffect(() => {
    if (!ahaReached || !notConfigured || alreadyBlocked) return;
    if (withinCooldown()) return;
    setOpen(true);
    localStorage.setItem(COOLDOWN_KEY, String(Date.now()));
  }, [ahaReached, notConfigured, alreadyBlocked]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open]);

  if (enrolling) {
    return <MFAEnrollmentDialog onClose={() => setEnrolling(false)} onEnrolled={() => setOpen(false)} />;
  }
  if (!open) return null;

  return createPortal(
    <div
      className="fixed inset-0 z-[75] flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(3px)' }}
      onClick={() => setOpen(false)}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="mfa-aha-title"
        aria-describedby="mfa-aha-body"
        data-testid="mfa-post-aha-prompt"
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-[420px] rounded-[16px] overflow-hidden"
        style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }}
      >
        <div className="px-[22px] pt-5 pb-4 flex items-center gap-3" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="w-9 h-9 rounded-[10px] flex items-center justify-center shrink-0" style={{ background: 'var(--accent-soft)', color: 'var(--accent-500)' }}>
            <ShieldCheck size={18} aria-hidden="true" />
          </div>
          <div id="mfa-aha-title" className="disp text-[17px] font-bold text-ink flex-1">
            {tr('Protégez ce que vous venez de construire', 'Protect what you just built')}
          </div>
          <button
            type="button"
            onClick={() => setOpen(false)}
            className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0 text-ink-soft hover:text-ink transition-colors"
            style={{ background: 'var(--bg-hover)' }}
            aria-label={tr('Fermer', 'Close')}
          >
            <X size={18} aria-hidden="true" />
          </button>
        </div>

        <div id="mfa-aha-body" className="px-[22px] py-5 text-[13px] text-ink-soft leading-relaxed">
          {tr(
            'Votre registre de risques et votre posture de conformité vivent désormais dans OpenRisk. L’authentification à deux facteurs empêche qu’un mot de passe compromis suffise à y accéder — deux minutes suffisent.',
            'Your risk register and compliance posture now live in OpenRisk. Two-factor authentication stops a leaked password from being enough to reach them — it takes two minutes.'
          )}
        </div>

        <div className="px-[22px] py-4 flex justify-end gap-2" style={{ borderTop: '1px solid var(--border)' }}>
          <button
            type="button"
            onClick={() => setOpen(false)}
            className="h-9 px-3.5 rounded-[10px] text-[13px] font-semibold text-ink-soft hover:text-ink transition-colors"
            style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
          >
            {tr('Plus tard', 'Later')}
          </button>
          <button
            type="button"
            onClick={() => setEnrolling(true)}
            className="h-9 px-4 rounded-[10px] text-[13px] font-semibold text-text-primary transition-all"
            style={{ border: 'none', background: 'var(--accent-solid)', color: 'var(--text-on-solid)' }}
          >
            {tr('Activer le MFA', 'Enable MFA')}
          </button>
        </div>
      </div>
    </div>,
    document.body
  );
}
