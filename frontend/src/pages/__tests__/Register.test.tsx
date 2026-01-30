import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BrowserRouter } from 'react-router-dom';
import { Register } from '../Register';
import { api } from '../../lib/api';

// Mock the API
vi.mock('../../lib/api');

// Mock toast notifications
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

describe('Register Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockClear();
  });

  it('should render register form with all required fields', () => {
    render(
      <BrowserRouter>
        <Register />
      </BrowserRouter>
    );

    expect(screen.getByRole('heading', { name: 'Create Account' })).toBeInTheDocument();
    expect(screen.getByPlaceholderText('John Doe')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('johndoe')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('name@company.com')).toBeInTheDocument();
    expect(screen.getAllByPlaceholderText('••••••••')).toHaveLength();
    expect(screen.getByRole('button', { name: /Create Account/i })).toBeInTheDocument();
  });

  it('should show link to login page', () => {
    render(
      <BrowserRouter>
        <Register />
      </BrowserRouter>
    );

    const loginLink = screen.getByRole('link', { name: /Sign In/i });
    expect(loginLink).toHaveAttribute('href', '/login');
  });

  it('should validate full name is required', async () => {
    const user = userEvent.setup();
    render(
      <BrowserRouter>
        <Register />
      </BrowserRouter>
    );

    const submitButton = screen.getByRole('button', { name: /Create Account/i });
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Full name is required')).toBeInTheDocument();
    });
  });

  it('should allow user to fill all form fields', async () => {
    const user = userEvent.setup();
    render(
      <BrowserRouter>
        <Register />
      </BrowserRouter>
    );

    const fullNameInput = screen.getByPlaceholderText('John Doe') as HTMLInputElement;
    const usernameInput = screen.getByPlaceholderText('johndoe') as HTMLInputElement;
    const emailInput = screen.getByPlaceholderText('name@company.com') as HTMLInputElement;
    const passwordInputs = screen.getAllByPlaceholderText('••••••••') as HTMLInputElement[];

    await user.type(fullNameInput, 'John Doe');
    await user.type(usernameInput, 'johndoe');
    await user.type(emailInput, 'test@example.com');
    await user.type(passwordInputs[], 'password');
    await user.type(passwordInputs[], 'password');

    expect(fullNameInput.value).toBe('John Doe');
    expect(usernameInput.value).toBe('johndoe');
    expect(emailInput.value).toBe('test@example.com');
    expect(passwordInputs[].value).toBe('password');
    expect(passwordInputs[].value).toBe('password');
  });

  it('should submit valid form', async () => {
    const user = userEvent.setup();
    vi.mocked(api.post).mockResolvedValueOnce({ data: { user: { id: '', email: 'test@example.com' } } });

    render(
      <BrowserRouter>
        <Register />
      </BrowserRouter>
    );

    const fullNameInput = screen.getByPlaceholderText('John Doe');
    const usernameInput = screen.getByPlaceholderText('johndoe');
    const emailInput = screen.getByPlaceholderText('name@company.com');
    const passwordInputs = screen.getAllByPlaceholderText('••••••••');
    const submitButton = screen.getByRole('button', { name: /Create Account/i });

    await user.type(fullNameInput, 'John Doe');
    await user.type(usernameInput, 'johndoe');
    await user.type(emailInput, 'test@example.com');
    await user.type(passwordInputs[], 'password');
    await user.type(passwordInputs[], 'password');
    await user.click(submitButton);

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/auth/register', expect.objectContaining({
        email: 'test@example.com',
        username: 'johndoe',
        full_name: 'John Doe',
        password: 'password',
      }));
    });
  });

  it('should handle registration error', async () => {
    const user = userEvent.setup();
    vi.mocked(api.post).mockRejectedValueOnce({
      response: {
        status: ,
        data: { error: 'Email already in use' },
      },
    });

    render(
      <BrowserRouter>
        <Register />
      </BrowserRouter>
    );

    const fullNameInput = screen.getByPlaceholderText('John Doe');
    const usernameInput = screen.getByPlaceholderText('johndoe');
    const emailInput = screen.getByPlaceholderText('name@company.com');
    const passwordInputs = screen.getAllByPlaceholderText('••••••••');
    const submitButton = screen.getByRole('button', { name: /Create Account/i });

    await user.type(fullNameInput, 'John Doe');
    await user.type(usernameInput, 'johndoe');
    await user.type(emailInput, 'test@example.com');
    await user.type(passwordInputs[], 'password');
    await user.type(passwordInputs[], 'password');
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Email or username already in use')).toBeInTheDocument();
    });
  });
});
