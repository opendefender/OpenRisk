// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import { useEffect, useState, useCallback } from 'react';

/**
 * Custom hook for debounced values
 * Delays value updates by specified milliseconds
 */
export function useDebounce<T>(value: T, delayMs: number = 300): T {
  const [debouncedValue, setDebouncedValue] = useState<T>(value);

  useEffect(() => {
    // Set up timer to update debounced value
    const handler = setTimeout(() => {
      setDebouncedValue(value);
    }, delayMs);

    // Clean up timer on component unmount or when value/delayMs changes
    return () => clearTimeout(handler);
  }, [value, delayMs]);

  return debouncedValue;
}

/**
 * Custom hook for debounced callback
 */
export function useDebouncedCallback<T extends (...args: any[]) => any>(
  callback: T,
  delayMs: number = 300
): T {
  const [timeoutId, setTimeoutId] = useState<NodeJS.Timeout | null>(null);

  const debouncedCallback = useCallback(
    (...args: any[]) => {
      if (timeoutId) clearTimeout(timeoutId);

      const newTimeoutId = setTimeout(() => {
        callback(...args);
      }, delayMs);

      setTimeoutId(newTimeoutId);
    },
    [callback, delayMs, timeoutId]
  ) as T;

  useEffect(() => {
    return () => {
      if (timeoutId) clearTimeout(timeoutId);
    };
  }, [timeoutId]);

  return debouncedCallback;
}
