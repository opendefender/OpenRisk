// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Drives a `Button` `feedback="success"` prop for the "the server just
 * acknowledged this" moment (#751 phase 2).
 *
 * `flashSuccess` is called once the mutation has actually resolved — never on
 * an optimistic update, never on client-side Zod validity, because the check
 * is a claim that the server accepted the write. The toast stays the only
 * announcement; this is a silent, transient visual on the button itself, so it
 * clears on its own after ~1.8s (or immediately, if the caller starts a new
 * `loading` submission first — Button gives loading precedence).
 */

import { useCallback, useEffect, useRef, useState } from 'react';

const HOLD_MS = 1800;

export function useSuccessFeedback(): {
  feedback: 'success' | undefined;
  flashSuccess: () => void;
} {
  const [feedback, setFeedback] = useState<'success' | undefined>(undefined);
  const timer = useRef<number | null>(null);

  const flashSuccess = useCallback(() => {
    if (timer.current !== null) window.clearTimeout(timer.current);
    setFeedback('success');
    timer.current = window.setTimeout(() => setFeedback(undefined), HOLD_MS);
  }, []);

  useEffect(
    () => () => {
      if (timer.current !== null) window.clearTimeout(timer.current);
    },
    [],
  );

  return { feedback, flashSuccess };
}
