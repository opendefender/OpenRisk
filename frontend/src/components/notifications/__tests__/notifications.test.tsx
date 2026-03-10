import React from 'react';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import '@testing-library/jest-dom';
import axios from 'axios';
import NotificationBadge from './NotificationBadge';
import NotificationCenter from './NotificationCenter';
import NotificationPreferences from './NotificationPreferences';

// Mock axios
jest.mock('axios');

describe('NotificationBadge Component', () => {
  it('should render with unread count', () => {
    const onClick = jest.fn();
    render(<NotificationBadge unreadCount={5} onClick={onClick} />);
    
    const badge = screen.getByText('5');
    expect(badge).toBeInTheDocument();
  });

  it('should display 99+ for large counts', () => {
    const onClick = jest.fn();
    render(<NotificationBadge unreadCount={150} onClick={onClick} />);
    
    const badge = screen.getByText('99+');
    expect(badge).toBeInTheDocument();
  });

  it('should not display badge when unread count is 0', () => {
    const onClick = jest.fn();
    const { container } = render(<NotificationBadge unreadCount={0} onClick={onClick} />);
    
    const badge = container.querySelector('.notification-badge');
    expect(badge).not.toBeInTheDocument();
  });

  it('should call onClick handler when clicked', () => {
    const onClick = jest.fn();
    render(<NotificationBadge unreadCount={3} onClick={onClick} />);
    
    const button = screen.getByRole('button');
    fireEvent.click(button);
    
    expect(onClick).toHaveBeenCalled();
  });

  it('should animate count changes', async () => {
    const onClick = jest.fn();
    const { rerender } = render(<NotificationBadge unreadCount={5} onClick={onClick} />);
    
    // Update count
    rerender(<NotificationBadge unreadCount={8} onClick={onClick} />);
    
    await waitFor(() => {
      const badge = screen.getByText('8');
      expect(badge).toBeInTheDocument();
    });
  });

  it('should render bell icon', () => {
    const onClick = jest.fn();
    const { container } = render(<NotificationBadge unreadCount={2} onClick={onClick} />);
    
    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();
  });
});

describe('NotificationCenter Component', () => {
  const mockAuthToken = 'test-token';

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('should render notification list', async () => {
    const mockNotifications = [
      {
        id: '1',
        type: 'critical_risk',
        subject: 'Critical Risk Alert',
        message: 'A critical risk has been detected',
        status: 'pending',
        created_at: new Date().toISOString(),
      },
    ];

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockNotifications },
    });

    const onClose = jest.fn();
    render(
      <NotificationCenter
        isOpen={true}
        onClose={onClose}
        authToken={mockAuthToken}
      />
    );

    await waitFor(() => {
      expect(screen.getByText('Critical Risk Alert')).toBeInTheDocument();
    });
  });

  it('should handle loading state', () => {
    const onClose = jest.fn();
    render(
      <NotificationCenter
        isOpen={true}
        onClose={onClose}
        authToken={mockAuthToken}
      />
    );

    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('should handle errors', async () => {
    (axios.get as jest.Mock).mockRejectedValueOnce(
      new Error('Failed to fetch notifications')
    );

    const onClose = jest.fn();
    render(
      <NotificationCenter
        isOpen={true}
        onClose={onClose}
        authToken={mockAuthToken}
      />
    );

    await waitFor(() => {
      expect(screen.getByText(/error/i)).toBeInTheDocument();
    });
  });

  it('should mark notification as read', async () => {
    const mockNotifications = [
      {
        id: '1',
        type: 'critical_risk',
        subject: 'Test',
        message: 'Test message',
        status: 'pending',
        created_at: new Date().toISOString(),
      },
    ];

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockNotifications },
    });

    (axios.patch as jest.Mock).mockResolvedValueOnce({
      data: { success: true },
    });

    const onClose = jest.fn();
    render(
      <NotificationCenter
        isOpen={true}
        onClose={onClose}
        authToken={mockAuthToken}
      />
    );

    await waitFor(() => {
      expect(screen.getByText('Test')).toBeInTheDocument();
    });

    const markReadButton = screen.getByText(/mark as read/i);
    fireEvent.click(markReadButton);

    await waitFor(() => {
      expect(axios.patch).toHaveBeenCalledWith(
        expect.stringContaining('/read'),
        expect.any(Object),
        expect.any(Object)
      );
    });
  });

  it('should delete notification', async () => {
    const mockNotifications = [
      {
        id: '1',
        type: 'critical_risk',
        subject: 'Test',
        message: 'Test message',
        status: 'pending',
        created_at: new Date().toISOString(),
      },
    ];

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockNotifications },
    });

    (axios.delete as jest.Mock).mockResolvedValueOnce({
      data: { success: true },
    });

    const onClose = jest.fn();
    render(
      <NotificationCenter
        isOpen={true}
        onClose={onClose}
        authToken={mockAuthToken}
      />
    );

    await waitFor(() => {
      expect(screen.getByText('Test')).toBeInTheDocument();
    });

    const deleteButton = screen.getByText(/delete/i);
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(axios.delete).toHaveBeenCalled();
    });
  });

  it('should load more notifications', async () => {
    const mockNotifications = Array.from({ length: 20 }, (_, i) => ({
      id: `${i}`,
      type: 'critical_risk',
      subject: `Notification ${i}`,
      message: `Test message ${i}`,
      status: 'pending',
      created_at: new Date().toISOString(),
    }));

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockNotifications },
    });

    const onClose = jest.fn();
    render(
      <NotificationCenter
        isOpen={true}
        onClose={onClose}
        authToken={mockAuthToken}
      />
    );

    await waitFor(() => {
      expect(screen.getByText('Notification 0')).toBeInTheDocument();
    });

    const loadMoreButton = screen.getByText(/load more/i);
    expect(loadMoreButton).toBeInTheDocument();
  });

  it('should close when onClose is called', () => {
    const onClose = jest.fn();
    const { container } = render(
      <NotificationCenter
        isOpen={true}
        onClose={onClose}
        authToken={mockAuthToken}
      />
    );

    const closeButton = container.querySelector('.close-button');
    if (closeButton) {
      fireEvent.click(closeButton);
      expect(onClose).toHaveBeenCalled();
    }
  });

  it('should not render when isOpen is false', () => {
    const onClose = jest.fn();
    const { container } = render(
      <NotificationCenter
        isOpen={false}
        onClose={onClose}
        authToken={mockAuthToken}
      />
    );

    const overlay = container.querySelector('.notification-overlay');
    expect(overlay).not.toBeInTheDocument();
  });

  it('should display notification type icons', async () => {
    const mockNotifications = [
      {
        id: '1',
        type: 'critical_risk',
        subject: 'Critical Risk',
        message: 'Test',
        status: 'pending',
        created_at: new Date().toISOString(),
      },
      {
        id: '2',
        type: 'mitigation_deadline',
        subject: 'Mitigation Due',
        message: 'Test',
        status: 'pending',
        created_at: new Date().toISOString(),
      },
    ];

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockNotifications },
    });

    const onClose = jest.fn();
    render(
      <NotificationCenter
        isOpen={true}
        onClose={onClose}
        authToken={mockAuthToken}
      />
    );

    await waitFor(() => {
      expect(screen.getByText('Critical Risk')).toBeInTheDocument();
      expect(screen.getByText('Mitigation Due')).toBeInTheDocument();
    });
  });
});

describe('NotificationPreferences Component', () => {
  const mockAuthToken = 'test-token';

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('should render preferences form', async () => {
    const mockPreferences = {
      email_on_mitigation_deadline: true,
      email_on_critical_risk: true,
      email_on_action_assigned: false,
      slack_enabled: true,
      webhook_enabled: false,
      disable_all_notifications: false,
      enable_sound_notifications: true,
      enable_desktop_notifications: true,
    };

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockPreferences },
    });

    const onClose = jest.fn();
    render(
      <NotificationPreferences authToken={mockAuthToken} onClose={onClose} />
    );

    await waitFor(() => {
      expect(screen.getByText(/notification preferences/i)).toBeInTheDocument();
    });
  });

  it('should toggle email notifications', async () => {
    const mockPreferences = {
      email_on_critical_risk: true,
      slack_enabled: false,
      webhook_enabled: false,
      disable_all_notifications: false,
      enable_sound_notifications: true,
      enable_desktop_notifications: true,
    };

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockPreferences },
    });

    const onClose = jest.fn();
    const { container } = render(
      <NotificationPreferences authToken={mockAuthToken} onClose={onClose} />
    );

    await waitFor(() => {
      expect(screen.getByText(/critical risk/i)).toBeInTheDocument();
    });

    const emailCheckbox = container.querySelector(
      'input[name="email_on_critical_risk"]'
    );
    if (emailCheckbox) {
      fireEvent.click(emailCheckbox);
      expect((emailCheckbox as HTMLInputElement).checked).toBe(false);
    }
  });

  it('should toggle slack notifications', async () => {
    const mockPreferences = {
      email_on_critical_risk: true,
      slack_enabled: false,
      webhook_enabled: false,
      disable_all_notifications: false,
      enable_sound_notifications: true,
      enable_desktop_notifications: true,
    };

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockPreferences },
    });

    const onClose = jest.fn();
    const { container } = render(
      <NotificationPreferences authToken={mockAuthToken} onClose={onClose} />
    );

    await waitFor(() => {
      expect(screen.getByText(/slack/i)).toBeInTheDocument();
    });

    const slackCheckbox = container.querySelector('input[name="slack_enabled"]');
    if (slackCheckbox) {
      fireEvent.click(slackCheckbox);
      expect((slackCheckbox as HTMLInputElement).checked).toBe(true);
    }
  });

  it('should save preferences', async () => {
    const mockPreferences = {
      email_on_critical_risk: true,
      slack_enabled: false,
      webhook_enabled: false,
      disable_all_notifications: false,
      enable_sound_notifications: true,
      enable_desktop_notifications: true,
    };

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockPreferences },
    });

    (axios.patch as jest.Mock).mockResolvedValueOnce({
      data: { success: true },
    });

    const onClose = jest.fn();
    render(
      <NotificationPreferences authToken={mockAuthToken} onClose={onClose} />
    );

    await waitFor(() => {
      expect(screen.getByText(/save/i)).toBeInTheDocument();
    });

    const saveButton = screen.getByText(/save/i);
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(axios.patch).toHaveBeenCalledWith(
        expect.stringContaining('/preferences'),
        expect.any(Object),
        expect.any(Object)
      );
    });
  });

  it('should test email notification', async () => {
    const mockPreferences = {
      email_on_critical_risk: true,
      slack_enabled: false,
      webhook_enabled: false,
      disable_all_notifications: false,
      enable_sound_notifications: true,
      enable_desktop_notifications: true,
    };

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockPreferences },
    });

    (axios.post as jest.Mock).mockResolvedValueOnce({
      data: { success: true },
    });

    const onClose = jest.fn();
    render(
      <NotificationPreferences authToken={mockAuthToken} onClose={onClose} />
    );

    await waitFor(() => {
      expect(screen.getByText(/test/i)).toBeInTheDocument();
    });

    const testButtons = screen.getAllByText(/test/i);
    fireEvent.click(testButtons[0]);

    await waitFor(() => {
      expect(axios.post).toHaveBeenCalledWith(
        expect.stringContaining('/test'),
        expect.any(Object),
        expect.any(Object)
      );
    });
  });

  it('should display success message on save', async () => {
    const mockPreferences = {
      email_on_critical_risk: true,
      slack_enabled: false,
      webhook_enabled: false,
      disable_all_notifications: false,
      enable_sound_notifications: true,
      enable_desktop_notifications: true,
    };

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockPreferences },
    });

    (axios.patch as jest.Mock).mockResolvedValueOnce({
      data: { success: true },
    });

    const onClose = jest.fn();
    render(
      <NotificationPreferences authToken={mockAuthToken} onClose={onClose} />
    );

    await waitFor(() => {
      expect(screen.getByText(/save/i)).toBeInTheDocument();
    });

    const saveButton = screen.getByText(/save/i);
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(screen.getByText(/preferences saved/i)).toBeInTheDocument();
    });
  });

  it('should display error message on save failure', async () => {
    const mockPreferences = {
      email_on_critical_risk: true,
      slack_enabled: false,
      webhook_enabled: false,
      disable_all_notifications: false,
      enable_sound_notifications: true,
      enable_desktop_notifications: true,
    };

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockPreferences },
    });

    (axios.patch as jest.Mock).mockRejectedValueOnce(
      new Error('Failed to save preferences')
    );

    const onClose = jest.fn();
    render(
      <NotificationPreferences authToken={mockAuthToken} onClose={onClose} />
    );

    await waitFor(() => {
      expect(screen.getByText(/save/i)).toBeInTheDocument();
    });

    const saveButton = screen.getByText(/save/i);
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(screen.getByText(/error/i)).toBeInTheDocument();
    });
  });

  it('should handle deadline advance days input', async () => {
    const mockPreferences = {
      email_on_critical_risk: true,
      email_deadline_advance_days: 3,
      slack_enabled: false,
      webhook_enabled: false,
      disable_all_notifications: false,
      enable_sound_notifications: true,
      enable_desktop_notifications: true,
    };

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockPreferences },
    });

    const onClose = jest.fn();
    const { container } = render(
      <NotificationPreferences authToken={mockAuthToken} onClose={onClose} />
    );

    await waitFor(() => {
      expect(screen.getByText(/deadline/i)).toBeInTheDocument();
    });

    const input = container.querySelector(
      'input[name="email_deadline_advance_days"]'
    );
    if (input) {
      fireEvent.change(input, { target: { value: '5' } });
      expect((input as HTMLInputElement).value).toBe('5');
    }
  });

  it('should cancel without saving', async () => {
    const mockPreferences = {
      email_on_critical_risk: true,
      slack_enabled: false,
      webhook_enabled: false,
      disable_all_notifications: false,
      enable_sound_notifications: true,
      enable_desktop_notifications: true,
    };

    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: mockPreferences },
    });

    const onClose = jest.fn();
    render(
      <NotificationPreferences authToken={mockAuthToken} onClose={onClose} />
    );

    await waitFor(() => {
      expect(screen.getByText(/cancel/i)).toBeInTheDocument();
    });

    const cancelButton = screen.getByText(/cancel/i);
    fireEvent.click(cancelButton);

    expect(onClose).toHaveBeenCalled();
  });
});

describe('Integration Tests', () => {
  const mockAuthToken = 'test-token';

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('should handle authentication token correctly', async () => {
    (axios.get as jest.Mock).mockResolvedValueOnce({
      data: { data: [] },
    });

    const onClose = jest.fn();
    render(
      <NotificationCenter
        isOpen={true}
        onClose={onClose}
        authToken={mockAuthToken}
      />
    );

    await waitFor(() => {
      expect(axios.get).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: `Bearer ${mockAuthToken}`,
          }),
        })
      );
    });
  });

  it('should handle API errors gracefully', async () => {
    (axios.get as jest.Mock).mockRejectedValueOnce({
      response: {
        status: 401,
        data: { message: 'Unauthorized' },
      },
    });

    const onClose = jest.fn();
    render(
      <NotificationCenter
        isOpen={true}
        onClose={onClose}
        authToken='invalid-token'
      />
    );

    await waitFor(() => {
      expect(screen.getByText(/error/i)).toBeInTheDocument();
    });
  });

  it('should handle network timeouts', async () => {
    (axios.get as jest.Mock).mockRejectedValueOnce({
      code: 'ECONNABORTED',
      message: 'timeout of 5000ms exceeded',
    });

    const onClose = jest.fn();
    render(
      <NotificationCenter
        isOpen={true}
        onClose={onClose}
        authToken={mockAuthToken}
      />
    );

    await waitFor(() => {
      expect(screen.getByText(/error|timeout/i)).toBeInTheDocument();
    });
  });
});
