// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 4 — per-type durations (success/info 4000, warning 6000,
// error 8000) and the WCAG 2.2.1 rule that a toast carrying an action must
// stay up until dismissed. Previously every type used sonner's own numbers
// (3000/3000/3500/4000) and an action never changed the duration.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';

const sonnerToast = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  loading: vi.fn(),
  promise: vi.fn(),
  dismiss: vi.fn(),
}));

vi.mock('sonner', () => ({
  toast: sonnerToast,
  Toaster: () => null,
}));

import { useToast } from '../useToast';

describe('useToast — durations (#751 phase 4)', () => {
  beforeEach(() => {
    Object.values(sonnerToast).forEach((fn) => fn.mockReset?.());
  });

  it.each([
    ['success', 4000] as const,
    ['info', 4000] as const,
    ['warning', 6000] as const,
    ['error', 8000] as const,
  ])('%s stays up for %dms by default', (type, ms) => {
    const { result } = renderHook(() => useToast());
    result.current[type]('hello');
    expect(sonnerToast[type]).toHaveBeenCalledWith(
      'hello',
      expect.objectContaining({ duration: ms }),
    );
  });

  it.each(['success', 'info', 'warning', 'error'] as const)(
    '%s with an action stays up until dismissed (Infinity), not its default duration',
    (type) => {
      const { result } = renderHook(() => useToast());
      const action = { label: 'Undo', onClick: vi.fn() };
      result.current[type]('hello', { action });
      expect(sonnerToast[type]).toHaveBeenCalledWith(
        'hello',
        expect.objectContaining({ duration: Infinity, action }),
      );
    },
  );

  it('an explicit duration always wins, even with an action', () => {
    const { result } = renderHook(() => useToast());
    result.current.success('hello', {
      action: { label: 'Undo', onClick: vi.fn() },
      duration: 2500,
    });
    expect(sonnerToast.success).toHaveBeenCalledWith(
      'hello',
      expect.objectContaining({ duration: 2500 }),
    );
  });
});
