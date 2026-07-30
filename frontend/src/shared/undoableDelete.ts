// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Undoable delete for MINOR, reversible items (UX-12 / UX-28, Kahneman System 1):
// no confirmation modal — the item disappears immediately and a toast offers
// "Undo" for a few seconds. The real (irreversible) commit only fires if the
// window elapses without an undo. Important/irreversible deletions use the
// ImpactDialog radiography instead.

import { toast } from 'sonner';

export interface UndoableDeleteOptions {
  /** Toast message, e.g. "Étiquette supprimée". */
  message: string;
  /** Undo button label (default "Annuler"). */
  undoLabel?: string;
  /** Grace window before the delete is committed (default 7000ms, UX-12 ≥ 7s). */
  delayMs?: number;
  /** The real, irreversible delete — called once the window elapses un-undone. */
  onCommit: () => void | Promise<void>;
  /** Restore the optimistic UI change when the user undoes. */
  onUndo?: () => void;
  /** Called if onCommit throws (so the caller can restore + surface an error). */
  onError?: (err: unknown) => void;
}

/**
 * Call AFTER optimistically removing the item from the UI. Shows an "Undo" toast;
 * commits the delete after the grace window unless undone.
 */
export function undoableDelete(opts: UndoableDeleteOptions): void {
  const delay = opts.delayMs ?? 7000;
  let undone = false;

  const timer = setTimeout(() => {
    if (undone) return;
    try {
      const r = opts.onCommit();
      if (r && typeof (r as Promise<void>).catch === 'function') {
        (r as Promise<void>).catch((e) => opts.onError?.(e));
      }
    } catch (e) {
      opts.onError?.(e);
    }
  }, delay);

  toast(opts.message, {
    duration: delay,
    action: {
      label: opts.undoLabel ?? 'Annuler',
      onClick: () => {
        undone = true;
        clearTimeout(timer);
        opts.onUndo?.();
      },
    },
  });
}
