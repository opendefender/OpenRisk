// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

const COLORS: Record<string, string> = {
  CRITICAL: 'bg-danger/10 text-danger-text border-danger/20',
  HIGH: 'bg-warning/10 text-warning-text border-warning/20',
  MEDIUM: 'bg-warning/10 text-warning-text border-warning/20',
  LOW: 'bg-accent-soft text-info-text border-accent-line',
};

export const CriticalityBadge = ({ level }: { level: string }) => (
  <span
    className={`px-2 py-0.5 rounded text-[10px] font-bold border ${COLORS[level] ?? 'bg-surface-3/10 border-border-strong/20 text-fg-secondary'}`}
  >
    {level}
  </span>
);
