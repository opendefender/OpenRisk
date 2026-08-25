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
import { AlertTriangle, type LucideIcon } from 'lucide-react';
import { useUIStore } from '../store/uiStore';
import { Button, Modal } from './ds';

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

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="sm"
      title={title}
      subtitle={subject}
      closeLabel={tr('Fermer', 'Close')}
      /* While the destructive action is running the dialog cannot be
         dismissed: closing it would leave the user with no feedback about an
         operation that is already half-applied. */
      dismissable={!busy}
      leading={
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-danger-surface text-danger-text">
          <AlertTriangle size={18} aria-hidden="true" />
        </span>
      }
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            {tr('Annuler', 'Cancel')}
          </Button>
          <Button variant="destructive" onClick={onConfirm} loading={busy}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        {intro && <p className="text-sm leading-relaxed text-text-secondary">{intro}</p>}

        {impact && impact.length > 0 && (
          /* The radiography. A definition list rather than divs: the pairs are
             label/value, and saying so is what lets a screen reader read them
             as pairs instead of as one run-on line. */
          <dl className="space-y-1.5 rounded-md bg-surface-3 p-3">
            {impact.map((r, i) => (
              <div key={i} className="flex items-center justify-between gap-3 text-xs">
                <dt className="text-text-muted">{r.label}</dt>
                <dd className="truncate text-right font-medium text-text-primary">{r.value}</dd>
              </div>
            ))}
          </dl>
        )}

        {alternatives && alternatives.length > 0 && (
          <div className="space-y-2">
            <p className="text-2xs font-semibold uppercase tracking-caps text-text-muted">
              {tr('Alternatives', 'Alternatives')}
            </p>
            {alternatives.map((a, i) => {
              const Icon = a.icon;
              return (
                <button
                  key={i}
                  type="button"
                  onClick={a.onClick}
                  disabled={busy}
                  className="flex w-full items-center gap-3 rounded-md border border-control p-2.5 text-left transition-colors duration-fast ease-out hover:bg-surface-3 disabled:opacity-60"
                >
                  {Icon && (
                    <span className="shrink-0 text-accent">
                      <Icon size={17} strokeWidth={1.8} aria-hidden="true" />
                    </span>
                  )}
                  <div className="min-w-0">
                    <span className="block text-sm font-semibold text-text-primary">{a.label}</span>
                    {a.description && (
                      <span className="block text-2xs text-text-muted">{a.description}</span>
                    )}
                  </div>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </Modal>
  );
}
