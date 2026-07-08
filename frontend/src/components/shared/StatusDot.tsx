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

// Accepts any backend status string — domain.RiskStatus alone has two overlapping
// vocabularies in play (lowercase "open"/"in_progress"/... and uppercase
// "DRAFT"/"ACTIVE"/...), and domain.MitigationStatus is a third ("PLANNED"/"REVIEW"/
// "DONE"/...). A strict literal union here previously fell through with no `default`,
// returning undefined and crashing every caller on `.bg` for any real backend value.
type Status = string;

interface StatusDotProps {
  status: Status;
  animated?: boolean;
  size?: 'xs' | 'sm' | 'md';
  className?: string;
  withLabel?: boolean;
}

const STATUS_CONFIG: Record<string, { bg: string; label: string; textColor: string }> = {
  open: { bg: 'bg-red-500', label: 'Ouvert', textColor: 'text-red-400' },
  draft: { bg: 'bg-zinc-500', label: 'Brouillon', textColor: 'text-zinc-400' },
  active: { bg: 'bg-red-500', label: 'Actif', textColor: 'text-red-400' },
  in_progress: { bg: 'bg-blue-500', label: 'En cours', textColor: 'text-blue-400' },
  planned: { bg: 'bg-blue-400', label: 'Planifié', textColor: 'text-blue-300' },
  review: { bg: 'bg-violet-500', label: 'En revue', textColor: 'text-violet-400' },
  mitigated: { bg: 'bg-amber-500', label: 'Atténué', textColor: 'text-amber-400' },
  accepted: { bg: 'bg-purple-500', label: 'Accepté', textColor: 'text-purple-400' },
  closed: { bg: 'bg-emerald-500', label: 'Fermé', textColor: 'text-emerald-400' },
  done: { bg: 'bg-emerald-500', label: 'Terminé', textColor: 'text-emerald-400' },
  cancelled: { bg: 'bg-zinc-500', label: 'Annulé', textColor: 'text-zinc-400' },
};

const DEFAULT_STATUS_CONFIG = { bg: 'bg-zinc-500', label: 'Inconnu', textColor: 'text-zinc-400' };

const getStatusConfig = (status: Status) => STATUS_CONFIG[status?.toLowerCase()] ?? DEFAULT_STATUS_CONFIG;

const getSizeClasses = (size: 'xs' | 'sm' | 'md') => {
  switch (size) {
    case 'xs':
      return 'w-2 h-2';
    case 'sm':
      return 'w-3 h-3';
    case 'md':
      return 'w-4 h-4';
  }
};

export const StatusDot = ({
  status,
  animated = true,
  size = 'sm',
  className,
  withLabel = false,
}: StatusDotProps) => {
  const config = getStatusConfig(status);
  const sizeClass = getSizeClasses(size);

  return (
    <div className={cn('flex items-center gap-2', className)}>
      <motion.div
        className={cn('rounded-full', config.bg, sizeClass)}
        animate={animated ? { scale: [1, 1.2, 1] } : undefined}
        transition={animated ? { duration: 2, repeat: Infinity } : undefined}
      />
      {withLabel && (
        <span className={cn('text-xs font-medium', config.textColor)}>
          {config.label}
        </span>
      )}
    </div>
  );
};
