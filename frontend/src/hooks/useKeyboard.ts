// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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
