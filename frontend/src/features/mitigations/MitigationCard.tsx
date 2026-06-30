// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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

const getRiskLevel = (score?: number): 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' => {
  if (!score) return 'LOW';
  if (score >= 80) return 'CRITICAL';
  if (score >= 60) return 'HIGH';
  if (score >= 40) return 'MEDIUM';
  return 'LOW';
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
  if (daysLeft < 0) return 'text-red-400 border-red-500/50 bg-red-500/10';
  if (daysLeft < 3) return 'text-red-400 border-red-500/50 bg-red-500/10';
  if (daysLeft < 7) return 'text-yellow-400 border-yellow-500/50 bg-yellow-500/10';
  return 'text-emerald-400 border-emerald-500/50 bg-emerald-500/10';
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
  const riskLevel = useMemo(() => getRiskLevel(mitigation.risk_score), [mitigation.risk_score]);

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
        isSelected ? 'border-blue-500 bg-blue-500/5' : 'border-zinc-700 bg-zinc-900/40 hover:border-zinc-600',
      )}
      onClick={onClick}
    >
      {/* Header: Title + Risk Badge */}
      <div className="flex items-start justify-between gap-2 mb-3">
        <div className="flex-1 min-w-0">
          <h3 className="font-semibold text-white text-sm truncate">{mitigation.title}</h3>
        </div>
        <RiskBadge level={riskLevel} size="sm" />
      </div>

      {/* Risk context (optional) */}
      {mitigation.risk_title && (
        <p className="text-xs text-zinc-400 mb-2 truncate">
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
      <div className="flex items-center justify-between gap-2 pt-3 border-t border-zinc-800">
        <div className="flex items-center gap-1">
          {mitigation.assigned_to_user ? (
            <UserAvatar
              name={mitigation.assigned_to_user.name}
              avatar={mitigation.assigned_to_user.avatar}
              size="xs"
              tooltip={true}
            />
          ) : (
            <div className="w-6 h-6 rounded-full border border-zinc-700 bg-zinc-800/50 flex items-center justify-center">
              <span className="text-xs text-zinc-500">−</span>
            </div>
          )}
        </div>

        {/* Editing lock indicator */}
        {mitigation.editing_lock && (
          <motion.div
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
            className="flex items-center gap-1 text-xs text-yellow-400 bg-yellow-500/10 px-2 py-1 rounded"
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
