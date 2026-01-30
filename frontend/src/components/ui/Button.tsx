import { type ButtonHTMLAttributes, forwardRef } from 'react';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Loader } from 'lucide-react';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
  isLoading?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = 'primary', isLoading, children, disabled, ...props }, ref) => {
    const variants = {
      primary: 'bg-primary hover:bg-blue- text-white shadow-glow border-transparent',
      secondary: 'bg-surface border-border hover:bg-zinc- text-zinc-',
      ghost: 'bg-transparent hover:bg-zinc-/ text-zinc- hover:text-white border-transparent',
      danger: 'bg-red-/ text-red- hover:bg-red-/ border-red-/',
    };

    return (
      <button
        ref={ref}
        disabled={isLoading || disabled}
        className={cn(
          'inline-flex items-center justify-center rounded-lg px- py- text-sm font-medium transition-all duration- border',
          'focus:outline-none focus:ring- focus:ring-primary/ focus:ring-offset- focus:ring-offset-background',
          'disabled:opacity- disabled:cursor-not-allowed',
          variants[variant],
          className
        )}
        {...props}
      >
        {isLoading && <Loader className="mr- h- w- animate-spin" />}
        {children}
      </button>
    );
  }
);
Button.displayName = 'Button';