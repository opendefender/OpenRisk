// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Impact-radiography confirmation for VITAL / irreversible actions (UX-4 /
// directive §4). Instead of a bare "Are you sure?", it shows what the action will
// affect, offers safer alternatives right in the dialog (e.g. "Deactivate instead"
// of deleting), and only then the destructive button. Use this for the actions
// that genuinely deserve friction — delete a user, a tenant, a role, revoke a
// token. For routine content, prefer the soft-delete Undo pattern (useSoftDelete).

import type { ReactNode } from 'react';
import { AlertTriangle, X, type LucideIcon } from 'lucide-react';
import { useUIStore } from '../store/uiStore';

export interface DangerAlternative {
  label: string;
  description?: string;
  icon?: LucideIcon;
  onClick: () => void;
}

interface DangerConfirmProps {
  open: boolean;
  onClose: () => void;
  title: string;
  /** The specific thing being acted on (name/email), shown under the title. */
  subject?: string;
  /** One-line consequence sentence. */
  intro?: ReactNode;
  /** The radiography: what this touches (label → value rows). */
  impact?: { label: string; value: ReactNode }[];
  /** Safer paths offered right here, so deletion isn't the only exit. */
  alternatives?: DangerAlternative[];
  confirmLabel: string;
  onConfirm: () => void;
  busy?: boolean;
}

export function DangerConfirm({
  open, onClose, title, subject, intro, impact, alternatives, confirmLabel, onConfirm, busy,
}: DangerConfirmProps) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-[90] flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.5)', backdropFilter: 'blur(6px)', WebkitBackdropFilter: 'blur(6px)', animation: 'or-fadein .16s ease' }}
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-[440px] rounded-[16px] overflow-hidden shadow-card-lg"
        style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', animation: 'or-scalein .18s cubic-bezier(.2,.8,.2,1)' }}
      >
        <div className="flex items-start gap-3 p-5 pb-3">
          <div className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0" style={{ background: 'color-mix(in srgb, var(--critical) 14%, transparent)', color: 'var(--critical)' }}>
            <AlertTriangle size={20} />
          </div>
          <div className="flex-1 min-w-0 pt-0.5">
            <div className="text-[15px] font-bold text-ink">{title}</div>
            {subject && <div className="text-[13px] text-ink-soft mt-0.5 truncate">{subject}</div>}
          </div>
          <button onClick={onClose} className="w-8 h-8 rounded-lg flex items-center justify-center text-ink-muted hover:bg-hover transition-colors" aria-label={tr('Fermer', 'Close')}>
            <X size={16} />
          </button>
        </div>

        <div className="px-5 pb-4 space-y-3">
          {intro && <p className="text-[13px] text-ink-soft leading-relaxed">{intro}</p>}

          {impact && impact.length > 0 && (
            <div className="rounded-[10px] p-3 space-y-1.5" style={{ background: 'var(--bg-hover)' }}>
              {impact.map((r, i) => (
                <div key={i} className="flex items-center justify-between gap-3 text-[12.5px]">
                  <span className="text-ink-muted">{r.label}</span>
                  <span className="text-ink font-medium text-right truncate">{r.value}</span>
                </div>
              ))}
            </div>
          )}

          {alternatives && alternatives.length > 0 && (
            <div className="space-y-2">
              <div className="text-[10.5px] font-semibold uppercase tracking-[.07em] text-ink-muted">{tr('Alternatives', 'Alternatives')}</div>
              {alternatives.map((a, i) => {
                const Icon = a.icon;
                return (
                  <button
                    key={i}
                    onClick={a.onClick}
                    disabled={busy}
                    className="w-full flex items-center gap-3 p-2.5 rounded-[10px] text-left transition-colors hover:bg-hover disabled:opacity-60"
                    style={{ border: '1px solid var(--border-strong)' }}
                  >
                    {Icon && <span className="text-accent shrink-0"><Icon size={17} strokeWidth={1.8} /></span>}
                    <div className="min-w-0">
                      <div className="text-[13px] font-semibold text-ink">{a.label}</div>
                      {a.description && <div className="text-[11.5px] text-ink-muted">{a.description}</div>}
                    </div>
                  </button>
                );
              })}
            </div>
          )}
        </div>

        <div className="flex items-center justify-end gap-2 px-5 py-3.5 border-t border-border">
          <button onClick={onClose} disabled={busy} className="h-9 px-4 rounded-[10px] text-[13px] font-semibold text-ink-soft hover:bg-hover transition-colors disabled:opacity-60">
            {tr('Annuler', 'Cancel')}
          </button>
          <button onClick={onConfirm} disabled={busy} className="h-9 px-4 rounded-[10px] text-[13px] font-semibold text-white transition-[filter] hover:brightness-110 disabled:opacity-60" style={{ background: 'var(--critical)' }}>
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
