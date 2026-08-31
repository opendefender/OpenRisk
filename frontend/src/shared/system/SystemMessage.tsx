// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Shared layout for the system pages (404 / 500 / maintenance / access denied).
// Branded, theme-aware (paints its own tokens), and quiet under
// prefers-reduced-motion (the subtle fade is gated by the global reduced-motion
// rule in index.css). Every system page offers a way out — a dead end is a bug.

import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { OpenRiskLogo } from '../Logo';

interface SystemMessageProps {
  icon: LucideIcon;
  /** Short status code shown large, e.g. "404" / "500". Optional. */
  code?: string;
  title: string;
  message: ReactNode;
  /** Primary + secondary actions (buttons/links), rendered in order. */
  actions?: ReactNode;
  /** Accent colour token for the icon, defaults to the muted ink. */
  tone?: string;
  /** Extra content below the message (e.g. an error id, a permission chip). */
  children?: ReactNode;
}

export function SystemMessage({ icon: Icon, code, title, message, actions, tone, children }: SystemMessageProps) {
  return (
    <div
      role="alert"
      className="min-h-screen w-full flex items-center justify-center p-6"
      style={{ background: 'var(--bg-primary)', color: 'var(--fg-primary)' }}
    >
      <div className="max-w-md w-full text-center or-fadeup">
        <div className="flex justify-center mb-6">
          <OpenRiskLogo size={34} />
        </div>

        <div
          className="w-16 h-16 rounded-2xl flex items-center justify-center mb-5 mx-auto"
          style={{ background: 'var(--bg-hover)', color: tone ?? 'var(--fg-muted)' }}
        >
          <Icon size={28} strokeWidth={1.7} />
        </div>

        {code && (
          <div className="disp mono text-[40px] font-bold leading-none mb-2" style={{ color: tone ?? 'var(--fg-muted)' }}>
            {code}
          </div>
        )}

        <h1 className="text-[18px] font-semibold text-ink mb-2">{title}</h1>
        <div className="text-[13.5px] text-ink-soft leading-relaxed mb-5">{message}</div>

        {children}

        {actions && <div className="flex items-center justify-center gap-2.5 mt-5 flex-wrap">{actions}</div>}
      </div>
    </div>
  );
}
