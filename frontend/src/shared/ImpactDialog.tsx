// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Impact radiography before a STRUCTURAL / irreversible action (UX-11, Kahneman
// System 2): show the full consequences (linked objects) and offer at least one
// alternative (e.g. transfer owned risks) before the destructive confirm. Reserve
// this for the rare + irreversible; minor reversible deletes use undoableDelete.

import type { ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';
import { Button, Modal } from './ds';

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
  return (
    <Modal
      open={open}
      onClose={onClose}
      size="sm"
      title={title}
      subtitle={description}
      closeLabel={cancelLabel}
      dismissable={!loading}
      leading={
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-danger-surface text-danger-text">
          <AlertTriangle size={18} aria-hidden="true" />
        </span>
      }
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={loading}>
            {cancelLabel}
          </Button>
          <Button variant="destructive" onClick={onConfirm} loading={loading}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <p className="mb-2 text-sm font-semibold text-fg-primary">{subject}</p>

      {impacts.length > 0 ? (
        <>
          <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
            Conséquences
          </p>
          <ul className="mb-4 flex flex-col gap-1.5">
            {impacts.map((it, i) => (
              <li key={i} className="flex items-center gap-2.5 rounded-md bg-surface-3 px-3 py-2">
                {it.icon && <span className="shrink-0 text-fg-muted">{it.icon}</span>}
                <span className="flex-1 text-sm text-fg-primary">{it.label}</span>
                {it.detail && (
                  <span className="mono text-xs tabular-nums text-fg-secondary">{it.detail}</span>
                )}
              </li>
            ))}
          </ul>
        </>
      ) : (
        /* An explicit "nothing else is affected" rather than an empty region:
           the absence of consequences is itself the answer the user came for. */
        <p className="mb-4 text-sm text-fg-secondary">
          Aucun objet lié — cette action est sans effet de bord.
        </p>
      )}

      {alternatives.length > 0 && (
        <div>
          <p className="mb-2 text-2xs font-semibold uppercase tracking-caps text-fg-muted">
            Alternatives
          </p>
          <div className="flex flex-col gap-2">
            {alternatives.map((alt, i) => (
              <button
                key={i}
                type="button"
                onClick={alt.onClick}
                disabled={loading}
                className="rounded-md border border-control bg-surface-1 px-3 py-2.5 text-left transition-colors duration-fast ease-out hover:bg-surface-3 disabled:opacity-60"
              >
                <span className="block text-sm font-semibold text-fg-primary">{alt.label}</span>
                {alt.description && (
                  <span className="mt-0.5 block text-xs text-fg-secondary">{alt.description}</span>
                )}
              </button>
            ))}
          </div>
        </div>
      )}
    </Modal>
  );
}
