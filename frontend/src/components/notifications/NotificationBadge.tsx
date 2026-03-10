import React, { useEffect, useState } from 'react';
import './NotificationBadge.css';

interface NotificationBadgeProps {
  unreadCount: number;
  onClick: () => void;
}

export const NotificationBadge: React.FC<NotificationBadgeProps> = ({
  unreadCount,
  onClick,
}) => {
  const [displayCount, setDisplayCount] = useState(unreadCount);
  const [isAnimating, setIsAnimating] = useState(false);

  useEffect(() => {
    if (unreadCount !== displayCount) {
      setIsAnimating(true);
      setTimeout(() => {
        setDisplayCount(unreadCount);
        setIsAnimating(false);
      }, 300);
    }
  }, [unreadCount, displayCount]);

  return (
    <button
      className="notification-badge"
      onClick={onClick}
      aria-label={`${unreadCount} unread notifications`}
      title={`${unreadCount} unread notifications`}
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
        <path d="M13.73 21a2 2 0 0 1-3.46 0" />
      </svg>

      {unreadCount > 0 && (
        <span
          className={`badge-count ${isAnimating ? 'animate' : ''}`}
          data-count={unreadCount > 99 ? '99+' : unreadCount}
        >
          {unreadCount > 99 ? '99+' : unreadCount}
        </span>
      )}
    </button>
  );
};

export default NotificationBadge;
