// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { motion } from 'framer-motion';
import { AlertCircle, AlertTriangle, Info, AlertOctagon } from 'lucide-react';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

type RiskLevel = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';

interface RiskBadgeProps {
  level: RiskLevel;
  animated?: boolean;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

const getRiskConfig = (level: RiskLevel) => {
  switch (level) {
    case 'CRITICAL':
      return {
        bg: 'bg-red-500/20',
        border: 'border-red-500/50',
        text: 'text-red-400',
        icon: AlertOctagon,
        label: 'Critique',
      };
    case 'HIGH':
      return {
        bg: 'bg-orange-500/20',
        border: 'border-orange-500/50',
        text: 'text-orange-400',
        icon: AlertTriangle,
        label: 'Élevé',
      };
    case 'MEDIUM':
      return {
        bg: 'bg-yellow-500/20',
        border: 'border-yellow-500/50',
        text: 'text-yellow-400',
        icon: AlertCircle,
        label: 'Moyen',
      };
    case 'LOW':
      return {
        bg: 'bg-emerald-500/20',
        border: 'border-emerald-500/50',
        text: 'text-emerald-400',
        icon: Info,
        label: 'Bas',
      };
  }
};

export const RiskBadge = ({
  level,
  animated = true,
  size = 'md',
  className,
}: RiskBadgeProps) => {
  const config = getRiskConfig(level);
  const Icon = config.icon;

  const sizeClasses = {
    sm: 'px-2 py-1 text-xs',
    md: 'px-3 py-1.5 text-sm',
    lg: 'px-4 py-2 text-base',
  };

  const iconSizes = {
    sm: 14,
    md: 16,
    lg: 20,
  };

  return (
    <motion.div
      initial={animated ? { scale: 0.9, opacity: 0 } : undefined}
      animate={animated ? { scale: 1, opacity: 1 } : undefined}
      whileHover={animated ? { scale: 1.05 } : undefined}
      transition={{ duration: 0.2 }}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border font-medium transition-colors',
        config.bg,
        config.border,
        config.text,
        sizeClasses[size],
        className
      )}
    >
      <Icon size={iconSizes[size]} className="shrink-0" />
      <span>{config.label}</span>
    </motion.div>
  );
};
