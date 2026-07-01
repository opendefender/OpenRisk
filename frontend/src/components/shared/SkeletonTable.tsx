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

interface SkeletonTableProps {
  rows?: number;
  columns?: number;
  className?: string;
}

export const SkeletonTable = ({
  rows = 5,
  columns = 6,
  className,
}: SkeletonTableProps) => {
  const pulseVariants = {
    animate: {
      opacity: [0.5, 1, 0.5],
      transition: {
        duration: 2,
        repeat: Infinity,
        ease: 'easeInOut',
      },
    },
  };

  return (
    <div className={cn('space-y-2', className)}>
      {/* Header */}
      <div className="grid gap-4 px-4 py-3 bg-zinc-900/30 border border-zinc-800 rounded-lg">
        {Array.from({ length: columns }).map((_, i) => (
          <motion.div
            key={`header-${i}`}
            variants={pulseVariants}
            animate="animate"
            className="h-4 bg-zinc-800 rounded w-full"
          />
        ))}
      </div>

      {/* Rows */}
      {Array.from({ length: rows }).map((_, rowIdx) => (
        <div
          key={`row-${rowIdx}`}
          className="grid gap-4 px-4 py-3 bg-zinc-900/20 border border-zinc-800/50 rounded-lg"
        >
          {Array.from({ length: columns }).map((_, colIdx) => (
            <motion.div
              key={`cell-${rowIdx}-${colIdx}`}
              variants={pulseVariants}
              animate="animate"
              transition={{ delay: colIdx * 0.05 }}
              className="h-4 bg-zinc-800/50 rounded w-full"
            />
          ))}
        </div>
      ))}
    </div>
  );
};
