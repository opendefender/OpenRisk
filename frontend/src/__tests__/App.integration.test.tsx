/ @vitest-environment jsdom /
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import App from '../App';
import { useAuthStore } from '../hooks/useAuthStore';

// Mock auth store
vi.mock('../../hooks/useAuthStore');

// Mock Sonner toast
vi.mock('sonner', () => ({
  Toaster: () => null,
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

// Mock API
vi.mock('../../lib/api', () => ({
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

    render(
      <BrowserRouter>
        <App />
      </BrowserRouter>
    );

    expect(screen.getByText('Welcome back')).toBeInTheDocument();
  });

  it('should render Dashboard when authenticated', async () => {
    vi.mocked(useAuthStore).mockImplementation((selector: any) => {
      const store = {
        isAuthenticated: true,
        user: { id: '', email: 'test@example.com', full_name: 'Test User', role: 'analyst' },
        token: 'fake-token',
      };
      return selector(store as any);
    });

    render(
      <BrowserRouter>
        <App />
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.queryByText('Welcome back')).not.toBeInTheDocument();
    });
  });

  it('should handle protected routes', async () => {
    const mockUser = {
      id: '',
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

    render(
      <BrowserRouter>
        <App />
      </BrowserRouter>
    );

    await waitFor(() => {
      expect(screen.queryByText('Welcome back')).not.toBeInTheDocument();
    });
  });
});
