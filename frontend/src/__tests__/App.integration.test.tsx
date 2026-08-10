// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import App from '../App';
import { useAuthStore } from '../hooks/useAuthStore';

// Mock auth store (path must match the import above — was ../../ which mocked
// nothing, so vi.mocked(useAuthStore) was not a mock).
vi.mock('../hooks/useAuthStore');

// Mock Sonner toast
vi.mock('sonner', () => ({
  Toaster: () => null,
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

// Mock API
vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn().mockResolvedValue({ data: {} }),
    post: vi.fn().mockResolvedValue({ data: {} }),
    put: vi.fn().mockResolvedValue({ data: {} }),
    patch: vi.fn().mockResolvedValue({ data: {} }),
    delete: vi.fn().mockResolvedValue({ data: {} }),
  },
}));

describe('App Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render Login page when not authenticated', () => {
    vi.mocked(useAuthStore).mockImplementation((selector: any) => {
      const store = {
        isAuthenticated: false,
        user: null,
      };
      return selector(store as any);
    });

    // App renders its own <BrowserRouter>; don't double-wrap.
    render(<App />);

    // Assert on the form itself, not on its heading. The previous version matched
    // the marketing copy verbatim, so an ordinary wording change broke a test
    // that is supposed to be about routing — and these ids are locale-independent
    // by construction.
    expect(screen.getByTestId('login-email')).toBeInTheDocument();
    expect(screen.getByTestId('login-password')).toBeInTheDocument();
    expect(screen.getByTestId('login-submit')).toBeInTheDocument();
  });

  it('should render Dashboard when authenticated', async () => {
    vi.mocked(useAuthStore).mockImplementation((selector: any) => {
      const store = {
        isAuthenticated: true,
        user: { id: '1', email: 'test@example.com', full_name: 'Test User', role: 'analyst' },
        token: 'fake-token',
      };
      return selector(store as any);
    });

    // App renders its own <BrowserRouter>; don't double-wrap.
    render(<App />);

    await waitFor(() => {
      expect(screen.queryByText('Welcome back')).not.toBeInTheDocument();
    });
  });

  it('should handle protected routes', async () => {
    const mockUser = {
      id: '1',
      email: 'test@example.com',
      full_name: 'Test User',
      role: 'analyst',
    };

    vi.mocked(useAuthStore).mockImplementation((selector: any) => {
      const store = {
        isAuthenticated: true,
        user: mockUser,
        token: 'fake-token',
      };
      return selector(store as any);
    });

    // App renders its own <BrowserRouter>; don't double-wrap.
    render(<App />);

    await waitFor(() => {
      expect(screen.queryByText('Welcome back')).not.toBeInTheDocument();
    });
  });
});
