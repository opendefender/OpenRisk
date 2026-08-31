// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { User, Database, ShieldAlert, Box } from 'lucide-react';
import type { Risk } from '../../../hooks/useRiskStore';

const SourceIcon = ({ source }: { source: string }) => {
  switch (source) {
    case 'THEHIVE':
      return <ShieldAlert size={12} className="text-warning-text" />;
    case 'OPENRMF':
      return <Database size={12} className="text-info-text" />;
    case 'OPENCTI':
      return <Box size={12} className="text-purple-500" />;
    default:
      return <User size={12} className="text-fg-muted" />; // Manual
  }
};

interface RiskCardProps {
  risk: Risk;
  onClick?: () => void;
}

export const RiskCard = ({ risk, onClick }: RiskCardProps) => {
  // Severity comes from the server-computed `criticality` band (derived from the
  // score), NOT the legacy `level` field, which is not reliably populated and
  // rendered every risk as MEDIUM regardless of score.
  const band = (risk.criticality ?? risk.level ?? 'low').toString().toUpperCase();
  const riskLevelColor = {
    CRITICAL: 'bg-danger-surface border-danger/50',
    HIGH: 'bg-warning-surface border-warning/50',
    MEDIUM: 'bg-warning-surface border-warning/50',
    LOW: 'bg-info-surface border-accent-line',
  }[band] || 'bg-info-surface border-accent-line';

  return (
    <div
      onClick={onClick}
      className={`border rounded-lg p-4 cursor-pointer transition-colors hover:bg-surface-2/50 ${riskLevelColor}`}
    >
      <div className="flex items-start justify-between mb-3">
        <h3 className="font-semibold text-fg-primary flex-1">{risk.title}</h3>
        <div className="flex items-center gap-2 ml-2">
          <span className="text-lg font-bold text-fg-primary">{Math.round(risk.score || 0)}</span>
          <span className="text-xs text-fg-secondary">/ 100</span>
        </div>
      </div>

      <p className="text-sm text-fg-secondary mb-3 line-clamp-2">{risk.description}</p>

      <div className="flex items-center gap-1 text-[10px] font-bold border border-border-strong/10 px-2 py-1 rounded bg-surface-1">
        <SourceIcon source={risk.source} />
        <span className="text-fg-secondary">{risk.source}</span>
      </div>
    </div>
  );
};