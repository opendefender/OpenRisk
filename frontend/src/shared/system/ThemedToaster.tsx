// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * The toast surface, in its own module so it can be loaded after first paint.
 *
 * Nothing can be toasted until the user has done something, and the first thing
 * a user does is never in the first frame. Keeping this in the entry meant
 * sonner — 64 KB of raw source — was fetched and parsed before the sign-in
 * screen could render, on every cold load. It is lazy in main.tsx instead.
 *
 * Callers are unaffected: they import `toast` from 'sonner' directly, and the
 * import that resolves it is theirs, not this file's.
 */

import { Toaster } from 'sonner';
import { useUIStore } from '../../store/uiStore';

/** Toasts follow the active theme (dc.html §8). */
export default function ThemedToaster() {
  const theme = useUIStore((s) => s.theme);
  return <Toaster position="top-right" theme={theme} richColors closeButton />;
}
