import { motion } from 'framer-motion';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

type Status = 'open' | 'in_progress' | 'mitigated' | 'accepted' | 'closed';

interface StatusDotProps {
  status: Status;
  animated?: boolean;
  size?: 'xs' | 'sm' | 'md';
  className?: string;
  withLabel?: boolean;
}

const getStatusConfig = (status: Status) => {
  switch (status) {
    case 'open':
      return { bg: 'bg-red-500', label: 'Ouvert', textColor: 'text-red-400' };
    case 'in_progress':
      return { bg: 'bg-blue-500', label: 'En cours', textColor: 'text-blue-400' };
    case 'mitigated':
      return { bg: 'bg-amber-500', label: 'Atténué', textColor: 'text-amber-400' };
    case 'accepted':
      return { bg: 'bg-purple-500', label: 'Accepté', textColor: 'text-purple-400' };
    case 'closed':
      return { bg: 'bg-emerald-500', label: 'Fermé', textColor: 'text-emerald-400' };
  }
};

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
