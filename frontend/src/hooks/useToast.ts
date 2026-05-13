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

export function useToast() {
  const success = useCallback(
    (message: string, options?: ToastOptions) => {
      return sonnerToast.success(message, {
        description: options?.description,
        duration: options?.duration ?? 3000,
        action: options?.action,
      });
    },
    []
  );

  const error = useCallback(
    (message: string, options?: ToastOptions) => {
      return sonnerToast.error(message, {
        description: options?.description,
        duration: options?.duration ?? 4000,
        action: options?.action,
      });
    },
    []
  );

  const warning = useCallback(
    (message: string, options?: ToastOptions) => {
      return sonnerToast.warning(message, {
        description: options?.description,
        duration: options?.duration ?? 3500,
        action: options?.action,
      });
    },
    []
  );

  const info = useCallback(
    (message: string, options?: ToastOptions) => {
      return sonnerToast.info(message, {
        description: options?.description,
        duration: options?.duration ?? 3000,
        action: options?.action,
      });
    },
    []
  );

  const loading = useCallback(
    (message: string, options?: Omit<ToastOptions, 'duration'>) => {
      return sonnerToast.loading(message, {
        description: options?.description,
        action: options?.action,
      });
    },
    []
  );

  const promise = useCallback(
    <T,>(
      promise: Promise<T>,
      messages: { loading: string; success: string; error: string },
      options?: ToastOptions
    ) => {
      return sonnerToast.promise(promise, messages, {
        duration: options?.duration,
        action: options?.action,
      });
    },
    []
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
