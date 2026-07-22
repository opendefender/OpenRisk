// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { motion } from 'framer-motion';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

interface ProgressBarProps {
  value: number;
  max: number;
  label?: string;
  showPercentage?: boolean;
  size?: 'sm' | 'md' | 'lg';
  variant?: 'default' | 'success' | 'warning' | 'danger';
  animated?: boolean;
  className?: string;
}

export const ProgressBar = ({
  value,
  max,
  label,
  showPercentage = true,
  size = 'md',
  variant = 'default',
  animated = true,
  className,
}: ProgressBarProps) => {
  const percentage = Math.min(Math.round((value / max) * 100), 100);

  const sizeClasses = {
    sm: 'h-1.5',
    md: 'h-2.5',
    lg: 'h-3',
  };

  const variantClasses = {
    default: 'bg-blue-500',
    success: 'bg-emerald-500',
    warning: 'bg-yellow-500',
    danger: 'bg-red-500',
  };

  return (
    <div className={cn('w-full', className)}>
      {(label || showPercentage) && (
        <div className="flex justify-between items-center mb-2">
          {label && <span className="text-xs font-medium text-zinc-400">{label}</span>}
          {showPercentage && (
            <span className="text-xs font-medium text-zinc-400">{percentage}%</span>
          )}
        </div>
      )}
      <div className={cn(
        'w-full bg-zinc-700 rounded-full overflow-hidden',
        sizeClasses[size]
      )}>
        <motion.div
          className={cn(
            'h-full rounded-full transition-all duration-500',
            variantClasses[variant]
          )}
          initial={animated ? { width: 0 } : undefined}
          animate={{ width: `${percentage}%` }}
          transition={animated ? { duration: 0.6, ease: 'easeOut' } : undefined}
        />
      </div>
    </div>
  );
};
