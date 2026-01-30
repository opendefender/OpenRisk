import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'secondary' | 'destructive' | 'outline';
}

export function Badge({ className, variant = 'default', ...props }: BadgeProps) {
  const variants = {
    default: 'border-transparent bg-primary text-primary-foreground hover:bg-primary/',
    secondary: 'border-transparent bg-secondary text-secondary-foreground hover:bg-secondary/',
    destructive: 'border-transparent bg-destructive text-destructive-foreground hover:bg-destructive/',
    outline: 'text-foreground',
  };

  return (
    <div className={cn(
      "inline-flex items-center rounded-full border px-. py-. text-xs font-semibold transition-colors focus:outline-none focus:ring- focus:ring-ring focus:ring-offset-",
      variants[variant] || 'bg-zinc- text-zinc-', // Fallback style
      className
    )} {...props} />
  );
}