// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { useCallback } from 'react';
import { toast as sonnerToast, Toaster } from 'sonner';

export interface ToastOptions {
  duration?: number;
  description?: string;
  action?: {
    label: string;
    onClick: () => void;
  };
}

/**
 * A toast carrying an action must not expire on its own (WCAG 2.2.1): a user
 * who has not yet reacted to it must not lose the chance to. `Infinity` is
 * sonner's own documented way to say "stays until dismissed".
 */
function resolveDuration(base: number, options?: ToastOptions): number {
  if (options?.duration !== undefined) return options.duration;
  return options?.action ? Infinity : base;
}

export function useToast() {
  const success = useCallback((message: string, options?: ToastOptions) => {
    return sonnerToast.success(message, {
      description: options?.description,
      duration: resolveDuration(4000, options),
      action: options?.action,
    });
  }, []);

  const error = useCallback((message: string, options?: ToastOptions) => {
    return sonnerToast.error(message, {
      description: options?.description,
      duration: resolveDuration(8000, options),
      action: options?.action,
    });
  }, []);

  const warning = useCallback((message: string, options?: ToastOptions) => {
    return sonnerToast.warning(message, {
      description: options?.description,
      duration: resolveDuration(6000, options),
      action: options?.action,
    });
  }, []);

  const info = useCallback((message: string, options?: ToastOptions) => {
    return sonnerToast.info(message, {
      description: options?.description,
      duration: resolveDuration(4000, options),
      action: options?.action,
    });
  }, []);

  const loading = useCallback((message: string, options?: Omit<ToastOptions, 'duration'>) => {
    return sonnerToast.loading(message, {
      description: options?.description,
      action: options?.action,
    });
  }, []);

  const promise = useCallback(
    <T>(
      promise: Promise<T>,
      messages: { loading: string; success: string; error: string },
      options?: ToastOptions,
    ) => {
      return sonnerToast.promise(promise, {
        ...messages,
        duration: options?.duration,
        action: options?.action,
      });
    },
    [],
  );

  return {
    success,
    error,
    warning,
    info,
    loading,
    promise,
    dismiss: sonnerToast.dismiss,
  };
}

// Export Toaster component for layout
export { Toaster };
