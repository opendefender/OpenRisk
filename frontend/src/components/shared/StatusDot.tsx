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
  open: { bg: 'bg-danger', label: 'Ouvert', textColor: 'text-danger-text' },
  draft: { bg: 'bg-surface-3', label: 'Brouillon', textColor: 'text-text-secondary' },
  active: { bg: 'bg-danger', label: 'Actif', textColor: 'text-danger-text' },
  in_progress: { bg: 'bg-accent', label: 'En cours', textColor: 'text-info-text' },
  planned: { bg: 'bg-blue-400', label: 'Planifié', textColor: 'text-info-text' },
  review: { bg: 'bg-violet-500', label: 'En revue', textColor: 'text-violet-400' },
  mitigated: { bg: 'bg-warning', label: 'Atténué', textColor: 'text-warning-text' },
  accepted: { bg: 'bg-purple-500', label: 'Accepté', textColor: 'text-purple-400' },
  closed: { bg: 'bg-success', label: 'Fermé', textColor: 'text-success-text' },
  done: { bg: 'bg-success', label: 'Terminé', textColor: 'text-success-text' },
  cancelled: { bg: 'bg-surface-3', label: 'Annulé', textColor: 'text-text-secondary' },
};

const DEFAULT_STATUS_CONFIG = { bg: 'bg-surface-3', label: 'Inconnu', textColor: 'text-text-secondary' };

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
