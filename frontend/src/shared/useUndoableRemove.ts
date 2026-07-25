// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React hook that pairs undoableDelete (UX-12/28) with a local set of
// optimistically-hidden ids, for lists backed by react-query mutations. The row
// vanishes the instant the user clicks (no confirm modal); an "Undo" toast lets
// them bring it back; the real mutation only fires once the grace window elapses
// un-undone. This is the mechanical adoption path for MINOR, reversible deletes —
// important/irreversible ones use ImpactDialog instead.
//
// Usage:
//   const { undoRemove, isHidden } = useUndoableRemove();
//   const visible = items.filter((i) => !isHidden(i.id));
//   // ...on the delete button:
//   undoRemove(item.id, {
//     message: tr('Élément supprimé', 'Item removed'),
//     undoLabel: tr('Annuler', 'Undo'),
//     onCommit: () => deleteItem.mutate(item.id),
//     onError: () => toast.error(tr('Suppression impossible', 'Could not delete')),
//   });

import { useCallback, useState } from 'react';
import { undoableDelete, type UndoableDeleteOptions } from './undoableDelete';

export function useUndoableRemove() {
  const [hidden, setHidden] = useState<ReadonlySet<string>>(() => new Set());

  const unhide = useCallback((id: string) => {
    setHidden((prev) => {
      if (!prev.has(id)) return prev;
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  }, []);

  const isHidden = useCallback((id: string) => hidden.has(id), [hidden]);

  const undoRemove = useCallback(
    (id: string, opts: UndoableDeleteOptions) => {
      // Optimistically hide the row first, then offer Undo.
      setHidden((prev) => new Set(prev).add(id));
      undoableDelete({
        ...opts,
        onUndo: () => {
          unhide(id);
          opts.onUndo?.();
        },
        onError: (err) => {
          unhide(id);
          opts.onError?.(err);
        },
      });
    },
    [unhide]
  );

  return { undoRemove, isHidden, hidden };
}
