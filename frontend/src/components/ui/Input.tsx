import { type InputHTMLAttributes, forwardRef } from 'react';
import { cn } from './Button';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, error, ...props }, ref) => {
    return (
      <div className="space-y-.">
        {label && (
          <label className="text-xs font-medium text-zinc- uppercase tracking-wider">
            {label}
          </label>
        )}
        <input
          ref={ref}
          className={cn(
            'flex h- w-full rounded-lg border border-border bg-zinc-/ px- py- text-sm text-white placeholder:text-zinc-',
            'focus:outline-none focus:ring- focus:ring-primary/ focus:border-primary',
            'disabled:cursor-not-allowed disabled:opacity- transition-all',
            error && 'border-red-/ focus:ring-red-/',
            className
          )}
          {...props}
        />
        {error && <p className="text-xs text-red- animate-fade-in">{error}</p>}
      </div>
    );
  }
);
Input.displayName = 'Input';