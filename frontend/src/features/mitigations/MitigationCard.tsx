// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useMemo } from 'react';
import { motion } from 'framer-motion';
import { Calendar, AlertCircle, Lock } from 'lucide-react';
import type { Mitigation } from '../../types/mitigation';
import { RiskBadge, UserAvatar, ProgressBar, StatusDot } from '../../components/shared';
import { AutoDetectedBadge } from '../../components/shared/AutoDetectedBadge';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

interface MitigationCardProps {
  mitigation: Mitigation;
  onClick?: () => void;
  isDragging?: boolean;
  isSelected?: boolean;
  onToggleSelect?: () => void;
}

// The card USED TO derive a band here from mitigation.risk_score with its own
// 80/60/40 cuts — a fifth threshold table, on a fourth scale, disagreeing with
// the three others. The plan's own priority is a server field and says the same
// thing without the arithmetic. See docs/scoring/SCORE_MODEL.md.
const getRiskLevel = (priority?: string): 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' => {
  switch ((priority ?? '').toLowerCase()) {
    case 'critical':
      return 'CRITICAL';
    case 'high':
      return 'HIGH';
    case 'medium':
      return 'MEDIUM';
    default:
      return 'LOW';
  }
};

const getDaysUntilDeadline = (dueDate: string): number => {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const deadline = new Date(dueDate);
  deadline.setHours(0, 0, 0, 0);
  const diffTime = deadline.getTime() - today.getTime();
  return Math.ceil(diffTime / (1000 * 60 * 60 * 24));
};

const getDeadlineColor = (daysLeft: number): string => {
  if (daysLeft < 0) return 'text-danger-text border-danger/50 bg-danger/10';
  if (daysLeft < 3) return 'text-danger-text border-danger/50 bg-danger/10';
  if (daysLeft < 7) return 'text-warning-text border-warning/50 bg-warning/10';
  return 'text-success-text border-success/50 bg-success/10';
};

const getDeadlineLabel = (daysLeft: number): string => {
  if (daysLeft < 0) return `${Math.abs(daysLeft)}j en retard`;
  if (daysLeft === 0) return 'Aujourd\'hui';
  if (daysLeft === 1) return 'Demain';
  return `${daysLeft}j restants`;
};

export const MitigationCard = ({
  mitigation,
  onClick,
  isDragging = false,
  isSelected = false,
  onToggleSelect,
}: MitigationCardProps) => {
  const daysLeft = useMemo(() => getDaysUntilDeadline(mitigation.due_date), [mitigation.due_date]);
  const deadlineColor = useMemo(() => getDeadlineColor(daysLeft), [daysLeft]);
  const deadlineLabel = useMemo(() => getDeadlineLabel(daysLeft), [daysLeft]);
  const riskLevel = useMemo(() => getRiskLevel(mitigation.priority), [mitigation.priority]);

  const completedSubActions = mitigation.sub_actions?.filter((s) => s.status === 'DONE')?.length ?? 0;
  const totalSubActions = mitigation.sub_actions?.length ?? 0;
  const autoCompletedCount = mitigation.auto_detected_count ?? 0;

  const hasAutoDetected = autoCompletedCount > 0;
  const isOverdue = daysLeft < 0;

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -10 }}
      whileHover={{ scale: 1.02, boxShadow: '0 8px 16px rgba(0,0,0,0.3)' }}
      whileTap={{ scale: 0.98 }}
      className={cn(
        'p-4 rounded-lg border transition-all duration-200 cursor-grab active:cursor-grabbing',
        isDragging ? 'opacity-50 scale-95' : 'opacity-100 scale-100',
        isSelected ? 'border-accent bg-accent-soft' : 'border-border-default bg-surface-1/40 hover:border-border-default',
      )}
      onClick={onClick}
    >
      {/* Header: Title + Risk Badge */}
      <div className="flex items-start justify-between gap-2 mb-3">
        <div className="flex-1 min-w-0">
          <h3 className="font-semibold text-fg-primary text-sm truncate">{mitigation.title}</h3>
        </div>
        <RiskBadge level={riskLevel} size="sm" />
      </div>

      {/* Risk context (optional) */}
      {mitigation.risk_title && (
        <p className="text-xs text-fg-secondary mb-2 truncate">
          Risque: {mitigation.risk_title}
        </p>
      )}

      {/* Progress + Sub-actions count */}
      <div className="mb-3">
        <ProgressBar
          value={completedSubActions}
          max={totalSubActions || 1}
          label={`${completedSubActions}/${totalSubActions} actions${autoCompletedCount > 0 ? `, ${autoCompletedCount} auto` : ''}`}
          size="sm"
        />
      </div>

      {/* Deadline + Auto-detected badge row */}
      <div className="flex items-center justify-between gap-2 mb-3 flex-wrap">
        <motion.div
          className={cn(
            'inline-flex items-center gap-1.5 px-2 py-1 rounded border text-xs font-medium',
            deadlineColor
          )}
          animate={isOverdue ? { boxShadow: [
            '0 0 0 0 rgba(239, 68, 68, 0.4)',
            '0 0 0 8px rgba(239, 68, 68, 0)',
          ] } : undefined}
          transition={isOverdue ? { duration: 1.5, repeat: Infinity } : undefined}
        >
          <Calendar size={12} />
          <span>{deadlineLabel}</span>
        </motion.div>

        {hasAutoDetected && (
          <AutoDetectedBadge
            scanId={mitigation.sub_actions?.[0]?.scanner_details?.scan_id}
            detectedAt={mitigation.sub_actions?.[0]?.scanner_details?.detected_at}
            size="sm"
          />
        )}
      </div>

      {/* Assigned user + Editing lock */}
      <div className="flex items-center justify-between gap-2 pt-3 border-t border-border-subtle">
        <div className="flex items-center gap-1">
          {mitigation.assigned_to_user ? (
            <UserAvatar
              name={mitigation.assigned_to_user.name}
              avatar={mitigation.assigned_to_user.avatar}
              size="xs"
              tooltip={true}
            />
          ) : (
            <div className="w-6 h-6 rounded-full border border-border-default bg-surface-2/50 flex items-center justify-center">
              <span className="text-xs text-fg-muted">−</span>
            </div>
          )}
        </div>

        {/* Editing lock indicator */}
        {mitigation.editing_lock && (
          <motion.div
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
            className="flex items-center gap-1 text-xs text-warning-text bg-warning/10 px-2 py-1 rounded"
            title={`Édité par ${mitigation.editing_lock.user_name}`}
          >
            <Lock size={12} />
            <span className="truncate">{mitigation.editing_lock.user_name}</span>
          </motion.div>
        )}
      </div>
    </motion.div>
  );
};
