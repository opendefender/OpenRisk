// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { type ButtonHTMLAttributes, forwardRef } from 'react';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { Loader2 } from 'lucide-react';

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
      primary: 'bg-primary hover:bg-blue-600 text-white shadow-glow border-transparent',
      secondary: 'bg-surface border-border hover:bg-zinc-800 text-zinc-100',
      ghost: 'bg-transparent hover:bg-zinc-800/50 text-zinc-400 hover:text-white border-transparent',
      danger: 'bg-red-500/10 text-red-500 hover:bg-red-500/20 border-red-500/20',
    };

    return (
      <button
        ref={ref}
        disabled={isLoading || disabled}
        className={cn(
          'inline-flex items-center justify-center rounded-lg px-4 py-2 text-sm font-medium transition-all duration-200 border',
          'focus:outline-none focus:ring-2 focus:ring-primary/50 focus:ring-offset-2 focus:ring-offset-background',
          'disabled:opacity-50 disabled:cursor-not-allowed',
          variants[variant],
          className
        )}
        {...props}
      >
        {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
        {children}
      </button>
    );
  }
);
Button.displayName = 'Button';