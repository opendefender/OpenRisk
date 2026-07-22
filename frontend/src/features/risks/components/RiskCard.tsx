// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { User, Database, ShieldAlert, Box } from 'lucide-react';
import type { Risk } from '../../../hooks/useRiskStore';

const SourceIcon = ({ source }: { source: string }) => {
  switch (source) {
    case 'THEHIVE':
      return <ShieldAlert size={12} className="text-yellow-500" />;
    case 'OPENRMF':
      return <Database size={12} className="text-blue-500" />;
    case 'OPENCTI':
      return <Box size={12} className="text-purple-500" />;
    default:
      return <User size={12} className="text-zinc-500" />; // Manual
  }
};

interface RiskCardProps {
  risk: Risk;
  onClick?: () => void;
}

export const RiskCard = ({ risk, onClick }: RiskCardProps) => {
  const riskLevelColor = {
    CRITICAL: 'bg-red-900/20 border-red-700/50',
    HIGH: 'bg-orange-900/20 border-orange-700/50',
    MEDIUM: 'bg-yellow-900/20 border-yellow-700/50',
    LOW: 'bg-blue-900/20 border-blue-700/50',
  }[risk.level || 'MEDIUM'] || 'bg-blue-900/20 border-blue-700/50';

  return (
    <div
      onClick={onClick}
      className={`border rounded-lg p-4 cursor-pointer transition-colors hover:bg-zinc-800/50 ${riskLevelColor}`}
    >
      <div className="flex items-start justify-between mb-3">
        <h3 className="font-semibold text-white flex-1">{risk.title}</h3>
        <div className="flex items-center gap-2 ml-2">
          <span className="text-lg font-bold text-white">{Math.round(risk.score || 0)}</span>
          <span className="text-xs text-zinc-400">/ 100</span>
        </div>
      </div>

      <p className="text-sm text-zinc-400 mb-3 line-clamp-2">{risk.description}</p>

      <div className="flex items-center gap-1 text-[10px] font-bold border border-white/10 px-2 py-1 rounded bg-zinc-900">
        <SourceIcon source={risk.source} />
        <span className="text-zinc-400">{risk.source}</span>
      </div>
    </div>
  );
};