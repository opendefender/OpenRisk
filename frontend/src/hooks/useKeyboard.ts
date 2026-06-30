// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useEffect, useCallback } from 'react';

/**
 * Custom hook for keyboard navigation
 * Provides simplified keyboard event handling
 */
interface KeyboardOptions {
  key: string;
  ctrl?: boolean;
  shift?: boolean;
  alt?: boolean;
  meta?: boolean;
}

export function useKeyboard(
  options: KeyboardOptions,
  callback: (event: KeyboardEvent) => void,
  disabled: boolean = false
) {
  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      if (disabled) return;

      const matchKey = event.key.toLowerCase() === options.key.toLowerCase();
      const matchCtrl = options.ctrl ? event.ctrlKey : !event.ctrlKey || options.ctrl === undefined;
      const matchShift = options.shift ? event.shiftKey : !event.shiftKey || options.shift === undefined;
      const matchAlt = options.alt ? event.altKey : !event.altKey || options.alt === undefined;
      const matchMeta = options.meta ? event.metaKey : !event.metaKey || options.meta === undefined;

      if (matchKey && matchCtrl && matchShift && matchAlt && matchMeta) {
        event.preventDefault();
        callback(event);
      }
    },
    [options, callback, disabled]
  );

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);
}

/**
 * Multiple keyboard shortcuts
 */
export function useKeyboardShortcuts(
  shortcuts: Array<{ options: KeyboardOptions; callback: (event: KeyboardEvent) => void }>,
  disabled: boolean = false
) {
  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      if (disabled) return;

      shortcuts.forEach(({ options, callback }) => {
        const matchKey = event.key.toLowerCase() === options.key.toLowerCase();
        const matchCtrl = options.ctrl ? event.ctrlKey : !event.ctrlKey || options.ctrl === undefined;
        const matchShift = options.shift ? event.shiftKey : !event.shiftKey || options.shift === undefined;
        const matchAlt = options.alt ? event.altKey : !event.altKey || options.alt === undefined;
        const matchMeta = options.meta ? event.metaKey : !event.metaKey || options.meta === undefined;

        if (matchKey && matchCtrl && matchShift && matchAlt && matchMeta) {
          event.preventDefault();
          callback(event);
        }
      });
    },
    [shortcuts, disabled]
  );

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);
}
