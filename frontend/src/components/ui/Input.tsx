// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { type InputHTMLAttributes, forwardRef, useId } from 'react';
import { cn } from './Button';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, error, id, ...props }, ref) => {
    // Associate the label with the input (htmlFor/id) so screen readers and
    // getByLabelText resolve the field. Honour a caller-supplied id if present.
    const autoId = useId();
    const inputId = id ?? autoId;
    return (
      <div className="space-y-1.5">
        {label && (
          <label htmlFor={inputId} className="text-xs font-medium text-ink-muted uppercase tracking-wider">
            {label}
          </label>
        )}
        <input
          id={inputId}
          ref={ref}
          className={cn(
            'flex h-10 w-full rounded-lg border border-border-strong bg-elevated px-3 py-2 text-sm text-ink placeholder:text-ink-muted',
            'focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary',
            'disabled:cursor-not-allowed disabled:opacity-50 transition-all',
            error && 'border-red-500/50 focus:ring-red-500/20',
            className
          )}
          {...props}
        />
        {error && <p className="text-xs text-red-400 animate-fade-in">{error}</p>}
      </div>
    );
  }
);
Input.displayName = 'Input';