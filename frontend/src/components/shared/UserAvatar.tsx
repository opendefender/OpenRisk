import { motion } from 'framer-motion';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

interface UserAvatarProps {
  name: string;
  avatar?: string;
  size?: 'xs' | 'sm' | 'md' | 'lg';
  className?: string;
  tooltip?: boolean;
  onClick?: () => void;
}

const getInitials = (name: string) => {
  return name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);
};

const getColorFromName = (name: string) => {
  const colors = [
    'from-red-500 to-red-600',
    'from-orange-500 to-orange-600',
    'from-yellow-500 to-yellow-600',
    'from-emerald-500 to-emerald-600',
    'from-blue-500 to-blue-600',
    'from-indigo-500 to-indigo-600',
    'from-purple-500 to-purple-600',
    'from-pink-500 to-pink-600',
  ];
  const hash = name.split('').reduce((acc, char) => acc + char.charCodeAt(0), 0);
  return colors[hash % colors.length];
};

const getSizeClasses = (size: 'xs' | 'sm' | 'md' | 'lg') => {
  switch (size) {
    case 'xs':
      return { container: 'w-6 h-6', text: 'text-xs' };
    case 'sm':
      return { container: 'w-8 h-8', text: 'text-xs' };
    case 'md':
      return { container: 'w-10 h-10', text: 'text-sm' };
    case 'lg':
      return { container: 'w-12 h-12', text: 'text-base' };
  }
};

export const UserAvatar = ({
  name,
  avatar,
  size = 'md',
  className,
  tooltip = true,
  onClick,
}: UserAvatarProps) => {
  const sizeClasses = getSizeClasses(size);
  const initials = getInitials(name);
  const bgColor = getColorFromName(name);

  const avatarContent = (
    <motion.div
      whileHover={onClick ? { scale: 1.1 } : undefined}
      whileTap={onClick ? { scale: 0.95 } : undefined}
      className={cn(
        'rounded-full flex items-center justify-center font-medium text-white',
        'bg-gradient-to-br border border-white/20',
        bgColor,
        sizeClasses.container,
        onClick && 'cursor-pointer',
        className
      )}
      onClick={onClick}
    >
      {avatar ? (
        <img src={avatar} alt={name} className="w-full h-full rounded-full object-cover" />
      ) : (
        <span className={sizeClasses.text}>{initials}</span>
      )}
    </motion.div>
  );

  if (!tooltip) return avatarContent;

  return (
    <div className="group relative">
      {avatarContent}
      <div className="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 px-2 py-1 bg-zinc-900 border border-zinc-700 rounded text-xs text-zinc-300 whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none">
        {name}
      </div>
    </div>
  );
};
