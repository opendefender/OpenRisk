// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Reversible navigation primitives.

import { useCallback, useEffect, useRef } from 'react';
import { useLocation, useNavigate } from 'react-router';
import { parentHref } from './routeModel';

/**
 * A "back" that always goes somewhere sensible.
 *
 * Popping history is right when the user navigated here, but wrong when they
 * arrived by deep link, opened a new tab, or landed from an email — there is no
 * entry to pop and `navigate(-1)` either does nothing or leaves the app. So:
 * pop only when this app put something in the history stack, otherwise walk up
 * the route tree.
 *
 * Returns the parent href too, so callers can render a real <a> (middle-click,
 * "open in new tab") rather than a button that only works on left click.
 */
export function useBackTo(): { goBack: () => void; parent: string } {
  const navigate = useNavigate();
  const { pathname, key } = useLocation();
  const parent = parentHref(pathname) ?? '/';

  // react-router gives the initial entry the key "default"; anything else means
  // we navigated within the app and there is something to pop.
  const canPop = key !== 'default';

  const goBack = useCallback(() => {
    if (canPop) navigate(-1);
    else navigate(parent);
  }, [canPop, navigate, parent]);

  return { goBack, parent };
}

/**
 * Closes an overlay on Escape.
 *
 * Registered on document with `keydown`. Only the most recently opened overlay
 * responds: a stack of registrations would otherwise close a drawer and the
 * dialog above it with one press, which reads as the app losing your place.
 */
const escStack: Array<() => void> = [];

export function useEscapeToClose(isOpen: boolean, onClose: () => void) {
  // Keep the latest handler without re-registering (and re-ordering) the stack
  // on every render. Assigned in an effect, not during render: a ref write in
  // the render body is not guaranteed to have happened by the time React
  // commits, and it breaks the rules-of-react lint for good reason.
  const handler = useRef(onClose);
  useEffect(() => {
    handler.current = onClose;
  }, [onClose]);

  useEffect(() => {
    if (!isOpen) return;
    const entry = () => handler.current();
    escStack.push(entry);
    return () => {
      const i = escStack.lastIndexOf(entry);
      if (i >= 0) escStack.splice(i, 1);
    };
  }, [isOpen]);
}

// One document listener for the whole app, dispatching to the top of the stack.
if (typeof document !== 'undefined') {
  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || escStack.length === 0) return;
    const top = escStack[escStack.length - 1];
    e.stopPropagation();
    top();
  });
}
