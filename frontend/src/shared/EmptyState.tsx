// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Canonical empty state (docs/UI_ELEVATION_PROPOSAL §5, UX-04): illustration +
// value line + a primary action. Every list screen's empty state should render
// this so no screen dead-ends on a blank surface.

import type { ReactNode } from 'react';

interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  message?: string;
  /** Primary action (a button/link). Encouraged — UX-04 requires an action. */
  action?: ReactNode;
  /** Secondary hint, e.g. "or see an example". */
  hint?: ReactNode;
}

export function EmptyState({ icon, title, message, action, hint }: EmptyStateProps) {
  return (
    <div className="or-empty">
      {icon && <div className="or-empty-icon">{icon}</div>}
      <h3>{title}</h3>
      {message && <p>{message}</p>}
      {action && <div style={{ marginTop: 6 }}>{action}</div>}
      {hint && <div style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>{hint}</div>}
    </div>
  );
}
