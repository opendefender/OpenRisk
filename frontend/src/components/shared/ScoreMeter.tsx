// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { motion } from 'framer-motion';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

interface ScoreMeterProps {
  score: number;
  maxScore?: number;
  size?: 'sm' | 'md' | 'lg';
  showLabel?: boolean;
  animated?: boolean;
  className?: string;
}

const getScoreColor = (percentage: number) => {
  if (percentage >= 80) return 'from-red-500 to-red-600';
  if (percentage >= 60) return 'from-orange-500 to-orange-600';
  if (percentage >= 40) return 'from-yellow-500 to-yellow-600';
  return 'from-emerald-500 to-emerald-600';
};

const getScoreLabel = (percentage: number) => {
  if (percentage >= 80) return 'Critique';
  if (percentage >= 60) return 'Élevé';
  if (percentage >= 40) return 'Moyen';
  return 'Bas';
};

export const ScoreMeter = ({
  score,
  maxScore = 100,
  size = 'md',
  showLabel = true,
  animated = true,
  className,
}: ScoreMeterProps) => {
  const percentage = Math.min(100, (score / maxScore) * 100);
  const bgColor = getScoreColor(percentage);

  const sizeClasses = {
    sm: { container: 'w-12 h-12', text: 'text-sm', label: 'text-xs' },
    md: { container: 'w-16 h-16', text: 'text-lg', label: 'text-xs' },
    lg: { container: 'w-24 h-24', text: 'text-3xl', label: 'text-sm' },
  };

  const classes = sizeClasses[size];

  return (
    <div className={cn('flex flex-col items-center gap-2', className)}>
      <div className={cn('relative', classes.container)}>
        {/* Background circle */}
        <svg className="w-full h-full transform -rotate-90" viewBox="0 0 100 100">
          {/* Background track */}
          <circle
            cx="50"
            cy="50"
            r="45"
            fill="none"
            stroke="currentColor"
            strokeWidth="8"
            className="text-zinc-800/30"
          />

          {/* Animated progress circle */}
          <motion.circle
            cx="50"
            cy="50"
            r="45"
            fill="none"
            strokeWidth="8"
            strokeLinecap="round"
            className={cn('transition-all', `bg-gradient-to-r ${bgColor}`)}
            stroke={getScoreGradientColor(percentage)}
            initial={animated ? { strokeDashoffset: 283 } : { strokeDashoffset: 283 - (283 * percentage) / 100 }}
            animate={animated ? { strokeDashoffset: 283 - (283 * percentage) / 100 } : undefined}
            transition={animated ? { duration: 1, ease: 'easeInOut' } : undefined}
            strokeDasharray="283"
          />
        </svg>

        {/* Center content */}
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <motion.div
            initial={animated ? { scale: 0 } : undefined}
            animate={animated ? { scale: 1 } : undefined}
            transition={animated ? { delay: 0.3 } : undefined}
            className={cn('font-bold', classes.text)}
          >
            {score.toFixed(1)}
          </motion.div>
          {size !== 'sm' && (
            <div className="text-xs text-zinc-400">/100</div>
          )}
        </div>
      </div>

      {showLabel && (
        <motion.div
          initial={animated ? { opacity: 0, y: 4 } : undefined}
          animate={animated ? { opacity: 1, y: 0 } : undefined}
          transition={animated ? { delay: 0.5 } : undefined}
          className={cn('font-medium', classes.label)}
        >
          {getScoreLabel(percentage)}
        </motion.div>
      )}
    </div>
  );
};

// Helper to get color for SVG stroke
function getScoreGradientColor(percentage: number) {
  if (percentage >= 80) return '#ef4444';
  if (percentage >= 60) return '#f97316';
  if (percentage >= 40) return '#eab308';
  return '#10b981';
}
