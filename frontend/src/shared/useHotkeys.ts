// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Global single-key shortcut layer (UX-26): discoverable keyboard shortcuts for
// the key actions, paired with the `?` help overlay (ShortcutsOverlay). Single
// letters (n, t, g, /, ?) MUST never hijack normal typing, so this ignores
// keystrokes while the user is in an input/textarea/select/contenteditable, and
// while ⌘/Ctrl/Alt are held (those belong to the browser or ⌘K). Shift is allowed
// because `?` is Shift+/. Modal-scoped keys (Esc, ⌘Enter) keep using useKeyboard.

import { useEffect, useRef } from 'react';

export interface Hotkey {
  /** Single key to match, case-insensitive: 'n', '/', '?', 't', 'g'. */
  key: string;
  handler: (e: KeyboardEvent) => void;
}

function isTyping(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el || !el.tagName) return false;
  const tag = el.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable;
}

export function useHotkeys(hotkeys: Hotkey[], enabled = true): void {
  // Keep the latest handlers without re-binding the listener every render.
  const ref = useRef(hotkeys);
  ref.current = hotkeys;

  useEffect(() => {
    if (!enabled) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return; // leave ⌘K / browser combos alone
      if (isTyping(e.target)) return; // never steal keystrokes from a field
      const key = e.key.toLowerCase();
      for (const hk of ref.current) {
        if (hk.key.toLowerCase() === key) {
          e.preventDefault();
          hk.handler(e);
          return;
        }
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [enabled]);
}
