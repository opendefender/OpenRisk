// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Impact radiography before a STRUCTURAL / irreversible action (UX-11, Kahneman
// System 2): show the full consequences (linked objects) and offer at least one
// alternative (e.g. transfer owned risks) before the destructive confirm. Reserve
// this for the rare + irreversible; minor reversible deletes use undoableDelete.

import type { ReactNode } from 'react';
import { AlertTriangle, X } from 'lucide-react';

export interface ImpactItem {
  label: string;
  detail?: string;
  icon?: ReactNode;
}

export interface ImpactAlternative {
  label: string;
  description?: string;
  onClick: () => void;
}

interface ImpactDialogProps {
  open: boolean;
  title: string;
  /** The thing being acted on, e.g. a member's name. */
  subject: string;
  /** One-line statement of what the action does. */
  description?: string;
  /** Linked objects / consequences to surface. Empty = nothing else affected. */
  impacts: ImpactItem[];
  /** Non-destructive escape hatches (e.g. "Transfer owned risks"). */
  alternatives?: ImpactAlternative[];
  confirmLabel: string;
  cancelLabel?: string;
  loading?: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

export function ImpactDialog({
  open,
  title,
  subject,
  description,
  impacts,
  alternatives = [],
  confirmLabel,
  cancelLabel = 'Annuler',
  loading = false,
  onConfirm,
  onClose,
}: ImpactDialogProps) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'var(--surface-overlay)' }} onClick={onClose}>
      <div
        className="w-full max-w-[460px] rounded-[16px] overflow-hidden"
        style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-overlay)', animation: 'or-scalein .14s cubic-bezier(.2,.8,.2,1)' }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
      >
        <div className="flex items-start gap-3 p-5" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="w-9 h-9 rounded-[10px] flex items-center justify-center shrink-0" style={{ background: 'color-mix(in srgb,var(--critical) 14%,transparent)', color: 'var(--critical)' }}>
            <AlertTriangle size={18} />
          </div>
          <div className="flex-1 min-w-0">
            <div className="text-[15px] font-bold text-ink">{title}</div>
            {description && <div className="text-[13px] text-ink-soft mt-0.5 leading-snug">{description}</div>}
          </div>
          <button onClick={onClose} className="w-8 h-8 rounded-[8px] flex items-center justify-center text-ink-muted hover:bg-hover transition-colors shrink-0" aria-label={cancelLabel}><X size={17} /></button>
        </div>

        <div className="p-5">
          <div className="text-[13.5px] font-semibold text-ink mb-2">{subject}</div>

          {impacts.length > 0 ? (
            <>
              <div className="text-[12px] text-ink-muted uppercase tracking-wide font-semibold mb-2">
                {'Conséquences'}
              </div>
              <div className="flex flex-col gap-1.5 mb-4">
                {impacts.map((it, i) => (
                  <div key={i} className="flex items-center gap-2.5 rounded-[10px] px-3 py-2" style={{ background: 'var(--bg-hover)' }}>
                    {it.icon && <span className="text-ink-muted shrink-0">{it.icon}</span>}
                    <span className="text-[13px] text-ink flex-1">{it.label}</span>
                    {it.detail && <span className="mono text-[12px] text-ink-soft">{it.detail}</span>}
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className="text-[13px] text-ink-soft mb-4">Aucun objet lié — cette action est sans effet de bord.</div>
          )}

          {alternatives.length > 0 && (
            <div className="mb-4">
              <div className="text-[12px] text-ink-muted uppercase tracking-wide font-semibold mb-2">Alternatives</div>
              <div className="flex flex-col gap-2">
                {alternatives.map((alt, i) => (
                  <button
                    key={i}
                    onClick={alt.onClick}
                    disabled={loading}
                    className="text-left rounded-[10px] px-3 py-2.5 transition-colors hover:bg-hover"
                    style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-app)' }}
                  >
                    <div className="text-[13px] font-semibold text-ink">{alt.label}</div>
                    {alt.description && <div className="text-[12px] text-ink-soft mt-0.5">{alt.description}</div>}
                  </button>
                ))}
              </div>
            </div>
          )}

          <div className="flex justify-end gap-2.5">
            <button onClick={onClose} disabled={loading} className="h-[40px] px-4 rounded-[10px] text-[13px] font-semibold text-ink" style={{ background: 'var(--bg-hover)', border: '1px solid var(--border-strong)' }}>
              {cancelLabel}
            </button>
            <button
              onClick={onConfirm}
              disabled={loading}
              className="h-[40px] px-4 rounded-[10px] text-[13px] font-semibold text-text-primary"
              style={{ background: 'var(--critical)', opacity: loading ? 0.7 : 1 }}
            >
              {loading ? '…' : confirmLabel}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
