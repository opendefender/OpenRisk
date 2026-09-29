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
 *
 * #751 phase 4 — restyled, not replaced. `richColors` brought sonner's own hex
 * palette in over our tokens; `classNames` maps the surface to
 * --bg-elevated/--border/--elev-3 instead, and every type gets its own icon so
 * the signal is never colour alone. `bottom-right` because `top-right`
 * collided with the notif panel (top: 44px) and the header controls above it.
 * Position/stacking geometry (`.notif-*` and CSS overrides) live in
 * index.css's toast section; only the props sonner needs live here.
 */

import { CheckCircle2, Info, AlertTriangle, XCircle, Loader2 } from 'lucide-react';
import { Toaster } from 'sonner';
import { useUIStore } from '../../store/uiStore';

const TOAST_CLASSNAMES = {
  toast: 'or-toast',
  title: 'or-toast-title',
  description: 'or-toast-description',
  icon: 'or-toast-icon',
};

export default function ThemedToaster() {
  const theme = useUIStore((s) => s.theme);
  return (
    <Toaster
      position="bottom-right"
      offset={16}
      theme={theme}
      closeButton
      expand
      gap={8}
      visibleToasts={3}
      toastOptions={{ classNames: TOAST_CLASSNAMES }}
      icons={{
        success: <CheckCircle2 size={16} style={{ color: 'var(--success-text)' }} />,
        info: <Info size={16} style={{ color: 'var(--info-text)' }} />,
        warning: <AlertTriangle size={16} style={{ color: 'var(--warning-text)' }} />,
        error: <XCircle size={16} style={{ color: 'var(--danger-text)' }} />,
        loading: (
          <Loader2 size={16} className="animate-spin" style={{ color: 'var(--fg-muted)' }} />
        ),
      }}
    />
  );
}
