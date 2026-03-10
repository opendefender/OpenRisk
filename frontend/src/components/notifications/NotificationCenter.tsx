import React, { useEffect, useState, useCallback } from 'react';
import axios from 'axios';
import './NotificationCenter.css';

interface Notification {
  id: string;
  user_id: string;
  type: string;
  channel: string;
  status: string;
  subject: string;
  message: string;
  description?: string;
  metadata?: Record<string, any>;
  created_at: string;
}

interface NotificationCenterProps {
  isOpen: boolean;
  onClose: () => void;
  authToken: string;
}

export const NotificationCenter: React.FC<NotificationCenterProps> = ({
  isOpen,
  onClose,
  authToken,
}) => {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [limit, setLimit] = useState(20);
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(true);

  const fetchNotifications = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const response = await axios.get('/api/v1/notifications', {
        params: { limit, offset },
        headers: { Authorization: `Bearer ${authToken}` },
      });

      const newNotifications = response.data.data || [];
      
      if (offset === 0) {
        setNotifications(newNotifications);
      } else {
        setNotifications([...notifications, ...newNotifications]);
      }

      setHasMore(newNotifications.length === limit);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to fetch notifications'
      );
    } finally {
      setLoading(false);
    }
  }, [authToken, limit, offset, notifications]);

  useEffect(() => {
    if (isOpen && offset === 0) {
      fetchNotifications();
    }
  }, [isOpen]);

  const handleLoadMore = () => {
    setOffset(offset + limit);
  };

  const handleMarkAsRead = async (notificationId: string) => {
    try {
      await axios.patch(
        `/api/v1/notifications/${notificationId}/read`,
        {},
        { headers: { Authorization: `Bearer ${authToken}` } }
      );

      setNotifications((prev) =>
        prev.map((n) =>
          n.id === notificationId ? { ...n, status: 'read' } : n
        )
      );
    } catch (err) {
      console.error('Failed to mark notification as read:', err);
    }
  };

  const handleMarkAllAsRead = async () => {
    try {
      await axios.patch(
        '/api/v1/notifications/read-all',
        {},
        { headers: { Authorization: `Bearer ${authToken}` } }
      );

      setNotifications((prev) =>
        prev.map((n) => ({ ...n, status: 'read' }))
      );
    } catch (err) {
      console.error('Failed to mark all as read:', err);
    }
  };

  const handleDelete = async (notificationId: string) => {
    try {
      await axios.delete(`/api/v1/notifications/${notificationId}`, {
        headers: { Authorization: `Bearer ${authToken}` },
      });

      setNotifications((prev) => prev.filter((n) => n.id !== notificationId));
    } catch (err) {
      console.error('Failed to delete notification:', err);
    }
  };

  const getNotificationIcon = (type: string) => {
    switch (type) {
      case 'critical_risk':
        return '🔴';
      case 'mitigation_deadline':
        return '🔶';
      case 'action_assigned':
        return '🔵';
      case 'risk_update':
        return '🟢';
      case 'risk_resolved':
        return '✅';
      default:
        return '📢';
    }
  };

  if (!isOpen) return null;

  return (
    <div className="notification-center-overlay" onClick={onClose}>
      <div
        className="notification-center"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="notification-center-header">
          <h2>Notifications</h2>
          <button
            className="close-button"
            onClick={onClose}
            aria-label="Close notifications"
          >
            ✕
          </button>
        </div>

        {notifications.length > 0 && (
          <button
            className="mark-all-read-button"
            onClick={handleMarkAllAsRead}
          >
            Mark all as read
          </button>
        )}

        <div className="notification-list">
          {error && <div className="error-message">{error}</div>}

          {loading && notifications.length === 0 && (
            <div className="loading">Loading notifications...</div>
          )}

          {!loading && notifications.length === 0 && (
            <div className="empty-state">
              <p>No notifications yet</p>
            </div>
          )}

          {notifications.map((notification) => (
            <div
              key={notification.id}
              className={`notification-item ${
                notification.status === 'pending' ? 'unread' : 'read'
              }`}
            >
              <div className="notification-icon">
                {getNotificationIcon(notification.type)}
              </div>

              <div className="notification-content">
                <div className="notification-subject">
                  {notification.subject}
                </div>
                <div className="notification-message">
                  {notification.message}
                </div>
                {notification.description && (
                  <div className="notification-description">
                    {notification.description}
                  </div>
                )}
                <div className="notification-time">
                  {new Date(notification.created_at).toLocaleString()}
                </div>
              </div>

              <div className="notification-actions">
                {notification.status === 'pending' && (
                  <button
                    className="action-button"
                    onClick={() => handleMarkAsRead(notification.id)}
                    title="Mark as read"
                  >
                    ✓
                  </button>
                )}
                <button
                  className="action-button delete"
                  onClick={() => handleDelete(notification.id)}
                  title="Delete notification"
                >
                  🗑
                </button>
              </div>
            </div>
          ))}
        </div>

        {hasMore && notifications.length > 0 && (
          <button
            className="load-more-button"
            onClick={handleLoadMore}
            disabled={loading}
          >
            {loading ? 'Loading...' : 'Load More'}
          </button>
        )}
      </div>
    </div>
  );
};

export default NotificationCenter;
