// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Small hook for the "?focus=<id>" deep-link param (UX-1). The universal search
// palette navigates to e.g. /risks?focus=<id>; the target page reads the id to
// open that entity's drawer, then clears the param so a refresh/back-navigation
// doesn't re-trigger it. Consumers decide WHEN to clear — e.g. only once the
// entity is actually present in the loaded list — so a deep-link still resolves
// after the list finishes loading.

import { useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';

export function useFocusParam(): { focusId: string | null; clearFocus: () => void } {
  const [params, setParams] = useSearchParams();
  const focusId = params.get('focus');
  const clearFocus = useCallback(() => {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.delete('focus');
        return next;
      },
      { replace: true }
    );
  }, [setParams]);
  return { focusId, clearFocus };
}
