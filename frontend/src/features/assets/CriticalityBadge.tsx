// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

const COLORS: Record<string, string> = {
  CRITICAL: 'bg-red-500/10 text-red-500 border-red-500/20',
  HIGH: 'bg-orange-500/10 text-orange-500 border-orange-500/20',
  MEDIUM: 'bg-yellow-500/10 text-yellow-500 border-yellow-500/20',
  LOW: 'bg-blue-500/10 text-blue-500 border-blue-500/20',
};

export const CriticalityBadge = ({ level }: { level: string }) => (
  <span
    className={`px-2 py-0.5 rounded text-[10px] font-bold border ${COLORS[level] ?? 'bg-zinc-500/10 border-zinc-500/20 text-zinc-400'}`}
  >
    {level}
  </span>
);
