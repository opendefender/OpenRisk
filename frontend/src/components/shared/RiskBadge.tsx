// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { motion } from 'framer-motion';
import { AlertCircle, AlertTriangle, Info, AlertOctagon } from 'lucide-react';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

type RiskLevel = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';

interface RiskBadgeProps {
  // Accept any string: the backend sends risk.level as lowercase ("medium"),
  // while other call sites pass the uppercase RiskLevel union. We normalize
  // below instead of trusting the caller to have the exact casing.
  level: RiskLevel | string;
  animated?: boolean;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

const RISK_CONFIGS = {
  CRITICAL: {
    bg: 'bg-red-500/20',
    border: 'border-red-500/50',
    text: 'text-red-400',
    icon: AlertOctagon,
    label: 'Critique',
  },
  HIGH: {
    bg: 'bg-orange-500/20',
    border: 'border-orange-500/50',
    text: 'text-orange-400',
    icon: AlertTriangle,
    label: 'Élevé',
  },
  MEDIUM: {
    bg: 'bg-yellow-500/20',
    border: 'border-yellow-500/50',
    text: 'text-yellow-400',
    icon: AlertCircle,
    label: 'Moyen',
  },
  LOW: {
    bg: 'bg-emerald-500/20',
    border: 'border-emerald-500/50',
    text: 'text-emerald-400',
    icon: Info,
    label: 'Bas',
  },
} as const;

// getRiskConfig normalizes any casing and always returns a valid config
// (defaults to LOW) — an unknown level must never crash the badge, which
// previously white-screened the whole Risk drawer when the backend sent a
// lowercase level like "medium".
const getRiskConfig = (level: RiskLevel | string) => {
  const key = String(level ?? '').toUpperCase() as keyof typeof RISK_CONFIGS;
  return RISK_CONFIGS[key] ?? RISK_CONFIGS.LOW;
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
